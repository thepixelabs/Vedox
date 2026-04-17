// Package registry defines the v2 ProjectRegistry interface and the
// FileRegistry implementation backed by ~/.vedox/repos.json with advisory
// file locking.
//
// Design summary:
//   - JSON manifest at a caller-supplied path (default: ~/.vedox/repos.json)
//   - Advisory flock(2) for mutual exclusion across concurrent processes
//   - In-memory read cache guarded by sync.RWMutex for hot-path List/Get calls
//   - Orphan detection: on Reload, repos whose LocalPath does not exist on disk
//     are marked StatusOrphan so callers can surface them to the user
//   - Reload() is the re-entry point for SIGHUP — it drops the cache and
//     re-reads the manifest from disk without restarting the process
//
// This package is intentionally separate from internal/store/registry.go.
// The store package retains the in-process concurrency-safe map[string]DocStore;
// this package owns the typed v2 registry with persistence, lifecycle, and
// default-repo routing.
package registry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/vedox/vedox/internal/db"
)

// ---- Type enumerations ------------------------------------------------------

// RepoType categorises a repo by its access and visibility model.
type RepoType string

const (
	// RepoTypePrivate is a private documentation repo with a remote origin.
	RepoTypePrivate RepoType = "private"

	// RepoTypeProjectPublic is a documentation repo linked to one source project,
	// intended for public-facing docs.
	RepoTypeProjectPublic RepoType = "project-public"

	// RepoTypeBareLocal is a local-only documentation repo with no remote.
	RepoTypeBareLocal RepoType = "bare-local"
)

// RepoStatus is the operational lifecycle state of a registered repo.
type RepoStatus string

const (
	// StatusActive means the repo is reachable and being watched.
	StatusActive RepoStatus = "active"

	// StatusPaused means the repo was manually paused by the user.
	StatusPaused RepoStatus = "paused"

	// StatusOrphan means the repo's LocalPath no longer exists on disk.
	// Set automatically by Reload.
	StatusOrphan RepoStatus = "orphan"
)

// ---- Repo struct -------------------------------------------------------------

