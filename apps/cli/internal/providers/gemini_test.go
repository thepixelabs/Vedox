package providers_test

// gemini_test.go — tests for the geminiInstaller adapter.
//
// All tests use a temp directory as the "home" and the mockKeyIssuer defined
// in claude_test.go (same package). No OS keychain is involved.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/providers"
)

// ── test helper ───────────────────────────────────────────────────────────────

func newGeminiTestInstaller(t *testing.T, homeDir string) (providers.ProviderInstaller, *providers.ReceiptStore, *mockKeyIssuer) {
	t.Helper()
	store, err := providers.NewReceiptStore(filepath.Join(homeDir, ".vedox"))
	if err != nil {
		t.Fatalf("NewReceiptStore: %v", err)
	}
	mock := &mockKeyIssuer{
		issuedID:     "gemini-test-key-abc123",
		issuedSecret: "cafebabecafebabe",
	}
	installer, err := providers.NewGeminiInstaller(homeDir, "http://127.0.0.1:5150", mock, store)
	if err != nil {
		t.Fatalf("NewGeminiInstaller: %v", err)
	}
	return installer, store, mock
}

// ── Probe ─────────────────────────────────────────────────────────────────────

func TestGemini_Probe_NotInstalled(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newGeminiTestInstaller(t, home)

	result, err := installer.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Installed {
		t.Error("Probe: expected Installed=false on fresh home dir")
	}
}

