package providers

// receipt.go — ReceiptStore persists install receipts to
// ~/.vedox/install-receipts/<provider>.json (one file per provider).
//
// The receipt file contains NO secrets — only public fields (key ID, paths,
// hashes, timestamps). Permissions are 0o600 so only the owning user can read.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const receiptsDirName = "install-receipts"

// ReceiptStore reads and writes install receipts for all providers.
type ReceiptStore struct {
	// vedoxDir is the user-global ~/.vedox directory.
	vedoxDir string
}

// NewReceiptStore constructs a ReceiptStore rooted at vedoxDir. If vedoxDir
// is empty it defaults to ~/.vedox. The directory is created on first write.
func NewReceiptStore(vedoxDir string) (*ReceiptStore, error) {
	if vedoxDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("receipt store: resolve home: %w", err)
		}
		vedoxDir = filepath.Join(home, ".vedox")
	}
	return &ReceiptStore{vedoxDir: vedoxDir}, nil
}

// receiptsDir returns the path of the install-receipts subdirectory.
func (rs *ReceiptStore) receiptsDir() string {
	return filepath.Join(rs.vedoxDir, receiptsDirName)
}

// receiptPath returns the path for a given provider's receipt file.
func (rs *ReceiptStore) receiptPath(provider ProviderID) string {
	return filepath.Join(rs.receiptsDir(), string(provider)+".json")
}

// Save atomically writes receipt to disk. The parent directory is created
// with 0o700 if absent; the receipt file lands with 0o600.
func (rs *ReceiptStore) Save(receipt *InstallReceipt) error {
	if receipt == nil {
		return fmt.Errorf("receipt store: Save called with nil receipt")
	}
	dir := rs.receiptsDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("receipt store: mkdir %s: %w", dir, err)
	}

	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("receipt store: marshal: %w", err)
	}
	data = append(data, '\n')

	path := rs.receiptPath(receipt.Provider)

	// Use atomicFileWrite — boundary is the receiptsDir so the rename is
	// same-filesystem (POSIX atomic) and no symlink traversal is possible.
	if err := atomicFileWrite(dir, path, data, 0o700, 0o600); err != nil {
		return fmt.Errorf("receipt store: write %s: %w", path, err)
	}
	return nil
}

// Load reads the stored receipt for provider. Returns (nil, nil) if no
// receipt file exists yet (not installed). Returns an error for any I/O or
// parse failure.
func (rs *ReceiptStore) Load(provider ProviderID) (*InstallReceipt, error) {
	path := rs.receiptPath(provider)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("receipt store: read %s: %w", path, err)
	}
	var r InstallReceipt
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("receipt store: parse %s: %w", path, err)
	}
	return &r, nil
}

// List returns the receipts for all providers that have a receipt on disk.
// Providers with no receipt file are silently skipped. Parse errors are
// returned immediately.
func (rs *ReceiptStore) List() ([]*InstallReceipt, error) {
	var out []*InstallReceipt
	for _, pid := range []ProviderID{ProviderClaude, ProviderCodex, ProviderCopilot, ProviderGemini} {
		r, err := rs.Load(pid)
		if err != nil {
			return nil, err
		}
		if r != nil {
			out = append(out, r)
		}
	}
	return out, nil
}

// Delete removes the receipt file for provider. Returns nil if the file did
// not exist (idempotent).
func (rs *ReceiptStore) Delete(provider ProviderID) error {
	path := rs.receiptPath(provider)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("receipt store: delete %s: %w", path, err)
	}
	return nil
}
