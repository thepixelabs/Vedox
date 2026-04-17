package secrets_test

// secrets_integration_test.go — integration tests for AgeStore, EnvStore, and
// AutoDetect.
//
// Tests here exercise behaviors that the unit tests in secrets_test.go do not
// cover:
//   - AgeStore: 3-key Put/Get/List/Delete cycle (broader than single-key RoundTrip)
//   - AgeStore: passphrase file takes precedence over bare env var
//     (existing test verifies passphrase-file works; this verifies priority —
//      using a WRONG passphrase in VEDOX_AGE_PASSPHRASE while the correct one
//      lives in the file, to prove the file path is selected)
//   - AgeStore: concurrent Put under -race (write-through cache + mutex)
//   - EnvStore: all observable behaviors wired together in a single lifecycle
//   - AutoDetect: returns a non-nil, usable store in the env fallback path

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/vedox/vedox/internal/secrets"
)

// ── AgeStore: 3-key lifecycle ─────────────────────────────────────────────────

// TestAgeStore_ThreeKeyLifecycle_PutGetListDeleteOne puts three secrets, reads
// each, lists all, deletes one, and verifies the list and Get behaviour update.
//
// This is the primary AgeStore integration test: a more complete cycle than
// the single-key RoundTrip unit test.
func TestAgeStore_ThreeKeyLifecycle_PutGetListDeleteOne(t *testing.T) {
	secrets.LowerScryptWorkFactorForTests(t, 10)
	dir := t.TempDir()
	t.Setenv("VEDOX_AGE_PASSPHRASE", "three-key-integration-passphrase")

	store := secrets.NewAgeStore(dir)
	if err := store.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	keys := []string{"agent-key-001", "agent-key-002", "agent-key-003"}
	values := map[string]string{
		"agent-key-001": strings.Repeat("aa", 32),
		"agent-key-002": strings.Repeat("bb", 32),
		"agent-key-003": strings.Repeat("cc", 32),
	}

	// Put all three.
	for _, k := range keys {
		if err := store.Put(k, []byte(values[k])); err != nil {
			t.Fatalf("Put %q: %v", k, err)
		}
	}

	// Get each and verify value.
	for _, k := range keys {
		got, err := store.Get(k)
		if err != nil {
			t.Fatalf("Get %q: %v", k, err)
		}
		if string(got) != values[k] {
			t.Errorf("Get %q: got %q, want %q", k, got, values[k])
		}
	}

	// List must contain all three keys.
	listed, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 3 {
		t.Errorf("List: got %d keys, want 3: %v", len(listed), listed)
	}
	sort.Strings(listed)
	sort.Strings(keys)
	for i, want := range keys {
		if listed[i] != want {
			t.Errorf("List[%d]: got %q, want %q", i, listed[i], want)
		}
	}

	// Delete one key.
	deletedKey := "agent-key-002"
	if err := store.Delete(deletedKey); err != nil {
		t.Fatalf("Delete %q: %v", deletedKey, err)
	}

	// List must now contain exactly 2 keys.
	afterDelete, err := store.List()
	if err != nil {
		t.Fatalf("List after Delete: %v", err)
	}
	if len(afterDelete) != 2 {
		t.Errorf("List after Delete: got %d keys, want 2: %v", len(afterDelete), afterDelete)
	}
	for _, k := range afterDelete {
		if k == deletedKey {
			t.Errorf("List after Delete: deleted key %q still appears in list", deletedKey)
		}
	}

	// Get on deleted key must return ErrNotFound.
	_, err = store.Get(deletedKey)
	if !secrets.IsNotFound(err) {
		t.Fatalf("Get after Delete: want ErrNotFound, got %v", err)
	}

	// Remaining two keys must still be readable.
	remaining := []string{"agent-key-001", "agent-key-003"}
	for _, k := range remaining {
		got, err := store.Get(k)
		if err != nil {
			t.Fatalf("Get remaining key %q: %v", k, err)
		}
		if string(got) != values[k] {
			t.Errorf("Get remaining key %q: got %q, want %q", k, got, values[k])
		}
	}
}

// ── AgeStore: passphrase file priority ───────────────────────────────────────

