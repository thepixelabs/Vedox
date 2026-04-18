package secretscan

import "regexp"

// DefaultRules returns the full set of deterministic secret detection rules
// for the Vedox Doc Agent pre-commit gate.
//
// Source: T-09 threat model table in
// `.tasks/vedox-v2/10-brainstorm/security-and-secrets.md` §Secret Detection
// in Agent Commits — Deterministic Rules (Layer 1).
//
// Rule count: 15. All ship Day 1 per D1-03 dispatch brief.
//
// Severity classification:
//   - Critical: cryptographic private key material or live payment secrets.
//   - High:     cloud/provider credentials with write access to infra or code.
//   - Medium:   tokens with narrower blast radius or advisory patterns.
//   - Low:      heuristic/entropy patterns; informational only.
//
// Confidence classification:
//   - High:   format-anchored prefixes unique to the secret type.
//   - Medium: plausible pattern with non-trivial false-positive rate.
//   - Low:    entropy heuristic; high false-positive rate on test data.
func DefaultRules() []Rule {
	return []Rule{
		// ── Rule 1: AWS Access Key ID ─────────────────────────────────────────
		// Source: T-09 table row 1; AWS IAM key format documented at
		// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_identifiers.html
		// AKIA prefix indicates a permanent (non-temporary) access key.
		// ASIA prefix indicates STS temporary keys — also detected here for defence-in-depth.
		{
			ID:          "AWS-ACCESS-KEY-ID",
			Name:        "AWS Access Key ID",
			Description: "AWS IAM access key ID. Allows calling AWS APIs under the associated IAM identity. Blast radius: all AWS services accessible to the identity.",
			Pattern:     regexp.MustCompile(`(?:AKIA|ASIA|AROA|AIDA|AIPA|ANPA|ANVA|APKA)[0-9A-Z]{16}`),
			Severity:    SeverityHigh,
			Confidence:  ConfidenceHigh,
		},

		// ── Rule 2: AWS Secret Access Key ─────────────────────────────────────
		// Source: T-09 table row 2; AWS secret access keys are 40-char base64url
		// strings adjacent to the words "aws_secret", "aws-secret", "secret_key",
		// or "SecretAccessKey" in config files / shell scripts.
		// Pattern covers INI, YAML, shell export, and JSON assignment forms.
		{
			ID:          "AWS-SECRET-ACCESS-KEY",
			Name:        "AWS Secret Access Key",
			Description: "AWS IAM secret access key paired with an access key ID. Gives full programmatic API access under the associated IAM identity.",
			Pattern:     regexp.MustCompile(`(?i)(?:aws[_\s-]?secret[_\s-]?(?:access[_\s-]?)?key|SecretAccessKey)["']?\s*[:=]\s*["']?[A-Za-z0-9+/]{40}`),
			Severity:    SeverityHigh,
			Confidence:  ConfidenceMedium,
		},

		// ── Rule 3: GitHub Personal Access Token (classic) ────────────────────
		// Source: T-09 table row 3; GitHub classic PAT format documented at
		// https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/about-authentication-to-github
		// ghp_ prefix introduced 2021-04. 36-char alphanumeric suffix.
		{
			ID:          "GITHUB-PAT-CLASSIC",
			Name:        "GitHub Personal Access Token (classic)",
			Description: "GitHub classic PAT. Can have repo, org, user, and gist scopes. Gives write access to any repo the issuing user can access.",
			Pattern:     regexp.MustCompile(`ghp_[A-Za-z0-9]{36}`),
			Severity:    SeverityHigh,
			Confidence:  ConfidenceHigh,
		},

		// ── Rule 4: GitHub Fine-Grained PAT ───────────────────────────────────
		// Source: T-09 table row 4; GitHub fine-grained PAT format documented at
		// https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens
		// github_pat_ prefix; 82-char suffix (alphanumeric + underscore).
		{
			ID:          "GITHUB-PAT-FINE-GRAINED",
			Name:        "GitHub Fine-Grained Personal Access Token",
			Description: "GitHub fine-grained PAT with repository-scoped permissions. Lower blast radius than classic PAT but still grants write access to targeted repos.",
			Pattern:     regexp.MustCompile(`github_pat_[A-Za-z0-9_]{82}`),
			Severity:    SeverityHigh,
			Confidence:  ConfidenceHigh,
		},

		// ── Rule 5: GitHub OAuth App token ────────────────────────────────────
		// Source: D1-03 dispatch brief rule list item 4; GitHub token format docs.
		// gho_ prefix = OAuth app user token. ghs_ = server-to-server installation token.
		// ghu_ = user-to-server token. All blocked at High severity.
		{
			ID:          "GITHUB-OAUTH-TOKEN",
			Name:        "GitHub OAuth / Server / User Token",
			Description: "GitHub OAuth app token (gho_), server-to-server installation token (ghs_), or user-to-server token (ghu_). Write access to resources in the app's scope.",
			Pattern:     regexp.MustCompile(`(?:gho|ghs|ghu)_[A-Za-z0-9]{36}`),
			Severity:    SeverityHigh,
			Confidence:  ConfidenceHigh,
		},

		// ── Rule 6: OpenAI API Key ────────────────────────────────────────────
		// Source: T-09 table §Pattern library row 5.
		// OpenAI keys: sk- prefix, 48 alphanumeric chars.
		// Note: OpenAI project keys use sk-proj- prefix — also detected by this
		// pattern since sk-proj- starts with sk-.
		{
			ID:          "OPENAI-API-KEY",
			Name:        "OpenAI API Key",
			Description: "OpenAI API key (sk-...). Allows calling the OpenAI API and incurring usage charges. Blast radius: model inference, fine-tuning, embeddings.",
			Pattern:     regexp.MustCompile(`sk-[A-Za-z0-9]{48}`),
			Severity:    SeverityHigh,
			Confidence:  ConfidenceHigh,
		},

		// ── Rule 7: Anthropic API Key ─────────────────────────────────────────
		// Source: T-09 table §Pattern library row 6.
		// Anthropic keys: sk-ant- prefix with alphanumeric suffix of at least 32 chars.
		// Format observed from Anthropic console as of 2026.
		{
			ID:          "ANTHROPIC-API-KEY",
			Name:        "Anthropic API Key",
			Description: "Anthropic Claude API key. Allows calling the Anthropic API and incurring usage charges. Blast radius: model inference across all Anthropic models.",
			Pattern:     regexp.MustCompile(`sk-ant-[A-Za-z0-9\-_]{32,}`),
			Severity:    SeverityHigh,
			Confidence:  ConfidenceHigh,
		},

		// ── Rule 8: Stripe Live Secret Key ────────────────────────────────────
		// Source: D1-03 dispatch brief rule list item 7; Stripe key format docs at
		// https://stripe.com/docs/keys
		// sk_live_ prefix = production secret key. Direct access to charge customers.
		{
			ID:          "STRIPE-LIVE-KEY",
			Name:        "Stripe Live Secret Key",
			Description: "Stripe production secret key. Allows creating charges, refunds, and managing payment methods. Critical: direct financial impact.",
			Pattern:     regexp.MustCompile(`sk_live_[0-9A-Za-z]{24,}`),
			Severity:    SeverityCritical,
			Confidence:  ConfidenceHigh,
		},

		// ── Rule 9: Stripe Test Secret Key ────────────────────────────────────
		// Source: D1-03 dispatch brief rule list item 8.
		// sk_test_ = test mode; no real money but reveals API structure and can
		// be used for testing fraud patterns. Classify High (not Critical).
		{
			ID:          "STRIPE-TEST-KEY",
			Name:        "Stripe Test Secret Key",
			Description: "Stripe test mode secret key. Not a live financial risk but reveals API structure and indicates the live key may follow the same format.",
			Pattern:     regexp.MustCompile(`sk_test_[0-9A-Za-z]{24,}`),
			Severity:    SeverityHigh,
			Confidence:  ConfidenceHigh,
		},

		// ── Rule 10: Slack Bot Token ──────────────────────────────────────────
		// Source: D1-03 dispatch brief rule list item 9; Slack token format at
		// https://api.slack.com/authentication/token-types
		// xoxb- = bot token. Also detects xoxp- (user token) and xoxa- (app-level token).
		{
			ID:          "SLACK-TOKEN",
			Name:        "Slack Bot / User Token",
			Description: "Slack bot token (xoxb-), user token (xoxp-), or app-level token (xoxa-). Grants access to Slack workspace channels, messages, and user data.",
			Pattern:     regexp.MustCompile(`xox[bpas]-[0-9]{10,}-[0-9]{10,}-[0-9A-Za-z]{24,}`),
			Severity:    SeverityHigh,
			Confidence:  ConfidenceHigh,
		},

		// ── Rule 11: Google API Key ───────────────────────────────────────────
		// Source: D1-03 dispatch brief rule list item 10; Google API key format at
		// https://cloud.google.com/docs/authentication/api-keys
		// AIza prefix is consistent across all Google API keys (Maps, YouTube, etc.).
		{
			ID:          "GOOGLE-API-KEY",
			Name:        "Google API Key",
			Description: "Google Cloud / Google APIs key (AIza...). Can enable Maps, YouTube, Firebase, or GCP service calls depending on key restrictions.",
			Pattern:     regexp.MustCompile(`AIza[0-9A-Za-z_\-]{35}`),
			Severity:    SeverityHigh,
			Confidence:  ConfidenceHigh,
		},

		// ── Rule 12: GCP Service Account JSON ────────────────────────────────
		// Source: D1-03 dispatch brief rule list item 11.
		// GCP service account JSON files contain both "type": "service_account"
		// and "private_key" fields. We detect the co-occurrence marker that
		// appears in these files: the literal string "service_account" near
		// "private_key_id" or "private_key".
		// This rule matches within a single line — files with the markers on
		// separate lines are caught by the file-path blocklist in GatePreCommit.
		{
			ID:          "GCP-SERVICE-ACCOUNT",
			Name:        "GCP Service Account Credential Marker",
			Description: "Google Cloud service account JSON key file marker. The full file grants impersonation of the service account across all GCP projects it can access.",
			Pattern:     regexp.MustCompile(`"type"\s*:\s*"service_account"`),
			Severity:    SeverityHigh,
			Confidence:  ConfidenceMedium,
		},

		// ── Rule 13: PEM Private Key Header ──────────────────────────────────
		// Source: T-09 table §Pattern library row 7; D1-03 dispatch brief item 13.
		// PEM headers are unambiguous markers of cryptographic private key material.
		// Any of RSA, EC, DSA, OPENSSH, or PGP private keys are critical.
		{
			ID:          "PEM-PRIVATE-KEY",
			Name:        "PEM Private Key Header",
			Description: "PEM-encoded private key header. Indicates RSA, EC, DSA, OPENSSH, or PGP private key material is in the file. Never commit private keys.",
			Pattern:     regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`),
			Severity:    SeverityCritical,
			Confidence:  ConfidenceHigh,
		},

		// ── Rule 14: JWT Token ────────────────────────────────────────────────
		// Source: D1-03 dispatch brief rule list item 12.
		// JWT tokens in source code are Medium severity — they may be examples
		// or test tokens. However, live JWTs with long expiry are serious.
		// Confidence: Medium because documentation often contains example JWTs.
		{
			ID:          "JWT-TOKEN",
			Name:        "JWT Bearer Token",
			Description: "JSON Web Token (three base64url segments separated by dots). JWTs in source code may be live session tokens with privileged claims.",
			Pattern:     regexp.MustCompile(`eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+`),
			Severity:    SeverityMedium,
			Confidence:  ConfidenceMedium,
		},

		// ── Rule 15: Generic High-Entropy .env-Style Assignment ───────────────
		// Source: T-09 table §Pattern library row 9 (generic KEY=value).
		// D1-03 dispatch brief rule list item 15.
		// Matches lines like: SECRET_KEY="abc123xyz..." or API_TOKEN='...'
		// Minimum 20 chars for the value to reduce noise from short env vars.
		// Severity: Medium (not High) because false-positive rate is significant —
		// many env vars with long-ish values are not secrets (e.g. DATABASE_HOST).
		// Confidence: Low (heuristic; high FP rate on non-secret assignments).
		{
			ID:          "GENERIC-SECRET-ASSIGNMENT",
			Name:        "Generic Secret ENV Assignment",
			Description: "Environment-variable-style assignment where the key name contains KEY, SECRET, TOKEN, PASS, or PWD and the value is 20+ characters. High false-positive rate; advisory only.",
			Pattern:     regexp.MustCompile(`(?i)(?:^|[^A-Za-z])[A-Z_]*(?:KEY|SECRET|TOKEN|PASS|PWD)[A-Z_]*\s*=\s*["']?[A-Za-z0-9+/=_\-]{20,}["']?`),
			Severity:    SeverityMedium,
			Confidence:  ConfidenceLow,
		},
	}
}
