// Package secrets provides the SecretStore abstraction used by Vedox to store
// HMAC secrets and other sensitive credentials across platforms and deployment
// environments.
//
// # Storage tiers (in descending preference)
//
//  1. OS keychain — macOS Keychain / Linux D-Bus Secret Service via go-keyring.
//     The default on interactive desktops. Secrets never touch disk.
//  2. age-encrypted file — ~/.vedox/secrets.age. Headless Linux, VPS, WSL2.
//     Passphrase delivery: VEDOX_AGE_PASSPHRASE_FILE → VEDOX_AGE_PASSPHRASE →
//     interactive TTY prompt → VDX-D04 error.
//  3. Env-file / bare env var — VEDOX_HMAC_KEY_FILE → VEDOX_HMAC_KEY. Dev and
//     container fallback. Emits security warnings at startup.
//
// AutoDetect selects the highest available tier automatically.
//
// Security invariants inherited from agentauth:
//   - Secrets are NEVER written to plaintext on disk.
//   - Secrets are NEVER logged, even at DEBUG level.
//   - Every write path is atomic (temp + fsync + rename for file backends).
package secrets

// SecretStore is the common interface all storage backends implement. Keys are
// opaque string identifiers (typically UUID-formatted key IDs from agentauth).
// Values are raw secret bytes — callers are responsible for encoding choices
// (agentauth uses hex-encoded 32-byte secrets).
//
// Implementations must be safe for concurrent use by multiple goroutines.
type SecretStore interface {
	// Get retrieves the secret for key. Returns ErrNotFound when the key does
	// not exist. Returns a non-nil error for all other failures (I/O, keychain
	// unavailable, decryption failure, etc.).
	Get(key string) ([]byte, error)

	// Put stores value under key. Overwrites any existing value. Implementations
	// must ensure a crash between write start and write completion cannot leave
	// a partially-written or corrupted store state.
	Put(key string, value []byte) error

	// Delete removes the secret for key. Returns ErrNotFound when the key does
	// not exist; callers may treat this as a no-op when idempotent deletion is
	// desired.
	Delete(key string) error

	// List returns all stored keys in unspecified order. The returned slice is
	// a copy — callers may mutate it freely.
	List() ([]string, error)
}

// ErrNotFound is returned by Get and Delete when the requested key does not
// exist in the store. Use errors.Is to check for this sentinel.
type ErrNotFound struct {
	Key string
}

func (e *ErrNotFound) Error() string {
	return "secret not found: " + e.Key
}

// IsNotFound returns true when err is (or wraps) ErrNotFound.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrNotFound)
	return ok
}
