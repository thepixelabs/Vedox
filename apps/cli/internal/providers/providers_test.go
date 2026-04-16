package providers_test

// providers_test.go — tests for shared types and ReceiptStore.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vedox/vedox/internal/providers"
)

// ── ReceiptStore tests ───────────────────────────────────────────────────────

func TestReceiptStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := providers.NewReceiptStore(dir)
	if err != nil {
		t.Fatalf("NewReceiptStore: %v", err)
	}

	receipt := &providers.InstallReceipt{
		Provider:    providers.ProviderClaude,
		Version:     "2.0",
		SchemaHash:  "abc123",
		AuthKeyID:   "key-uuid-1234",
		DaemonURL:   "http://127.0.0.1:5150",
		FileHashes:  map[string]string{"/home/user/.claude/agents/vedox-doc.md": "deadbeef"},
		InstalledAt: time.Now().UTC().Truncate(time.Second),
	}

	if err := store.Save(receipt); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := store.Load(providers.ProviderClaude)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil for a saved receipt")
	}
	if loaded.Provider != receipt.Provider {
		t.Errorf("Provider: got %q, want %q", loaded.Provider, receipt.Provider)
	}
	if loaded.AuthKeyID != receipt.AuthKeyID {
		t.Errorf("AuthKeyID: got %q, want %q", loaded.AuthKeyID, receipt.AuthKeyID)
	}
	if loaded.Version != receipt.Version {
		t.Errorf("Version: got %q, want %q", loaded.Version, receipt.Version)
	}
	if len(loaded.FileHashes) != 1 {
		t.Errorf("FileHashes: got %d entries, want 1", len(loaded.FileHashes))
	}
}

func TestReceiptStore_LoadMissing(t *testing.T) {
	dir := t.TempDir()
	store, err := providers.NewReceiptStore(dir)
	if err != nil {
		t.Fatalf("NewReceiptStore: %v", err)
	}

	r, err := store.Load(providers.ProviderClaude)
	if err != nil {
		t.Fatalf("Load missing: unexpected error: %v", err)
	}
	if r != nil {
		t.Errorf("Load missing: expected nil, got %+v", r)
	}
}

func TestReceiptStore_List(t *testing.T) {
	dir := t.TempDir()
	store, err := providers.NewReceiptStore(dir)
	if err != nil {
		t.Fatalf("NewReceiptStore: %v", err)
	}

	// Save two receipts.
	for _, pid := range []providers.ProviderID{providers.ProviderClaude, providers.ProviderCodex} {
		r := &providers.InstallReceipt{
			Provider:    pid,
			Version:     "2.0",
			AuthKeyID:   "key-" + string(pid),
			FileHashes:  map[string]string{},
			InstalledAt: time.Now().UTC(),
		}
		if err := store.Save(r); err != nil {
			t.Fatalf("Save %s: %v", pid, err)
		}
	}

	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List: got %d receipts, want 2", len(list))
	}
}

func TestReceiptStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store, err := providers.NewReceiptStore(dir)
	if err != nil {
		t.Fatalf("NewReceiptStore: %v", err)
	}

	r := &providers.InstallReceipt{
		Provider:    providers.ProviderClaude,
		Version:     "2.0",
		AuthKeyID:   "key-1",
		FileHashes:  map[string]string{},
		InstalledAt: time.Now().UTC(),
	}
	if err := store.Save(r); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Delete(providers.ProviderClaude); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	loaded, err := store.Load(providers.ProviderClaude)
	if err != nil {
		t.Fatalf("Load after delete: %v", err)
	}
	if loaded != nil {
		t.Errorf("Load after delete: expected nil, got %+v", loaded)
	}
}

func TestReceiptStore_Delete_Idempotent(t *testing.T) {
	dir := t.TempDir()
	store, err := providers.NewReceiptStore(dir)
	if err != nil {
		t.Fatalf("NewReceiptStore: %v", err)
	}
	// Delete on a provider that was never installed should not error.
	if err := store.Delete(providers.ProviderClaude); err != nil {
		t.Errorf("Delete non-existent: %v", err)
	}
}

func TestReceiptStore_CorruptJSON(t *testing.T) {
	dir := t.TempDir()
	store, err := providers.NewReceiptStore(dir)
	if err != nil {
		t.Fatalf("NewReceiptStore: %v", err)
	}

	// Manually write a corrupt receipt file.
	receiptsDir := filepath.Join(dir, "install-receipts")
	if err := os.MkdirAll(receiptsDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(receiptsDir, "claude.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	_, err = store.Load(providers.ProviderClaude)
	if err == nil {
		t.Error("Load corrupt JSON: expected error, got nil")
	}
}

// TestInstallReceiptJSON verifies the JSON field names are stable — changing
// them would break existing receipt files.
func TestInstallReceiptJSON(t *testing.T) {
	r := &providers.InstallReceipt{
		Provider:    providers.ProviderClaude,
		Version:     "2.0",
		SchemaHash:  "sha",
		AuthKeyID:   "kid",
		DaemonURL:   "http://127.0.0.1:5150",
		FileHashes:  map[string]string{"path": "hash"},
		InstalledAt: time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(data)
	for _, want := range []string{`"provider"`, `"version"`, `"schemaHash"`, `"authKeyID"`, `"daemonURL"`, `"fileHashes"`, `"installedAt"`} {
		if !containsStr(s, want) {
			t.Errorf("JSON missing field %s", want)
		}
	}
}

func containsStr(s, sub string) bool {
	return strings.Contains(s, sub)
}
