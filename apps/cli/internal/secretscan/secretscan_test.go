package secretscan_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/secretscan"
)

// ── Redact ────────────────────────────────────────────────────────────────────

func TestRedact(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"short-3", "abc", "***"},
		{"exactly-8", "abcdefgh", "********"},
		{"aws-key", "AKIAIOSFODNN7EXAMPLE", "AKIA…********"},
		{"ghp-token", "ghp_abcdefghijklmnopqrstuvwxyz123456", "ghp_…********"},
		{"stripe-live", "sk_live_4eC39HqLyjWDarjtT1zdp7dc", "sk_l…********"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := secretscan.Redact(c.input)
			if got != c.want {
				t.Errorf("Redact(%q) = %q, want %q", c.input, got, c.want)
			}
		})
	}
}

// ── Per-rule table tests ──────────────────────────────────────────────────────

// ruleCase defines a single test vector for one rule.
type ruleCase struct {
	name        string
	line        string
	wantMatch   bool   // true = must produce a finding for targetRuleID
	targetRuleID string
}

// runRuleCases runs all test cases through a scanner that has all DefaultRules
// loaded, then asserts match/no-match for the target rule ID.
func runRuleCases(t *testing.T, cases []ruleCase) {
	t.Helper()
	s := secretscan.New(secretscan.DefaultRules())
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			findings := s.Scan("test.txt", []byte(c.line))
			found := false
			for _, f := range findings {
				if f.RuleID == c.targetRuleID {
					found = true
					break
				}
			}
			if c.wantMatch && !found {
				t.Errorf("expected finding for rule %s on line %q, got none", c.targetRuleID, c.line)
			}
			if !c.wantMatch && found {
				t.Errorf("expected NO finding for rule %s on line %q, but got one", c.targetRuleID, c.line)
			}
		})
	}
}

// ── Rule 1: AWS Access Key ID ─────────────────────────────────────────────────

func TestRule_AWSAccessKeyID(t *testing.T) {
	runRuleCases(t, []ruleCase{
		{
			name:        "positive-AKIA",
			line:        `aws_access_key_id = AKIAIOSFODNN7EXAMPLE`,
			wantMatch:   true,
			targetRuleID: "AWS-ACCESS-KEY-ID",
		},
		{
			name:        "positive-ASIA-sts",
			line:        `export AWS_ACCESS_KEY_ID=ASIAIOSFODNN7EXAMPLE`,
			wantMatch:   true,
			targetRuleID: "AWS-ACCESS-KEY-ID",
		},
		{
			name:        "near-miss-too-short",
			// AKIA + only 15 chars — pattern requires 16 after prefix
			line:        `aws_key = AKIAIOSFODNN7EXAMPL`,
			wantMatch:   false,
			targetRuleID: "AWS-ACCESS-KEY-ID",
		},
		{
			name:        "must-allow-no-prefix",
			// Random 20-char uppercase string without AKIA/ASIA prefix
			line:        `export SOME_ID=ZZZAIOSFODNN7EXAMPLE`,
			wantMatch:   false,
			targetRuleID: "AWS-ACCESS-KEY-ID",
		},
	})
}

// ── Rule 2: AWS Secret Access Key ────────────────────────────────────────────

func TestRule_AWSSecretAccessKey(t *testing.T) {
	runRuleCases(t, []ruleCase{
		{
			name:        "positive-ini-style",
			line:        `aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY`,
			wantMatch:   true,
			targetRuleID: "AWS-SECRET-ACCESS-KEY",
		},
		{
			name:        "positive-json-SecretAccessKey",
			line:        `"SecretAccessKey": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"`,
			wantMatch:   true,
			targetRuleID: "AWS-SECRET-ACCESS-KEY",
		},
		{
			name:        "near-miss-short-value",
			// value is only 20 chars — pattern requires exactly 40
			line:        `aws_secret_access_key = wJalrXUtnFEMI/K7MD`,
			wantMatch:   false,
			targetRuleID: "AWS-SECRET-ACCESS-KEY",
		},
		{
			name:        "must-allow-generic-variable",
			// "secret" in the var name but value is not 40 chars
			line:        `my_secret_value = short`,
			wantMatch:   false,
			targetRuleID: "AWS-SECRET-ACCESS-KEY",
		},
	})
}

