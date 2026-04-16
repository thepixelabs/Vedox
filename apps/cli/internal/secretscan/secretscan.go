// Package secretscan implements the deterministic Layer 1 secret detection
// scanner for the Vedox Doc Agent pre-commit gate (T-09 threat model).
//
// Architecture overview:
//
//	Scanner.Scan(path, body) → []Finding
//
// Each Finding carries the rule ID, file path, line number, a redacted match
// preview, the severity classification, and a confidence level. The scanner
// is purely deterministic — no LLM calls, no network, no disk I/O beyond the
// bytes passed in.
//
// Severity levels drive the commit gate:
//   - Critical  → BLOCK the commit unconditionally.
//   - High      → BLOCK the commit. Requires explicit override to skip.
//   - Medium    → WARN the user. Does not block by default (configurable).
//   - Low       → LOG only. Informational; never blocks.
//
// Confidence levels annotate how sure we are a match is a real secret:
//   - High    → pattern is highly specific (format-anchored, e.g. AKIA prefix).
//   - Medium  → pattern is plausible but has a non-trivial false-positive rate.
//   - Low     → heuristic match (high-entropy strings); informational only.
//
// Integration contract for WS-C adapters:
//
//	findings, err := scanner.Scan(filePath, fileBytes)
//	if err != nil { return err }
//	for _, f := range findings {
//	    if f.Severity >= SeverityHigh {
//	        return fmt.Errorf("secret detected: rule %s in %s:%d", f.RuleID, f.FilePath, f.Line)
//	    }
//	}
//
// Call GatePreCommit for the full pre-commit gate API.
package secretscan

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
)

// Severity classifies how dangerous a detected secret is.
// Higher numeric values are more severe.
type Severity int