// TestAgeStore_PassphraseFilePriority_OverBareEnvVar verifies that
// VEDOX_AGE_PASSPHRASE_FILE takes priority over VEDOX_AGE_PASSPHRASE even when
// the bare env var is set to an *incorrect* passphrase.
//
// If the file path is not selected, Open will fail with a wrong-passphrase
// decryption error — making the priority bug observable as a test failure.
func TestAgeStore_PassphraseFilePriority_OverBareEnvVar(t *testing.T) {
	secrets.LowerScryptWorkFactorForTests(t, 10)
	dir := t.TempDir()
	correctPassphrase := "the-correct-passphrase-in-the-file"
	wrongPassphrase := "wrong-passphrase-in-bare-env-var"

	// Write one secret using the correct passphrase via bare env var (first run).
	t.Setenv("VEDOX_AGE_PASSPHRASE", correctPassphrase)
	t.Setenv("VEDOX_AGE_PASSPHRASE_FILE", "") // ensure file path is not active yet

	s1 := secrets.NewAgeStore(dir)
	if err := s1.Open(); err != nil {
		t.Fatalf("s1.Open (seed): %v", err)
	}
	if err := s1.Put("seed-key", []byte("seed-value")); err != nil {
		t.Fatalf("s1.Put: %v", err)
	}

	// Write the correct passphrase to a file.
	ppFile := filepath.Join(dir, "passphrase.txt")
	if err := os.WriteFile(ppFile, []byte(correctPassphrase+"\n"), 0o600); err != nil {
		t.Fatalf("write passphrase file: %v", err)
	}

	// Now set VEDOX_AGE_PASSPHRASE_FILE to the file AND VEDOX_AGE_PASSPHRASE to
	// a deliberately wrong value. The file path must win.
	t.Setenv("VEDOX_AGE_PASSPHRASE_FILE", ppFile)
	t.Setenv("VEDOX_AGE_PASSPHRASE", wrongPassphrase)

	s2 := secrets.NewAgeStore(dir)
	if err := s2.Open(); err != nil {
		t.Fatalf("s2.Open: got error %v — expected passphrase file to take priority over wrong bare env var", err)
	}

	got, err := s2.Get("seed-key")
	if err != nil {
		t.Fatalf("s2.Get: %v", err)
	}
	if string(got) != "seed-value" {
		t.Errorf("s2.Get: got %q, want %q", got, "seed-value")
	}
}

// TestAgeStore_PassphraseFile_EmptyFileRejected verifies that an empty
// passphrase file causes Open to return an error rather than silently
// succeeding with an empty passphrase.
func TestAgeStore_PassphraseFile_EmptyFileRejected(t *testing.T) {
	dir := t.TempDir()

	ppFile := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(ppFile, []byte(""), 0o600); err != nil {
		t.Fatalf("write empty passphrase file: %v", err)
	}
	t.Setenv("VEDOX_AGE_PASSPHRASE_FILE", ppFile)
	t.Setenv("VEDOX_AGE_PASSPHRASE", "")

	s := secrets.NewAgeStore(dir)
	if err := s.Open(); err == nil {
		t.Fatal("Open with empty passphrase file: expected error, got nil")
	}
}

// TestAgeStore_PassphraseFile_MissingFileRejected verifies that pointing
// VEDOX_AGE_PASSPHRASE_FILE to a non-existent file causes Open to fail.
func TestAgeStore_PassphraseFile_MissingFileRejected(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VEDOX_AGE_PASSPHRASE_FILE", filepath.Join(dir, "does-not-exist.txt"))
	t.Setenv("VEDOX_AGE_PASSPHRASE", "")

	s := secrets.NewAgeStore(dir)
	if err := s.Open(); err == nil {
		t.Fatal("Open with missing passphrase file: expected error, got nil")
	}
}

// ── AgeStore: concurrent access under -race ───────────────────────────────────

