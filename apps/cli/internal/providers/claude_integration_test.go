package providers_test

// claude_integration_test.go — end-to-end lifecycle tests for the Claude
// provider installer.
//
// These tests exercise a single cohesive state machine:
//   install → persist receipt → reload receipt → verify → drift → repair →
//   verify healthy → uninstall → assert all files and receipt gone.
//
// Every test uses a temp directory as the mock home dir. No OS keychain is
// involved — the mockKeyIssuer defined in claude_test.go is reused.
//
// Build tag: none. These run with `go test ./...` and `go test -race ./...`.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/providers"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// newLifecycleFixture builds a claudeInstaller + ReceiptStore pair rooted at
// a fresh temp home. The mockKeyIssuer is returned for assertion on revocations.
func newLifecycleFixture(t *testing.T) (home string, installer providers.ProviderInstaller, store *providers.ReceiptStore, mock *mockKeyIssuer) {
	t.Helper()
	home = t.TempDir()
	// Resolve symlinks — macOS t.TempDir returns /var/... which is a symlink to
	// /private/var/...; atomicFileWrite's assertNoSymlink would reject this.
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}
	installer, store, mock = newTestInstaller(t, home)
	return
}

// doInstall runs Plan → Install → Save and returns the receipt. Fails the test
// on any error so callers can focus on the behavior under test.
func doInstall(t *testing.T, ctx context.Context, installer providers.ProviderInstaller, store *providers.ReceiptStore) *providers.InstallReceipt {
	t.Helper()
	plan, err := installer.Plan(ctx)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	receipt, err := installer.Install(ctx, plan)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if err := store.Save(receipt); err != nil {
		t.Fatalf("Save receipt: %v", err)
	}
	return receipt
}

// ── Full lifecycle ────────────────────────────────────────────────────────────

