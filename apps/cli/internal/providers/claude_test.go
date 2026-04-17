package providers_test

// claude_test.go — tests for the claudeInstaller adapter.
//
// All tests use a temp directory as the "home" and a mockKeyIssuer so
// no OS keychain is involved.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/providers"
)

// ── mock KeyIssuer ────────────────────────────────────────────────────────────

type mockKeyIssuer struct {
	issuedID     string
	issuedSecret string
	revokedIDs   []string
	issueErr     error
	revokeErr    error
}

func (m *mockKeyIssuer) IssueKey(name, project, pathPrefix string) (string, string, error) {
	if m.issueErr != nil {
		return "", "", m.issueErr
	}
	return m.issuedID, m.issuedSecret, nil
}

func (m *mockKeyIssuer) RevokeKey(id string) error {
	if m.revokeErr != nil {
		return m.revokeErr
	}
	m.revokedIDs = append(m.revokedIDs, id)
	return nil
}

// ── test helpers ──────────────────────────────────────────────────────────────

func newTestInstaller(t *testing.T, homeDir string) (providers.ProviderInstaller, *providers.ReceiptStore, *mockKeyIssuer) {
	t.Helper()
	store, err := providers.NewReceiptStore(filepath.Join(homeDir, ".vedox"))
	if err != nil {
		t.Fatalf("NewReceiptStore: %v", err)
	}
	mock := &mockKeyIssuer{
		issuedID:     "test-key-id-abc123",
		issuedSecret: "deadbeefdeadbeef",
	}
	installer, err := providers.NewClaudeInstaller(homeDir, "http://127.0.0.1:5150", mock, store)
	if err != nil {
		t.Fatalf("NewClaudeInstaller: %v", err)
	}
	return installer, store, mock
}

// ── Probe ─────────────────────────────────────────────────────────────────────

func TestClaude_Probe_NotInstalled(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newTestInstaller(t, home)

	result, err := installer.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Installed {
		t.Error("Probe: expected Installed=false on fresh home dir")
	}
}

func TestClaude_Probe_AlreadyInstalled(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newTestInstaller(t, home)

	// Manually create the agent file with the expected name.
	agentsDir := filepath.Join(home, ".claude", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	agentFile := filepath.Join(agentsDir, "vedox-doc.md")
	content := "---\nname: vedox-doc-agent\ndescription: test\nversion: \"2.0\"\n---\nbody\n"
	if err := os.WriteFile(agentFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write agent file: %v", err)
	}

	result, err := installer.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !result.Installed {
		t.Error("Probe: expected Installed=true when agent file exists")
	}
	if result.ConfigPath == "" {
		t.Error("Probe: expected ConfigPath to be set")
	}
}

// ── Plan ─────────────────────────────────────────────────────────────────────

func TestClaude_Plan_GeneratesFileOps(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newTestInstaller(t, home)

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.Provider != providers.ProviderClaude {
		t.Errorf("Plan.Provider: got %q, want %q", plan.Provider, providers.ProviderClaude)
	}
	if len(plan.FileOps) == 0 {
		t.Fatal("Plan.FileOps is empty")
	}
	if plan.PlanHash == "" {
		t.Error("Plan.PlanHash is empty")
	}

	// Verify the agent file op is present.
	var hasAgentOp bool
	for _, op := range plan.FileOps {
		if strings.HasSuffix(op.Path, "vedox-doc.md") {
			hasAgentOp = true
			if len(op.Content) == 0 {
				t.Error("agent file op has empty content")
			}
			// Content should have the placeholder, not a real key.
			if !bytes.Contains(op.Content, []byte("{{HMAC_KEY_ID}}")) {
				t.Error("agent file content missing {{HMAC_KEY_ID}} placeholder")
			}
			// Content should have the daemon port.
			if !bytes.Contains(op.Content, []byte("5150")) {
				t.Error("agent file content missing daemon port")
			}
		}
	}
	if !hasAgentOp {
		t.Error("Plan: no FileOp for vedox-doc.md")
	}
}

func TestClaude_Plan_PlanHashIsStable(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newTestInstaller(t, home)

	plan1, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan 1: %v", err)
	}
	plan2, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan 2: %v", err)
	}
	if plan1.PlanHash != plan2.PlanHash {
		t.Errorf("Plan hash not stable: %q != %q", plan1.PlanHash, plan2.PlanHash)
	}
}

// ── Install ───────────────────────────────────────────────────────────────────

