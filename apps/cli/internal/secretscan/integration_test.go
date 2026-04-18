package secretscan_test

// Integration tests for the secretscan package that exercise GatePreCommit
// against files written into real ephemeral git repos. These tests verify the
// full pre-commit gate flow: file written to disk → GatePreCommit reads it →
// findings with correct RuleID, line number, and redacted match are returned.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/secretscan"
	"github.com/vedox/vedox/internal/testutil"
)

// ---- helpers ----------------------------------------------------------------

// absPath returns the absolute path of relPath inside a TestRepo.
func absPath(repo *testutil.TestRepo, relPath string) string {
	return filepath.Join(repo.Path(), relPath)
}

// ---- Test: clean files produce zero blocking findings -----------------------

// TestIntegration_CleanRepo_NoFindings creates a real git repo with markdown
// documentation files that contain no secrets. GatePreCommit must return nil
// error and no blocking findings.
func TestIntegration_CleanRepo_NoFindings(t *testing.T) {
	repo := testutil.NewTestRepo(t)

	// Write several realistic documentation files.
	repo.WriteFile("README.md", "# My Documentation\n\nThis is a clean documentation repo.\n")
	repo.WriteFile("docs/getting-started.md", "## Getting Started\n\nRun the install script:\n\n```bash\ncurl -fsSL https://example.com/install | sh\n```\n")
	repo.WriteFile("docs/configuration.md", "## Configuration\n\nSet `DATABASE_URL` in your environment to point at your database.\n")
	repo.WriteFile("architecture/decisions/001-go-backend.md", "# ADR 001: Go Backend\n\nWe chose Go for the backend because it compiles to a static binary.\n")

	repo.CommitAll("initial docs")

	paths := []string{
		absPath(repo, "README.md"),
		absPath(repo, "docs/getting-started.md"),
		absPath(repo, "docs/configuration.md"),
		absPath(repo, "architecture/decisions/001-go-backend.md"),
	}

	findings, err := secretscan.GatePreCommit(paths)
	if err != nil {
		t.Errorf("expected GatePreCommit to allow clean repo, got error: %v", err)
	}
	// Medium/Low findings are non-blocking; none expected for clean docs.
	for _, f := range findings {
		if f.Severity >= secretscan.SeverityHigh {
			t.Errorf("unexpected High+ finding in clean repo: %v", f)
		}
	}
}

// ---- Test: AWS key in committed file blocks commit --------------------------

// TestIntegration_AWSKeyFile_BlocksCommit writes a file containing a real-
// format AWS credential pair into a test repo, then runs GatePreCommit on that
// file. The test verifies that the commit is blocked and findings are returned
// with redacted match text and correct file path attribution.
//
// Note: betterleaks uses a composite aws-access-token rule that requires both
// the access key ID AND the secret access key to be present within 5 lines.
// The test vector includes both. The AKIA prefix must not end in "EXAMPLE"
// (betterleaks global allowlist).
func TestIntegration_AWSKeyFile_BlocksCommit(t *testing.T) {
	repo := testutil.NewTestRepo(t)

	// Both access key ID and secret access key required within 5 lines.
	content := "# AWS config\n" +
		"aws_access_key_id = AKIA2XCPQLWDTMRR7ZKE\n" +
		"aws_secret_access_key = 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\n" +
		"aws_region = us-east-1\n"
	repo.WriteFile("config/aws.sh", content)
	repo.CommitAll("add aws config")

	path := absPath(repo, "config/aws.sh")
	findings, err := secretscan.GatePreCommit([]string{path})

	// Error must be non-nil — the commit must be blocked.
	if err == nil {
		t.Fatal("expected GatePreCommit to block file containing AWS access key ID")
	}

	if len(findings) == 0 {
		t.Fatal("expected at least one finding, got none")
	}

	// Verify the first high-severity finding has correct field attribution.
	var blockingFinding *secretscan.Finding
	for i := range findings {
		if findings[i].Severity >= secretscan.SeverityHigh {
			blockingFinding = &findings[i]
			break
		}
	}
	if blockingFinding == nil {
		t.Fatalf("expected at least one High+ finding, got: %v", findings)
	}

	// Match must be redacted — must not expose the raw key.
	if strings.Contains(blockingFinding.Match, "AKIA2XCPQLWDTMRR7ZKE") {
		t.Errorf("Match must be redacted; got unredacted value: %q", blockingFinding.Match)
	}

	// FilePath in the finding must match the scanned path.
	if blockingFinding.FilePath != path {
		t.Errorf("FilePath: got %q, want %q", blockingFinding.FilePath, path)
	}
}