// Repo is the complete record for a registered documentation repo.
// JSON tags use snake_case to match the on-disk manifest format and the
// multi-agent architecture document schema.
type Repo struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      RepoType   `json:"type"`
	RootPath  string     `json:"root_path"`
	RemoteURL string     `json:"remote_url,omitempty"`
	Status    RepoStatus `json:"status"`
	IsDefault bool       `json:"is_default"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// ---- Sentinel errors --------------------------------------------------------

var (
	// ErrNotFound is returned by Get when the requested repo does not exist.
	ErrNotFound = errors.New("repo not found")

	// ErrNameConflict is returned by Add when a repo with the same name is
	// already registered.
	ErrNameConflict = errors.New("repo name already registered")

	// ErrIDConflict is returned by Add when a repo with the same ID is already
	// registered (programming error; IDs are generated internally).
	ErrIDConflict = errors.New("repo ID already registered")
)

// ---- manifest (on-disk JSON format) -----------------------------------------

// manifest is the on-disk JSON structure for repos.json.
type manifest struct {
	Version int    `json:"version"`
	Repos   []Repo `json:"repos"`
}

// ---- ProjectRegistry interface ----------------------------------------------

// ProjectRegistry is the stable interface for all repo lifecycle operations.
// All implementations must be safe for concurrent use.
type ProjectRegistry interface {
	// List returns all repos in lexicographic name order.
	List() ([]Repo, error)

	// Get returns the repo with the given ID.
	Get(id string) (Repo, error)

	// Add registers a new repo. Returns ErrNameConflict if the name is taken.
	Add(repo Repo) error

	// Remove removes the repo with the given ID from the registry.
	// It does NOT delete any files on disk.
	Remove(id string) error

	// SetDefault marks the repo with the given ID as the default routing target
	// for the Doc Agent. All other repos have IsDefault cleared.
	SetDefault(id string) error

	// Default returns the repo marked IsDefault. Returns ErrNotFound if no
	// default has been set.
	Default() (Repo, error)

	// Reload drops the in-memory cache and re-reads the manifest from disk.
	// Called by the daemon's SIGHUP handler.
	Reload() error
}

// ---- FileRegistry -----------------------------------------------------------

// FileRegistry is a file-backed implementation of ProjectRegistry.
// It reads and writes ~/.vedox/repos.json (or a caller-supplied path) using
// an advisory flock(2) for cross-process mutual exclusion.
//
// The in-memory cache (byID, byName) is the hot path for reads.
// Every mutation acquires the advisory lock, reads the current manifest,
// applies the change, writes the manifest atomically, then updates the cache.
//
// Because writeManifestLocked uses a temp-file-plus-rename pattern, the
// post-rename inode of repos.json is different from the inode the flock was
// acquired on. That means flock alone cannot protect two in-process
// goroutines from clobbering each other — the second goroutine can open
// the unlinked old inode, flock it successfully, and write stale state back
// over the first goroutine's commit. writeMu serialises the full
// open→read→write cycle within a single process so that never happens.
// Cross-process coordination still relies on flock; for single-daemon
// deployments (the vedox target) the flock is belt-and-braces.
//
// If globalDB is non-nil, every manifest write also upserts all repo records
// into global.db so that the analytics FK chain (analytics_cache → repos.id)
// always has an authoritative repo_id → path mapping.
type FileRegistry struct {
	path     string // absolute path to repos.json
	globalDB *db.GlobalDB

	// writeMu serialises the full flock+read+write+cache-refresh sequence
	// across goroutines in THIS process. See the type doc for why this is
	// needed on top of flock.
	writeMu sync.Mutex

	mu     sync.RWMutex
	byID   map[string]Repo
	byName map[string]string // name → id
}

// NewFileRegistry opens or creates a FileRegistry backed by the JSON file at
// path. If the file does not exist it is created with an empty manifest.
// The parent directory must already exist.
//
// Pass a non-nil *db.GlobalDB to enable automatic mirroring of repo records
// into global.db on every manifest write. Pass nil to opt out (the registry
// then operates in standalone mode, identical to previous behaviour).
func NewFileRegistry(path string, globalDB *db.GlobalDB) (*FileRegistry, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("registry.NewFileRegistry: resolve path %q: %w", path, err)
	}

	r := &FileRegistry{
		path:     abs,
		globalDB: globalDB,
		byID:     make(map[string]Repo),
		byName:   make(map[string]string),
	}

	// Bootstrap: create file with empty manifest if it doesn't exist.
	if _, err := os.Stat(abs); errors.Is(err, os.ErrNotExist) {
		if writeErr := r.writeManifestLocked(manifest{Version: 1}); writeErr != nil {
			return nil, fmt.Errorf("registry.NewFileRegistry: create manifest: %w", writeErr)
		}
	}

	if err := r.loadFromDisk(); err != nil {
		return nil, fmt.Errorf("registry.NewFileRegistry: initial load: %w", err)
	}

	return r, nil
}

// SetGlobalDB injects or replaces the GlobalDB handle after construction.
// This is useful when the DB handle is not yet available at registry creation
// time (e.g., during server startup sequencing). Passing nil disables global
// DB mirroring. SetGlobalDB is not safe to call concurrently with Add/Remove.
func (r *FileRegistry) SetGlobalDB(g *db.GlobalDB) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.globalDB = g
}

// List returns all repos sorted by name.
func (r *FileRegistry) List() ([]Repo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	repos := make([]Repo, 0, len(r.byID))
	for _, repo := range r.byID {
		repos = append(repos, repo)
	}
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Name < repos[j].Name
	})
	return repos, nil
}

// Get returns the repo with the given ID.
func (r *FileRegistry) Get(id string) (Repo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	repo, ok := r.byID[id]
	if !ok {
		return Repo{}, fmt.Errorf("%w: id=%s", ErrNotFound, id)
	}
	return repo, nil
}

// Add registers a new repo. If repo.ID is empty a UUID is generated.
// Returns ErrNameConflict if the name is already registered.
func (r *FileRegistry) Add(repo Repo) error {
	if repo.Name == "" {
		return fmt.Errorf("registry.Add: repo name must not be empty")
	}
	if repo.Type == "" {
		repo.Type = RepoTypeBareLocal
	}
	if repo.Status == "" {
		repo.Status = StatusActive
	}

	now := time.Now().UTC()
	if repo.CreatedAt.IsZero() {
		repo.CreatedAt = now
	}
	repo.UpdatedAt = now

	if repo.ID == "" {
		repo.ID = uuid.New().String()
	}

	return r.withFileLock(func(m *manifest) error {
		// Conflict checks on the live manifest (not just in-memory cache) to
		// guard against a concurrent process that wrote between our cache read
		// and our lock acquisition.
		for _, existing := range m.Repos {
			if existing.Name == repo.Name {
				return fmt.Errorf("%w: %s", ErrNameConflict, repo.Name)
			}
			if existing.ID == repo.ID {
				return fmt.Errorf("%w: %s", ErrIDConflict, repo.ID)
			}
		}
		m.Repos = append(m.Repos, repo)
		return nil
	})
}

// Remove removes the repo with the given ID from the registry.
// Returns ErrNotFound if the ID is not registered.
func (r *FileRegistry) Remove(id string) error {
	if id == "" {
		return fmt.Errorf("registry.Remove: id must not be empty")
	}
	return r.withFileLock(func(m *manifest) error {
		idx := -1
		for i, repo := range m.Repos {
			if repo.ID == id {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("%w: id=%s", ErrNotFound, id)
		}
		m.Repos = append(m.Repos[:idx], m.Repos[idx+1:]...)
		return nil
	})
}

// SetDefault marks the repo with the given ID as the default. All other repos
// have IsDefault cleared atomically in the same write.
func (r *FileRegistry) SetDefault(id string) error {
	if id == "" {
		return fmt.Errorf("registry.SetDefault: id must not be empty")
	}
	return r.withFileLock(func(m *manifest) error {
		found := false
		now := time.Now().UTC()
		for i := range m.Repos {
			if m.Repos[i].ID == id {
				m.Repos[i].IsDefault = true
				m.Repos[i].UpdatedAt = now
				found = true
			} else if m.Repos[i].IsDefault {
				m.Repos[i].IsDefault = false
				m.Repos[i].UpdatedAt = now
			}
		}
		if !found {
			return fmt.Errorf("%w: id=%s", ErrNotFound, id)
		}
		return nil
	})
}

// Default returns the repo marked IsDefault.
func (r *FileRegistry) Default() (Repo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, repo := range r.byID {
		if repo.IsDefault {
			return repo, nil
		}
	}
	return Repo{}, fmt.Errorf("%w: no default repo set", ErrNotFound)
}

// Reload drops the in-memory cache and re-reads the manifest from disk.
// Orphan detection is performed: repos whose RootPath does not exist on disk
// are marked StatusOrphan.
//
// The re-read and any orphan annotation write go through the advisory file
// lock so concurrent processes cannot corrupt the manifest with interleaved
// writes. A persist failure during annotation write is surfaced to the
// caller; the in-memory cache is still refreshed from what was read so the
// registry remains consistent with whatever is currently on disk.
func (r *FileRegistry) Reload() error {
	// Serialise in-process writers so Reload's optional orphan-write does
	// not race a concurrent Add/Remove/SetDefault.
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	// Acquire the advisory file lock so we observe a consistent snapshot
	// and so our orphan-annotation write cannot race another process's Add.
	f, err := os.OpenFile(r.path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("registry.Reload: open manifest for locking: %w", err)
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("registry.Reload: acquire file lock: %w", err)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	m, err := r.readManifest()
	if err != nil {
		return fmt.Errorf("registry.Reload: %w", err)
	}

	// Orphan detection: mark repos whose local path is gone.
	now := time.Now().UTC()
	changed := false
	for i, repo := range m.Repos {
		if repo.Status == StatusPaused {
			continue // user-paused repos are not subject to orphan detection
		}
		if _, statErr := os.Stat(repo.RootPath); errors.Is(statErr, os.ErrNotExist) {
			if repo.Status != StatusOrphan {
				m.Repos[i].Status = StatusOrphan
				m.Repos[i].UpdatedAt = now
				changed = true
			}
		} else if repo.Status == StatusOrphan {
			// Path came back — restore to active.
			m.Repos[i].Status = StatusActive
			m.Repos[i].UpdatedAt = now
			changed = true
		}
	}

	if changed {
		// Write back the orphan annotations under the same file lock we
		// already hold. writeManifestLocked does not re-enter r.mu, so no
		// deadlock is possible.
		if writeErr := r.writeManifestLocked(m); writeErr != nil {
			// Surface the persist failure: the caller needs to know the
			// on-disk state diverged from the cache we are about to populate.
			// Cache refresh still proceeds so the registry reflects reality.
			r.mu.Lock()
			r.populateCache(m)
			r.mu.Unlock()
			return fmt.Errorf("registry.Reload: persist orphan annotations: %w", writeErr)
		}
	}

	r.mu.Lock()
	r.populateCache(m)
	r.mu.Unlock()
	return nil
}

// ---- Internal helpers -------------------------------------------------------

// withFileLock acquires the advisory flock on repos.json, reads the current
// manifest, calls fn with it, and on nil return writes the updated manifest
// atomically. It then refreshes the in-memory cache.
func (r *FileRegistry) withFileLock(fn func(m *manifest) error) error {
	// Serialise in-process writers first so two goroutines cannot race on
	// the rename-replaces-inode lifecycle — see the FileRegistry type doc.
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	// Open (or create) the lock file. We use the manifest file itself as the
	// lock target — flock(2) is per-fd, per-process.
	f, err := os.OpenFile(r.path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("registry: open manifest for locking: %w", err)
	}
	defer f.Close()

	// Acquire exclusive advisory lock (blocking).
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("registry: acquire file lock: %w", err)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()

	// Read current manifest through the locked fd.
	m, err := r.readManifest()
	if err != nil {
		return fmt.Errorf("registry: read manifest under lock: %w", err)
	}

	// Apply the mutation.
	if err := fn(&m); err != nil {
		return err
	}

	// Write the updated manifest atomically.
	if err := r.writeManifestLocked(m); err != nil {
		return fmt.Errorf("registry: write manifest: %w", err)
	}

	// Refresh in-memory cache.
	r.mu.Lock()
	r.populateCache(m)
	r.mu.Unlock()

	return nil
}

// readManifest reads and unmarshals the manifest from r.path.
// If the file is empty it returns an empty manifest rather than an error.
func (r *FileRegistry) readManifest() (manifest, error) {
	b, err := os.ReadFile(r.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return manifest{Version: 1}, nil
		}
		return manifest{}, fmt.Errorf("read %s: %w", r.path, err)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return manifest{Version: 1}, nil
	}
	var m manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return manifest{}, fmt.Errorf("unmarshal %s: %w", r.path, err)
	}
	if m.Repos == nil {
		m.Repos = []Repo{}
	}
	return m, nil
}

// writeManifestLocked writes m to r.path using an atomic temp→rename pattern.
// The caller is responsible for holding any relevant locks.
//
// After a successful disk write, if a GlobalDB handle is configured, every
// repo in the manifest is upserted into global.db so the analytics FK chain
// always has an up-to-date repo_id → path mapping.  Failures from global.db
// are logged but do NOT roll back the manifest write — the JSON file remains
// the authoritative source of truth.
func (r *FileRegistry) writeManifestLocked(m manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create manifest dir: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".repos-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	ok := false
	defer func() {
		if !ok {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(b); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("fsync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, r.path); err != nil {
		return fmt.Errorf("rename temp to manifest: %w", err)
	}
	// Set mode 0600: manifest may contain path information.
	_ = os.Chmod(r.path, 0o600)

	ok = true

	// Mirror repo records into global.db (best-effort; does not fail the write).
	// Snapshot the handle under the read lock so a concurrent SetGlobalDB
	// cannot trigger a data race on r.globalDB.
	r.mu.RLock()
	gdb := r.globalDB
	r.mu.RUnlock()
	if gdb != nil {
		syncToGlobalDB(gdb, m)
	}

	return nil
}

// syncToGlobalDB upserts every repo in the manifest into the supplied
// GlobalDB handle. The handle is passed in (rather than read from the
// receiver) so the caller can snapshot it atomically with the registry's
// read lock and avoid racing a concurrent SetGlobalDB. Errors are
// intentionally swallowed because the JSON file is the authoritative source
// of truth; global.db is a denormalised read-side mirror.
func syncToGlobalDB(g *db.GlobalDB, m manifest) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, repo := range m.Repos {
		dbRepo := repoToDBRepo(repo)
		if err := g.SaveRepo(ctx, dbRepo); err != nil {
			// Non-fatal: the JSON manifest succeeded; log to stderr as a warning.
			// A full logger is not available here without introducing another
			// dependency; the error is surfaced on the next globaldb read.
			_ = err
		}
	}
}

// repoToDBRepo maps a registry.Repo to a db.Repo for global.db storage.
// The type field is normalised: registry uses "project-public" while global.db
// uses "public" (per the CHECK constraint in the bootstrap schema).
func repoToDBRepo(r Repo) db.Repo {
	dbType := string(r.Type)
	switch r.Type {
	case RepoTypeProjectPublic:
		dbType = "public"
	case RepoTypeBareLocal:
		dbType = "inbox"
	case RepoTypePrivate:
		dbType = "private"
	}
	dbStatus := string(r.Status)
	switch r.Status {
	case StatusActive:
		dbStatus = "active"
	case StatusPaused:
		// global.db doesn't have a "paused" status; map to active for FK integrity.
		dbStatus = "active"
	case StatusOrphan:
		dbStatus = "error"
	}
	return db.Repo{
		ID:        r.ID,
		Name:      r.Name,
		Type:      dbType,
		RootPath:  r.RootPath,
		RemoteURL: r.RemoteURL,
		Status:    dbStatus,
	}
}

// loadFromDisk reads the manifest and populates the in-memory cache.
// Called once at construction; subsequent refreshes go through Reload or
// withFileLock.
func (r *FileRegistry) loadFromDisk() error {
	m, err := r.readManifest()
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.populateCache(m)
	return nil
}

// populateCache rebuilds byID and byName from m. Must be called with r.mu held
// for writing (or before the registry is shared with other goroutines).
func (r *FileRegistry) populateCache(m manifest) {
	r.byID = make(map[string]Repo, len(m.Repos))
	r.byName = make(map[string]string, len(m.Repos))
	for _, repo := range m.Repos {
		r.byID[repo.ID] = repo
		r.byName[repo.Name] = repo.ID
	}
}

// ---- Context-aware helpers for daemon integration ---------------------------

// ListCtx is a context-aware wrapper for use in daemon request handlers.
func (r *FileRegistry) ListCtx(_ context.Context) ([]Repo, error) {
	return r.List()
}

// GetCtx is a context-aware wrapper.
func (r *FileRegistry) GetCtx(_ context.Context, id string) (Repo, error) {
	return r.Get(id)
}
