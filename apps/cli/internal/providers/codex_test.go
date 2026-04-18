package providers_test

// codex_test.go — tests for the codexInstaller adapter.
//
// All tests use a temp directory as the "home" and a mockKeyIssuer so
// no OS keychain is involved. The mockKeyIssuer type is defined in
// claude_test.go (same package).

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/providers"
)

// ── test helpers ──────────────────────────────────────────────────────────────

func newCodexTestInstaller(t *testing.T, homeDir string) (providers.ProviderInstaller, *providers.ReceiptStore, *mockKeyIssuer) {
	t.Helper()
	store, err := providers.NewReceiptStore(filepath.Join(homeDir, ".vedox"))
	if err != nil {
		t.Fatalf("NewReceiptStore: %v", err)
	}
	mock := &mockKeyIssuer{
		issuedID:     "codex-test-key-abc123",
		issuedSecret: "deadbeefdeadbeef",
	}
	installer, err := providers.NewCodexInstaller(homeDir, "http://127.0.0.1:5150", mock, store)
	if err != nil {
		t.Fatalf("NewCodexInstaller: %v", err)
	}
	return installer, store, mock
}

// ── Probe ─────────────────────────────────────────────────────────────────────

func TestCodex_Probe_NotInstalled(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newCodexTestInstaller(t, home)

	result, err := installer.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Installed {
		t.Error("Probe: expected Installed=false on fresh home dir")
	}
}

func TestCodex_Probe_DetectsConfigTOML(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newCodexTestInstaller(t, home)

	// Pre-create ~/.codex/config.toml with an mcp_servers.vedox entry.
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(codexDir, "config.toml")
	tomlContent := `[mcp_servers]
[mcp_servers.vedox]
url = "http://127.0.0.1:5150"
key_id = "some-key-id"
transport = "http"
`
	if err := os.WriteFile(configPath, []byte(tomlContent), 0o600); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	result, err := installer.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !result.Installed {
		t.Error("Probe: expected Installed=true when mcp_servers.vedox exists")
	}
	if result.ConfigPath == "" {
		t.Error("Probe: expected ConfigPath to be set")
	}
	if result.SchemaHash == "" {
		t.Error("Probe: expected SchemaHash to be set")
	}
}

