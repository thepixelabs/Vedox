package secrets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"filippo.io/age"
	"golang.org/x/term"
)

// ageSecretsFileName is the name of the age-encrypted secrets file relative to
// the Vedox global config directory (~/.vedox/).
const ageSecretsFileName = "secrets.age"

// agePayload is the JSON structure that is encrypted inside the age file.
// All active HMAC secrets are stored in a single ciphertext so partial reads
// are impossible — an attacker with the file sees nothing without the
// passphrase.
type agePayload struct {
	Version int               `json:"version"`
	Secrets map[string]string `json:"secrets"` // key ID → hex-encoded secret bytes
}

// AgeStore implements SecretStore using filippo.io/age encryption of a JSON
// map at ~/.vedox/secrets.age. It is the recommended backend for headless Linux
// deployments (VPS, servers, WSL2) where the D-Bus Secret Service is not
// available.
//
// Passphrase resolution order (mirrors §2.2 of the design doc):
//  1. VEDOX_AGE_PASSPHRASE_FILE — path to a file containing the passphrase.
//  2. VEDOX_AGE_PASSPHRASE — raw passphrase in an environment variable.
//     Weaker: leaks to /proc/<pid>/environ. Emits a WARN log.
//  3. Interactive TTY prompt — only when os.Stdin is a terminal.
//  4. Return VDX-D04-equivalent error — refuse to start.
//
// Thread safety: all mutating operations hold mu. The in-memory secrets map
// acts as a write-through cache; every mutation re-encrypts and overwrites
// secrets.age atomically.
type AgeStore struct {
	mu         sync.Mutex
	configDir  string       // absolute path of the directory holding secrets.age
	passphrase []byte       // held in memory after first unlock; never nil after Open
	cache      map[string]string // key → hex-encoded secret (plaintext after decrypt)
	loaded     bool
}

// NewAgeStore creates an AgeStore that will use the given configDir
// (usually ~/.vedox/). Call Open before the first Get/Put/Delete/List.
func NewAgeStore(configDir string) *AgeStore {
	return &AgeStore{
		configDir: configDir,
		cache:     make(map[string]string),
	}
}

// Open resolves the passphrase and decrypts the secrets.age file (if it
// exists) into the in-memory cache. Open must be called exactly once before
// any other method. Calling Open twice returns an error.
//
// If the secrets.age file does not yet exist, Open resolves the passphrase but
// does not fail — the file will be created on the first Put.
func (s *AgeStore) Open() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loaded {
		return fmt.Errorf("AgeStore.Open called twice")
	}

	passphrase, err := resolvePassphrase()
	if err != nil {
		return err
	}
	s.passphrase = passphrase

	path := filepath.Join(s.configDir, ageSecretsFileName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// First run — no file yet. Cache remains empty; passphrase is set.
		s.loaded = true
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	payload, err := decryptAge(data, passphrase)
	if err != nil {
		return fmt.Errorf("decrypt secrets.age: %w", err)
	}
	s.cache = payload.Secrets
	s.loaded = true
	return nil
}

// Get retrieves the secret for key from the in-memory cache.
func (s *AgeStore) Get(key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		return nil, fmt.Errorf("AgeStore: Open has not been called")
	}
	val, ok := s.cache[key]
	if !ok {
		return nil, &ErrNotFound{Key: key}
	}
	return []byte(val), nil
}

// Put stores value under key and re-encrypts the full cache to disk atomically.
func (s *AgeStore) Put(key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		return fmt.Errorf("AgeStore: Open has not been called")
	}
	s.cache[key] = string(value)
	return s.writeLocked()
}

// Delete removes key from the cache and re-encrypts to disk.
func (s *AgeStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		return fmt.Errorf("AgeStore: Open has not been called")
	}
	if _, ok := s.cache[key]; !ok {
		return &ErrNotFound{Key: key}
	}
	delete(s.cache, key)
	return s.writeLocked()
}

// List returns all stored keys in unspecified order.
func (s *AgeStore) List() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loaded {
		return nil, fmt.Errorf("AgeStore: Open has not been called")
	}
	keys := make([]string, 0, len(s.cache))
	for k := range s.cache {
		keys = append(keys, k)
	}
	return keys, nil
}

// writeLocked serialises the cache, encrypts it with age, and atomically
// overwrites secrets.age. Caller must hold s.mu.
//
// Atomic write: temp file → fsync → chmod 0o600 → rename. On POSIX, rename(2)
// within the same filesystem is atomic — readers never see a partial write.
func (s *AgeStore) writeLocked() error {
	if err := os.MkdirAll(s.configDir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", s.configDir, err)
	}

	payload := agePayload{Version: 1, Secrets: s.cache}
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal secrets: %w", err)
	}

	ciphertext, err := encryptAge(plaintext, s.passphrase)
	if err != nil {
		return fmt.Errorf("encrypt secrets.age: %w", err)
	}

	finalPath := filepath.Join(s.configDir, ageSecretsFileName)
	tmp, err := os.CreateTemp(s.configDir, ".secrets-*.age.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(ciphertext); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("fsync temp file: %w", err)
	}
	// fchmod via the open descriptor before Close closes the TOCTOU window
	// that a path-based os.Chmod(tmpName, ...) would leave open.
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, finalPath); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	cleanup = false
	return nil
}