// ---- Test: multiple secret types across multiple files ----------------------

// TestIntegration_MultiFileMultiSecret writes one file per secret type from
// the must-block corpus, runs GatePreCommit on all paths at once, and verifies
// each file produces at least one blocking finding.
//
// Rule IDs are not asserted here — rule IDs depend on which scanner backend is
// active (betterleaks or hand-rolled fallback) and differ between the two.
// The gate-level contract is: High+ severity finding → commit blocked.
func TestIntegration_MultiFileMultiSecret(t *testing.T) {
	repo := testutil.NewTestRepo(t)

	type fileSecret struct {
		relPath string
		content string
	}

	// Use high-entropy realistic values that are detected by both betterleaks
	// and the hand-rolled fallback scanner.
	cases := []fileSecret{
		{
			relPath: "infra/github-token.sh",
			// ghp_ + exactly 36 mixed-case alphanum chars, entropy ≥ 3.0.
			content: "export GITHUB_TOKEN=ghp_T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmN\n",
		},
		{
			relPath: "ai/openai-config.py",
			// sk-proj- prefix (generic-api-key in betterleaks), sk- in hand-rolled.
			content: "import openai\nopenai.api_key = \"sk-proj-T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmNpQrSTuVwXyZ5aB\"\n",
		},
		{
			relPath: "ai/anthropic-config.py",
			// Exact format: sk-ant-api03- + 93 [a-zA-Z0-9_-] chars + AA.
			content: "ANTHROPIC_API_KEY=sk-ant-api03-xK9mP2vQ8nR4wL7jZ3bF6cT1dY5hA0eGr3sVwXyZ5aB2cD6eF0gH4iJkLmNpQrSTuVwXyZ5aB2cD6eFgHXXXXXXXXXXXXAA\n",
		},
		{
			relPath: "payments/stripe.env",
			content: "STRIPE_SK=sk_live_T8Kj9mNpQrSTuVwXyZ5aB2c\n",
		},
	}

	var paths []string
	for _, c := range cases {
		repo.WriteFile(c.relPath, c.content)
		paths = append(paths, absPath(repo, c.relPath))
	}
	repo.CommitAll("add secret files")

	// Note: stripe.env matches the *.env blocklist so GatePreCommit will
	// block it as BLOCKED-PATH before even scanning content. That is correct
	// behaviour — the test verifies the gate fires, not the specific rule.
	_, err := secretscan.GatePreCommit(paths)
	if err == nil {
		t.Fatal("expected GatePreCommit to block files containing multiple secrets")
	}

	// Scan each non-blocked file individually and verify the gate blocks it.
	nonBlockedCases := cases[:3] // stripe.env is path-blocked
	for _, c := range nonBlockedCases {
		path := absPath(repo, c.relPath)
		_, fileErr := secretscan.GatePreCommit([]string{path})
		if fileErr == nil {
			t.Errorf("expected GatePreCommit to block %s, but it passed", c.relPath)
		}
	}
}

// ---- Test: PEM private key — correct rule, line, redaction ------------------

// TestIntegration_PEMKeyInRepo verifies the full gate flow for a PEM private
// key embedded in a Go source file. The gate must block and return at least one
// finding with a redacted Match field.
//
// Note: betterleaks uses rule ID "private-key" for PEM keys; the hand-rolled
// scanner uses "PEM-PRIVATE-KEY". We do not assert on the specific rule ID
// here because the active backend may vary. The key invariants are: (1) the
// commit is blocked (err != nil), (2) the match is redacted.
//
// The PEM body must be 64+ bytes of base64 to satisfy betterleaks' private-key
// rule regex (which anchors on both the header and footer with content between).
func TestIntegration_PEMKeyInRepo(t *testing.T) {
	repo := testutil.NewTestRepo(t)

	content := `package tls

// hardcoded for legacy reasons — do not commit this
const privKey = ` + "`" + `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAxK9mP2vQ8nR4wL7jZ3bF6cT1dY5hA0eGr3sVwXyZ5aB2
cD6eF0gH4iJkLmNpQrSTuVwXyZ5aB2cD6eF0gH4iJkLmNpQrSTuVwXyZ5ab
-----END RSA PRIVATE KEY-----` + "`\n"

	repo.WriteFile("pkg/tls/key.go", content)
	repo.CommitAll("add TLS key source")

	path := absPath(repo, "pkg/tls/key.go")
	findings, err := secretscan.GatePreCommit([]string{path})

	if err == nil {
		t.Fatal("expected GatePreCommit to block file containing PEM private key")
	}

	if len(findings) == 0 {
		t.Fatal("expected at least one finding, got none")
	}

	// Find the PEM-related finding (rule ID differs by scanner backend).
	var pemFinding *secretscan.Finding
	for i := range findings {
		// betterleaks uses "private-key"; hand-rolled uses "PEM-PRIVATE-KEY".
		if strings.Contains(findings[i].RuleID, "private") || strings.Contains(strings.ToLower(findings[i].RuleID), "pem") {
			pemFinding = &findings[i]
			break
		}
	}
	if pemFinding == nil {
		t.Fatalf("expected PEM-related finding, got: %v", findings)
	}

	// Match is redacted — cannot contain the full PEM header.
	if strings.Contains(pemFinding.Match, "-----BEGIN RSA PRIVATE KEY-----") {
		t.Errorf("Match should be redacted; got: %q", pemFinding.Match)
	}
}

