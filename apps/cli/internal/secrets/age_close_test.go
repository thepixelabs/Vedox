package secrets_test

// age_close_test.go — FIX-SEC-05 regression coverage for AgeStore.Close.
//
// Covers two behaviours:
//  1. Close unsets the bare VEDOX_AGE_PASSPHRASE env var so child processes
//     forked after daemon shutdown cannot inherit the raw passphrase via
//     /proc/<pid>/environ (the weakest passphrase tier, per §2.2 of the
//     design doc).
//  2. Close is safe when the env var was never set (passphrase-file path).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vedox/vedox/internal/secrets"
)

// TestAgeStore_Close_UnsetsPassphraseEnv verifies FIX-SEC-05: Close removes
// VEDOX_AGE_PASSPHRASE from the process environment so child processes forked
// after shutdown do not inherit the raw passphrase. Before the fix, the env
// var remained set for the lifetime of the daemon process.
func TestAgeStore_Close_UnsetsPassphraseEnv(t *testing.T) {
	secrets.LowerScryptWorkFactorForTests(t, 10)
	dir := t.TempDir()
	// Use the bare env-var passphrase path so Close has something to unset.
	os.Unsetenv("VEDOX_AGE_PASSPHRASE_FILE")
	t.Setenv("VEDOX_AGE_PASSPHRASE", "close-test-passphrase")

	s := secrets.NewAgeStore(dir)
	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Sanity: env var is set right after Open.
	if v := os.Getenv("VEDOX_AGE_PASSPHRASE"); v != "close-test-passphrase" {
		t.Fatalf("env var lost before Close: %q", v)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// After Close the raw passphrase env var must be gone.
	if _, ok := os.LookupEnv("VEDOX_AGE_PASSPHRASE"); ok {
		t.Fatalf("VEDOX_AGE_PASSPHRASE still set after Close")
	}

	// Idempotent: a second Close with the env already gone must not panic.
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestAgeStore_Close_NoEnvVar verifies Close is safe when the env var was
// never set (passphrase-file or TTY path). Guards against a regression where
// os.Unsetenv is called unconditionally and affects unrelated env vars.
func TestAgeStore_Close_NoEnvVar(t *testing.T) {
	secrets.LowerScryptWorkFactorForTests(t, 10)
	dir := t.TempDir()

	ppFile := filepath.Join(dir, "passphrase.txt")
	if err := os.WriteFile(ppFile, []byte("from-file\n"), 0o600); err != nil {
		t.Fatalf("write passphrase file: %v", err)
	}
	t.Setenv("VEDOX_AGE_PASSPHRASE_FILE", ppFile)
	// Explicitly unset so LookupEnv returns ok=false.
	os.Unsetenv("VEDOX_AGE_PASSPHRASE")

	s := secrets.NewAgeStore(dir)
	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// File env var is a path, not a secret — Close must NOT touch it.
	if v := os.Getenv("VEDOX_AGE_PASSPHRASE_FILE"); v != ppFile {
		t.Fatalf("VEDOX_AGE_PASSPHRASE_FILE was modified by Close: got %q, want %q", v, ppFile)
	}
}