// Close best-effort zeroes the in-memory passphrase so it is not present in a
// core dump or page-swap after the store is no longer needed. Cached secret
// values are stored as Go strings, which are immutable and not zeroable —
// that is an acknowledged limitation of holding secrets in a map[string]string.
// Callers should prefer a short-lived AgeStore where possible.
//
// Close is idempotent.
func (s *AgeStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.passphrase {
		s.passphrase[i] = 0
	}
	s.passphrase = nil
	s.loaded = false
	return nil
}

// scryptWorkFactor is the log2 N parameter passed to age's scrypt recipient.
// A value of 0 means "use the age default" (currently 18, ~0.3 s on modern
// hardware). Tests may lower this via testLowerScryptWorkFactor to keep the
// unit test suite under a few seconds — production callers never touch it.
//
// The guard in encryptAge defends against a stray call lowering this in a
// non-test binary: we refuse to encrypt with a factor below minWorkFactor.
var scryptWorkFactor int // 0 = age default

// minWorkFactor is the floor enforced by encryptAge when scryptWorkFactor is
// explicitly set. 10 is low enough for tests (<1 s) and high enough to hide
// accidental production use of the test hook.
const minWorkFactor = 10

// encryptAge encrypts plaintext using an age scrypt (passphrase) recipient.
// The default work factor (2^18) is used — ~0.3 s on modern hardware. Do not
// lower this value in production; the scrypt cost is the primary defence
// against offline brute-force on the exfiltrated secrets.age file.
func encryptAge(plaintext, passphrase []byte) ([]byte, error) {
	recipient, err := age.NewScryptRecipient(string(passphrase))
	if err != nil {
		return nil, fmt.Errorf("age recipient: %w", err)
	}
	if scryptWorkFactor != 0 {
		if scryptWorkFactor < minWorkFactor {
			return nil, fmt.Errorf("age: refusing work factor %d below minimum %d", scryptWorkFactor, minWorkFactor)
		}
		recipient.SetWorkFactor(scryptWorkFactor)
	}

	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recipient)
	if err != nil {
		return nil, fmt.Errorf("age encrypt init: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("age encrypt write: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("age encrypt close: %w", err)
	}
	return buf.Bytes(), nil
}

// decryptAge decrypts an age-encrypted ciphertext using a scrypt (passphrase)
// identity. Returns the plaintext agePayload on success.
func decryptAge(ciphertext, passphrase []byte) (*agePayload, error) {
	identity, err := age.NewScryptIdentity(string(passphrase))
	if err != nil {
		return nil, fmt.Errorf("age identity: %w", err)
	}

	r, err := age.Decrypt(bytes.NewReader(ciphertext), identity)
	if err != nil {
		return nil, fmt.Errorf("age decrypt: %w", err)
	}

	var payload agePayload
	if err := json.NewDecoder(r).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode secrets payload: %w", err)
	}
	return &payload, nil
}

// resolvePassphrase returns the age passphrase using the priority order
// described in §2.2 of the design doc.
func resolvePassphrase() ([]byte, error) {
	// 1. VEDOX_AGE_PASSPHRASE_FILE — recommended for servers and containers.
	if path := os.Getenv("VEDOX_AGE_PASSPHRASE_FILE"); path != "" {
		return readPassphraseFromFile(path)
	}

	// 2. VEDOX_AGE_PASSPHRASE — bare env var; weaker, warns at startup.
	if raw := os.Getenv("VEDOX_AGE_PASSPHRASE"); raw != "" {
		slog.Warn("secrets: using VEDOX_AGE_PASSPHRASE env var; " +
			"this leaks to /proc/<pid>/environ and child processes. " +
			"Use VEDOX_AGE_PASSPHRASE_FILE for production deployments.")
		pp := []byte(strings.TrimRight(raw, "\n\r "))
		if len(pp) == 0 {
			return nil, fmt.Errorf("VDX-D04: VEDOX_AGE_PASSPHRASE is set but empty")
		}
		return pp, nil
	}

	// 3. Interactive TTY prompt — only when stdin is a real terminal.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return promptPassphrase()
	}

	// 4. No source found — refuse to start.
	return nil, fmt.Errorf(
		"VDX-D04: daemon cannot start: age passphrase required for headless secret storage. " +
			"Set VEDOX_AGE_PASSPHRASE_FILE or VEDOX_AGE_PASSPHRASE. " +
			"See https://vedox.pixelabs.sh/docs/deploy/headless",
	)
}

// readPassphraseFromFile reads up to 4096 bytes from path and strips trailing
// whitespace. Returns an error if the file is missing, unreadable, or empty.
func readPassphraseFromFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("VDX-D04: cannot open passphrase file %s: %w", path, err)
	}
	defer f.Close()

	buf, err := io.ReadAll(io.LimitReader(f, 4096))
	if err != nil {
		return nil, fmt.Errorf("VDX-D04: read passphrase file %s: %w", path, err)
	}

	pp := bytes.TrimRight(buf, "\n\r ")
	if len(pp) == 0 {
		return nil, fmt.Errorf("VDX-D04: passphrase file %s is empty", path)
	}
	return pp, nil
}

// promptPassphrase reads a passphrase from the terminal without echo.
func promptPassphrase() ([]byte, error) {
	fmt.Fprint(os.Stderr, "Vedox secrets passphrase: ")
	pp, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // newline after the hidden input
	if err != nil {
		return nil, fmt.Errorf("read passphrase from terminal: %w", err)
	}
	pp = bytes.TrimRight(pp, "\n\r ")
	if len(pp) == 0 {
		return nil, fmt.Errorf("VDX-D04: passphrase cannot be empty")
	}
	return pp, nil
}
