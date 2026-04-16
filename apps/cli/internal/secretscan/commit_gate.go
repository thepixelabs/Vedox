package secretscan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
	return gatePreCommitWithScanner(New(DefaultRules()), paths)
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

		body, err := os.ReadFile(path)
		if err != nil {
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
