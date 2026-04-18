package secretscan_test

import (
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/secretscan"
)

// TestNewBetterleaksScanner_ConstructorSucceeds verifies that
// NewBetterleaksScanner returns a non-nil scanner and no error. A failure here
// means the embedded betterleaks.toml could not be parsed — that would cause
// GatePreCommit to silently fall back to the hand-rolled rules, so this test
// acts as a build-time sanity check.
func TestNewBetterleaksScanner_ConstructorSucceeds(t *testing.T) {
	s, err := secretscan.NewBetterleaksScanner()
	if err != nil {
		t.Fatalf("NewBetterleaksScanner() returned error: %v", err)
	}
	if s == nil {
		t.Fatal("NewBetterleaksScanner() returned nil scanner")
	}
}

// TestBetterleaksScanner_DetectsAWSAccessKey feeds a paired AWS access key
// ID + secret access key (betterleaks uses a composite rule requiring both
// within 5 lines) and asserts at least one finding is returned.
// The test does NOT assert a specific rule ID — betterleaks' rule IDs come
// from its embedded TOML and may differ from our hand-rolled IDs across
// versions.
func TestBetterleaksScanner_DetectsAWSAccessKey(t *testing.T) {
	s, err := secretscan.NewBetterleaksScanner()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Betterleaks' aws-access-token rule is a composite rule: it requires both
	// the access key ID AND the secret access key to be present within 5 lines.
	// The AKIA prefix must not match the global stopword regex (.+EXAMPLE$).
	input := []byte(
		"aws_access_key_id = AKIA2XCPQLWDTMRR7ZKE\n" +
			"aws_secret_access_key = Tq8vR3nZ7xK2pF5mJ0wL6hY1bG4dC9sA+eQ3rN7v\n",
	)
	findings := s.Scan("config.ini", input)

	if len(findings) == 0 {
		t.Fatal("expected at least one finding for AWS access key + secret pair, got none")
	}
}

// TestBetterleaksScanner_DetectsGitHubPAT feeds a GitHub classic PAT and
// asserts at least one finding.
func TestBetterleaksScanner_DetectsGitHubPAT(t *testing.T) {
	s, err := secretscan.NewBetterleaksScanner()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// ghp_ + exactly 36 alphanumeric characters, entropy >= 3.0.
	input := []byte("export GITHUB_TOKEN=ghp_T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmN")
	findings := s.Scan("deploy.sh", input)

	if len(findings) == 0 {
		t.Fatal("expected at least one finding for GitHub PAT, got none")
	}
}

// TestBetterleaksScanner_CleanContentProducesNoFindings verifies that content
// with no secrets returns zero findings. This exercises the happy path through
// GatePreCommit — a clean commit must not be blocked.
func TestBetterleaksScanner_CleanContentProducesNoFindings(t *testing.T) {
	s, err := secretscan.NewBetterleaksScanner()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Typical documentation content: no secrets, no patterns that look like keys.
	input := []byte(`# Architecture Notes

This document describes the system design. Configuration values are injected
via environment variables at runtime. No credentials are stored in source.

Example config snippet (values are placeholders):
  db_host = "localhost"
  db_port = 5432
  log_level = "info"
`)

	findings := s.Scan("docs/architecture.md", input)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for clean content, got %d:", len(findings))
		for _, f := range findings {
			t.Errorf("  unexpected finding: %s", f)
		}
	}
}

// TestBetterleaksScanner_FindingFieldsPopulated verifies that the translated
// Finding struct has all required fields populated correctly.
func TestBetterleaksScanner_FindingFieldsPopulated(t *testing.T) {
	s, err := secretscan.NewBetterleaksScanner()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	const filePath = "secrets/deploy.sh"
	// A GitHub classic PAT: ghp_ + 36 chars with high entropy.
	input := []byte("export GITHUB_TOKEN=ghp_T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmN")
	findings := s.Scan(filePath, input)

	if len(findings) == 0 {
		t.Fatal("no findings produced; cannot check field population")
	}

	f := findings[0]

	if f.RuleID == "" {
		t.Error("Finding.RuleID must not be empty")
	}
	if f.FilePath != filePath {
		t.Errorf("Finding.FilePath = %q, want %q", f.FilePath, filePath)
	}
	if f.Match == "" {
		t.Error("Finding.Match must not be empty (should be redacted preview of secret)")
	}
	// Verify that the match is in redacted form (not the raw secret).
	// Our Redact() contract: ≤8 chars → all stars; >8 → first-4 + "…" + 8 stars.
	// The GitHub token is >8 chars so it must contain "…".
	if !strings.Contains(f.Match, "…") && !strings.Contains(f.Match, "*") {
		t.Errorf("Finding.Match %q does not look redacted (expected '…' or '*')", f.Match)
	}
	// Verify Severity is a recognised value (not the zero value).
	switch f.Severity {
	case secretscan.SeverityLow, secretscan.SeverityMedium,
		secretscan.SeverityHigh, secretscan.SeverityCritical:
		// good
	default:
		t.Errorf("Finding.Severity = %v, want a valid Severity constant", f.Severity)
	}
}

// TestBetterleaksScanner_ScanReaderInterface verifies the ScanReader method.
func TestBetterleaksScanner_ScanReaderInterface(t *testing.T) {
	s, err := secretscan.NewBetterleaksScanner()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Stripe live key: sk_live_ + 24+ alphanum, no entropy gate on the specific rule.
	content := strings.NewReader("payment_key=sk_live_T8Kj9mNpQrSTuVwXyZ5aB2c")
	findings, err := s.ScanReader("app.env", content)
	if err != nil {
		t.Fatalf("ScanReader returned unexpected error: %v", err)
	}
	if len(findings) == 0 {
		t.Error("ScanReader: expected at least one finding for Stripe live key pattern, got none")
	}
}