// ── Rule 3: GitHub Classic PAT ────────────────────────────────────────────────

func TestRule_GitHubPATClassic(t *testing.T) {
	runRuleCases(t, []ruleCase{
		{
			name:        "positive-ghp-in-shell",
			line:        `GITHUB_TOKEN=ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ1234567890ABCD`,
			wantMatch:   true,
			targetRuleID: "GITHUB-PAT-CLASSIC",
		},
		{
			name:        "positive-ghp-in-yaml",
			line:        `token: ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ1234567890ABCD`,
			wantMatch:   true,
			targetRuleID: "GITHUB-PAT-CLASSIC",
		},
		{
			name:        "near-miss-too-short",
			// only 35 chars after ghp_ instead of 36 — no match
			line:        "token: ghp_" + strings.Repeat("a", 35),
			wantMatch:   false,
			targetRuleID: "GITHUB-PAT-CLASSIC",
		},
		{
			name:        "must-allow-wrong-prefix",
			line:        `token: pat_aBcDeFgHiJkLmNoPqRsTuVwXyZ123456`,
			wantMatch:   false,
			targetRuleID: "GITHUB-PAT-CLASSIC",
		},
	})
}

// ── Rule 4: GitHub Fine-Grained PAT ──────────────────────────────────────────

func TestRule_GitHubPATFineGrained(t *testing.T) {
	// 82-char alphanumeric+underscore suffix after github_pat_
	suffix82 := strings.Repeat("a", 82)
	runRuleCases(t, []ruleCase{
		{
			name:        "positive-exact-82",
			line:        "token: github_pat_" + suffix82,
			wantMatch:   true,
			targetRuleID: "GITHUB-PAT-FINE-GRAINED",
		},
		{
			name:        "near-miss-81-chars",
			line:        "token: github_pat_" + strings.Repeat("a", 81),
			wantMatch:   false,
			targetRuleID: "GITHUB-PAT-FINE-GRAINED",
		},
		{
			name:        "must-allow-wrong-prefix",
			line:        "token: gitlab_pat_" + suffix82,
			wantMatch:   false,
			targetRuleID: "GITHUB-PAT-FINE-GRAINED",
		},
	})
}

// ── Rule 5: GitHub OAuth / Server / User Token ───────────────────────────────

func TestRule_GitHubOAuthToken(t *testing.T) {
	runRuleCases(t, []ruleCase{
		{
			name:        "positive-gho",
			line:        "auth: gho_" + strings.Repeat("a", 36),
			wantMatch:   true,
			targetRuleID: "GITHUB-OAUTH-TOKEN",
		},
		{
			name:        "positive-ghs",
			line:        "auth: ghs_" + strings.Repeat("b", 36),
			wantMatch:   true,
			targetRuleID: "GITHUB-OAUTH-TOKEN",
		},
		{
			name:        "positive-ghu",
			line:        "auth: ghu_" + strings.Repeat("c", 36),
			wantMatch:   true,
			targetRuleID: "GITHUB-OAUTH-TOKEN",
		},
		{
			name:        "near-miss-too-short",
			line:        "auth: gho_" + strings.Repeat("a", 35),
			wantMatch:   false,
			targetRuleID: "GITHUB-OAUTH-TOKEN",
		},
		{
			name:        "must-allow-ghp-handled-by-different-rule",
			// ghp_ is handled by GITHUB-PAT-CLASSIC, not this rule
			line:        `auth: ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ1234567890ABCD`,
			wantMatch:   false,
			targetRuleID: "GITHUB-OAUTH-TOKEN",
		},
	})
}

// ── Rule 6: OpenAI API Key ────────────────────────────────────────────────────

func TestRule_OpenAIAPIKey(t *testing.T) {
	key48 := "sk-" + strings.Repeat("a", 48)
	runRuleCases(t, []ruleCase{
		{
			name:        "positive-exact-format",
			line:        "OPENAI_API_KEY=" + key48,
			wantMatch:   true,
			targetRuleID: "OPENAI-API-KEY",
		},
		{
			name:        "positive-in-python",
			line:        `openai.api_key = "` + key48 + `"`,
			wantMatch:   true,
			targetRuleID: "OPENAI-API-KEY",
		},
		{
			name:        "near-miss-too-short",
			line:        "OPENAI_API_KEY=sk-" + strings.Repeat("a", 47),
			wantMatch:   false,
			targetRuleID: "OPENAI-API-KEY",
		},
		{
			name:        "must-allow-no-sk-prefix",
			line:        "OPENAI_API_KEY=" + strings.Repeat("a", 51),
			wantMatch:   false,
			targetRuleID: "OPENAI-API-KEY",
		},
	})
}