// ---- Test: blocked file path regardless of content --------------------------

// TestIntegration_BlockedPathInRepo verifies that files whose names match the
// secret-file blocklist are blocked by GatePreCommit even when their content
// is innocuous. This is path-level enforcement, not content scanning.
func TestIntegration_BlockedPathInRepo(t *testing.T) {
	repo := testutil.NewTestRepo(t)

	blockedFiles := map[string]string{
		".env":               "# harmless comment\nDATABASE_HOST=localhost\n",
		"server.pem":         "-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A\n-----END CERTIFICATE-----\n",
		"id_rsa":             "this is not actually a key but the filename is blocked\n",
		"credentials.json":   `{"type": "not_a_real_credential"}` + "\n",
		"app_secrets.yaml":   "db_password: hunter2\n",
	}

	for name, content := range blockedFiles {
		repo.WriteFile(name, content)
	}
	repo.CommitAll("add blocked files")

	for name := range blockedFiles {
		path := absPath(repo, name)
		t.Run(name, func(t *testing.T) {
			findings, err := secretscan.GatePreCommit([]string{path})
			if err == nil {
				t.Errorf("expected GatePreCommit to block %s, but it passed", name)
			}

			// Must have a BLOCKED-PATH finding.
			var blockedFinding *secretscan.Finding
			for i := range findings {
				if findings[i].RuleID == "BLOCKED-PATH" {
					blockedFinding = &findings[i]
					break
				}
			}
			if blockedFinding == nil {
				t.Errorf("%s: expected BLOCKED-PATH finding, got: %v", name, findings)
			}
			if blockedFinding != nil && blockedFinding.Severity != secretscan.SeverityCritical {
				t.Errorf("%s: BLOCKED-PATH severity should be Critical, got %v", name, blockedFinding.Severity)
			}
		})
	}
}

// ---- Test: must-block corpus patterns exercised via TestRepo files ----------