const (
	// SeverityLow is informational only — does not trigger any gate.
	SeverityLow Severity = iota + 1
	// SeverityMedium warns the user but does not block the commit by default.
	SeverityMedium
	// SeverityHigh blocks the commit. Explicit --allow-secrets flag required to skip.
	SeverityHigh
	// SeverityCritical blocks unconditionally; no override flag accepted.
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Confidence captures how likely a match is a real secret rather than a
// false positive (e.g. a SHA256 hash or test data).
type Confidence int

const (
	// ConfidenceLow is used for heuristic matches (high-entropy strings).
	// These are informational and never block commits.
	ConfidenceLow Confidence = iota + 1
	// ConfidenceMedium is used for patterns that are plausible but can match
	// non-secret data (e.g. JWT tokens in documentation examples).
	ConfidenceMedium
	// ConfidenceHigh is used for format-anchored patterns with distinctive
	// prefixes or structure that rarely occur in non-secret content.
	ConfidenceHigh
)

func (c Confidence) String() string {
	switch c {
	case ConfidenceLow:
		return "low"
	case ConfidenceMedium:
		return "medium"
	case ConfidenceHigh:
		return "high"
	default:
		return "unknown"
	}
}

// Rule defines a single detection pattern. Each rule is sourced from the
// T-09 threat model table in security-and-secrets.md.
type Rule struct {
	// ID is the unique identifier for this rule (e.g. "AWS-ACCESS-KEY-ID").
	// Used in Finding.RuleID and audit log entries.
	ID string

	// Name is a short human-readable name shown in UI alerts.
	Name string

	// Description explains what this rule detects and why it matters.
	Description string

	// Pattern is the compiled regular expression that triggers this rule.
	// Patterns are applied per-line in ScanBytes/ScanReader.
	Pattern *regexp.Regexp

	// Severity controls whether a match blocks the commit (High/Critical)
	// or is advisory (Medium/Low).
	Severity Severity

	// Confidence reflects the expected false-positive rate for this pattern.
	Confidence Confidence
}

// Finding represents a single secret match returned by the scanner.
type Finding struct {
	// RuleID is the Rule.ID that triggered this finding.
	RuleID string

	// FilePath is the file path passed to Scan (not resolved by the scanner —
	// the caller is responsible for path safety).
	FilePath string

	// Line is the 1-indexed line number in the file where the match occurred.
	Line int

	// Column is the 1-indexed byte offset within the line where the match starts.
	Column int

	// Match is a redacted preview of the matched text (first 4 chars + "…").
	// The full secret is never stored in a Finding.
	Match string

	// Severity is copied from the triggering Rule.
	Severity Severity

	// Confidence is copied from the triggering Rule.
	Confidence Confidence
}

// String returns a human-readable summary of the finding suitable for
// terminal output.
func (f Finding) String() string {
	return fmt.Sprintf("[%s] %s (%s/%s) at %s:%d:%d — match: %s",
		f.RuleID, ruleNameForID(f.RuleID), f.Severity, f.Confidence,
		f.FilePath, f.Line, f.Column, f.Match)
}

// Scanner is the public interface for secret detection.
// Create a new Scanner via New.
type Scanner interface {
	// Scan inspects the given file content and returns all findings.
	// path is used only for attribution in Finding.FilePath — the scanner
	// does not open any files.
	//
	// Scan is safe for concurrent use from multiple goroutines.
	Scan(path string, body []byte) []Finding

	// ScanReader is a convenience wrapper that reads from r before scanning.
	// It buffers the full content in memory, so use Scan directly for large
	// files where you already hold the bytes.
	ScanReader(path string, r io.Reader) ([]Finding, error)
}

// Option is a functional option for configuring a Scanner at construction time.
type Option func(*scanner)

// WithAllowlist returns an Option that suppresses findings whose RuleID is in
// the provided set. Useful for per-repo policy overrides (Week 5 roadmap item).
func WithAllowlist(ruleIDs ...string) Option {
	return func(s *scanner) {
		for _, id := range ruleIDs {
			s.allowlist[id] = true
		}
	}
}

// WithSeverityFilter returns an Option that suppresses findings with severity
// strictly below minSeverity. For example, passing SeverityHigh causes the
// scanner to return only High and Critical findings.
func WithSeverityFilter(minSeverity Severity) Option {
	return func(s *scanner) {
		s.minSeverity = minSeverity
	}
}

// scanner is the concrete Scanner implementation.
type scanner struct {
	rules       []Rule
	allowlist   map[string]bool
	minSeverity Severity
}

// ruleNameIndex maps rule ID → rule name for String() formatting without
// scanning the rules slice on every call. Populated in New.
var ruleNameIndex = map[string]string{}

func ruleNameForID(id string) string {
	if name, ok := ruleNameIndex[id]; ok {
		return name
	}
	return id
}

// New creates a Scanner with the provided rules and applies any Options.
// Passing nil or an empty rules slice is allowed (the scanner will return no
// findings); callers should typically pass DefaultRules().
//
// Rules are copied internally; mutating the input slice after New returns has
// no effect on the scanner.
func New(rules []Rule, opts ...Option) Scanner {
	s := &scanner{
		rules:       make([]Rule, len(rules)),
		allowlist:   make(map[string]bool),
		minSeverity: SeverityLow,
	}
	copy(s.rules, rules)
	for _, opt := range opts {
		opt(s)
	}
	// Build rule name index.
	for _, r := range s.rules {
		ruleNameIndex[r.ID] = r.Name
	}
	return s
}

// Scan implements Scanner.
func (s *scanner) Scan(path string, body []byte) []Finding {
	var findings []Finding
	sc := bufio.NewScanner(bytes.NewReader(body))
	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := sc.Text()
		for _, rule := range s.rules {
			if s.allowlist[rule.ID] {
				continue
			}
			if rule.Severity < s.minSeverity {
				continue
			}
			loc := rule.Pattern.FindStringIndex(line)
			if loc == nil {
				continue
			}
			match := line[loc[0]:loc[1]]
			findings = append(findings, Finding{
				RuleID:     rule.ID,
				FilePath:   path,
				Line:       lineNum,
				Column:     loc[0] + 1, // 1-indexed
				Match:      Redact(match),
				Severity:   rule.Severity,
				Confidence: rule.Confidence,
			})
		}
	}
	return findings
}

// ScanReader implements Scanner.
func (s *scanner) ScanReader(path string, r io.Reader) ([]Finding, error) {
	body, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("secretscan: read %s: %w", path, err)
	}
	return s.Scan(path, body), nil
}