// ── Rule 7: Anthropic API Key ─────────────────────────────────────────────────

func TestRule_AnthropicAPIKey(t *testing.T) {
	runRuleCases(t, []ruleCase{
		{
			name:        "positive-typical-format",
			line:        `ANTHROPIC_API_KEY=sk-ant-api03-` + strings.Repeat("z", 80),
			wantMatch:   true,
			targetRuleID: "ANTHROPIC-API-KEY",
		},
		{
			name:        "near-miss-too-short",
			// Only 20 chars after sk-ant- — pattern requires 32+
			line:        `ANTHROPIC_API_KEY=sk-ant-` + strings.Repeat("z", 20),
			wantMatch:   false,
			targetRuleID: "ANTHROPIC-API-KEY",
		},
		{
			name:        "must-allow-wrong-prefix",
			line:        `SOME_KEY=sk-xyz-` + strings.Repeat("z", 40),
			wantMatch:   false,
			targetRuleID: "ANTHROPIC-API-KEY",
		},
	})
}

// ── Rule 8: Stripe Live Key ───────────────────────────────────────────────────

func TestRule_StripeLiveKey(t *testing.T) {
	runRuleCases(t, []ruleCase{
		{
			name:        "positive-in-env",
			line:        `STRIPE_SECRET_KEY=sk_live_4eC39HqLyjWDarjtT1zdp7dc`,
			wantMatch:   true,
			targetRuleID: "STRIPE-LIVE-KEY",
		},
		{
			name:        "positive-in-config",
			line:        `stripe_secret: sk_live_AbCdEfGhIjKlMnOpQrStUvWx`,
			wantMatch:   true,
			targetRuleID: "STRIPE-LIVE-KEY",
		},
		{
			name:        "near-miss-too-short",
			// Only 20 chars after sk_live_ — pattern requires 24+
			line:        `STRIPE_SECRET_KEY=sk_live_4eC39HqLyjWDarjt`,
			wantMatch:   false,
			targetRuleID: "STRIPE-LIVE-KEY",
		},
		{
			name:        "must-allow-test-key",
			// test key — handled by STRIPE-TEST-KEY rule, not this one
			line:        `STRIPE_SECRET_KEY=sk_test_4eC39HqLyjWDarjtT1zdp7dc`,
			wantMatch:   false,
			targetRuleID: "STRIPE-LIVE-KEY",
		},
	})
}

// ── Rule 9: Stripe Test Key ───────────────────────────────────────────────────

func TestRule_StripeTestKey(t *testing.T) {
	runRuleCases(t, []ruleCase{
		{
			name:        "positive-test-key",
			line:        `STRIPE_SECRET_KEY=sk_test_4eC39HqLyjWDarjtT1zdp7dc`,
			wantMatch:   true,
			targetRuleID: "STRIPE-TEST-KEY",
		},
		{
			name:        "near-miss-too-short",
			line:        `STRIPE_SECRET_KEY=sk_test_4eC39HqLyjWDarjt`,
			wantMatch:   false,
			targetRuleID: "STRIPE-TEST-KEY",
		},
		{
			name:        "must-allow-live-key-goes-to-different-rule",
			line:        `STRIPE_SECRET_KEY=sk_live_4eC39HqLyjWDarjtT1zdp7dc`,
			wantMatch:   false,
			targetRuleID: "STRIPE-TEST-KEY",
		},
	})
}

// ── Rule 10: Slack Token ──────────────────────────────────────────────────────