func TestCodex_Probe_DetectsAgentsMDFence(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newCodexTestInstaller(t, home)

	// Pre-create ~/.codex/AGENTS.md with only the fence (no config.toml).
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	agentsPath := filepath.Join(codexDir, "AGENTS.md")
	content := "# My notes\n\n<!-- vedox-agent:start -->\nsome content\n<!-- vedox-agent:end -->\n"
	if err := os.WriteFile(agentsPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	result, err := installer.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !result.Installed {
		t.Error("Probe: expected Installed=true when AGENTS.md fence exists")
	}
}

func TestCodex_Probe_XDGFallback(t *testing.T) {
	home := t.TempDir()

	// Create only the XDG path, not ~/.codex.
	xdgDir := filepath.Join(home, ".config", "codex")
	if err := os.MkdirAll(xdgDir, 0o755); err != nil {
		t.Fatalf("mkdir xdg: %v", err)
	}
	configPath := filepath.Join(xdgDir, "config.toml")
	tomlContent := `[mcp_servers]
[mcp_servers.vedox]
url = "http://127.0.0.1:5150"
key_id = "xdg-key-id"
transport = "http"
`
	if err := os.WriteFile(configPath, []byte(tomlContent), 0o600); err != nil {
		t.Fatalf("write xdg config.toml: %v", err)
	}

	installer, _, _ := newCodexTestInstaller(t, home)
	result, err := installer.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !result.Installed {
		t.Error("Probe: expected Installed=true when XDG config.toml has vedox entry")
	}
	if !strings.Contains(result.ConfigPath, ".config/codex") {
		t.Errorf("Probe: expected XDG ConfigPath, got %q", result.ConfigPath)
	}
}

// ── Plan ─────────────────────────────────────────────────────────────────────

func TestCodex_Plan_GeneratesFileOps(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newCodexTestInstaller(t, home)

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.Provider != providers.ProviderCodex {
		t.Errorf("Plan.Provider: got %q, want %q", plan.Provider, providers.ProviderCodex)
	}
	if len(plan.FileOps) != 2 {
		t.Fatalf("Plan: expected 2 file ops, got %d", len(plan.FileOps))
	}
	if plan.PlanHash == "" {
		t.Error("Plan.PlanHash is empty")
	}

	// Op 0: config.toml.
	if !strings.HasSuffix(plan.FileOps[0].Path, "config.toml") {
		t.Errorf("FileOps[0].Path: expected config.toml, got %q", plan.FileOps[0].Path)
	}
	if !bytes.Contains(plan.FileOps[0].Content, []byte("{{HMAC_KEY_ID}}")) {
		t.Error("config.toml op missing {{HMAC_KEY_ID}} placeholder")
	}
	if !bytes.Contains(plan.FileOps[0].Content, []byte("vedox")) {
		t.Error("config.toml op missing vedox key")
	}

	// Op 1: AGENTS.md.
	if !strings.HasSuffix(plan.FileOps[1].Path, "AGENTS.md") {
		t.Errorf("FileOps[1].Path: expected AGENTS.md, got %q", plan.FileOps[1].Path)
	}
	if !bytes.Contains(plan.FileOps[1].Content, []byte("{{HMAC_KEY_ID}}")) {
		t.Error("AGENTS.md op missing {{HMAC_KEY_ID}} placeholder")
	}
}

func TestCodex_Plan_PlanHashIsStable(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newCodexTestInstaller(t, home)

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

func TestCodex_Plan_PreservesExistingTOML(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newCodexTestInstaller(t, home)

	// Pre-populate config.toml with an existing key the adapter should not touch.
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := `approval_mode = "suggest"
sandbox = "workspace-write"
`
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(existing), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	// The config.toml op should still contain existing keys.
	tomlOp := plan.FileOps[0]
	if !bytes.Contains(tomlOp.Content, []byte("approval_mode")) {
		t.Error("Plan: existing approval_mode key was not preserved in config.toml op")
	}
	if !bytes.Contains(tomlOp.Content, []byte("sandbox")) {
		t.Error("Plan: existing sandbox key was not preserved in config.toml op")
	}
}

// ── Install ───────────────────────────────────────────────────────────────────

func TestCodex_Install_WritesFiles(t *testing.T) {
	home := t.TempDir()
	installer, _, mock := newCodexTestInstaller(t, home)

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Receipt must be populated.
	if receipt.Provider != providers.ProviderCodex {
		t.Errorf("receipt.Provider: got %q, want %q", receipt.Provider, providers.ProviderCodex)
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

	// config.toml must exist and contain the real key ID.
	configPath := filepath.Join(home, ".codex", "config.toml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	if bytes.Contains(configData, []byte("{{HMAC_KEY_ID}}")) {
		t.Error("config.toml still contains {{HMAC_KEY_ID}} placeholder after install")
	}
	if !bytes.Contains(configData, []byte(mock.issuedID)) {
		t.Errorf("config.toml does not contain issued key ID %q", mock.issuedID)
	}

	// AGENTS.md must exist and contain the real key ID.
	agentsPath := filepath.Join(home, ".codex", "AGENTS.md")
	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if bytes.Contains(agentsData, []byte("{{HMAC_KEY_ID}}")) {
		t.Error("AGENTS.md still contains {{HMAC_KEY_ID}} placeholder after install")
	}
	if !bytes.Contains(agentsData, []byte(mock.issuedID)) {
		t.Errorf("AGENTS.md does not contain issued key ID %q", mock.issuedID)
	}
}

func TestCodex_Install_KeyRevocationOnFileOpFailure(t *testing.T) {
	home := t.TempDir()
	mock := &mockKeyIssuer{
		issuedID:     "codex-fail-key",
		issuedSecret: "secret",
	}
	store, err := providers.NewReceiptStore(filepath.Join(home, ".vedox"))
	if err != nil {
		t.Fatalf("NewReceiptStore: %v", err)
	}

	// Point the installer at a path outside the home boundary to force a file-op failure.
	installer, err := providers.NewCodexInstaller(home, "http://127.0.0.1:5150", mock, store)
	if err != nil {
		t.Fatalf("NewCodexInstaller: %v", err)
	}

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	// Make ~/.codex a file, not a directory, to cause MkdirAll to fail.
	codexPath := filepath.Join(home, ".codex")
	if err := os.WriteFile(codexPath, []byte("blocker"), 0o644); err != nil {
		t.Fatalf("create blocker file: %v", err)
	}

	_, installErr := installer.Install(context.Background(), plan)
	if installErr == nil {
		t.Fatal("Install: expected error due to blocked path, got nil")
	}

	// Key must have been revoked on failure.
	found := false
	for _, id := range mock.revokedIDs {
		if id == mock.issuedID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Install: key %q was not revoked on failure; revoked: %v", mock.issuedID, mock.revokedIDs)
	}
}

func TestCodex_Install_Idempotent(t *testing.T) {
	home := t.TempDir()

	// First install.
	installer1, store, _ := newCodexTestInstaller(t, home)
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
	installer2, _, _ := newCodexTestInstaller(t, home)
	plan2, err := installer2.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan 2: %v", err)
	}
	receipt2, err := installer2.Install(context.Background(), plan2)
	if err != nil {
		t.Fatalf("Install 2: %v", err)
	}
	_ = receipt2

	// config.toml must still be valid and contain exactly one vedox section.
	configPath := filepath.Join(home, ".codex", "config.toml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config.toml: %v", err)
	}
	count := strings.Count(string(configData), codexMCPServerKeyForTest)
	if count != 1 {
		t.Errorf("config.toml: expected 1 vedox key, got %d", count)
	}

	// AGENTS.md fence should appear exactly once.
	agentsPath := filepath.Join(home, ".codex", "AGENTS.md")
	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	fenceCount := strings.Count(string(agentsData), "<!-- vedox-agent:start -->")
	if fenceCount != 1 {
		t.Errorf("AGENTS.md: expected 1 vedox-agent:start fence, got %d", fenceCount)
	}
}

// codexMCPServerKeyForTest mirrors the unexported constant in the package.
const codexMCPServerKeyForTest = "vedox"

// ── Verify ────────────────────────────────────────────────────────────────────

func TestCodex_Verify_Healthy(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newCodexTestInstaller(t, home)

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

func TestCodex_Verify_DriftOnTamperedConfigTOML(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newCodexTestInstaller(t, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Tamper with config.toml.
	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.WriteFile(configPath, []byte("tampered = true\n"), 0o600); err != nil {
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

func TestCodex_Verify_FileMissing(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newCodexTestInstaller(t, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Remove config.toml.
	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.Remove(configPath); err != nil {
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

func TestCodex_Uninstall_RemovesVedoxEntry(t *testing.T) {
	home := t.TempDir()
	installer, store, mock := newCodexTestInstaller(t, home)

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

	// Key should have been revoked.
	found := false
	for _, id := range mock.revokedIDs {
		if id == mock.issuedID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Uninstall: key %q was not revoked; revoked: %v", mock.issuedID, mock.revokedIDs)
	}

	// config.toml should no longer contain the vedox mcp_servers entry.
	configPath := filepath.Join(home, ".codex", "config.toml")
	configData, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read config.toml after uninstall: %v", err)
	}
	if err == nil && strings.Contains(string(configData), codexMCPServerKeyForTest) {
		t.Error("Uninstall: vedox entry still present in config.toml")
	}
}

func TestCodex_Uninstall_StripsAgentsMDBlock(t *testing.T) {
	home := t.TempDir()
	installer, store, _ := newCodexTestInstaller(t, home)

	// Pre-create AGENTS.md with user content before install.
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	agentsPath := filepath.Join(codexDir, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# My custom rules\n\nuser content here.\n"), 0o644); err != nil {
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

	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md after uninstall: %v", err)
	}
	content := string(data)

	if strings.Contains(content, "<!-- vedox-agent:start -->") {
		t.Error("Uninstall: vedox-agent:start still present in AGENTS.md")
	}
	if !strings.Contains(content, "user content here.") {
		t.Error("Uninstall: user content was removed from AGENTS.md")
	}
}

func TestCodex_Uninstall_PreservesOtherTOMLKeys(t *testing.T) {
	home := t.TempDir()
	installer, store, _ := newCodexTestInstaller(t, home)

	// Pre-create config.toml with extra keys that must survive uninstall.
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := `approval_mode = "suggest"
sandbox = "workspace-write"
`
	if err := os.WriteFile(filepath.Join(codexDir, "config.toml"), []byte(existing), 0o600); err != nil {
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

	configPath := filepath.Join(codexDir, "config.toml")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config.toml after uninstall: %v", err)
	}
	content := string(configData)

	if !strings.Contains(content, "approval_mode") {
		t.Error("Uninstall: approval_mode key was removed from config.toml")
	}
	if !strings.Contains(content, "sandbox") {
		t.Error("Uninstall: sandbox key was removed from config.toml")
	}
}

// ── Repair ────────────────────────────────────────────────────────────────────

func TestCodex_Repair_ReinstallsOnDrift(t *testing.T) {
	home := t.TempDir()
	installer, store, _ := newCodexTestInstaller(t, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := store.Save(receipt); err != nil {
		t.Fatalf("Save receipt: %v", err)
	}

	// Tamper with config.toml.
	configPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.WriteFile(configPath, []byte("tampered = true\n"), 0o600); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	if err := installer.Repair(context.Background()); err != nil {
		t.Fatalf("Repair: %v", err)
	}

	// config.toml should be restored with the vedox entry.
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read after repair: %v", err)
	}
	if !strings.Contains(string(data), codexMCPServerKeyForTest) {
		t.Error("Repair: config.toml does not contain vedox entry after repair")
	}
}

func TestCodex_Repair_NoOpWhenHealthy(t *testing.T) {
	home := t.TempDir()
	installer, store, mock := newCodexTestInstaller(t, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := store.Save(receipt); err != nil {
		t.Fatalf("Save receipt: %v", err)
	}

	// Clear revoked list to test cleanly.
	mock.revokedIDs = nil

	if err := installer.Repair(context.Background()); err != nil {
		t.Fatalf("Repair on healthy: %v", err)
	}

	// If healthy, Repair must not revoke any key.
	if len(mock.revokedIDs) > 0 {
		t.Errorf("Repair on healthy: unexpected key revocation: %v", mock.revokedIDs)
	}
}