func TestClaude_Install_WritesFiles(t *testing.T) {
	home := t.TempDir()
	installer, _, mock := newTestInstaller(t, home)

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Receipt must be populated.
	if receipt.Provider != providers.ProviderClaude {
		t.Errorf("receipt.Provider: %q", receipt.Provider)
	}
	if receipt.AuthKeyID != mock.issuedID {
		t.Errorf("receipt.AuthKeyID: got %q, want %q", receipt.AuthKeyID, mock.issuedID)
	}
	if receipt.InstalledAt.IsZero() {
		t.Error("receipt.InstalledAt is zero")
	}
	if len(receipt.FileHashes) == 0 {
		t.Error("receipt.FileHashes is empty")
	}

	// Agent file must exist on disk.
	agentFile := filepath.Join(home, ".claude", "agents", "vedox-doc.md")
	data, err := os.ReadFile(agentFile)
	if err != nil {
		t.Fatalf("read agent file: %v", err)
	}

	// The real key ID must be present; placeholder must not be.
	if bytes.Contains(data, []byte("{{HMAC_KEY_ID}}")) {
		t.Error("agent file still contains {{HMAC_KEY_ID}} placeholder after install")
	}
	if !bytes.Contains(data, []byte(mock.issuedID)) {
		t.Errorf("agent file does not contain issued key ID %q", mock.issuedID)
	}
}

func TestClaude_Install_AppendsClaudeMD(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newTestInstaller(t, home)

	// Pre-create CLAUDE.md with some existing content.
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	claudeMD := filepath.Join(claudeDir, "CLAUDE.md")
	existing := "# My existing CLAUDE.md\n\nsome user content here.\n"
	if err := os.WriteFile(claudeMD, []byte(existing), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if _, err := installer.Install(context.Background(), plan); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	content := string(data)

	// Existing content must be preserved.
	if !strings.Contains(content, "some user content here.") {
		t.Error("CLAUDE.md: existing user content was lost")
	}
	// Vedox block must be present.
	if !strings.Contains(content, "<!-- vedox-agent:start -->") {
		t.Error("CLAUDE.md: vedox-agent:start fence missing")
	}
	if !strings.Contains(content, "<!-- vedox-agent:end -->") {
		t.Error("CLAUDE.md: vedox-agent:end fence missing")
	}
}

func TestClaude_Install_Idempotent(t *testing.T) {
	home := t.TempDir()

	// First install.
	installer1, store, _ := newTestInstaller(t, home)
	plan1, err := installer1.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan 1: %v", err)
	}
	receipt1, err := installer1.Install(context.Background(), plan1)
	if err != nil {
		t.Fatalf("Install 1: %v", err)
	}
	if err := store.Save(receipt1); err != nil {
		t.Fatalf("Save receipt 1: %v", err)
	}

	// Second install (fresh installer, same home).
	installer2, _, _ := newTestInstaller(t, home)
	plan2, err := installer2.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan 2: %v", err)
	}
	receipt2, err := installer2.Install(context.Background(), plan2)
	if err != nil {
		t.Fatalf("Install 2: %v", err)
	}

	// Both runs should succeed; the file should exist and contain the key.
	agentFile := filepath.Join(home, ".claude", "agents", "vedox-doc.md")
	if _, err := os.Stat(agentFile); err != nil {
		t.Errorf("agent file missing after second install: %v", err)
	}
	_ = receipt2

	// CLAUDE.md fence should appear exactly once.
	claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")
	data, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	count := strings.Count(string(data), "<!-- vedox-agent:start -->")
	if count != 1 {
		t.Errorf("CLAUDE.md: expected 1 vedox-agent:start fence, got %d", count)
	}
}

// ── Verify ────────────────────────────────────────────────────────────────────

