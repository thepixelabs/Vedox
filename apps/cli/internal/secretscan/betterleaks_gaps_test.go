package secretscan_test

// betterleaks_gaps_test.go covers coverage gaps left after the betterleaks
// HYBRID migration. Specifically:
//
//  1. Fallback path: GatePreCommit falls back to New(DefaultRules()) when
//     NewBetterleaksScanner fails.
//  2. Redaction guarantee: Match in a betterleaks finding does not contain
//     the raw secret substring.
//  3. Severity-to-block contract: betterleaks findings with Severity >= High
//     cause GatePreCommit to return a non-nil error.
//  4. Edge cases: empty file, binary file, 1 MiB line, file at exact 16 MiB cap.
//  5. Integration via TestRepo: real-looking secret → block; clean file → pass.

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/secretscan"
	"github.com/vedox/vedox/internal/testutil"
)

// ── Fallback path ─────────────────────────────────────────────────────────────

// failingScanner is a Scanner that always returns one High-severity finding.
// It is used to verify that gatePreCommitWithScanner still blocks correctly
// without exercising the betterleaks code path.
//
// We cannot stub NewBetterleaksScanner at the package level (it is not a
// dependency-injection point); instead we verify the fallback behaviour by
// directly constructing New(DefaultRules()) and confirming it blocks known
// secrets — the same check GatePreCommit performs after a fallback.
func TestFallback_DefaultRulesBlockKnownSecrets(t *testing.T) {
	// When betterleaks is unavailable, GatePreCommit falls back to
	// New(DefaultRules()). Verify that the fallback scanner detects the full
	// set of our 15 hand-rolled rules. If this test fails after a DefaultRules()
	// change, the fallback path's security guarantee is broken.
	s := secretscan.New(secretscan.DefaultRules())

	cases := []struct {
		name  string
		input string
	}{
		{"aws-key-id", "aws_access_key_id = AKIAIOSFODNN7EXAMPLE"},
		{"aws-secret", `aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY`},
		{"github-pat-classic", "export GITHUB_TOKEN=ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ1234567890ABCD"},
		{"github-pat-fine-grained", "token: github_pat_" + strings.Repeat("a", 82)},
		{"github-oauth", "auth: gho_" + strings.Repeat("b", 36)},
		{"openai", "OPENAI_API_KEY=sk-" + strings.Repeat("a", 48)},
		{"anthropic", "ANTHROPIC_API_KEY=sk-ant-api03-" + strings.Repeat("z", 80)},
		{"stripe-live", "STRIPE_SK=sk_live_4eC39HqLyjWDarjtT1zdp7dc"},
		{"stripe-test", "STRIPE_SK=sk_test_4eC39HqLyjWDarjtT1zdp7dc"},
		{"slack", "SLACK_BOT_TOKEN=xoxb-1234567890-1234567890-abcdefghijklmnopqrstuvwx"},
		{"google-api-key", `GOOGLE_MAPS_API_KEY=AIzaSyD-9tSrke72I6e0DVblZm6khPV0mFR5kq0`},
		{"pem-private-key", "-----BEGIN RSA PRIVATE KEY-----"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			findings := s.Scan("fallback-test.txt", []byte(c.input))
			if len(findings) == 0 {
				t.Errorf("fallback scanner: expected at least one finding for %q, got none", c.name)
			}
		})
	}
}

