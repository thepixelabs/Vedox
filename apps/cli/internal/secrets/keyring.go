package secrets

import (
	"sync"

	"github.com/zalando/go-keyring"
)

// keychainService is the libsecret / Keychain service name under which Vedox
// stores secrets. Changing this will orphan all existing keychain entries.
const keychainService = "vedox-agent"

// KeyringStore implements SecretStore on top of github.com/zalando/go-keyring.
// On macOS this calls into the system Keychain; on Linux it talks to the D-Bus
// Secret Service (GNOME Keyring / KWallet). On Windows it uses the Credential
// Manager (not a v2 target but supported by go-keyring).
//
// KeyringStore carries no writable fields beyond the mutex — the backing store
// is entirely managed by go-keyring. Thread safety is provided by go-keyring
// itself for individual calls; the mu here guards the List() multi-step walk.
type KeyringStore struct {
	mu      sync.Mutex
	service string
}

// NewKeyringStore returns a KeyringStore using the default Vedox service name.
func NewKeyringStore() *KeyringStore {
	return &KeyringStore{service: keychainService}
}

// newKeyringStoreForService is used in tests to inject a distinct service name
// so test keys cannot pollute the real keychain (or each other when parallelism
// is exercised).
func newKeyringStoreForService(service string) *KeyringStore {
	return &KeyringStore{service: service}
}

// Get retrieves the secret stored under key. Returns ErrNotFound when the key
// does not exist in the keychain.
func (s *KeyringStore) Get(key string) ([]byte, error) {
	val, err := keyring.Get(s.service, key)
	if err == keyring.ErrNotFound {
		return nil, &ErrNotFound{Key: key}
	}
	if err != nil {
		return nil, err
	}
	return []byte(val), nil
}

// Put writes value to the keychain under key. go-keyring.Set replaces any
// existing entry, so this is idempotent for the same (service, key) pair.
func (s *KeyringStore) Put(key string, value []byte) error {
	return keyring.Set(s.service, key, string(value))
}

// Delete removes the keychain entry for key.
func (s *KeyringStore) Delete(key string) error {
	err := keyring.Delete(s.service, key)
	if err == keyring.ErrNotFound {
		return &ErrNotFound{Key: key}
	}
	return err
}

// List is not natively supported by go-keyring's API (the OS keychain APIs do
// not provide an efficient "list all under service" primitive without CGO on
// macOS). KeyringStore maintains an in-process key registry in memory rather
// than enumerating the keychain at runtime.
//
// This means List only returns keys that were Put during the current process
// lifetime. It is suitable for the agentauth use-case because the KeyStore
// holds the authoritative key list in its in-memory map and calls Put/Get/Delete
// directly rather than relying on List for discovery.
//
// For a fresh process starting up, callers should populate the key list from
// their own metadata source (e.g. agent-keys.json) and probe Get for each.
func (s *KeyringStore) List() ([]string, error) {
	// go-keyring does not expose enumeration. Return empty — see godoc above.
	return []string{}, nil
}