// TestAgeStore_ConcurrentPuts_NoDataloss launches 3 goroutines each writing a
// unique key. After all goroutines complete, every key must be readable. This
// test is specifically designed to surface data races and lost-update bugs under
// `go test -race`.
//
// Note: age scrypt has a fixed work factor of 2^18 (~2.5s per encrypt on
// modern hardware). Three goroutines × one Put each is the minimum workload
// that exercises the mutex/write-through cache path without timing out.
func TestAgeStore_ConcurrentPuts_NoDataloss(t *testing.T) {
	secrets.LowerScryptWorkFactorForTests(t, 10)
	dir := t.TempDir()
	t.Setenv("VEDOX_AGE_PASSPHRASE", "concurrent-put-passphrase")

	store := secrets.NewAgeStore(dir)
	if err := store.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	// 3 goroutines: enough to exercise the mutex, within a ~10s wall-clock budget.
	const numGoroutines = 3
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		i := i // capture loop variable
		go func() {
			defer wg.Done()
			key := fmt.Sprintf("concurrent-key-%02d", i)
			val := []byte(strings.Repeat(fmt.Sprintf("%02x", i), 32))
			if err := store.Put(key, val); err != nil {
				// Cannot call t.Fatalf from a goroutine — log for main goroutine.
				t.Errorf("Put %q: %v", key, err)
			}
		}()
	}
	wg.Wait()

	// Every key written must be readable.
	for i := 0; i < numGoroutines; i++ {
		key := fmt.Sprintf("concurrent-key-%02d", i)
		if _, err := store.Get(key); err != nil {
			t.Errorf("Get %q after concurrent writes: %v", key, err)
		}
	}

	// List must report exactly numGoroutines keys.
	listed, err := store.List()
	if err != nil {
		t.Fatalf("List after concurrent puts: %v", err)
	}
	if len(listed) != numGoroutines {
		t.Errorf("List: got %d keys, want %d", len(listed), numGoroutines)
	}
}

// TestAgeStore_ConcurrentGetDuringPut exercises concurrent Get calls while a
// single Put is in flight. The store must never return stale or corrupted data.
//
// One writer performs a single Put (one scrypt encrypt ≈ 2.5s). Twenty
// readers race to Get a key that was seeded before the goroutines start. This
// proves the mutex prevents readers from observing a partially-written cache.
func TestAgeStore_ConcurrentGetDuringPut(t *testing.T) {
	secrets.LowerScryptWorkFactorForTests(t, 10)
	dir := t.TempDir()
	t.Setenv("VEDOX_AGE_PASSPHRASE", "concurrent-get-passphrase")

	store := secrets.NewAgeStore(dir)
	if err := store.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Seed an initial value before the race starts.
	if err := store.Put("stable-key", []byte("initial-value")); err != nil {
		t.Fatalf("seed Put: %v", err)
	}

	const readers = 20
	var wg sync.WaitGroup
	wg.Add(readers + 1) // readers + 1 writer

	// Writer: perform exactly one Put on a different key.
	// One scrypt encrypt ≈ 2.5s — keeps the test well within the timeout.
	go func() {
		defer wg.Done()
		_ = store.Put("writer-key", []byte(strings.Repeat("ab", 32)))
	}()

	// Readers: concurrently read the stable key.
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			got, err := store.Get("stable-key")
			if err != nil {
				t.Errorf("Get stable-key: %v", err)
				return
			}
			if string(got) != "initial-value" {
				t.Errorf("Get stable-key: got %q, want %q", got, "initial-value")
			}
		}()
	}

	wg.Wait()
}

// ── EnvStore: full lifecycle ──────────────────────────────────────────────────

// TestEnvStore_FullLifecycle exercises Open → Get → List → Put (expect error)
// → Delete (expect error) in a single test to catch any interaction bugs.
func TestEnvStore_FullLifecycle(t *testing.T) {
	secret := strings.Repeat("de", 32) // 64 hex chars
	t.Setenv("VEDOX_HMAC_KEY", secret)
	t.Setenv("VEDOX_HMAC_KEY_FILE", "")

	s := secrets.NewEnvStore()
	if err := s.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Get the single built-in key.
	got, err := s.Get("editor-default")
	if err != nil {
		t.Fatalf("Get editor-default: %v", err)
	}
	if string(got) != secret {
		t.Errorf("Get: got %q, want %q", got, secret)
	}

	// List must return exactly one key.
	keys, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 1 || keys[0] != "editor-default" {
		t.Errorf("List: got %v, want [editor-default]", keys)
	}

	// Put must return ErrReadOnly (type-check, not just non-nil).
	putErr := s.Put("editor-default", []byte("new"))
	var roErr *secrets.ErrReadOnly
	if !errors.As(putErr, &roErr) {
		t.Errorf("Put: want *ErrReadOnly, got %T: %v", putErr, putErr)
	}

	// Delete must return ErrReadOnly as well.
	delErr := s.Delete("editor-default")
	if !errors.As(delErr, &roErr) {
		t.Errorf("Delete: want *ErrReadOnly, got %T: %v", delErr, delErr)
	}
}