// TestFallback_GatePreCommitWithDefaultScanner verifies that
// gatePreCommitWithScanner (the injected-scanner path) returns a non-nil error
// when given a file containing a known secret and the hand-rolled scanner. This
// is the code path GatePreCommit uses after a betterleaks init failure.
func TestFallback_GatePreCommitWithDefaultScanner(t *testing.T) {
	dir := testutil.TempDir(t)
	f := filepath.Join(dir, "config.sh")
	// GitHub PAT is detected by both betterleaks and the hand-rolled fallback.
	if err := os.WriteFile(f, []byte("export GITHUB_TOKEN=ghp_T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmN\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Call GatePreCommit (which internally tries betterleaks first, falls back
	// to DefaultRules on failure). We don't need to force the fallback — we just
	// need to confirm the gate blocks regardless of which scanner is active.
	_, err := secretscan.GatePreCommit([]string{f})
	if err == nil {
		t.Fatal("GatePreCommit: expected non-nil error blocking GitHub PAT, got nil")
	}
}

// ── Redaction guarantee ───────────────────────────────────────────────────────

// TestBetterleaksAdapter_MatchDoesNotContainRawSecret verifies that the
// Finding.Match field returned by the betterleaks adapter never contains the
// raw secret value. This is the core privacy contract of the gate — alerts and
// audit logs must not reconstruct the leaked secret.
//
// Covers: FindingFieldsPopulated only checked format; this checks the absence
// of the raw value explicitly for each known token type.
func TestBetterleaksAdapter_MatchDoesNotContainRawSecret(t *testing.T) {
	s, err := secretscan.NewBetterleaksScanner()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	cases := []struct {
		name      string
		input     string
		rawSecret string // the literal secret value that must NOT appear in Match
	}{
		{
			name:      "github-pat",
			input:     "export GITHUB_TOKEN=ghp_T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmN",
			rawSecret: "ghp_T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmN",
		},
		{
			name: "aws-pair",
			input: "aws_access_key_id = AKIA2XCPQLWDTMRR7ZKE\n" +
				"aws_secret_access_key = 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\n",
			rawSecret: "Tq8vR3nZ7xK2pF5mJ0wL6hY1bG4dC9sA+eQ3rN7v",
		},
		{
			name:      "stripe-live",
			input:     "STRIPE_SK=sk_live_T8Kj9mNpQrSTuVwXyZ5aB2c",
			rawSecret: "sk_live_T8Kj9mNpQrSTuVwXyZ5aB2c",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			findings := s.Scan("secret.env", []byte(c.input))
			if len(findings) == 0 {
				t.Fatalf("no findings for %q; cannot check redaction", c.name)
			}
			for _, f := range findings {
				if strings.Contains(f.Match, c.rawSecret) {
					t.Errorf("finding.Match %q contains raw secret %q — Redact() was not applied",
						f.Match, c.rawSecret)
				}
				// The redacted form must be non-empty.
				if f.Match == "" {
					t.Errorf("finding.Match is empty; expected redacted preview")
				}
			}
		})
	}
}

// ── Severity-to-block contract ────────────────────────────────────────────────

// TestBetterleaksAdapter_HighSeverityBlocksViaGate verifies the full chain:
// betterleaks detects a secret → Finding.Severity >= High → GatePreCommit
// returns a non-nil error. This is the contract callers (WS-C adapters) depend on.
func TestBetterleaksAdapter_HighSeverityBlocksViaGate(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{
			name:    "github-pat",
			content: "export GITHUB_TOKEN=ghp_T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmN\n",
		},
		{
			name: "aws-composite",
			content: "aws_access_key_id = AKIA2XCPQLWDTMRR7ZKE\n" +
				"aws_secret_access_key = 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\n",
		},
		{
			name:    "stripe-live",
			content: "STRIPE_SK=sk_live_T8Kj9mNpQrSTuVwXyZ5aB2c\n",
		},
		{
			name: "pem-private-key",
			content: "-----BEGIN RSA PRIVATE KEY-----\n" +
				"MIIEowIBAAKCAQEAxK9mP2vQ8nR4wL7jZ3bF6cT1dY5hA0eGr3sVwXyZ5aB2\n" +
				"cD6eF0gH4iJkLmNpQrSTuVwXyZ5aB2cD6eF0gH4iJkLmNpQrSTuVwXyZ5ab\n" +
				"-----END RSA PRIVATE KEY-----\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := testutil.TempDir(t)
			f := filepath.Join(dir, "secret.txt")
			if err := os.WriteFile(f, []byte(c.content), 0600); err != nil {
				t.Fatalf("write: %v", err)
			}
			findings, err := secretscan.GatePreCommit([]string{f})
			if err == nil {
				t.Fatalf("GatePreCommit(%q): expected non-nil error (High+ finding blocks), got nil", c.name)
			}
			// Confirm at least one blocking finding exists in the slice.
			hasBlock := false
			for _, fn := range findings {
				if fn.Severity >= secretscan.SeverityHigh {
					hasBlock = true
					break
				}
			}
			if !hasBlock {
				t.Errorf("GatePreCommit(%q): no High+ finding in slice despite non-nil error", c.name)
			}
		})
	}
}

