package secretscan

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// errFileTooLarge is a sentinel returned by readCappedFile when the file on
// disk exceeds the scan size cap.
var errFileTooLarge = errors.New("secretscan: file exceeds scan size cap")

// readCappedFile reads at most cap+1 bytes from path. If the file exceeds
// cap bytes, errFileTooLarge is returned. Any other I/O error is wrapped
// and returned.
func readCappedFile(path string, cap int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Read cap+1 so we can distinguish "exactly cap bytes" from "too big".
	b, err := io.ReadAll(io.LimitReader(f, cap+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > cap {
		return nil, errFileTooLarge
	}
	return b, nil
}

// maxScanFileBytes caps how much of a single file GatePreCommit will buffer
// into memory for scanning. Files exceeding this are treated as a blocking
// finding rather than being either silently skipped or OOM-loaded, since a
// multi-gigabyte file in a documentation commit is almost certainly either
// a mistake (a binary checked in) or an attack (an attempt to exhaust the
// agent host). 16 MiB is generous for real documentation — the largest
// markdown file in practice is <1 MiB — while keeping the gate responsive
// and memory-bounded.
const maxScanFileBytes = 16 * 1024 * 1024

// GatePreCommit is the API the Vedox Doc Agent calls before executing a
// git commit. It scans each file listed in paths and returns findings that
// are severe enough to block the commit.
//
// Blocking semantics (per FINAL_PLAN changelog items 1, 2):
//   - Findings with Severity >= SeverityHigh BLOCK the commit.
//   - Findings with SeverityMedium or SeverityLow are returned in the slice
//     but do NOT block the commit — the caller receives them as warnings.
//   - File paths that match the secret file path blocklist (e.g. *.pem,
//     *credentials.json, *.env) are blocked unconditionally regardless of
//     content, because even an "empty" credentials file should not be committed
//     to a documentation repo.
//
// The returned []Finding slice contains ALL findings (block + warn).
// The returned error is non-nil if and only if at least one Critical or High
// finding was detected, or a blocked path was encountered. Callers must treat
// a non-nil error as a hard commit block.
//
// The error message includes all blocking rule IDs and paths so the Doc Agent
// can surface them to the user in a single alert rather than aborting on first
// finding.
//
// Example caller pattern (WS-C adapter):
//
//	findings, err := secretscan.GatePreCommit(changedPaths)
//	if err != nil {
//	    // show findings to user; do NOT call git commit
//	    return err
//	}
//	// warnings (medium/low findings) are in findings; log them
//	callGitCommit(...)
func GatePreCommit(paths []string) ([]Finding, error) {
	s, err := NewBetterleaksScanner()
	if err != nil {
		// betterleaks config parse failure — fall back to hand-rolled rules so
		// the gate is never left open. This branch is unlikely in practice
		// (the config is embedded in the binary) but must be handled correctly.
		log.Printf("secretscan: betterleaks unavailable (%v); falling back to hand-rolled rules", err)
		s = New(DefaultRules())
	}
	return gatePreCommitWithScanner(s, paths)
}

// gatePreCommitWithScanner is the internal version that accepts an injected
// Scanner, used by tests to inject a scanner with reduced/custom rules.
func gatePreCommitWithScanner(s Scanner, paths []string) ([]Finding, error) {
	var allFindings []Finding
	var blockReasons []string

	for _, path := range paths {
		// Check the file path against the secret-file blocklist FIRST.
		// This mirrors the existing VDX-006 ErrSecretFileBlocked check in the
		// store adapters, extended to cover additional patterns relevant to the
		// documentation commit path.
		if blockedPath(path) {
			blockReasons = append(blockReasons, fmt.Sprintf("blocked path: %s (matches secret-file pattern)", filepath.Base(path)))
			// Add a synthetic Finding for the path block so the caller can
			// surface it in the same findings list as content-based findings.
			allFindings = append(allFindings, Finding{
				RuleID:     "BLOCKED-PATH",
				FilePath:   path,
				Line:       0,
				Column:     0,
				Match:      Redact(filepath.Base(path)),
				Severity:   SeverityCritical,
				Confidence: ConfidenceHigh,
			})
			continue
		}

		body, err := readCappedFile(path, maxScanFileBytes)
		if err != nil {
			if errors.Is(err, errFileTooLarge) {
				// Oversize file — block the commit rather than risk an OOM on
				// a malicious or accidental multi-GB input. Surfaced as a
				// synthetic finding so the caller sees it in the same list as
				// content-based findings.
				blockReasons = append(blockReasons, fmt.Sprintf("oversize file: %s (>%d bytes)", filepath.Base(path), maxScanFileBytes))
				allFindings = append(allFindings, Finding{
					RuleID:     "OVERSIZE-FILE",
					RuleName:   "File exceeds scan size limit",
					FilePath:   path,
					Line:       0,
					Column:     0,
					Match:      fmt.Sprintf("%d-byte cap", maxScanFileBytes),
					Severity:   SeverityHigh,
					Confidence: ConfidenceHigh,
				})
				continue
			}
			// Non-existent or unreadable files are not secret findings; return the
			// read error so the caller can decide how to handle missing files.
			return allFindings, fmt.Errorf("secretscan: read %s: %w", path, err)
		}

		findings := s.Scan(path, body)
		allFindings = append(allFindings, findings...)

		for _, f := range findings {
			if f.Severity >= SeverityHigh {
				blockReasons = append(blockReasons,
					fmt.Sprintf("rule %s (%s) at %s:%d", f.RuleID, f.Severity, path, f.Line))
			}
		}
	}

	if len(blockReasons) > 0 {
		return allFindings, fmt.Errorf(
			"secretscan: commit blocked — %d issue(s) detected:\n  %s\n\nReview the matched files, remove the secrets, and retry. "+
				"To override (audit-logged), pass --allow-secrets to the vedox agent command.",
			len(blockReasons),
			strings.Join(blockReasons, "\n  "),
		)
	}

	return allFindings, nil
}

// secretFilePatterns is the blocklist of file name patterns that are always
// rejected regardless of content. This extends the existing VDX-006 blocklist
// in local_adapter.go and importer.go.
//
// Patterns are matched against the base filename (not the full path) using
// filepath.Match semantics. They are lowercase for case-insensitive comparison.
var secretFilePatterns = []string{
	"*.env",
	".env",
	"*.pem",
	"*.key",
	"*.p12",
	"*.pfx",
	"*.pkcs12",
	"id_rsa",
	"id_rsa.pub",
	"id_ed25519",
	"id_ecdsa",
	"id_dsa",
	"*.openssh",
	"credentials",
	"credentials.json",
	"service-account*.json",
	"serviceaccount*.json",
	"*secret*.json",
	"*token*.json",
	"*.password",
	"*.secret",
	"*_secrets.yaml",
	"*_secrets.yml",
	"*.kdbx",
	"*.keystore",
}

// blockedPath returns true if the file's basename matches any entry in the
// secret file blocklist.
func blockedPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	for _, pattern := range secretFilePatterns {
		matched, err := filepath.Match(pattern, base)
		if err == nil && matched {
			return true
		}
	}
	return false
}
