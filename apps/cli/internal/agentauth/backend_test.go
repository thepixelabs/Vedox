package agentauth

// backend_test.go — FIX-QA-05
//
// Tests for the pluggable SecretStore backend path:
//   - LoadKeyStoreWithBackend + InMemoryStore round-trip (IssueKey → getSecret)
//   - getSecret wraps a backend Get error as VDX-302
//   - LoadKeyStoreWithBackend with a corrupt metadata file returns a clean error
//     (no panic)
//
// None of these tests touch the real OS keychain. All secret operations go
// through secrets.NewInMemoryStore().

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	vdxerr "github.com/vedox/vedox/internal/errors"
	"github.com/vedox/vedox/internal/secrets"
)

// ---------------------------------------------------------------------------
// Test 1: LoadKeyStoreWithBackend + InMemoryStore round-trip
// ---------------------------------------------------------------------------

// TestLoadKeyStoreWithBackend_InMemRoundtrip verifies that:
//
//  1. LoadKeyStoreWithBackend wires the InMemoryStore correctly.
//  2. IssueKey stores the secret in the backend via setSecret.
//  3. getSecret retrieves the same secret bytes back via the backend.
//
// The test reads the secret back through the unexported getSecret method so
// it exercises the pluggable path end-to-end without involving the OS keychain.
func TestLoadKeyStoreWithBackend_InMemRoundtrip(t *testing.T) {
	store := secrets.NewInMemoryStore()
	ks, err := LoadKeyStoreWithBackend(t.TempDir(), store)
	if err != nil {
		t.Fatalf("LoadKeyStoreWithBackend: %v", err)
	}

	id, issuedSecret, err := ks.IssueKey("roundtrip-agent", "proj-a", "/api/")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}

	// getSecret is unexported but accessible in this white-box test file.
	retrieved, err := ks.getSecret(id)
	if err != nil {
		t.Fatalf("getSecret: %v", err)
	}

	if retrieved != issuedSecret {
		t.Errorf("getSecret returned %q, want %q", retrieved, issuedSecret)
	}

	// Sanity-check: the secret was stored in the InMemoryStore, not the keychain.
	raw, err := store.Get(id)
	if err != nil {
		t.Fatalf("InMemoryStore.Get: %v", err)
	}
	if string(raw) != issuedSecret {
		t.Errorf("InMemoryStore has %q, want %q", string(raw), issuedSecret)
	}
}

// ---------------------------------------------------------------------------
// erroring secret store — test double for Test 2
// ---------------------------------------------------------------------------

// erroringOnGetStore is a SecretStore that accepts Puts (so IssueKey can
// pre-seed it) but returns a hard I/O-style error from every Get call.
// This simulates a backend becoming unavailable between write and read
// (e.g. a locked age file, a crashed Secret Service daemon).
type erroringOnGetStore struct {
	inner *secrets.InMemoryStore
	getErr error
}

func (s *erroringOnGetStore) Get(key string) ([]byte, error) {
	return nil, s.getErr
}
func (s *erroringOnGetStore) Put(key string, value []byte) error {
	return s.inner.Put(key, value)
}
func (s *erroringOnGetStore) Delete(key string) error {
	return s.inner.Delete(key)
}
func (s *erroringOnGetStore) List() ([]string, error) {
	return s.inner.List()
}

// ---------------------------------------------------------------------------
// Test 2: backend returns error on Get → VDX-302 propagates cleanly
// ---------------------------------------------------------------------------

// TestGetSecret_BackendGetError verifies that when the pluggable SecretStore
// returns a non-ErrNotFound error from Get, getSecret wraps it as VDX-302
// (ErrKeychainUnavailable) and never panics.
//
// This exercises the error branch in getSecret:
//
//	if err != nil {
//	    return "", vdxerr.Wrap(vdxerr.ErrKeychainUnavailable, …, err)
//	}
func TestGetSecret_BackendGetError(t *testing.T) {
	backendFailure := fmt.Errorf("simulated age backend I/O failure")
	estore := &erroringOnGetStore{
		inner:  secrets.NewInMemoryStore(),
		getErr: backendFailure,
	}

	ks, err := LoadKeyStoreWithBackend(t.TempDir(), estore)
	if err != nil {
		t.Fatalf("LoadKeyStoreWithBackend: %v", err)
	}

	// Pre-seed the inner store so IssueKey (which calls Put) succeeds and a key
	// ID exists. The subsequent getSecret call will hit the erroringOnGetStore.Get
	// path and must return a VDX-302 error.
	id, _, err := ks.IssueKey("error-probe-agent", "", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}

	_, getErr := ks.getSecret(id)
	if getErr == nil {
		t.Fatal("expected an error from getSecret when backend Get fails, got nil")
	}

	// The error must be a *vdxerr.VedoxError with code VDX-302.
	var vdxErr *vdxerr.VedoxError
	if !errors.As(getErr, &vdxErr) {
		t.Fatalf("expected *vdxerr.VedoxError, got %T: %v", getErr, getErr)
	}
	if vdxErr.Code != vdxerr.ErrKeychainUnavailable {
		t.Errorf("want VDX-302, got %s", vdxErr.Code)
	}

	// The underlying cause must be reachable via Unwrap for debug logging.
	if !errors.Is(getErr, backendFailure) {
		t.Errorf("expected underlying cause %v to be reachable via errors.Is, got chain: %v", backendFailure, getErr)
	}
}

// ---------------------------------------------------------------------------
// Test 3: corrupt metadata file returns clean VDX-style error (no panic)
// ---------------------------------------------------------------------------

// TestLoadKeyStoreWithBackend_CorruptMetadataFile verifies that when
// agent-keys.json contains invalid JSON, LoadKeyStoreWithBackend returns a
// descriptive error that:
//   - is non-nil (the corruption is not silently ignored)
//   - does not panic
//   - contains the word "parse" (matches the fmt.Errorf format string in the
//     implementation, confirming the right branch was taken)
//
// This is the pluggable-backend analogue of TestLoadKeyStore_CorruptJSON.
func TestLoadKeyStoreWithBackend_CorruptMetadataFile(t *testing.T) {
	dir := t.TempDir()
	vedoxDir := dir + "/.vedox"
	if err := os.MkdirAll(vedoxDir, 0o700); err != nil {
		t.Fatalf("mkdir .vedox: %v", err)
	}
	if err := os.WriteFile(vedoxDir+"/agent-keys.json", []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write corrupt metadata: %v", err)
	}

	store := secrets.NewInMemoryStore()
	_, err := LoadKeyStoreWithBackend(dir, store)
	if err == nil {
		t.Fatal("expected error for corrupt agent-keys.json, got nil")
	}

	// The error message must identify the parse failure so operators know what
	// to fix. We do not assert the full string — just the distinguishing word.
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected error message to contain %q, got: %v", "parse", err)
	}
}