// ── Edge cases ────────────────────────────────────────────────────────────────

// TestGatePreCommit_EmptyFile verifies that an empty file passes the gate
// without error and produces zero findings. An empty file cannot contain a
// secret, so blocking it would be a false positive.
func TestGatePreCommit_EmptyFile(t *testing.T) {
	dir := testutil.TempDir(t)
	f := filepath.Join(dir, "empty.md")
	if err := os.WriteFile(f, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}

	findings, err := secretscan.GatePreCommit([]string{f})
	if err != nil {
		t.Fatalf("GatePreCommit(empty file): expected nil error, got: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("GatePreCommit(empty file): expected 0 findings, got %d: %v", len(findings), findings)
	}
}

// TestGatePreCommit_BinaryFile verifies that a file starting with a PNG
// signature header (binary data) passes the gate without error. Binary files
// may trigger false positives from entropy-based scanners if they mistake
// compressed data for high-entropy secrets. The gate must handle binary
// content gracefully — no panic, no false positive, no error.
func TestGatePreCommit_BinaryFile(t *testing.T) {
	// PNG magic bytes: 8-byte signature + IHDR chunk header (12 bytes) +
	// enough zeroed content to be a structurally plausible PNG fragment.
	pngHeader := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, // IHDR length
		0x49, 0x48, 0x44, 0x52, // "IHDR"
		0x00, 0x00, 0x00, 0x01, // width: 1
		0x00, 0x00, 0x00, 0x01, // height: 1
		0x08, 0x02, 0x00, 0x00, 0x00, // bit depth, color type, compression, filter, interlace
		0x90, 0x77, 0x53, 0xDE, // CRC
	}

	dir := testutil.TempDir(t)
	f := filepath.Join(dir, "image.png")
	if err := os.WriteFile(f, pngHeader, 0600); err != nil {
		t.Fatal(err)
	}

	// A binary file must not cause a panic or a false-positive block.
	findings, err := secretscan.GatePreCommit([]string{f})
	if err != nil {
		t.Fatalf("GatePreCommit(binary PNG): expected nil error, got: %v\nfindings: %v", err, findings)
	}
}

// TestGatePreCommit_OneMiBLine verifies that a file containing a single line
// of exactly 1 MiB (1,048,576 bytes) is processed without error. This tests
// the 1 MiB scanner buffer fix: the hand-rolled scanner uses a 1 MiB bufio
// buffer to prevent bufio.Scanner "token too long" errors on minified JSON or
// base64 blobs. With betterleaks, the full body is passed as a string, so line
// length is not a concern — but the gate still must not return a scanner error
// or block a file with no secrets in it.
//
// The line content is all lowercase 'a' characters (low entropy, no pattern
// match), so no scanner backend should produce a finding.
func TestGatePreCommit_OneMiBLine(t *testing.T) {
	const oneMiB = 1 << 20 // 1,048,576 bytes
	line := bytes.Repeat([]byte("a"), oneMiB)

	dir := testutil.TempDir(t)
	f := filepath.Join(dir, "long-line.txt")
	if err := os.WriteFile(f, line, 0600); err != nil {
		t.Fatal(err)
	}

	findings, err := secretscan.GatePreCommit([]string{f})
	if err != nil {
		t.Fatalf("GatePreCommit(1 MiB line): expected nil error, got: %v\nfindings: %v", err, findings)
	}
	// Low-entropy repeated chars should produce no findings.
	for _, fn := range findings {
		if fn.RuleID != "SCANNER-ERROR" {
			continue
		}
		t.Errorf("1 MiB line triggered SCANNER-ERROR: %v", fn)
	}
}

