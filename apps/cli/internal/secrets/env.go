package secrets

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

// EnvStore implements SecretStore by reading a single HMAC secret from
// environment variables. It is the lowest tier in the fallback chain — used
// only for developer workflows and container deployments where no other secret
// delivery mechanism is configured.
//
// Priority order:
//  1. VEDOX_HMAC_KEY_FILE — path to a file containing a hex-encoded 32-byte
//     secret (64 hex characters). Recommended for container deployments via
//     Docker secrets.
//  2. VEDOX_HMAC_KEY — raw hex-encoded 32-byte secret in an environment
//     variable. Weakest option: leaks to /proc/<pid>/environ, docker inspect,
//     shell history, and child processes.
//
// EnvStore covers exactly one key — the single editor-class HMAC key. It does
// NOT support multi-key stores (for agent-issued keys, the age or keyring
// backends are required). Get/List return only the one built-in key ID
// ("editor-default"). Put and Delete return ErrReadOnly because EnvStore is
// read-only from the process's perspective.
//
// Security warnings:
//   - VEDOX_HMAC_KEY emits a WARN log at Open time.
//   - vedox doctor (separate tool) reports EnvStore as an error (red).
//
// Thread safety: EnvStore is immutable after Open; all methods are safe for
// concurrent use.
type EnvStore struct {
	mu     sync.RWMutex
	keyID  string // fixed key ID for the single editor-class key
	secret []byte // hex-encoded secret bytes as loaded from env
}

// editorDefaultKeyID is the fixed key identifier for the single editor-class
// HMAC key delivered via environment variables.
const editorDefaultKeyID = "editor-default"

// ErrReadOnly is returned by Put and Delete on read-only stores (EnvStore).
type ErrReadOnly struct{}

func (e *ErrReadOnly) Error() string {
	return "secret store is read-only; update the VEDOX_HMAC_KEY_FILE or VEDOX_HMAC_KEY environment variable"
}

// NewEnvStore returns an EnvStore. Call Open before any other method.
func NewEnvStore() *EnvStore {
	return &EnvStore{keyID: editorDefaultKeyID}
}

// Open reads the HMAC secret from the environment using the priority order
// above. Returns an error if neither source is configured, if the key file
// cannot be read, or if the hex secret has an invalid length.
func (s *EnvStore) Open() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Priority 1: file-based secret (preferred for containers).
	if path := os.Getenv("VEDOX_HMAC_KEY_FILE"); path != "" {
		raw, err := readHexSecretFromFile(path)
		if err != nil {
			return err
		}
		s.secret = raw
		return nil
	}

	// Priority 2: bare env var (dev only — warn loudly).
	if raw := os.Getenv("VEDOX_HMAC_KEY"); raw != "" {
		slog.Warn("SECURITY WARNING: using bare VEDOX_HMAC_KEY env var; " +
			"this is insecure and should not be used in production. " +
			"Use VEDOX_HMAC_KEY_FILE or upgrade to keychain / age-file storage.")
		secret, err := validateHexSecret(raw)
		if err != nil {
			return fmt.Errorf("VEDOX_HMAC_KEY: %w", err)
		}
		s.secret = secret
		return nil
	}

	return fmt.Errorf("EnvStore: neither VEDOX_HMAC_KEY_FILE nor VEDOX_HMAC_KEY is set")
}

// Get returns the HMAC secret for key. Only editorDefaultKeyID ("editor-default")
// is stored; all other keys return ErrNotFound.
func (s *EnvStore) Get(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.secret == nil {
		return nil, fmt.Errorf("EnvStore: Open has not been called")
	}
	if key != s.keyID {
		return nil, &ErrNotFound{Key: key}
	}
	// Return a copy — callers must not be able to mutate s.secret.
	out := make([]byte, len(s.secret))
	copy(out, s.secret)
	return out, nil
}

// Put always returns ErrReadOnly — EnvStore is a read-only view of the
// environment. Use a KeyringStore or AgeStore to create writable storage.
func (s *EnvStore) Put(_ string, _ []byte) error {
	return &ErrReadOnly{}
}

// Delete always returns ErrReadOnly — see Put.
func (s *EnvStore) Delete(_ string) error {
	return &ErrReadOnly{}
}

// List returns [editorDefaultKeyID] if the store was opened successfully,
// or an empty slice if Open has not been called.
func (s *EnvStore) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.secret == nil {
		return []string{}, nil
	}
	return []string{s.keyID}, nil
}

// readHexSecretFromFile reads up to 4096 bytes from path, strips trailing
// whitespace, and validates that the result is a 64-character hex string
// (32 bytes).
func readHexSecretFromFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open VEDOX_HMAC_KEY_FILE %s: %w", path, err)
	}
	defer f.Close()

	buf, err := io.ReadAll(io.LimitReader(f, 4096))
	if err != nil {
		return nil, fmt.Errorf("read VEDOX_HMAC_KEY_FILE %s: %w", path, err)
	}

	raw := string(bytes.TrimRight(buf, "\n\r "))
	return validateHexSecret(raw)
}

// validateHexSecret ensures that raw is a valid 64-character lowercase hex
// string (representing a 32-byte HMAC key) and returns the raw string bytes.
func validateHexSecret(raw string) ([]byte, error) {
	decoded, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("HMAC secret must be 32 bytes (64 hex chars), got %d bytes", len(decoded))
	}
	// Return the original hex-encoded form, not the decoded bytes, to match
	// the agentauth convention where secrets are stored and passed as hex strings.
	return []byte(raw), nil
}