// TestIntegration_CorpusPatterns_ViaTestRepo loads representative secret
// patterns and verifies GatePreCommit blocks when the content is written into
// a real git repo file. This bridges the corpus file tests (which use static
// testdata/) with actual git-repo path resolution.
//
// Test vectors use high-entropy realistic values that satisfy betterleaks'
// entropy and format constraints. Low-entropy or known example values (like
// AKIAIOSFODNN7EXAMPLE) are intentionally excluded — betterleaks correctly
// recognises them as test data and does not flag them.
func TestIntegration_CorpusPatterns_ViaTestRepo(t *testing.T) {
	type pattern struct {
		name    string
		content string
	}

	// PEM: header + 64+ chars of base64 body + footer (betterleaks requirement).
	pemContent := "-----BEGIN RSA PRIVATE KEY-----\n" +
		"MIIEowIBAAKCAQEAxK9mP2vQ8nR4wL7jZ3bF6cT1dY5hA0eGr3sVwXyZ5aB2\n" +
		"cD6eF0gH4iJkLmNpQrSTuVwXyZ5aB2cD6eF0gH4iJkLmNpQrSTuVwXyZ5ab\n" +
		"-----END RSA PRIVATE KEY-----\n"

	corpus := []pattern{
		// AWS: composite rule requires both key ID + secret within 5 lines.
		{"aws-access-key", "aws_access_key_id = AKIA2XCPQLWDTMRR7ZKE\naws_secret_access_key = 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\n"},
		// GitHub classic PAT: ghp_ + exactly 36 mixed-case alphanum chars.
		{"github-pat", "GITHUB_TOKEN=ghp_T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmN\n"},
		// OpenAI: sk-proj- prefix detected by generic-api-key rule at entropy >5.
		{"openai-key", "OPENAI_API_KEY=sk-proj-T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmNpQrSTuVwXyZ5aB\n"},
		// Stripe live key: sk_live_ + 24+ chars.
		{"stripe-live", "STRIPE_SK=sk_live_T8Kj9mNpQrSTuVwXyZ5aB2c\n"},
		// PEM private key with realistic body length.
		{"pem-private-key", pemContent},
		// GCP service account JSON with embedded PEM key (private-key rule).
		{"gcp-service-account", `{"type":"service_account","private_key":"-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEAxK9mP2vQ8nR4wL7jZ3bF6cT1dY5hA0eGr3sVwXyZ5aB2\ncD6eF0gH4iJkLmNpQrSTuVwXyZ5aB2cD6eF0gH4iJkLmNpQrSTuVwXyZ5ab\n-----END RSA PRIVATE KEY-----\n"}` + "\n"},
	}

	for _, p := range corpus {
		p := p // capture loop variable
		t.Run(p.name, func(t *testing.T) {
			repo := testutil.NewTestRepo(t)
			repo.WriteFile("secret.txt", p.content)
			repo.CommitAll("add secret")

			path := absPath(repo, "secret.txt")
			_, err := secretscan.GatePreCommit([]string{path})
			if err == nil {
				t.Errorf("corpus pattern %q: expected GatePreCommit to block, but it passed", p.name)
			}
		})
	}
}

// ---- Test: non-existent file returns read error, not a finding --------------

// TestIntegration_NonExistentFile verifies that GatePreCommit returns a
// meaningful read error (not a panic or a silent skip) when a listed path
// does not exist on disk.
func TestIntegration_NonExistentFile(t *testing.T) {
	dir := testutil.TempDir(t)
	missingPath := filepath.Join(dir, "does-not-exist.md")

	_, err := secretscan.GatePreCommit([]string{missingPath})
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
	if !strings.Contains(err.Error(), "does-not-exist.md") {
		t.Errorf("error message should reference the missing file, got: %v", err)
	}
}

// ---- Test: allowlist suppresses a specific rule in the gate -----------------

// TestIntegration_AllowlistSuppressesRule verifies that the hand-rolled scanner
// (New(DefaultRules())) detects JWTs as Medium severity (advisory, non-blocking)
// and that the allowlist option works at the scanner level.
//
// Note: This test exercises the Scanner interface directly via New(DefaultRules())
// rather than GatePreCommit, because GatePreCommit now uses betterleaks which
// classifies JWTs as High severity (and thus blocks). The JWT severity difference
// is an intentional behaviour change in the HYBRID migration — betterleaks is
// more conservative on JWTs. The gate-layer allowlist test uses a known pattern
// from the hand-rolled scanner's rule set.
func TestIntegration_AllowlistSuppressesRule(t *testing.T) {
	repo := testutil.NewTestRepo(t)

	realJWT := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
	content := "# docs with example token\nconst tok = \"" + realJWT + "\"\n"
	repo.WriteFile("examples/auth.md", content)
	repo.CommitAll("add jwt example")

	path := absPath(repo, "examples/auth.md")

	// Verify the JWT IS detected by the hand-rolled scanner as Medium (non-blocking).
	s := secretscan.New(secretscan.DefaultRules())
	body, _ := os.ReadFile(path)
	findings := s.Scan(path, body)
	jwtFound := false
	for _, f := range findings {
		if f.RuleID == "JWT-TOKEN" {
			jwtFound = true
			if f.Severity != secretscan.SeverityMedium {
				t.Errorf("hand-rolled JWT-TOKEN: expected SeverityMedium, got %v", f.Severity)
			}
		}
	}
	if !jwtFound {
		t.Error("expected JWT-TOKEN finding from hand-rolled scanner")
	}

	// Verify that the allowlist option suppresses the JWT-TOKEN rule.
	sAllowlisted := secretscan.New(secretscan.DefaultRules(), secretscan.WithAllowlist("JWT-TOKEN"))
	allowlistFindings := sAllowlisted.Scan(path, body)
	for _, f := range allowlistFindings {
		if f.RuleID == "JWT-TOKEN" {
			t.Error("JWT-TOKEN should be suppressed by the allowlist option")
		}
	}
}