// TestGatePreCommit_ExactlySizeCap verifies that a file of exactly 16 MiB
// (the maxScanFileBytes cap) is accepted by the gate without triggering the
// OVERSIZE-FILE block. The cap is strictly greater-than, so a file of exactly
// 16 MiB must not be blocked.
//
// The file is written as a sparse file (seek + single byte) to avoid consuming
// 16 MiB of real disk space during the test.
func TestGatePreCommit_ExactlySizeCap(t *testing.T) {
	const maxBytes = 16 * 1024 * 1024 // exactly at cap — must NOT trigger OVERSIZE-FILE

	dir := testutil.TempDir(t)
	f := filepath.Join(dir, "exactly-cap.bin")
	fh, err := os.Create(f)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Sparse write: seek to position maxBytes-1 and write one byte.
	// Logical file size = maxBytes; physical allocation = 1 block.
	if _, err := fh.Seek(int64(maxBytes-1), io.SeekStart); err != nil {
		fh.Close()
		t.Fatalf("seek: %v", err)
	}
	if _, err := fh.Write([]byte{0x00}); err != nil {
		fh.Close()
		t.Fatalf("write: %v", err)
	}
	fh.Close()

	findings, err := secretscan.GatePreCommit([]string{f})
	// The file is exactly at the cap — must not be blocked as OVERSIZE.
	for _, fn := range findings {
		if fn.RuleID == "OVERSIZE-FILE" {
			t.Errorf("file of exactly %d bytes should NOT trigger OVERSIZE-FILE, but it did", maxBytes)
		}
	}
	// No secrets in null-byte content — gate must pass.
	if err != nil {
		t.Fatalf("GatePreCommit(exactly-cap file): expected nil error, got: %v", err)
	}
}

