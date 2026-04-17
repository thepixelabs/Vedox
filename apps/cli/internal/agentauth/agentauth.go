// Package agentauth implements HMAC-SHA256 signed API key authentication
// for autonomous AI agent writes into Vedox.
//
// Security invariants:
//   - HMAC secrets are NEVER written to disk. They live only in the OS
//     keychain (via github.com/zalando/go-keyring).
//   - The JSON metadata file under .vedox/agent-keys.json stores only public
//     fields (ID, Name, Project, PathPrefix, CreatedAt, Revoked). No secret,
//     no hash, no salt — nothing an attacker with disk access can use.
//   - IssueKey returns the plaintext secret exactly once, to its caller.
//     There is no API to retrieve it again. Lost secret = revoke + reissue.
//   - File writes are atomic (temp + fsync + rename) so a crash mid-write
//     cannot corrupt the metadata file.
//   - If the keychain is unavailable, operations fail with VDX-302 rather
//     than silently falling back to plaintext storage.
package agentauth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zalando/go-keyring"
	vdxerr "github.com/vedox/vedox/internal/errors"
	"github.com/vedox/vedox/internal/secrets"
)

// keychainService is the service name prefix used when storing secrets in
// the OS keychain. The full keychain key for a given API key is:
//
//	keychainService + "/" + key.ID
//
// Changing this constant will orphan all existing stored secrets.
const keychainService = "vedox-agent"

// metadataFileName is the filename (relative to .vedox/) where public API key
// metadata is persisted. The directory is created on first write if absent.
const metadataFileName = "agent-keys.json"

// APIKey is the public, serialisable representation of an agent API key.
// It deliberately contains NO secret, hash, or salt — the plaintext secret
// lives only in the OS keychain under keychainService/<ID>.
type APIKey struct {
	// ID is a v4 UUID generated at issuance time. It is the primary lookup
	// key for both the metadata file and the keychain entry.
	ID string `json:"id"`

	// Name is a human-friendly label chosen by the operator at `vedox keys add`
	// time (e.g. "claude-docs-agent"). It is not unique; only ID is.
	Name string `json:"name"`

	// Project restricts the key to a single workspace project. Empty string
	// means "any project" — only recommended for trusted local agents.
	Project string `json:"project,omitempty"`

	// PathPrefix further restricts the key to URL paths starting with this
	// string (e.g. "/api/projects/foo/docs/reference/"). Empty string means
	// "any path within the project scope".
	PathPrefix string `json:"pathPrefix,omitempty"`

	// CreatedAt records issuance time in UTC.
	CreatedAt time.Time `json:"createdAt"`

	// Revoked marks the key as tombstoned. Revoked keys are retained in the
	// metadata file for audit purposes but the RequireAgent middleware rejects
	// them and the keychain entry is deleted at revocation time.
	Revoked bool `json:"revoked,omitempty"`
}

// KeyStore manages the set of agent API keys for a workspace. It is the only
// component aware of how secrets are persisted (keychain) vs how metadata is
// persisted (JSON file under .vedox/).
//
// Thread safety: all exported methods hold an internal mutex. Concurrent
// IssueKey / RevokeKey / ListKeys calls are safe.
type KeyStore struct {
	mu sync.RWMutex

	// workspaceRoot is the absolute path of the Vedox workspace. The metadata
	// file lives at filepath.Join(workspaceRoot, ".vedox", metadataFileName).
	workspaceRoot string

	// keys is the in-memory view of the metadata file, indexed by ID for
	// O(1) lookup in the hot middleware path.
	keys map[string]APIKey

	// store is the pluggable secret backend. When nil, the legacy go-keyring
	// path is used (backward-compatible default). Inject a non-nil store via
	// NewKeyStoreWithBackend for age / env fallback support.
	store secrets.SecretStore
}

// NewKeyStore constructs an empty, un-persisted KeyStore backed by the legacy
// go-keyring path. Prefer LoadKeyStore for normal usage — it reads the
// existing metadata file if present.
//
// For the pluggable secrets-backend path, use NewKeyStoreWithBackend instead.
func NewKeyStore(workspaceRoot string) *KeyStore {
	return &KeyStore{
		workspaceRoot: workspaceRoot,
		keys:          make(map[string]APIKey),
		store:         nil, // nil → use go-keyring directly (legacy path)
	}
}

// NewKeyStoreWithBackend constructs an empty, un-persisted KeyStore that uses
// the supplied SecretStore for all secret read/write operations. This is the
// injection point for the age, env, and future keychain-helper backends.
//
// The caller is responsible for calling store.Open() (if required by the
// backend) before any IssueKey / RevokeKey / getSecret calls.
//
// Prefer LoadKeyStoreWithBackend for normal usage — it reads the existing
// metadata file if present.
func NewKeyStoreWithBackend(workspaceRoot string, store secrets.SecretStore) *KeyStore {
	return &KeyStore{
		workspaceRoot: workspaceRoot,
		keys:          make(map[string]APIKey),
		store:         store,
	}
}