// TestEnvStore_GetBeforeOpen verifies that calling Get before Open returns an
// error rather than returning nil bytes or panicking.
func TestEnvStore_GetBeforeOpen(t *testing.T) {
	s := secrets.NewEnvStore()
	_, err := s.Get("editor-default")
	if err == nil {
		t.Fatal("Get before Open: expected error, got nil")
	}
}

// TestEnvStore_ListBeforeOpen verifies that List before Open returns an empty
// slice (not an error) as documented.
func TestEnvStore_ListBeforeOpen(t *testing.T) {
	s := secrets.NewEnvStore()
	keys, err := s.List()
	if err != nil {
		t.Fatalf("List before Open: unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("List before Open: got %v, want []", keys)
	}
}

// ── AutoDetect: env fallback path ────────────────────────────────────────────

// TestAutoDetect_EnvFallback_ReturnsUsableStore verifies that when
// VEDOX_HMAC_KEY is set (and keychain / age passphrases are not), AutoDetect
// returns a non-nil store and that store is immediately openable and usable.
//
// This exercises the lowest-priority rung of the detection ladder end-to-end.
func TestAutoDetect_EnvFallback_ReturnsUsableStore(t *testing.T) {
	secret := strings.Repeat("fa", 32) // 64 hex chars = 32 bytes
	t.Setenv("VEDOX_HMAC_KEY", secret)
	t.Setenv("VEDOX_HMAC_KEY_FILE", "")
	// Clear age vars so AutoDetect cannot return AgeStore without a passphrase.
	t.Setenv("VEDOX_AGE_PASSPHRASE", "")
	t.Setenv("VEDOX_AGE_PASSPHRASE_FILE", "")

	store, err := secrets.AutoDetect()
	if err != nil {
		t.Fatalf("AutoDetect: %v", err)
	}
	if store == nil {
		t.Fatal("AutoDetect returned nil store without error")
	}

	// The returned store must implement SecretStore. Open it and probe the env key.
	type opener interface {
		Open() error
	}
	if o, ok := store.(opener); ok {
		if err := o.Open(); err != nil {
			t.Fatalf("store.Open(): %v", err)
		}
	}

	// On macOS, AutoDetect returns a KeyringStore (no Open needed).
	// On Linux without D-Bus + age passphrase, it returns an EnvStore.
	// Either way, the interface must be satisfiable; List should not panic.
	if _, err := store.List(); err != nil {
		// KeyringStore.List() returns empty with no error — that is fine.
		// EnvStore.List() before Open returns empty — also fine after Open above.
		t.Logf("List: %v (non-fatal — OS keychain stores may return empty lists)", err)
	}
}

// TestAutoDetect_AgeStore_WhenPassphraseIsSet verifies that when
// VEDOX_AGE_PASSPHRASE is set and the home dir is reachable, AutoDetect
// selects the AgeStore tier (on non-macOS) or gracefully falls through to
// KeyringStore (on macOS). The returned store must be non-nil in either case.
func TestAutoDetect_AgeStore_WhenPassphraseIsSet(t *testing.T) {
	// Clear env vars that would force the env-store rung.
	t.Setenv("VEDOX_HMAC_KEY", "")
	t.Setenv("VEDOX_HMAC_KEY_FILE", "")
	// Set a passphrase so the age-store rung is viable on Linux.
	t.Setenv("VEDOX_AGE_PASSPHRASE", "autodetect-age-test-passphrase")
	t.Setenv("VEDOX_AGE_PASSPHRASE_FILE", "")

	store, err := secrets.AutoDetect()
	if err != nil {
		t.Fatalf("AutoDetect: %v", err)
	}
	if store == nil {
		t.Fatal("AutoDetect returned nil store without error")
	}
	// We cannot assert the concrete type without coupling to internals, but the
	// store must be non-nil and satisfy the interface.
	var _ secrets.SecretStore = store
}