// TestClaude_FullLifecycle_InstallVerifyRepairUninstall exercises the complete
// state machine in sequence within a single test, verifying each transition.
//
// This is the primary integration test: if this passes under -race the adapter
// wires together correctly end-to-end.
func TestClaude_FullLifecycle_InstallVerifyRepairUninstall(t *testing.T) {
	ctx := context.Background()
	home, installer, store, mock := newLifecycleFixture(t)

	agentFile := filepath.Join(home, ".claude", "agents", "vedox-doc.md")
	claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")

	// ── 1. Install ────────────────────────────────────────────────────────────
	receipt := doInstall(t, ctx, installer, store)

	if receipt.Version != "2.0" {
		t.Errorf("receipt.Version: got %q, want %q", receipt.Version, "2.0")
	}
	if receipt.DaemonURL != "http://127.0.0.1:5150" {
		t.Errorf("receipt.DaemonURL: got %q", receipt.DaemonURL)
	}

	// ── 2. Receipt is persisted and readable by a new ReceiptStore instance ──
	store2, err := providers.NewReceiptStore(filepath.Join(home, ".vedox"))
	if err != nil {
		t.Fatalf("NewReceiptStore (second instance): %v", err)
	}
	loaded, err := store2.Load(providers.ProviderClaude)
	if err != nil {
		t.Fatalf("Load receipt from second store: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load: returned nil — receipt was not persisted to disk")
	}
	if loaded.Provider != providers.ProviderClaude {
		t.Errorf("loaded.Provider: got %q, want %q", loaded.Provider, providers.ProviderClaude)
	}
	if loaded.AuthKeyID != mock.issuedID {
		t.Errorf("loaded.AuthKeyID: got %q, want %q", loaded.AuthKeyID, mock.issuedID)
	}
	if len(loaded.FileHashes) == 0 {
		t.Error("loaded.FileHashes: empty — file hashes were not persisted")
	}

	// ── 3. Installed files exist with correct content ─────────────────────────
	agentData, err := os.ReadFile(agentFile)
	if err != nil {
		t.Fatalf("read agent file: %v", err)
	}
	if !strings.Contains(string(agentData), "vedox-doc-agent") {
		t.Error("agent file does not contain the expected agent name")
	}
	// Placeholder must be replaced with the real key ID.
	if strings.Contains(string(agentData), "{{HMAC_KEY_ID}}") {
		t.Error("agent file still contains {{HMAC_KEY_ID}} placeholder — substitution failed")
	}
	if !strings.Contains(string(agentData), mock.issuedID) {
		t.Errorf("agent file does not contain issued key ID %q", mock.issuedID)
	}
	// CLAUDE.md fenced block must be present.
	claudeMDData, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(claudeMDData), "<!-- vedox-agent:start -->") {
		t.Error("CLAUDE.md missing vedox-agent:start block after install")
	}

	// ── 4. Verify reports healthy immediately after install ───────────────────
	v, err := installer.Verify(ctx, loaded)
	if err != nil {
		t.Fatalf("Verify (post-install): %v", err)
	}
	if !v.Healthy {
		t.Errorf("Verify (post-install): expected Healthy=true, issues: %v", v.Issues)
	}
	if v.Drift {
		t.Errorf("Verify (post-install): expected Drift=false, issues: %v", v.Issues)
	}

	// ── 5. Drift detection: modify installed file, Verify reports drift ───────
	if err := os.WriteFile(agentFile, []byte("out-of-band modification"), 0o644); err != nil {
		t.Fatalf("tamper agent file: %v", err)
	}
	v2, err := installer.Verify(ctx, loaded)
	if err != nil {
		t.Fatalf("Verify (post-tamper): %v", err)
	}
	if v2.Healthy {
		t.Error("Verify (post-tamper): expected Healthy=false after modification")
	}
	if !v2.Drift {
		t.Error("Verify (post-tamper): expected Drift=true after modification")
	}
	if len(v2.Issues) == 0 {
		t.Error("Verify (post-tamper): Issues slice must be non-empty when drift is detected")
	}

	// ── 6. Repair restores the drifted file ───────────────────────────────────
	if err := installer.Repair(ctx); err != nil {
		t.Fatalf("Repair: %v", err)
	}

	// Re-load the updated receipt written by Repair.
	repairedReceipt, err := store.Load(providers.ProviderClaude)
	if err != nil {
		t.Fatalf("Load receipt after Repair: %v", err)
	}
	if repairedReceipt == nil {
		t.Fatal("receipt is nil after Repair — Repair must persist a new receipt")
	}

	// The restored file must not contain the tampered content.
	restoredData, err := os.ReadFile(agentFile)
	if err != nil {
		t.Fatalf("read agent file after Repair: %v", err)
	}
	if string(restoredData) == "out-of-band modification" {
		t.Error("Repair: agent file was not restored (still shows tampered content)")
	}
	if len(restoredData) < 50 {
		t.Errorf("Repair: restored agent file looks suspiciously short (%d bytes)", len(restoredData))
	}

	// Verify should now report healthy again.
	v3, err := installer.Verify(ctx, repairedReceipt)
	if err != nil {
		t.Fatalf("Verify (post-Repair): %v", err)
	}
	if !v3.Healthy {
		t.Errorf("Verify (post-Repair): expected Healthy=true, issues: %v", v3.Issues)
	}

	// ── 7. Uninstall removes all files and the receipt ────────────────────────
	revokedBefore := len(mock.revokedIDs)
	if err := installer.Uninstall(ctx); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	// Agent file must be gone.
	if _, err := os.Stat(agentFile); err == nil {
		t.Error("Uninstall: agent file still exists after uninstall")
	}

	// Vedox block must be stripped from CLAUDE.md.
	claudeMDPost, err := os.ReadFile(claudeMD)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read CLAUDE.md after uninstall: %v", err)
	}
	if !os.IsNotExist(err) && strings.Contains(string(claudeMDPost), "<!-- vedox-agent:start -->") {
		t.Error("Uninstall: vedox-agent:start block still present in CLAUDE.md")
	}

	// Receipt must be deleted.
	finalReceipt, err := store.Load(providers.ProviderClaude)
	if err != nil {
		t.Fatalf("Load receipt after Uninstall: %v", err)
	}
	if finalReceipt != nil {
		t.Error("Uninstall: receipt still present after uninstall — should have been deleted")
	}

	// Key must have been revoked.
	if len(mock.revokedIDs) == revokedBefore {
		t.Error("Uninstall: no new key revocation recorded")
	}
}

// ── Receipt persistence properties ───────────────────────────────────────────