// LoadKeyStore reads .vedox/agent-keys.json from the given workspace root and
// returns a populated KeyStore backed by the legacy go-keyring path. If the
// file does not exist, an empty file is created on disk and an empty KeyStore
// is returned — this makes first-run behaviour consistent with subsequent runs.
//
// A corrupt JSON file returns an error rather than silently discarding keys.
func LoadKeyStore(workspaceRoot string) (*KeyStore, error) {
	ks := NewKeyStore(workspaceRoot)

	path := ks.metadataPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// First run — create an empty metadata file so the path exists and
		// operators can see it. Ignore errors here; the next IssueKey will
		// retry the directory creation if needed.
		if writeErr := ks.writeLocked(); writeErr != nil {
			return nil, fmt.Errorf("create empty agent-keys.json: %w", writeErr)
		}
		return ks, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read agent-keys.json: %w", err)
	}

	if len(data) == 0 {
		return ks, nil
	}

	var list []APIKey
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse agent-keys.json: %w", err)
	}
	for _, k := range list {
		ks.keys[k.ID] = k
	}
	return ks, nil
}

// LoadKeyStoreWithBackend is the pluggable-backend variant of LoadKeyStore. It
// reads the metadata file and returns a KeyStore wired to the given
// SecretStore. The caller must call store.Open() before issuing any key
// operations.
func LoadKeyStoreWithBackend(workspaceRoot string, store secrets.SecretStore) (*KeyStore, error) {
	ks := NewKeyStoreWithBackend(workspaceRoot, store)

	path := ks.metadataPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if writeErr := ks.writeLocked(); writeErr != nil {
			return nil, fmt.Errorf("create empty agent-keys.json: %w", writeErr)
		}
		return ks, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read agent-keys.json: %w", err)
	}
	if len(data) == 0 {
		return ks, nil
	}
	var list []APIKey
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parse agent-keys.json: %w", err)
	}
	for _, k := range list {
		ks.keys[k.ID] = k
	}
	return ks, nil
}

// metadataPath returns the absolute path of the metadata JSON file.
func (ks *KeyStore) metadataPath() string {
	return filepath.Join(ks.workspaceRoot, ".vedox", metadataFileName)
}

// IssueKey creates a new API key with a random 32-byte secret, stores the
// secret in the OS keychain, persists the public metadata to disk, and
// returns (id, plaintextSecret, nil). The plaintext secret is returned to
// the caller exactly once — it is never stored in memory beyond this call.
//
// Callers should print the secret to the user immediately with a warning
// that it will not be shown again.
//
// If the keychain is unavailable, IssueKey returns VDX-302 and does NOT
// write the metadata file, so the store remains consistent.
func (ks *KeyStore) IssueKey(name, project, pathPrefix string) (string, string, error) {
	// Generate 32 bytes of cryptographic randomness, hex-encode for transport.
	// Hex is used (not base64) so the secret is URL-safe and shell-safe with
	// no special characters operators need to quote.
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", fmt.Errorf("generate secret: %w", err)
	}
	secret := hex.EncodeToString(secretBytes)

	id := uuid.NewString()
	key := APIKey{
		ID:         id,
		Name:       name,
		Project:    project,
		PathPrefix: pathPrefix,
		CreatedAt:  time.Now().UTC(),
	}

	// Store secret in the secret backend BEFORE persisting metadata — if the
	// backend fails we do not want a dangling metadata entry with no retrievable
	// secret.
	if err := ks.setSecret(id, secret); err != nil {
		return "", "", err
	}

	ks.mu.Lock()
	defer ks.mu.Unlock()
	ks.keys[id] = key
	if err := ks.writeLocked(); err != nil {
		// Rollback: remove the secret so we do not leak an entry for a key
		// that was never persisted to the metadata file.
		_ = ks.deleteSecret(id)
		delete(ks.keys, id)
		return "", "", fmt.Errorf("persist agent-keys.json: %w", err)
	}

	return id, secret, nil
}

// RevokeKey tombstones a key: sets Revoked=true in metadata, deletes the
// secret from the keychain, and persists. A revoked key is retained in the
// metadata file for audit purposes so operators can see the full history.
//
// Revoking an already-revoked key is a no-op (idempotent).
// Revoking an unknown key returns an error.
func (ks *KeyStore) RevokeKey(id string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	key, ok := ks.keys[id]
	if !ok {
		return fmt.Errorf("unknown key ID: %s", id)
	}
	if key.Revoked {
		return nil
	}

	// Delete the secret first. A "not found" error on delete is fine — the
	// desired end state (no entry) is achieved regardless.
	if err := ks.deleteSecret(id); err != nil && !secrets.IsNotFound(err) {
		return vdxerr.Wrap(
			vdxerr.ErrKeychainUnavailable,
			"Could not delete agent API key secret from the secret store.",
			err,
		)
	}

	key.Revoked = true
	ks.keys[id] = key
	return ks.writeLocked()
}

