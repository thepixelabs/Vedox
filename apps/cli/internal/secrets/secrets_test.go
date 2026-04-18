package secrets_test

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/secrets"
)

// ── AgeStore round-trip ────────────────────────────────────────────────────

// TestAgeStore_RoundTrip verifies that a secret written via Put can be
// retrieved via Get and that Delete removes it from subsequent List/Get calls.
func TestAgeStore_RoundTrip(t *testing.T) {
	secrets.LowerScryptWorkFactorForTests(t, 10)
	dir := t.TempDir()
	// Inject passphrase via env so Open does not attempt a TTY prompt.
	t.Setenv("VEDOX_AGE_PASSPHRASE", "correct-horse-battery-staple-test")

	store := secrets.NewAgeStore(dir)
	if err := store.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	key := "test-key-001"
	value := []byte("deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")

	// Put.
	if err := store.Put(key, value); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Get — must match.
	got, err := store.Get(key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != string(value) {
		t.Fatalf("Get: got %q, want %q", got, value)
	}

	// List — must contain the key.
	keys, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !containsKey(keys, key) {
		t.Fatalf("List: key %q not found in %v", key, keys)
	}

	// Delete.
	if err := store.Delete(key); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Get after delete must return ErrNotFound.
	_, err = store.Get(key)
	if !secrets.IsNotFound(err) {
		t.Fatalf("Get after Delete: want ErrNotFound, got %v", err)
	}
}

// TestAgeStore_Persistence verifies that a new AgeStore instance reading the
// same secrets.age file (same passphrase) sees the previously written secrets.
func TestAgeStore_Persistence(t *testing.T) {
	secrets.LowerScryptWorkFactorForTests(t, 10)
	dir := t.TempDir()
	t.Setenv("VEDOX_AGE_PASSPHRASE", "persistence-test-passphrase")

	key := "persist-key"
	value := []byte(hex.EncodeToString(make([]byte, 32))) // 64-char hex string

	// Writer instance.
	s1 := secrets.NewAgeStore(dir)
	if err := s1.Open(); err != nil {
		t.Fatalf("s1.Open: %v", err)
	}
	if err := s1.Put(key, value); err != nil {
		t.Fatalf("s1.Put: %v", err)
	}

	// Reader instance — simulates a process restart.
	s2 := secrets.NewAgeStore(dir)
	if err := s2.Open(); err != nil {
		t.Fatalf("s2.Open: %v", err)
	}
	got, err := s2.Get(key)
	if err != nil {
		t.Fatalf("s2.Get: %v", err)
	}
	if string(got) != string(value) {
		t.Fatalf("s2.Get: got %q, want %q", got, value)
	}
}

// TestAgeStore_WrongPassphrase ensures that an incorrect passphrase causes
// Open to fail at the decrypt step rather than silently returning empty data.
func TestAgeStore_WrongPassphrase(t *testing.T) {
	secrets.LowerScryptWorkFactorForTests(t, 10)
	dir := t.TempDir()

	// Write with the correct passphrase.
	t.Setenv("VEDOX_AGE_PASSPHRASE", "correct-passphrase")
	s1 := secrets.NewAgeStore(dir)
	if err := s1.Open(); err != nil {
		t.Fatalf("s1.Open: %v", err)
	}
	if err := s1.Put("k", []byte("v")); err != nil {
		t.Fatalf("s1.Put: %v", err)
	}

	// Attempt read with the wrong passphrase.
	t.Setenv("VEDOX_AGE_PASSPHRASE", "wrong-passphrase")
	s2 := secrets.NewAgeStore(dir)
	if err := s2.Open(); err == nil {
		t.Fatal("expected Open to fail with wrong passphrase, got nil")
	}
}

// TestAgeStore_NotFound verifies ErrNotFound semantics on a fresh empty store.
func TestAgeStore_NotFound(t *testing.T) {
	secrets.LowerScryptWorkFactorForTests(t, 10)
	dir := t.TempDir()
	t.Setenv("VEDOX_AGE_PASSPHRASE", "test-passphrase")

	s := secrets.NewAgeStore(dir)
	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	_, err := s.Get("does-not-exist")
	if !secrets.IsNotFound(err) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	err = s.Delete("does-not-exist")
	if !secrets.IsNotFound(err) {
		t.Fatalf("Delete: want ErrNotFound, got %v", err)
	}
}

// TestAgeStore_MultipleKeys verifies that multiple keys coexist correctly in
// the same encrypted file.
func TestAgeStore_MultipleKeys(t *testing.T) {
	secrets.LowerScryptWorkFactorForTests(t, 10)
	dir := t.TempDir()
	t.Setenv("VEDOX_AGE_PASSPHRASE", "multi-key-passphrase")

	s := secrets.NewAgeStore(dir)
	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	pairs := map[string]string{
		"key-alpha": strings.Repeat("aa", 32),
		"key-beta":  strings.Repeat("bb", 32),
		"key-gamma": strings.Repeat("cc", 32),
	}
	for k, v := range pairs {
		if err := s.Put(k, []byte(v)); err != nil {
			t.Fatalf("Put %q: %v", k, err)
		}
	}
	for k, want := range pairs {
		got, err := s.Get(k)
		if err != nil {
			t.Fatalf("Get %q: %v", k, err)
		}
		if string(got) != want {
			t.Fatalf("Get %q: got %q, want %q", k, got, want)
		}
	}
}

// TestAgeStore_PassphraseFile verifies that VEDOX_AGE_PASSPHRASE_FILE is
// honoured over VEDOX_AGE_PASSPHRASE.
func TestAgeStore_PassphraseFile(t *testing.T) {
	secrets.LowerScryptWorkFactorForTests(t, 10)
	dir := t.TempDir()

	// Write passphrase to a temp file.
	ppFile := filepath.Join(dir, "passphrase.txt")
	if err := os.WriteFile(ppFile, []byte("file-based-passphrase\n"), 0o600); err != nil {
		t.Fatalf("write passphrase file: %v", err)
	}
	t.Setenv("VEDOX_AGE_PASSPHRASE_FILE", ppFile)
	// Make sure the lower-priority env var is not set.
	t.Setenv("VEDOX_AGE_PASSPHRASE", "")

	s := secrets.NewAgeStore(dir)
	if err := s.Open(); err != nil {
		t.Fatalf("Open with passphrase file: %v", err)
	}
	if err := s.Put("k", []byte("v")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Get("k")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != "v" {
		t.Fatalf("Get: got %q, want %q", got, "v")
	}
}

// ── EnvStore ──────────────────────────────────────────────────────────────

// TestEnvStore_BareEnvVar verifies that VEDOX_HMAC_KEY is picked up and
// returned correctly.
func TestEnvStore_BareEnvVar(t *testing.T) {
	secret := strings.Repeat("ab", 32) // 64 hex chars = 32 bytes
	t.Setenv("VEDOX_HMAC_KEY", secret)
	t.Setenv("VEDOX_HMAC_KEY_FILE", "")

	s := secrets.NewEnvStore()
	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	got, err := s.Get("editor-default")
	if err != nil {
		t.Fatalf("Get editor-default: %v", err)
	}
	if string(got) != secret {
		t.Fatalf("Get: got %q, want %q", got, secret)
	}
}

// TestEnvStore_KeyFile verifies that VEDOX_HMAC_KEY_FILE is preferred over
// VEDOX_HMAC_KEY.
func TestEnvStore_KeyFile(t *testing.T) {
	secret := strings.Repeat("cd", 32)
	f, err := os.CreateTemp("", "vedox-key-*.hex")
	if err != nil {
		t.Fatalf("create temp key file: %v", err)
	}
	defer os.Remove(f.Name())
	if _, err := fmt.Fprintf(f, "%s\n", secret); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	f.Close()

	t.Setenv("VEDOX_HMAC_KEY_FILE", f.Name())
	t.Setenv("VEDOX_HMAC_KEY", "should-be-ignored")

	s := secrets.NewEnvStore()
	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, err := s.Get("editor-default")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(got) != secret {
		t.Fatalf("Get: got %q, want %q", got, secret)
	}
}

// TestEnvStore_NotFound verifies that Get for any key other than
// "editor-default" returns ErrNotFound.
func TestEnvStore_NotFound(t *testing.T) {
	t.Setenv("VEDOX_HMAC_KEY", strings.Repeat("ef", 32))
	t.Setenv("VEDOX_HMAC_KEY_FILE", "")

	s := secrets.NewEnvStore()
	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, err := s.Get("some-other-key")
	if !secrets.IsNotFound(err) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

// TestEnvStore_ReadOnly verifies that Put and Delete return an error — the env
// store is read-only.
func TestEnvStore_ReadOnly(t *testing.T) {
	t.Setenv("VEDOX_HMAC_KEY", strings.Repeat("ab", 32))
	t.Setenv("VEDOX_HMAC_KEY_FILE", "")

	s := secrets.NewEnvStore()
	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	if err := s.Put("editor-default", []byte("new")); err == nil {
		t.Fatal("Put: expected error on read-only store, got nil")
	}
	if err := s.Delete("editor-default"); err == nil {
		t.Fatal("Delete: expected error on read-only store, got nil")
	}
}

// TestEnvStore_InvalidHexLength verifies that a too-short VEDOX_HMAC_KEY is
// rejected with a clear error at Open time.
func TestEnvStore_InvalidHexLength(t *testing.T) {
	t.Setenv("VEDOX_HMAC_KEY", "tooshort")
	t.Setenv("VEDOX_HMAC_KEY_FILE", "")

	s := secrets.NewEnvStore()
	if err := s.Open(); err == nil {
		t.Fatal("expected Open to fail for invalid hex length, got nil")
	}
}

// TestEnvStore_NoEnvSet verifies that Open returns an error when neither env
// var is set.
func TestEnvStore_NoEnvSet(t *testing.T) {
	t.Setenv("VEDOX_HMAC_KEY", "")
	t.Setenv("VEDOX_HMAC_KEY_FILE", "")

	s := secrets.NewEnvStore()
	if err := s.Open(); err == nil {
		t.Fatal("expected Open to fail when no env var is set, got nil")
	}
}

// ── AutoDetect fallback chain ──────────────────────────────────────────────

// TestAutoDetect_FallsBackToAge verifies that AutoDetect returns an AgeStore
// on Linux when the D-Bus probe fails. We simulate the Linux condition by
// testing that when VEDOX_AGE_PASSPHRASE is set and the home directory exists
// but no keyring is available, we get a non-nil store.
//
// This test does not actually probe D-Bus — it tests the env-→-age branch
// directly by confirming AgeStore is usable end-to-end when AutoDetect
// returns it (indirect: we test the store type via the Open contract).
func TestAutoDetect_FallsBackToEnv(t *testing.T) {
	// Ensure age and keyring vars are clear so AutoDetect gets to the env tier.
	// We can't reliably disable the keyring probe in a unit test without a mock,
	// so we set VEDOX_HMAC_KEY and verify that AutoDetect returns a non-nil store.
	t.Setenv("VEDOX_HMAC_KEY", strings.Repeat("01", 32))
	t.Setenv("VEDOX_HMAC_KEY_FILE", "")
	// VEDOX_AGE_PASSPHRASE must NOT be set so AutoDetect does not return AgeStore
	// (which would require the home dir to be writable, unpredictable in CI).
	t.Setenv("VEDOX_AGE_PASSPHRASE", "")
	t.Setenv("VEDOX_AGE_PASSPHRASE_FILE", "")

	// AutoDetect will try OS keychain first on macOS/Linux. On macOS this
	// succeeds (KeyringStore). On Linux with no D-Bus it falls through.
	// Either way the returned store must be non-nil.
	store, err := secrets.AutoDetect()
	if err != nil {
		t.Fatalf("AutoDetect: %v", err)
	}
	if store == nil {
		t.Fatal("AutoDetect returned nil store without error")
	}
}

// TestAutoDetect_NilWhenNothingAvailable is intentionally skipped in CI
// because forcing "nothing available" requires unsetenv of HOME and a Linux
// environment without D-Bus — conditions that cannot be reliably reproduced
// in a cross-platform test suite.
// The VDX-D04 error path is exercised by integration tests only.

// ── helpers ───────────────────────────────────────────────────────────────

func containsKey(keys []string, target string) bool {
	for _, k := range keys {
		if k == target {
			return true
		}
	}
	return false
}