// TestClaude_Receipt_FilePermissions verifies that the persisted receipt file
// has 0o600 permissions (owner read/write only).
func TestClaude_Receipt_FilePermissions(t *testing.T) {
	ctx := context.Background()
	home, installer, store, _ := newLifecycleFixture(t)

	receipt := doInstall(t, ctx, installer, store)
	_ = receipt

	receiptPath := filepath.Join(home, ".vedox", "install-receipts", "claude.json")
	info, err := os.Stat(receiptPath)
	if err != nil {
		t.Fatalf("stat receipt file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("receipt file permissions: got %o, want 0600", perm)
	}
}

// TestClaude_Receipt_AllFieldsPersisted verifies that every field of the receipt
// round-trips through JSON correctly — a regression guard against accidental
// field omission or tag rename.
func TestClaude_Receipt_AllFieldsPersisted(t *testing.T) {
	ctx := context.Background()
	home, installer, store, mock := newLifecycleFixture(t)

	receipt := doInstall(t, ctx, installer, store)

	// Re-load from a fresh store instance to exercise the full JSON round-trip.
	store2, err := providers.NewReceiptStore(filepath.Join(home, ".vedox"))
	if err != nil {
		t.Fatalf("NewReceiptStore: %v", err)
	}
	loaded, err := store2.Load(providers.ProviderClaude)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load: nil — receipt not on disk")
	}

	if loaded.Provider != providers.ProviderClaude {
		t.Errorf("Provider: got %q, want %q", loaded.Provider, providers.ProviderClaude)
	}
	if loaded.Version != receipt.Version {
		t.Errorf("Version: got %q, want %q", loaded.Version, receipt.Version)
	}
	if loaded.AuthKeyID != mock.issuedID {
		t.Errorf("AuthKeyID: got %q, want %q", loaded.AuthKeyID, mock.issuedID)
	}
	if loaded.DaemonURL != receipt.DaemonURL {
		t.Errorf("DaemonURL: got %q, want %q", loaded.DaemonURL, receipt.DaemonURL)
	}
	if loaded.InstalledAt.IsZero() {
		t.Error("InstalledAt: zero after round-trip")
	}
	if len(loaded.FileHashes) == 0 {
		t.Error("FileHashes: empty after round-trip")
	}
	if loaded.SchemaHash == "" {
		t.Error("SchemaHash: empty after round-trip")
	}
}

// ── CLAUDE.md user-content preservation ──────────────────────────────────────

// TestClaude_Uninstall_PreservesUserContentInClaudeMD verifies that uninstall
// strips only the Vedox fenced block and leaves all user-authored content intact.
func TestClaude_Uninstall_PreservesUserContentInClaudeMD(t *testing.T) {
	ctx := context.Background()
	home, installer, store, _ := newLifecycleFixture(t)

	// Seed CLAUDE.md with user content that must survive the full lifecycle.
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	claudeMD := filepath.Join(claudeDir, "CLAUDE.md")
	userContent := "# My personal instructions\n\nAlways respond in formal English.\n"
	if err := os.WriteFile(claudeMD, []byte(userContent), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}

	doInstall(t, ctx, installer, store)

	if err := installer.Uninstall(ctx); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	data, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("read CLAUDE.md after uninstall: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "Always respond in formal English.") {
		t.Error("Uninstall: user content was stripped from CLAUDE.md")
	}
	if strings.Contains(content, "<!-- vedox-agent:start -->") {
		t.Error("Uninstall: Vedox block was not removed from CLAUDE.md")
	}
}

// ── Repair when no receipt exists ────────────────────────────────────────────

// TestClaude_Repair_WhenNoReceipt verifies that Repair on a store with no prior
// receipt performs a fresh install rather than returning an error.
func TestClaude_Repair_WhenNoReceipt(t *testing.T) {
	ctx := context.Background()
	home, installer, store, _ := newLifecycleFixture(t)

	// No install — call Repair cold.
	if err := installer.Repair(ctx); err != nil {
		t.Fatalf("Repair with no prior receipt: %v", err)
	}

	// A receipt should now exist.
	receipt, err := store.Load(providers.ProviderClaude)
	if err != nil {
		t.Fatalf("Load after cold Repair: %v", err)
	}
	if receipt == nil {
		t.Error("Repair with no prior receipt: expected receipt to be created, got nil")
	}

	// Agent file must exist.
	agentFile := filepath.Join(home, ".claude", "agents", "vedox-doc.md")
	if _, err := os.Stat(agentFile); err != nil {
		t.Errorf("agent file missing after cold Repair: %v", err)
	}
}

// ── Drift from file deletion ──────────────────────────────────────────────────

// TestClaude_Verify_MissingFileReportedAsDrift verifies that Verify treats a
// deleted managed file as drift (not a hard error), and that the drift is
// repaired by a subsequent Repair call.
func TestClaude_Verify_MissingFileReportedAsDrift(t *testing.T) {
	ctx := context.Background()
	home, installer, store, _ := newLifecycleFixture(t)

	receipt := doInstall(t, ctx, installer, store)

	agentFile := filepath.Join(home, ".claude", "agents", "vedox-doc.md")
	if err := os.Remove(agentFile); err != nil {
		t.Fatalf("remove agent file: %v", err)
	}

	v, err := installer.Verify(ctx, receipt)
	if err != nil {
		t.Fatalf("Verify after deletion: %v", err)
	}
	if !v.Drift {
		t.Error("Verify: missing file should be reported as drift")
	}
	if v.Healthy {
		t.Error("Verify: missing file should mark install as unhealthy")
	}

	// Repair must restore the file.
	if err := installer.Repair(ctx); err != nil {
		t.Fatalf("Repair after deletion: %v", err)
	}
	if _, err := os.Stat(agentFile); err != nil {
		t.Errorf("agent file still missing after Repair: %v", err)
	}
}