func TestClaude_Verify_Healthy(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newTestInstaller(t, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	result, err := installer.Verify(context.Background(), receipt)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.Healthy {
		t.Errorf("Verify: expected Healthy=true, issues: %v", result.Issues)
	}
	if result.Drift {
		t.Errorf("Verify: expected Drift=false, issues: %v", result.Issues)
	}
}

func TestClaude_Verify_DriftDetected(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newTestInstaller(t, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Tamper with the agent file after install.
	agentFile := filepath.Join(home, ".claude", "agents", "vedox-doc.md")
	if err := os.WriteFile(agentFile, []byte("tampered content"), 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	result, err := installer.Verify(context.Background(), receipt)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Healthy {
		t.Error("Verify: expected Healthy=false after tampering")
	}
	if !result.Drift {
		t.Error("Verify: expected Drift=true after tampering")
	}
	if len(result.Issues) == 0 {
		t.Error("Verify: expected at least one issue reported")
	}
}

func TestClaude_Verify_FileMissing(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newTestInstaller(t, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Remove the agent file.
	agentFile := filepath.Join(home, ".claude", "agents", "vedox-doc.md")
	if err := os.Remove(agentFile); err != nil {
		t.Fatalf("remove: %v", err)
	}

	result, err := installer.Verify(context.Background(), receipt)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.Healthy {
		t.Error("Verify: expected Healthy=false when file missing")
	}
	if !result.Drift {
		t.Error("Verify: expected Drift=true when file missing")
	}
}

// ── Uninstall ─────────────────────────────────────────────────────────────────

func TestClaude_Uninstall_RemovesAgentFile(t *testing.T) {
	home := t.TempDir()
	installer, store, mock := newTestInstaller(t, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := store.Save(receipt); err != nil {
		t.Fatalf("Save receipt: %v", err)
	}

	if err := installer.Uninstall(context.Background()); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	// Agent file should be gone.
	agentFile := filepath.Join(home, ".claude", "agents", "vedox-doc.md")
	if _, err := os.Stat(agentFile); err == nil {
		t.Error("Uninstall: agent file still exists after uninstall")
	}

	// Key should have been revoked.
	if len(mock.revokedIDs) == 0 {
		t.Error("Uninstall: no key revocation recorded")
	}
	found := false
	for _, id := range mock.revokedIDs {
		if id == mock.issuedID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Uninstall: expected revoked key %q, got %v", mock.issuedID, mock.revokedIDs)
	}
}

func TestClaude_Uninstall_StripsClaudeMDBlock(t *testing.T) {
	home := t.TempDir()
	installer, store, _ := newTestInstaller(t, home)

	// Put existing content in CLAUDE.md before install.
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	claudeMD := filepath.Join(claudeDir, "CLAUDE.md")
	if err := os.WriteFile(claudeMD, []byte("# My notes\n\nuser content.\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := store.Save(receipt); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := installer.Uninstall(context.Background()); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	data, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("read CLAUDE.md after uninstall: %v", err)
	}
	content := string(data)

	// Vedox block must be gone.
	if strings.Contains(content, "<!-- vedox-agent:start -->") {
		t.Error("Uninstall: vedox-agent:start still present in CLAUDE.md")
	}
	// User content must survive.
	if !strings.Contains(content, "user content.") {
		t.Error("Uninstall: user content was removed from CLAUDE.md")
	}
}

// ── Repair ────────────────────────────────────────────────────────────────────

func TestClaude_Repair_ReinstallsOnDrift(t *testing.T) {
	home := t.TempDir()
	installer, store, _ := newTestInstaller(t, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := store.Save(receipt); err != nil {
		t.Fatalf("Save receipt: %v", err)
	}

	// Tamper with the agent file.
	agentFile := filepath.Join(home, ".claude", "agents", "vedox-doc.md")
	if err := os.WriteFile(agentFile, []byte("tampered"), 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	if err := installer.Repair(context.Background()); err != nil {
		t.Fatalf("Repair: %v", err)
	}

	// File should be restored.
	data, err := os.ReadFile(agentFile)
	if err != nil {
		t.Fatalf("read after repair: %v", err)
	}
	if string(data) == "tampered" {
		t.Error("Repair: file was not restored")
	}
	if len(data) < 50 {
		t.Error("Repair: restored file looks suspiciously short")
	}
}

// TestClaude_Probe_SchemaHashIsDeterministic is a regression test for a bug
// where schemaHashFromAgentFile iterated over a Go map without sorting keys
// first, producing a different hash on every Probe() call because Go map
// iteration order is randomised. Verify would then report spurious drift
// even when the file on disk was byte-identical.
func TestClaude_Probe_SchemaHashIsDeterministic(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newTestInstaller(t, home)

	// Write a realistic agent file with several frontmatter keys so that
	// random iteration order has a chance to produce different hashes.
	agentsDir := filepath.Join(home, ".claude", "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	agentFile := filepath.Join(agentsDir, "vedox-doc.md")
	content := "---\nname: vedox-doc-agent\ndescription: test\nversion: \"2.0\"\nprovider: claude\nextra_a: 1\nextra_b: 2\nextra_c: 3\n---\nbody\n"
	if err := os.WriteFile(agentFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Call Probe repeatedly; the schema hash must not change between calls.
	var firstHash string
	for i := 0; i < 20; i++ {
		result, err := installer.Probe(context.Background())
		if err != nil {
			t.Fatalf("Probe[%d]: %v", i, err)
		}
		if i == 0 {
			firstHash = result.SchemaHash
			if firstHash == "" {
				t.Fatal("Probe: first schema hash is empty")
			}
			continue
		}
		if result.SchemaHash != firstHash {
			t.Fatalf("Probe[%d]: schema hash non-deterministic: got %q, want %q",
				i, result.SchemaHash, firstHash)
		}
	}
}

func TestClaude_Repair_NoOpWhenHealthy(t *testing.T) {
	home := t.TempDir()
	installer, store, mock := newTestInstaller(t, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := store.Save(receipt); err != nil {
		t.Fatalf("Save receipt: %v", err)
	}

	// Record the key ID before repair.
	keyIDBeforeRepair := mock.issuedID

	if err := installer.Repair(context.Background()); err != nil {
		t.Fatalf("Repair on healthy: %v", err)
	}

	// If healthy, Repair should not issue a new key.
	// The mock's issued ID is fixed so we can only check the revoked list is empty.
	_ = keyIDBeforeRepair
	if len(mock.revokedIDs) > 0 {
		t.Errorf("Repair on healthy: unexpected key revocation: %v", mock.revokedIDs)
	}
}