func TestRule_SlackToken(t *testing.T) {
	runRuleCases(t, []ruleCase{
		{
			name:        "positive-xoxb-bot-token",
			line:        `SLACK_BOT_TOKEN=xoxb-1234567890-1234567890-abcdefghijklmnopqrstuvwx`,
			wantMatch:   true,
			targetRuleID: "SLACK-TOKEN",
		},
		{
			name:        "positive-xoxp-user-token",
			line:        `token: xoxp-1234567890-1234567890-abcdefghijklmnopqrstuvwx`,
			wantMatch:   true,
			targetRuleID: "SLACK-TOKEN",
		},
		{
			name:        "near-miss-short-segment",
			// Middle segment only 9 digits — pattern requires 10+
			line:        `SLACK_TOKEN=xoxb-123456789-1234567890-abcdefghijklmnopqrstuvwx`,
			wantMatch:   false,
			targetRuleID: "SLACK-TOKEN",
		},
		{
			name:        "must-allow-wrong-prefix",
			line:        `token: oauth-1234567890-1234567890-abcdefghijklmnopqrstuvwx`,
			wantMatch:   false,
			targetRuleID: "SLACK-TOKEN",
		},
	})
}

// ── Rule 11: Google API Key ───────────────────────────────────────────────────

func TestRule_GoogleAPIKey(t *testing.T) {
	runRuleCases(t, []ruleCase{
		{
			name:        "positive-maps-api-key",
			line:        `GOOGLE_MAPS_API_KEY=AIzaSyD-9tSrke72I6e0DVblZm6khPV0mFR5kq0`,
			wantMatch:   true,
			targetRuleID: "GOOGLE-API-KEY",
		},
		{
			name:        "positive-firebase-key",
			line:        `apiKey: "AIzaSyBOti4mM-6x9WDnZIjIeyEU21OpBXqHxeA"`,
			wantMatch:   true,
			targetRuleID: "GOOGLE-API-KEY",
		},
		{
			name:        "near-miss-too-short",
			// Only 34 chars after AIza — pattern requires exactly 35
			line:        `GOOGLE_API_KEY=AIzaSyD-9tSrke72I6e0DVblZm6khPV0mFR5k`,
			wantMatch:   false,
			targetRuleID: "GOOGLE-API-KEY",
		},
		{
			name:        "must-allow-wrong-prefix",
			line:        `GOOGLE_API_KEY=GIzaSyD-9tSrke72I6e0DVblZm6khPV0mFR5kq0`,
			wantMatch:   false,
			targetRuleID: "GOOGLE-API-KEY",
		},
	})
}

// ── Rule 12: GCP Service Account JSON ────────────────────────────────────────

func TestRule_GCPServiceAccount(t *testing.T) {
	runRuleCases(t, []ruleCase{
		{
			name:        "positive-type-field",
			line:        `  "type": "service_account",`,
			wantMatch:   true,
			targetRuleID: "GCP-SERVICE-ACCOUNT",
		},
		{
			name:        "positive-compact-json",
			line:        `{"type":"service_account","project_id":"my-project"}`,
			wantMatch:   true,
			targetRuleID: "GCP-SERVICE-ACCOUNT",
		},
		{
			name:        "near-miss-user-account",
			line:        `  "type": "user_account",`,
			wantMatch:   false,
			targetRuleID: "GCP-SERVICE-ACCOUNT",
		},
		{
			name:        "must-allow-documentation-reference",
			// String "service_account" appears in text but not in the JSON key=value form
			line:        `The type field for a service account credential file contains service account.`,
			wantMatch:   false,
			targetRuleID: "GCP-SERVICE-ACCOUNT",
		},
	})
}

// ── Rule 13: PEM Private Key ──────────────────────────────────────────────────

func TestRule_PEMPrivateKey(t *testing.T) {
	runRuleCases(t, []ruleCase{
		{
			name:        "positive-rsa-private-key",
			line:        `-----BEGIN RSA PRIVATE KEY-----`,
			wantMatch:   true,
			targetRuleID: "PEM-PRIVATE-KEY",
		},
		{
			name:        "positive-ec-private-key",
			line:        `-----BEGIN EC PRIVATE KEY-----`,
			wantMatch:   true,
			targetRuleID: "PEM-PRIVATE-KEY",
		},
		{
			name:        "positive-openssh-private-key",
			line:        `-----BEGIN OPENSSH PRIVATE KEY-----`,
			wantMatch:   true,
			targetRuleID: "PEM-PRIVATE-KEY",
		},
		{
			name:        "positive-generic-private-key",
			line:        `-----BEGIN PRIVATE KEY-----`,
			wantMatch:   true,
			targetRuleID: "PEM-PRIVATE-KEY",
		},
		{
			name:        "near-miss-public-key",
			line:        `-----BEGIN RSA PUBLIC KEY-----`,
			wantMatch:   false,
			targetRuleID: "PEM-PRIVATE-KEY",
		},
		{
			name:        "must-allow-certificate",
			line:        `-----BEGIN CERTIFICATE-----`,
			wantMatch:   false,
			targetRuleID: "PEM-PRIVATE-KEY",
		},
	})
}