// ListKeys returns a snapshot of all keys, sorted by CreatedAt ascending.
// The returned slice is safe for the caller to mutate; it is a copy.
func (ks *KeyStore) ListKeys() []APIKey {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	out := make([]APIKey, 0, len(ks.keys))
	for _, k := range ks.keys {
		out = append(out, k)
	}
	// Simple insertion sort keeps this allocation-free and avoids pulling in
	// sort.Slice for a list that is typically <10 entries in practice.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].CreatedAt.After(out[j].CreatedAt); j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// Lookup returns the APIKey with the given ID and whether it was found.
// This is the hot-path function called by the auth middleware on every
// request — keep it O(1) and allocation-free.
func (ks *KeyStore) Lookup(id string) (APIKey, bool) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	k, ok := ks.keys[id]
	return k, ok
}

// keystoreCloser is the optional Close interface implemented by AgeStore (and
// any future SecretStore backend that caches passphrase material in memory).
// We test for it at runtime so the go-keyring / env backends, which have
// nothing to close, are unaffected.
type keystoreCloser interface {
	Close() error
}

// Close releases any resources held by the underlying secret backend. For an
// AgeStore this zeros the in-memory passphrase and unsets VEDOX_AGE_PASSPHRASE
// so child processes spawned after shutdown cannot inherit it. For the legacy
// go-keyring path and EnvStore, Close is a no-op.
//
// Close is idempotent and safe to call multiple times.
func (ks *KeyStore) Close() error {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	if ks.store == nil {
		return nil
	}
	c, ok := ks.store.(keystoreCloser)
	if !ok {
		return nil
	}
	return c.Close()
}

// getSecret fetches the plaintext HMAC secret for the given key ID.
//
// When ks.store is non-nil the pluggable backend is used; otherwise it falls
// back to the legacy go-keyring path. Returns VDX-302 if the backend is
// unreachable, or a plain error if the entry does not exist (caller treats
// that as auth fail).
func (ks *KeyStore) getSecret(id string) (string, error) {
	if ks.store != nil {
		val, err := ks.store.Get(id)
		if secrets.IsNotFound(err) {
			return "", fmt.Errorf("no secret entry for %s", id)
		}
		if err != nil {
			return "", vdxerr.Wrap(
				vdxerr.ErrKeychainUnavailable,
				"Could not read agent API key secret from the secret store.",
				err,
			)
		}
		return string(val), nil
	}

	// Legacy go-keyring path (macOS Keychain default, existing behaviour).
	secret, err := keyring.Get(keychainService, id)
	if err == keyring.ErrNotFound {
		return "", fmt.Errorf("no keychain entry for %s", id)
	}
	if err != nil {
		return "", vdxerr.Wrap(
			vdxerr.ErrKeychainUnavailable,
			"Could not read agent API key secret from the OS keychain.",
			err,
		)
	}
	return secret, nil
}

// setSecret stores a HMAC secret for the given key ID in the configured backend.
func (ks *KeyStore) setSecret(id, secret string) error {
	if ks.store != nil {
		if err := ks.store.Put(id, []byte(secret)); err != nil {
			return vdxerr.Wrap(
				vdxerr.ErrKeychainUnavailable,
				"Could not store agent API key secret in the secret store.",
				err,
			)
		}
		return nil
	}
	// Legacy go-keyring path.
	if err := keyring.Set(keychainService, id, secret); err != nil {
		return vdxerr.Wrap(
			vdxerr.ErrKeychainUnavailable,
			"Could not store agent API key secret in the OS keychain.",
			err,
		)
	}
	return nil
}

// deleteSecret removes the HMAC secret for the given key ID from the backend.
// Returns secrets.ErrNotFound (or keyring.ErrNotFound) if the entry is absent.
func (ks *KeyStore) deleteSecret(id string) error {
	if ks.store != nil {
		return ks.store.Delete(id)
	}
	// Legacy go-keyring path.
	return keyring.Delete(keychainService, id)
}

// writeLocked persists ks.keys to disk atomically. Caller must hold ks.mu.
//
// Atomic write protocol:
//  1. Ensure .vedox/ exists (0o700 — operator-only).
//  2. Write to a temp file in the same directory.
//  3. fsync the temp file.
//  4. os.Rename to the final path (atomic on POSIX for same-filesystem renames).
//
// The rename step guarantees readers never see a partially-written file.
func (ks *KeyStore) writeLocked() error {
	dir := filepath.Join(ks.workspaceRoot, ".vedox")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir .vedox: %w", err)
	}

	list := make([]APIKey, 0, len(ks.keys))
	for _, k := range ks.keys {
		list = append(list, k)
	}
	// Sort by CreatedAt for deterministic file contents — makes diffs clean
	// when operators commit this file to source control (though they shouldn't).
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j-1].CreatedAt.After(list[j].CreatedAt); j-- {
			list[j-1], list[j] = list[j], list[j-1]
		}
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal agent-keys.json: %w", err)
	}

	finalPath := ks.metadataPath()
	tmp, err := os.CreateTemp(dir, ".agent-keys-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	// Best-effort cleanup of the temp file if anything below fails.
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("fsync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, finalPath); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	cleanup = false
	return nil
}
