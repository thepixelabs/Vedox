package providers_test

// copilot_test.go — tests for the copilotInstaller adapter.
//
// All tests use a temp directory as the "project root" (and a separate temp
// dir as "home") with a mockKeyIssuer so no OS keychain is involved.
// The mockKeyIssuer type is defined in claude_test.go (same package).

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

func newCopilotTestInstaller(t *testing.T, projectRoot, homeDir string) (providers.ProviderInstaller, *providers.ReceiptStore, *mockKeyIssuer) {
	t.Helper()
	store, err := providers.NewReceiptStore(filepath.Join(homeDir, ".vedox"))
	if err != nil {
		t.Fatalf("NewReceiptStore: %v", err)
	}
	mock := &mockKeyIssuer{
		issuedID:     "copilot-test-key-abc123",
		issuedSecret: "deadbeefdeadbeef",
	}
	installer, err := providers.NewCopilotInstaller(projectRoot, homeDir, "http://127.0.0.1:5150", mock, store)
	if err != nil {
		t.Fatalf("NewCopilotInstaller: %v", err)
	}
	return installer, store, mock
}

// copilotInstructionsPath returns the expected path to copilot-instructions.md
// inside a given project root.
func copilotInstructionsPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".github", "copilot-instructions.md")
}

// ── Probe ─────────────────────────────────────────────────────────────────────

func TestCopilot_Probe_NotInstalled(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()
	installer, _, _ := newCopilotTestInstaller(t, proj, home)

	result, err := installer.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Installed {
		t.Error("Probe: expected Installed=false when instructions file absent")
	}
	if result.ConfigPath != "" {
		t.Errorf("Probe: expected empty ConfigPath, got %q", result.ConfigPath)
	}
}