// ── Rule 14: JWT Token ────────────────────────────────────────────────────────

func TestRule_JWTToken(t *testing.T) {
	// A real-looking JWT (header.payload.signature in base64url format)
	realJWT := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c`

	runRuleCases(t, []ruleCase{
		{
			name:        "positive-full-jwt",
			line:        `Authorization: Bearer ` + realJWT,
			wantMatch:   true,
			targetRuleID: "JWT-TOKEN",
		},
		{
			name:        "positive-jwt-in-source",
			line:        `const defaultToken = "` + realJWT + `"`,
			wantMatch:   true,
			targetRuleID: "JWT-TOKEN",
		},
		{
			name:        "near-miss-only-two-segments",
			// JWT requires 3 segments; two is not a JWT
			line:        `token: eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMifQ`,
			wantMatch:   false,
			targetRuleID: "JWT-TOKEN",
		},
		{
			name:        "must-allow-non-eyj-base64",
			// Does not start with eyJ
			line:        `data: aGVsbG8.d29ybGQ.dGVzdA`,
			wantMatch:   false,
			targetRuleID: "JWT-TOKEN",
		},
	})
}

// ── Rule 15: Generic Secret Assignment ───────────────────────────────────────

func TestRule_GenericSecretAssignment(t *testing.T) {
	runRuleCases(t, []ruleCase{
		{
			name:        "positive-api-key-assignment",
			line:        `API_KEY=thisIsALongSecretValueThatShouldBeDetected`,
			wantMatch:   true,
			targetRuleID: "GENERIC-SECRET-ASSIGNMENT",
		},
		{
			name:        "positive-password-with-quotes",
			line:        `DB_PASSWORD="averylongsecretpasswordthatismorethan20chars"`,
			wantMatch:   true,
			targetRuleID: "GENERIC-SECRET-ASSIGNMENT",
		},
		{
			name:        "near-miss-short-value",
			// Value is only 10 chars — pattern requires 20+
			line:        `SECRET_KEY=tooshort`,
			wantMatch:   false,
			targetRuleID: "GENERIC-SECRET-ASSIGNMENT",
		},
		{
			name:        "must-allow-non-secret-variable",
			// Variable name doesn't contain KEY/SECRET/TOKEN/PASS/PWD
			line:        `DATABASE_HOST=my-long-database-hostname.example.com`,
			wantMatch:   false,
			targetRuleID: "GENERIC-SECRET-ASSIGNMENT",
		},
	})
}

// ── Scanner options ───────────────────────────────────────────────────────────

func TestScannerAllowlist(t *testing.T) {
	// With allowlist for the JWT rule, a JWT in the body should NOT produce a finding.
	s := secretscan.New(secretscan.DefaultRules(), secretscan.WithAllowlist("JWT-TOKEN"))
	realJWT := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c`
	findings := s.Scan("test.go", []byte("const tok = \""+realJWT+"\""))
	for _, f := range findings {
		if f.RuleID == "JWT-TOKEN" {
			t.Errorf("expected JWT-TOKEN to be suppressed by allowlist, but got finding: %v", f)
		}
	}
}

func TestScannerSeverityFilter(t *testing.T) {
	// With SeverityHigh filter, Medium findings (JWT, generic assignment) should be absent.
	s := secretscan.New(secretscan.DefaultRules(), secretscan.WithSeverityFilter(secretscan.SeverityHigh))
	// JWT is Medium severity
	realJWT := `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c`
	findings := s.Scan("test.go", []byte("const tok = \""+realJWT+"\""))
	for _, f := range findings {
		if f.Severity < secretscan.SeverityHigh {
			t.Errorf("severity filter should suppress findings below High, got %v", f)
		}
	}
}

