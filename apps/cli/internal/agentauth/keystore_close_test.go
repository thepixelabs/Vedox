package agentauth

// keystore_close_test.go — FIX-SEC-05 wiring coverage for KeyStore.Close.
//
// The daemon shutdown path calls ks.Close() unconditionally so that any
// AgeStore backend can zero its passphrase and unset VEDOX_AGE_PASSPHRASE.
// These tests verify:
//  1. Close is a no-op (returns nil) when the legacy go-keyring backend is
//     used — store is nil and there is nothing to clean up.
//  2. Close dispatches through the io.Closer-style interface to the pluggable
//     SecretStore when one is wired, which is how AgeStore's passphrase is
//     cleared in production.

import "testing"

// TestKeyStore_Close_LegacyPathIsNoOp verifies Close returns nil when the
// KeyStore uses the legacy go-keyring backend (store == nil). The daemon
// shutdown path calls ks.Close() unconditionally, so a panic or error here
// would surface as a shutdown warning.
func TestKeyStore_Close_LegacyPathIsNoOp(t *testing.T) {
	ks := newTestStore(t)
	if err := ks.Close(); err != nil {
		t.Fatalf("Close on legacy keyring-backed store: %v", err)
	}
	// Idempotent: a second Close must also succeed.
	if err := ks.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// closingSecretStore is a fake SecretStore that counts Close calls. We use it
// to assert that KeyStore.Close dispatches through the optional Closer
// interface to backends that implement it (AgeStore in production).
type closingSecretStore struct {
	closeCount int
}

func (*closingSecretStore) Get(string) ([]byte, error) { return nil, nil }
func (*closingSecretStore) Put(string, []byte) error   { return nil }
func (*closingSecretStore) Delete(string) error        { return nil }
func (*closingSecretStore) List() ([]string, error)    { return nil, nil }
func (c *closingSecretStore) Close() error             { c.closeCount++; return nil }

// TestKeyStore_Close_DispatchesToBackend verifies that KeyStore.Close calls
// Close on the pluggable backend when it implements the Closer interface.
// This is the wiring the daemon shutdown path relies on for AgeStore.
func TestKeyStore_Close_DispatchesToBackend(t *testing.T) {
	cs := &closingSecretStore{}
	ks := NewKeyStoreWithBackend(t.TempDir(), cs)
	if err := ks.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if cs.closeCount != 1 {
		t.Fatalf("backend Close not invoked: count=%d", cs.closeCount)
	}
}