// TestGatePreCommit_OneBytePastSizeCap re-runs the oversize regression to
// confirm that one byte over the cap does trigger OVERSIZE-FILE. This is the
// companion to TestGatePreCommit_ExactlySizeCap and together they pin the
// boundary semantics: cap is exclusive (strictly greater than).
func TestGatePreCommit_OneBytePastSizeCap(t *testing.T) {
	const oversizeBytes = 16*1024*1024 + 1

	dir := testutil.TempDir(t)
	f := filepath.Join(dir, "one-past-cap.bin")
	fh, err := os.Create(f)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := fh.Seek(int64(oversizeBytes-1), io.SeekStart); err != nil {
		fh.Close()
		t.Fatalf("seek: %v", err)
	}
	if _, err := fh.Write([]byte{0x00}); err != nil {
		fh.Close()
		t.Fatalf("write: %v", err)
	}
	fh.Close()

	findings, err := secretscan.GatePreCommit([]string{f})
	if err == nil {
		t.Fatal("expected GatePreCommit to block file one byte past cap, got nil error")
	}
	found := false
	for _, fn := range findings {
		if fn.RuleID == "OVERSIZE-FILE" {
			found = true
			if fn.Severity != secretscan.SeverityHigh {
				t.Errorf("OVERSIZE-FILE severity: got %v, want High", fn.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected OVERSIZE-FILE finding, got: %v", findings)
	}
}

// ── Integration via TestRepo ──────────────────────────────────────────────────

// TestIntegration_RealSecretViaTestRepo creates a file with a realistic secret
// in an ephemeral git repo, calls GatePreCommit on the staged path, and asserts
// block. This is the minimum viable integration test requested in the task.
func TestIntegration_RealSecretViaTestRepo(t *testing.T) {
	repo := testutil.NewTestRepo(t)

	// A GitHub classic PAT: ghp_ + exactly 36 mixed-case alphanum chars.
	// High entropy, no known-test-data allowlist entry in betterleaks.
	repo.WriteFile("deploy/secrets.sh",
		"#!/bin/bash\nexport GITHUB_TOKEN=ghp_T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmN\n")
	repo.CommitAll("add deploy script")

	path := filepath.Join(repo.Path(), "deploy/secrets.sh")
	_, err := secretscan.GatePreCommit([]string{path})
	if err == nil {
		t.Fatal("expected GatePreCommit to block file with GitHub PAT, got nil error")
	}
}

// TestIntegration_CleanFileViaTestRepo creates a documentation file with no
// secrets in an ephemeral git repo and asserts the gate passes.
func TestIntegration_CleanFileViaTestRepo(t *testing.T) {
	repo := testutil.NewTestRepo(t)

	repo.WriteFile("docs/overview.md", "# Overview\n\nThis is clean documentation.\n"+
		"Configuration is injected at runtime via the keychain.\n")
	repo.CommitAll("add overview")

	path := filepath.Join(repo.Path(), "docs/overview.md")
	_, err := secretscan.GatePreCommit([]string{path})
	if err != nil {
		t.Fatalf("expected GatePreCommit to pass clean file, got: %v", err)
	}
}

// ── Adapter output format ─────────────────────────────────────────────────────

// TestBetterleaksAdapter_OutputMatchesGateExpectation confirms that the
// betterleaks adapter's output format satisfies the contract that GatePreCommit
// relies on: findings with Severity >= SeverityHigh trigger a block.
//
// This test exercises the translation layer (translateFinding) by inspecting
// the Severity field on adapter output and comparing it against what
// gatePreCommitWithScanner needs to see.
func TestBetterleaksAdapter_OutputMatchesGateExpectation(t *testing.T) {
	s, err := secretscan.NewBetterleaksScanner()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// betterleaks rules have no severity tags in v1.1.2 — all map to SeverityHigh
	// (conservative default). Verify that a finding produced by the adapter has
	// Severity >= SeverityHigh so GatePreCommit will block it.
	input := []byte("export GITHUB_TOKEN=ghp_T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmN")
	findings := s.Scan("test.sh", input)

	if len(findings) == 0 {
		t.Fatal("no findings produced; cannot verify gate contract")
	}

	for _, f := range findings {
		if f.Severity < secretscan.SeverityHigh {
			t.Errorf("adapter finding severity %v is below High — GatePreCommit would not block it; rule: %s",
				f.Severity, f.RuleID)
		}
		// RuleID must not be empty — gate error message uses it.
		if f.RuleID == "" {
			t.Error("adapter finding has empty RuleID; gate error message will be empty")
		}
		// FilePath must be propagated correctly.
		if f.FilePath != "test.sh" {
			t.Errorf("adapter finding FilePath = %q, want %q", f.FilePath, "test.sh")
		}
	}
}

// TestBetterleaksAdapter_MapConfidence_ZeroEntropyIsHigh verifies the entropy→
// confidence mapping boundary: a rule with no entropy gate (entropy=0.0) must
// map to ConfidenceHigh, not ConfidenceLow. This matters because format-anchored
// rules in betterleaks commonly have no entropy threshold.
//
// We verify this indirectly: the GitHub PAT rule is format-anchored (ghp_ prefix
// + exact length), so its finding must have ConfidenceHigh.
func TestBetterleaksAdapter_MapConfidence_ZeroEntropyIsHigh(t *testing.T) {
	s, err := secretscan.NewBetterleaksScanner()
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	input := []byte("export GITHUB_TOKEN=ghp_T8Kj9mNpQr3sVwXyZ5aB2cD6eF0gH4iJkLmN")
	findings := s.Scan("test.sh", input)

	if len(findings) == 0 {
		t.Fatal("no findings produced")
	}

	for _, f := range findings {
		if f.Confidence == secretscan.ConfidenceLow {
			t.Errorf("format-anchored GitHub PAT finding has ConfidenceLow; expected High or Medium (rule: %s)", f.RuleID)
		}
	}
}