func TestScannerSeverityFilterBlocksCritical(t *testing.T) {
	// With no filter override, a PEM key should still be Critical.
	s := secretscan.New(secretscan.DefaultRules())
	findings := s.Scan("keyfile.go", []byte("-----BEGIN RSA PRIVATE KEY-----"))
	if len(findings) == 0 {
		t.Fatal("expected finding for PEM private key")
	}
	if findings[0].Severity != secretscan.SeverityCritical {
		t.Errorf("PEM private key should be Critical, got %v", findings[0].Severity)
	}
}

// ── ScanReader ────────────────────────────────────────────────────────────────

func TestScanReader(t *testing.T) {
	s := secretscan.New(secretscan.DefaultRules())
	content := "STRIPE_SECRET_KEY=sk_live_4eC39HqLyjWDarjtT1zdp7dc\n"
	findings, err := s.ScanReader("config.go", strings.NewReader(content))
	if err != nil {
		t.Fatalf("ScanReader error: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.RuleID == "STRIPE-LIVE-KEY" {
			found = true
		}
	}
	if !found {
		t.Error("expected STRIPE-LIVE-KEY finding from ScanReader")
	}
}

// ── GatePreCommit ─────────────────────────────────────────────────────────────

func TestGatePreCommit_BlocksHighSeverity(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.sh")
	// Use a high-entropy GitHub PAT (ghp_ + exactly 36 mixed-case alphanum chars)
	// that satisfies betterleaks' entropy gate as well as our hand-rolled rule.
	if err := os.WriteFile(f, []byte("export GITHUB_TOKEN=ghp_T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmN\n"), 0600); err != nil {
		t.Fatal(err)
	}
	findings, err := secretscan.GatePreCommit([]string{f})
	if err == nil {
		t.Fatal("expected GatePreCommit to return an error for GitHub PAT")
	}
	if len(findings) == 0 {
		t.Error("expected findings slice to be non-empty")
	}
}

func TestGatePreCommit_AllowsCleanFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "readme.md")
	if err := os.WriteFile(f, []byte("# My project\n\nThis is documentation.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	findings, err := secretscan.GatePreCommit([]string{f})
	if err != nil {
		t.Fatalf("expected no error for clean file, got: %v", err)
	}
	_ = findings
}

func TestGatePreCommit_BlocksSecretFilePath(t *testing.T) {
	dir := t.TempDir()
	// Write a benign .env file — the path itself should trigger the block.
	f := filepath.Join(dir, ".env")
	if err := os.WriteFile(f, []byte("NOTHING_SECRET=hello\n"), 0600); err != nil {
		t.Fatal(err)
	}
	findings, err := secretscan.GatePreCommit([]string{f})
	if err == nil {
		t.Fatal("expected GatePreCommit to block .env file path")
	}
	if len(findings) == 0 {
		t.Error("expected findings for blocked path")
	}
	blockedFound := false
	for _, f := range findings {
		if f.RuleID == "BLOCKED-PATH" {
			blockedFound = true
		}
	}
	if !blockedFound {
		t.Error("expected BLOCKED-PATH finding for .env file")
	}
}

func TestGatePreCommit_BlocksPEMFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "server.pem")
	if err := os.WriteFile(f, []byte("-----BEGIN CERTIFICATE-----\nABCDE==\n-----END CERTIFICATE-----\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := secretscan.GatePreCommit([]string{f})
	if err == nil {
		t.Fatal("expected GatePreCommit to block .pem file path")
	}
}

func TestGatePreCommit_BlocksCriticalPEMContent(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "setup.go")
	// The PEM key body needs 64+ chars of base64 content so betterleaks'
	// private-key rule (which anchors on the full header+body+footer) fires.
	// Our hand-rolled PEM-PRIVATE-KEY rule fires on the header alone, so both
	// scanner paths block this file. The gate-level assertion is scanner-agnostic.
	content := `package main

// privateKey is the server's TLS key.
const privateKey = ` + "`" + `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAxK9mP2vQ8nR4wL7jZ3bF6cT1dY5hA0eGr3sVwXyZ5aB2
cD6eF0gH4iJkLmNpQrSTuVwXyZ5aB2cD6eF0gH4iJkLmNpQrSTuVwXyZ5ab
-----END RSA PRIVATE KEY-----` + "`"
	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	findings, err := secretscan.GatePreCommit([]string{f})
	if err == nil {
		t.Fatal("expected GatePreCommit to block file containing PEM private key")
	}
	// Verify that the gate produced at least one blocking finding.
	// We do not assert a specific rule ID here because the active scanner
	// (betterleaks or hand-rolled fallback) may use different identifiers.
	// The gate test is: did the commit get blocked? Yes it did (err != nil).
	if len(findings) == 0 {
		t.Errorf("expected at least one finding, got none")
	}
}