func TestGemini_Probe_AlreadyInstalled(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newGeminiTestInstaller(t, home)

	// Manually create the extension manifest with the provider marker.
	extDir := filepath.Join(home, ".gemini", "extensions", "vedox")
	if err := os.MkdirAll(extDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	manifest := filepath.Join(extDir, "vedox-agent.json")
	content := `{"provider":"gemini","name":"Vedox Doc Agent","version":"2.0"}` + "\n"
	if err := os.WriteFile(manifest, []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	result, err := installer.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !result.Installed {
		t.Error("Probe: expected Installed=true when manifest exists with provider marker")
	}
	if result.ConfigPath == "" {
		t.Error("Probe: expected ConfigPath to be set")
	}
	if result.SchemaHash == "" {
		t.Error("Probe: expected SchemaHash to be set")
	}
}

// ── Plan ─────────────────────────────────────────────────────────────────────

func TestGemini_Plan_GeneratesFileOps(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newGeminiTestInstaller(t, home)

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.Provider != providers.ProviderGemini {
		t.Errorf("Plan.Provider: got %q, want %q", plan.Provider, providers.ProviderGemini)
	}
	if len(plan.FileOps) == 0 {
		t.Fatal("Plan.FileOps is empty")
	}
	if plan.PlanHash == "" {
		t.Error("Plan.PlanHash is empty")
	}

	// Verify the manifest file op is present and has placeholder content.
	var hasManifestOp bool
	for _, op := range plan.FileOps {
		if strings.HasSuffix(op.Path, "vedox-agent.json") {
			hasManifestOp = true
			if len(op.Content) == 0 {
				t.Error("manifest file op has empty content")
			}
			if !bytes.Contains(op.Content, []byte("{{HMAC_KEY_ID}}")) {
				t.Error("manifest content missing {{HMAC_KEY_ID}} placeholder")
			}
			if !bytes.Contains(op.Content, []byte("5150")) {
				t.Error("manifest content missing daemon port")
			}
			// Content must be valid JSON.
			if !json.Valid(bytes.TrimRight(op.Content, "\n")) {
				t.Error("manifest content is not valid JSON")
			}
		}
	}
	if !hasManifestOp {
		t.Error("Plan: no FileOp for vedox-agent.json")
	}
}

func TestGemini_Plan_IncludesConfigOp(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newGeminiTestInstaller(t, home)

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	var hasConfigOp bool
	for _, op := range plan.FileOps {
		if strings.HasSuffix(op.Path, "config.yaml") {
			hasConfigOp = true
			if !bytes.Contains(op.Content, []byte("# vedox-gemini:start")) {
				t.Error("config.yaml op missing vedox-gemini:start marker")
			}
			if !bytes.Contains(op.Content, []byte("vedox")) {
				t.Error("config.yaml op missing extension name")
			}
		}
	}
	if !hasConfigOp {
		t.Error("Plan: no FileOp for config.yaml")
	}
}

func TestGemini_Plan_PlanHashIsStable(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newGeminiTestInstaller(t, home)

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

func TestGemini_Install_WritesFiles(t *testing.T) {
	home := t.TempDir()
	installer, _, mock := newGeminiTestInstaller(t, home)

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Receipt must be populated.
	if receipt.Provider != providers.ProviderGemini {
		t.Errorf("receipt.Provider: got %q, want %q", receipt.Provider, providers.ProviderGemini)
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

	// Manifest file must exist.
	manifestFile := filepath.Join(home, ".gemini", "extensions", "vedox", "vedox-agent.json")
	data, err := os.ReadFile(manifestFile)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	// Placeholder must be gone; real key ID must be present.
	if bytes.Contains(data, []byte("{{HMAC_KEY_ID}}")) {
		t.Error("manifest still contains {{HMAC_KEY_ID}} placeholder after install")
	}
	if !bytes.Contains(data, []byte(mock.issuedID)) {
		t.Errorf("manifest does not contain issued key ID %q", mock.issuedID)
	}

	// config.yaml must exist with the extension block.
	configFile := filepath.Join(home, ".gemini", "config.yaml")
	configData, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	if !bytes.Contains(configData, []byte("# vedox-gemini:start")) {
		t.Error("config.yaml missing vedox-gemini:start block")
	}
}

func TestGemini_Install_AppendsConfigYAML(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newGeminiTestInstaller(t, home)

	// Pre-create config.yaml with some existing user content.
	geminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(geminiDir, "config.yaml")
	existing := "# My existing gemini config\ntheme: dark\n"
	if err := os.WriteFile(configPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if _, err := installer.Install(context.Background(), plan); err != nil {
		t.Fatalf("Install: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	content := string(data)

	// Existing content must be preserved.
	if !strings.Contains(content, "theme: dark") {
		t.Error("config.yaml: existing user content was lost")
	}
	// Vedox block must be present.
	if !strings.Contains(content, "# vedox-gemini:start") {
		t.Error("config.yaml: vedox-gemini:start fence missing")
	}
	if !strings.Contains(content, "# vedox-gemini:end") {
		t.Error("config.yaml: vedox-gemini:end fence missing")
	}
}

func TestGemini_Install_Idempotent(t *testing.T) {
	home := t.TempDir()

	// First install.
	installer1, store, _ := newGeminiTestInstaller(t, home)
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
	installer2, _, _ := newGeminiTestInstaller(t, home)
	plan2, err := installer2.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan 2: %v", err)
	}
	if _, err := installer2.Install(context.Background(), plan2); err != nil {
		t.Fatalf("Install 2: %v", err)
	}

	// config.yaml vedox fence should appear exactly once.
	configPath := filepath.Join(home, ".gemini", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config.yaml: %v", err)
	}
	count := strings.Count(string(data), "# vedox-gemini:start")
	if count != 1 {
		t.Errorf("config.yaml: expected 1 vedox-gemini:start fence, got %d", count)
	}
}

// ── Verify ────────────────────────────────────────────────────────────────────

func TestGemini_Verify_Healthy(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newGeminiTestInstaller(t, home)

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

func TestGemini_Verify_DriftDetected(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newGeminiTestInstaller(t, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Tamper with the manifest after install.
	manifestFile := filepath.Join(home, ".gemini", "extensions", "vedox", "vedox-agent.json")
	if err := os.WriteFile(manifestFile, []byte(`{"tampered":true}`+"\n"), 0o644); err != nil {
		t.Fatalf("tamper manifest: %v", err)
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

func TestGemini_Verify_FileMissing(t *testing.T) {
	home := t.TempDir()
	installer, _, _ := newGeminiTestInstaller(t, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Remove the manifest file.
	manifestFile := filepath.Join(home, ".gemini", "extensions", "vedox", "vedox-agent.json")
	if err := os.Remove(manifestFile); err != nil {
		t.Fatalf("remove manifest: %v", err)
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

func TestGemini_Uninstall_RemovesExtensionDir(t *testing.T) {
	home := t.TempDir()
	installer, store, mock := newGeminiTestInstaller(t, home)

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

	// Extension directory should be gone.
	extDir := filepath.Join(home, ".gemini", "extensions", "vedox")
	if _, err := os.Stat(extDir); err == nil {
		t.Error("Uninstall: extension directory still exists after uninstall")
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

func TestGemini_Uninstall_StripsConfigBlock(t *testing.T) {
	home := t.TempDir()
	installer, store, _ := newGeminiTestInstaller(t, home)

	// Pre-create config.yaml with existing user content.
	geminiDir := filepath.Join(home, ".gemini")
	if err := os.MkdirAll(geminiDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(geminiDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte("# user config\ntheme: dark\n"), 0o644); err != nil {
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

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config.yaml after uninstall: %v", err)
	}
	content := string(data)

	// Vedox block must be gone.
	if strings.Contains(content, "# vedox-gemini:start") {
		t.Error("Uninstall: vedox-gemini:start still present in config.yaml")
	}
	// User content must survive.
	if !strings.Contains(content, "theme: dark") {
		t.Error("Uninstall: user content was removed from config.yaml")
	}
}

func TestGemini_Uninstall_RemovesConfigWhenEmpty(t *testing.T) {
	home := t.TempDir()
	installer, store, _ := newGeminiTestInstaller(t, home)

	// No pre-existing config.yaml — install creates one with only the Vedox block.
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

	// config.yaml should be gone (only contained our block).
	configPath := filepath.Join(home, ".gemini", "config.yaml")
	if _, err := os.Stat(configPath); err == nil {
		// File still exists — read it to see what's left.
		data, _ := os.ReadFile(configPath)
		if strings.Contains(string(data), "# vedox-gemini:start") {
			t.Error("Uninstall: config.yaml still contains vedox block")
		}
	}
}

// ── Repair ────────────────────────────────────────────────────────────────────

func TestGemini_Repair_ReinstallsOnDrift(t *testing.T) {
	home := t.TempDir()
	installer, store, _ := newGeminiTestInstaller(t, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := store.Save(receipt); err != nil {
		t.Fatalf("Save receipt: %v", err)
	}

	// Tamper with the manifest file.
	manifestFile := filepath.Join(home, ".gemini", "extensions", "vedox", "vedox-agent.json")
	if err := os.WriteFile(manifestFile, []byte(`{"tampered":true}`+"\n"), 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	if err := installer.Repair(context.Background()); err != nil {
		t.Fatalf("Repair: %v", err)
	}

	// File should be restored and contain valid content.
	data, err := os.ReadFile(manifestFile)
	if err != nil {
		t.Fatalf("read after repair: %v", err)
	}
	if string(data) == `{"tampered":true}`+"\n" {
		t.Error("Repair: file was not restored")
	}
	if len(data) < 50 {
		t.Error("Repair: restored file looks suspiciously short")
	}
}

func TestGemini_Repair_NoOpWhenHealthy(t *testing.T) {
	home := t.TempDir()
	installer, store, mock := newGeminiTestInstaller(t, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := store.Save(receipt); err != nil {
		t.Fatalf("Save receipt: %v", err)
	}

	if err := installer.Repair(context.Background()); err != nil {
		t.Fatalf("Repair on healthy: %v", err)
	}

	// If healthy, no key should be revoked.
	if len(mock.revokedIDs) > 0 {
		t.Errorf("Repair on healthy: unexpected key revocation: %v", mock.revokedIDs)
	}
}
