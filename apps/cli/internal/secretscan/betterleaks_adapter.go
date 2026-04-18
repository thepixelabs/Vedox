package secretscan

// betterleaks_adapter.go wires the betterleaks detect package into our Scanner
// interface. The adapter keeps our gate API and Finding types unchanged; only
// the rule execution engine is replaced.
//
// Architecture:
//
//	GatePreCommit → gatePreCommitWithScanner(betterleaksScanner{…}, paths)
//	                       └── betterleaksScanner.Scan(path, body)
//	                               └── detector.DetectString(string(body))
//	                                       └── translate report.Finding → Finding
//
// The hand-rolled scanner (New(DefaultRules())) is kept as the fallback path
// and remains the primary baseline for unit tests. See commit_gate.go.

import (
	"fmt"
	"io"
	"strings"
	"sync"

	bldetect "github.com/betterleaks/betterleaks/detect"
)

// betterleaksScanner implements Scanner using the betterleaks detect package.
// It wraps a *bldetect.Detector configured with betterleaks' embedded default
// rule set (~262 rules as of v1.1.2, vs our 15 hand-rolled rules).
//
// H-01 mitigation: betterleaks v1.1.2's DetectString is read-only on the
// Detector today, but the Detector has unguarded fields (findings slice,
// findingsCh channel) that AddFinding would touch. A future version could
// route content matches through AddFinding and break our concurrency contract.
// We serialize Scan() with scanMu as a conservative guard — Scan is part of
// a synchronous commit gate anyway, so the serialization cost is negligible.
type betterleaksScanner struct {
	detector *bldetect.Detector
	scanMu   sync.Mutex
}

// NewBetterleaksScanner creates a Scanner backed by betterleaks' default rule
// set. It calls detect.NewDetectorDefaultConfig() which parses the embedded
// betterleaks.toml. Returns an error if the TOML config cannot be parsed —
// callers should fall back to New(DefaultRules()) in that case.
func NewBetterleaksScanner() (Scanner, error) {
	d, err := bldetect.NewDetectorDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("secretscan: betterleaks init: %w", err)
	}
	return &betterleaksScanner{detector: d}, nil
}

// Scan implements Scanner. It converts body to a string and runs it through
// betterleaks' detection engine, then translates each report.Finding into our
// Finding type. path is used only for attribution; the scanner does not open
// any files.
//
// Scan is safe for concurrent use from multiple goroutines via scanMu.
// See the struct doc comment for the rationale (H-01 mitigation).
func (s *betterleaksScanner) Scan(path string, body []byte) []Finding {
	s.scanMu.Lock()
	blFindings := s.detector.DetectString(string(body))
	s.scanMu.Unlock()
	out := make([]Finding, 0, len(blFindings))
	for _, f := range blFindings {
		out = append(out, translateFinding(path, f.RuleID, f.Description, f.StartLine, f.StartColumn, f.Secret, f.Entropy, f.Tags))
	}
	return out
}

// ScanReader implements Scanner. It buffers r fully before scanning.
func (s *betterleaksScanner) ScanReader(path string, r io.Reader) ([]Finding, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("secretscan: read %s: %w", path, err)
	}
	return s.Scan(path, body), nil
}

// translateFinding maps a betterleaks finding into our Finding type.
// We accept individual fields rather than the whole report.Finding to avoid
// importing the betterleaks report package — keeping the import surface minimal.
//
// H-02 mitigation: we intentionally do NOT accept betterleaks' Match field.
// Match is the full regex match (potentially wider than the secret itself,
// e.g. `key="sk_live_..."`), and a future upstream version could widen it
// further. Using Secret only — the narrow extracted capture — ensures
// Redact() receives the tightest possible input. In betterleaks v1.1.2 every
// content finding populates Secret, so no fallback is needed. If a future
// version ever emits an empty Secret, we'd rather log the finding with a
// blank redaction than risk storing a wider, less-redacted match.
//
// Field mapping:
//   - ruleID      → Finding.RuleID   (betterleaks rule identifier)
//   - description → Finding.RuleName (human-readable name)
//   - startLine   → Finding.Line     (1-indexed, same convention)
//   - startColumn → Finding.Column   (1-indexed, same convention)
//   - secret      → redacted preview (narrow capture — never the wider Match)
//   - entropy     → mapConfidence
//   - tags        → mapSeverityFromTags
func translateFinding(path, ruleID, description string, startLine, startColumn int, secret string, entropy float32, tags []string) Finding {
	return Finding{
		RuleID:     ruleID,
		RuleName:   description,
		FilePath:   path,
		Line:       startLine,
		Column:     startColumn,
		Match:      Redact(secret),
		Severity:   mapSeverityFromTags(tags),
		Confidence: mapConfidence(float64(entropy)),
	}
}

// mapSeverityFromTags extracts a Severity level from betterleaks rule tags.
// betterleaks' default rule set (v1.1.2) does not embed severity in its
// betterleaks.toml — rules have no tags. For those rules we default to
// SeverityHigh, which is the conservative choice for a commit gate: block
// unless the user explicitly overrides. If a future betterleaks version adds
// severity tags, this function will honour them.
//
// Expected tag values (if present): "critical", "high", "medium", "low".
func mapSeverityFromTags(tags []string) Severity {
	for _, tag := range tags {
		switch strings.ToLower(tag) {
		case "critical":
			return SeverityCritical
		case "high":
			return SeverityHigh
		case "medium":
			return SeverityMedium
		case "low":
			return SeverityLow
		}
	}
	// No severity tag — default to High (conservative: block the commit).
	return SeverityHigh
}

// mapConfidence converts a betterleaks Shannon entropy value to our Confidence
// enum. Betterleaks uses entropy as an additional filter (rules specify a
// minimum entropy threshold). We reuse the raw entropy score to estimate how
// specific the match is:
//
//	entropy >= 4.0 → ConfidenceHigh   (dense, non-natural-language string)
//	entropy >= 3.0 → ConfidenceMedium (plausible secret but some FP risk)
//	entropy <  3.0 → ConfidenceLow    (lower specificity; heuristic)
//
// An entropy of 0.0 means no entropy check was applied by the rule (common);
// we treat that as ConfidenceHigh because the pattern itself was sufficiently
// specific to match without an entropy gate.
func mapConfidence(entropy float64) Confidence {
	if entropy == 0.0 {
		// No entropy threshold in the rule — the regex is format-anchored.
		return ConfidenceHigh
	}
	if entropy >= 4.0 {
		return ConfidenceHigh
	}
	if entropy >= 3.0 {
		return ConfidenceMedium
	}
	return ConfidenceLow
}