// ── GatePreCommit — file size cap regression ────────────────────────────────

// TestGatePreCommit_OversizeFileBlocked regresses an unbounded-read OOM
// vector: before the fix, GatePreCommit called os.ReadFile with no size
// limit, so a malicious or accidental multi-GB file on the commit path
// would exhaust daemon memory. The gate now caps reads and blocks with a
// synthetic OVERSIZE-FILE finding when the cap is exceeded.
//
// We use a file one byte over the exported cap via a hole-sparse write so
// the test runs fast and uses very little disk.
func TestGatePreCommit_OversizeFileBlocked(t *testing.T) {
	const oversizeBytes = 16*1024*1024 + 1 // exceed the 16 MiB cap by one byte.
	dir := t.TempDir()
	f := filepath.Join(dir, "huge.md")
	fh, err := os.Create(f)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Sparse file: seek then write a single byte. The logical size is
	// oversizeBytes but the physical allocation is minimal.
	if _, err := fh.Seek(int64(oversizeBytes-1), 0); err != nil {
		fh.Close()
		t.Fatalf("seek: %v", err)
	}
	if _, err := fh.Write([]byte{0}); err != nil {
		fh.Close()
		t.Fatalf("write: %v", err)
	}
	fh.Close()

	findings, err := secretscan.GatePreCommit([]string{f})
	if err == nil {
		t.Fatal("expected GatePreCommit to block oversize file, got nil error")
	}
	oversizeFound := false
	for _, fn := range findings {
		if fn.RuleID == "OVERSIZE-FILE" {
			oversizeFound = true
			if fn.Severity != secretscan.SeverityHigh {
				t.Errorf("OVERSIZE-FILE severity: got %v, want High", fn.Severity)
			}
		}
	}
	if !oversizeFound {
		t.Errorf("expected OVERSIZE-FILE finding, got findings: %v", findings)
	}
}

// ── Corpus files ──────────────────────────────────────────────────────────────

// TestCorpusMustBlock loads all files from testdata/corpus/must-block/ and
// verifies that GatePreCommit returns an error (blocking finding) for each.
func TestCorpusMustBlock(t *testing.T) {
	corpusDir := filepath.Join("testdata", "corpus", "must-block")
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Fatalf("could not read must-block corpus dir %s: %v", corpusDir, err)
	}
	if len(entries) == 0 {
		t.Fatalf("must-block corpus is empty — add fixture files")
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(corpusDir, e.Name())
		t.Run(e.Name(), func(t *testing.T) {
			_, err := secretscan.GatePreCommit([]string{path})
			if err == nil {
				t.Errorf("expected GatePreCommit to block %s, but it passed", path)
			}
		})
	}
}

// TestCorpusMustAllow loads all files from testdata/corpus/must-allow/ and
// verifies that GatePreCommit does NOT return an error for each.
func TestCorpusMustAllow(t *testing.T) {
	corpusDir := filepath.Join("testdata", "corpus", "must-allow")
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Fatalf("could not read must-allow corpus dir %s: %v", corpusDir, err)
	}
	if len(entries) == 0 {
		t.Fatalf("must-allow corpus is empty — add fixture files")
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(corpusDir, e.Name())
		t.Run(e.Name(), func(t *testing.T) {
			_, err := secretscan.GatePreCommit([]string{path})
			if err != nil {
				t.Errorf("expected GatePreCommit to ALLOW %s, but got error: %v", path, err)
			}
		})
	}
}