func TestCopilot_Probe_FileExistsNoVedoxBlock(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()

	// Pre-create .github/copilot-instructions.md without a Vedox block.
	githubDir := filepath.Join(proj, ".github")
	if err := os.MkdirAll(githubDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(githubDir, "copilot-instructions.md")
	if err := os.WriteFile(path, []byte("# My custom Copilot rules\n\nsome user content.\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	installer, _, _ := newCopilotTestInstaller(t, proj, home)
	result, err := installer.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.Installed {
		t.Error("Probe: expected Installed=false when file has no Vedox block")
	}
	if result.ConfigPath == "" {
		t.Error("Probe: expected ConfigPath to be set when file exists")
	}
	if result.SchemaHash == "" {
		t.Error("Probe: expected SchemaHash to be set when file exists")
	}
}

func TestCopilot_Probe_AlreadyInstalled(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()

	// Pre-create .github/copilot-instructions.md with a Vedox block.
	githubDir := filepath.Join(proj, ".github")
	if err := os.MkdirAll(githubDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(githubDir, "copilot-instructions.md")
	content := "# My rules\n\n<!-- vedox-copilot:start -->\nsome vedox content\n<!-- vedox-copilot:end -->\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	installer, _, _ := newCopilotTestInstaller(t, proj, home)
	result, err := installer.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !result.Installed {
		t.Error("Probe: expected Installed=true when Vedox block present")
	}
	if result.ConfigPath == "" {
		t.Error("Probe: expected ConfigPath to be set")
	}
}

// ── Plan ─────────────────────────────────────────────────────────────────────

func TestCopilot_Plan_GeneratesOneFileOp(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()
	installer, _, _ := newCopilotTestInstaller(t, proj, home)

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if plan.Provider != providers.ProviderCopilot {
		t.Errorf("Plan.Provider: got %q, want %q", plan.Provider, providers.ProviderCopilot)
	}
	if len(plan.FileOps) != 1 {
		t.Fatalf("Plan: expected 1 file op, got %d", len(plan.FileOps))
	}
	if plan.PlanHash == "" {
		t.Error("Plan.PlanHash is empty")
	}

	op := plan.FileOps[0]
	if !strings.HasSuffix(op.Path, "copilot-instructions.md") {
		t.Errorf("FileOps[0].Path: expected copilot-instructions.md, got %q", op.Path)
	}
	if len(op.Content) == 0 {
		t.Error("FileOps[0].Content is empty")
	}
}

func TestCopilot_Plan_ContentContainsVedoxSection(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()
	installer, _, _ := newCopilotTestInstaller(t, proj, home)

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	content := string(plan.FileOps[0].Content)
	for _, want := range []string{
		"<!-- vedox-copilot:start -->",
		"<!-- vedox-copilot:end -->",
		"## Vedox Documentation Agent",
		"vedox document everything",
		"vedox document this folder",
		"degraded mode",
		"Routing rules",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("Plan content missing expected string: %q", want)
		}
	}
}

func TestCopilot_Plan_NoHMACKeyIDInContent(t *testing.T) {
	// Copilot is degraded — the key ID must not appear in the instruction
	// file content (there is no tool surface to authenticate against).
	proj := t.TempDir()
	home := t.TempDir()
	installer, _, _ := newCopilotTestInstaller(t, proj, home)

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if bytes.Contains(plan.FileOps[0].Content, []byte("{{HMAC_KEY_ID}}")) {
		t.Error("Plan content must not embed {{HMAC_KEY_ID}} placeholder (Copilot is degraded — no tool surface)")
	}
}

func TestCopilot_Plan_PlanHashIsStable(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()
	installer, _, _ := newCopilotTestInstaller(t, proj, home)

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

func TestCopilot_Plan_PreservesExistingUserContent(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()

	// Pre-populate copilot-instructions.md with user rules.
	githubDir := filepath.Join(proj, ".github")
	if err := os.MkdirAll(githubDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(githubDir, "copilot-instructions.md")
	existing := "# My custom rules\n\nAlways use TypeScript strict mode.\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	installer, _, _ := newCopilotTestInstaller(t, proj, home)
	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	content := string(plan.FileOps[0].Content)
	if !strings.Contains(content, "Always use TypeScript strict mode.") {
		t.Error("Plan: existing user content was not preserved in instructions op")
	}
	if !strings.Contains(content, "<!-- vedox-copilot:start -->") {
		t.Error("Plan: Vedox section missing")
	}
}

// ── Install ───────────────────────────────────────────────────────────────────

func TestCopilot_Install_WritesFile(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()
	installer, _, mock := newCopilotTestInstaller(t, proj, home)

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Receipt must be correct.
	if receipt.Provider != providers.ProviderCopilot {
		t.Errorf("receipt.Provider: got %q, want %q", receipt.Provider, providers.ProviderCopilot)
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

	// Degraded version suffix must be present.
	if !strings.Contains(receipt.Version, "degraded") {
		t.Errorf("receipt.Version: expected 'degraded' suffix, got %q", receipt.Version)
	}

	// copilot-instructions.md must exist on disk.
	path := copilotInstructionsPath(proj)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read instructions file: %v", err)
	}
	if !bytes.Contains(data, []byte("<!-- vedox-copilot:start -->")) {
		t.Error("installed file missing vedox-copilot:start marker")
	}
}

func TestCopilot_Install_KeyIssuedButNotInFile(t *testing.T) {
	// The HMAC key is issued and stored, but must not appear in the
	// copilot-instructions.md (Copilot cannot call the daemon directly).
	proj := t.TempDir()
	home := t.TempDir()
	installer, _, mock := newCopilotTestInstaller(t, proj, home)

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	_, err = installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	path := copilotInstructionsPath(proj)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if bytes.Contains(data, []byte(mock.issuedID)) {
		t.Errorf("instructions file must not contain the HMAC key ID %q (Copilot degraded mode)", mock.issuedID)
	}
}

func TestCopilot_Install_KeyRevocationOnFileOpFailure(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()

	mock := &mockKeyIssuer{
		issuedID:     "copilot-fail-key",
		issuedSecret: "secret",
	}
	store, err := providers.NewReceiptStore(filepath.Join(home, ".vedox"))
	if err != nil {
		t.Fatalf("NewReceiptStore: %v", err)
	}

	installer, err := providers.NewCopilotInstaller(proj, home, "http://127.0.0.1:5150", mock, store)
	if err != nil {
		t.Fatalf("NewCopilotInstaller: %v", err)
	}

	plan, err := installer.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	// Make .github a file (not a directory) so MkdirAll fails.
	githubPath := filepath.Join(proj, ".github")
	if err := os.WriteFile(githubPath, []byte("blocker"), 0o644); err != nil {
		t.Fatalf("create blocker: %v", err)
	}

	_, installErr := installer.Install(context.Background(), plan)
	if installErr == nil {
		t.Fatal("Install: expected error due to blocked .github path, got nil")
	}

	// Key must be revoked on failure.
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

func TestCopilot_Install_Idempotent(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()

	// First install.
	installer1, store, _ := newCopilotTestInstaller(t, proj, home)
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

	// Second install (fresh installer, same dirs).
	installer2, _, _ := newCopilotTestInstaller(t, proj, home)
	plan2, err := installer2.Plan(context.Background())
	if err != nil {
		t.Fatalf("Plan 2: %v", err)
	}
	_, err = installer2.Install(context.Background(), plan2)
	if err != nil {
		t.Fatalf("Install 2: %v", err)
	}

	// Vedox section must appear exactly once.
	path := copilotInstructionsPath(proj)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	count := strings.Count(string(data), "<!-- vedox-copilot:start -->")
	if count != 1 {
		t.Errorf("Idempotent install: expected 1 vedox-copilot:start, got %d", count)
	}
}

// ── Verify ────────────────────────────────────────────────────────────────────

func TestCopilot_Verify_Healthy(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()
	installer, _, _ := newCopilotTestInstaller(t, proj, home)

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

func TestCopilot_Verify_DriftDetected(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()
	installer, _, _ := newCopilotTestInstaller(t, proj, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Tamper with the instructions file after install.
	path := copilotInstructionsPath(proj)
	if err := os.WriteFile(path, []byte("tampered content"), 0o644); err != nil {
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

func TestCopilot_Verify_FileMissing(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()
	installer, _, _ := newCopilotTestInstaller(t, proj, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Remove the instructions file.
	path := copilotInstructionsPath(proj)
	if err := os.Remove(path); err != nil {
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

func TestCopilot_Uninstall_StripsVedoxSection(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()

	// Pre-create instructions file with user content.
	githubDir := filepath.Join(proj, ".github")
	if err := os.MkdirAll(githubDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(githubDir, "copilot-instructions.md")
	if err := os.WriteFile(path, []byte("# My rules\n\nuser content here.\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	installer, store, mock := newCopilotTestInstaller(t, proj, home)

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

	// Key must have been revoked.
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

	// Vedox section must be gone; user content must survive.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after uninstall: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "<!-- vedox-copilot:start -->") {
		t.Error("Uninstall: vedox-copilot:start still present")
	}
	if !strings.Contains(content, "user content here.") {
		t.Error("Uninstall: user content was removed")
	}
}

func TestCopilot_Uninstall_RemovesFileWhenEmpty(t *testing.T) {
	// When the instructions file contained only the Vedox section,
	// uninstall should remove the file entirely rather than leave an empty file.
	proj := t.TempDir()
	home := t.TempDir()
	installer, store, _ := newCopilotTestInstaller(t, proj, home)

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

	// File should be absent (was only Vedox content).
	path := copilotInstructionsPath(proj)
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("Uninstall: instructions file still exists after full Vedox-only removal; expected file to be deleted")
	}
}

// ── Repair ────────────────────────────────────────────────────────────────────

func TestCopilot_Repair_ReinstallsOnDrift(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()
	installer, store, _ := newCopilotTestInstaller(t, proj, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := store.Save(receipt); err != nil {
		t.Fatalf("Save receipt: %v", err)
	}

	// Tamper with the instructions file.
	path := copilotInstructionsPath(proj)
	if err := os.WriteFile(path, []byte("tampered"), 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}

	if err := installer.Repair(context.Background()); err != nil {
		t.Fatalf("Repair: %v", err)
	}

	// File must be restored and contain the Vedox section.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after repair: %v", err)
	}
	if string(data) == "tampered" {
		t.Error("Repair: file was not restored")
	}
	if !bytes.Contains(data, []byte("<!-- vedox-copilot:start -->")) {
		t.Error("Repair: restored file missing vedox-copilot:start marker")
	}
}

func TestCopilot_Repair_NoOpWhenHealthy(t *testing.T) {
	proj := t.TempDir()
	home := t.TempDir()
	installer, store, mock := newCopilotTestInstaller(t, proj, home)

	plan, _ := installer.Plan(context.Background())
	receipt, err := installer.Install(context.Background(), plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := store.Save(receipt); err != nil {
		t.Fatalf("Save receipt: %v", err)
	}

	// Clear revoked list.
	mock.revokedIDs = nil

	if err := installer.Repair(context.Background()); err != nil {
		t.Fatalf("Repair on healthy: %v", err)
	}

	// Repair on a healthy install must not revoke any key.
	if len(mock.revokedIDs) > 0 {
		t.Errorf("Repair on healthy: unexpected key revocation: %v", mock.revokedIDs)
	}
}
