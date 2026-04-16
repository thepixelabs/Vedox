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
type FileRegistry struct {
	path string // absolute path to repos.json

	mu     sync.RWMutex
	byID   map[string]Repo
	byName map[string]string // name → id
}

// NewFileRegistry opens or creates a FileRegistry backed by the JSON file at
// path. If the file does not exist it is created with an empty manifest.
// The parent directory must already exist.
func NewFileRegistry(path string) (*FileRegistry, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("registry.NewFileRegistry: resolve path %q: %w", path, err)
	}

	r := &FileRegistry{
		path:   abs,
		byID:   make(map[string]Repo),
		byName: make(map[string]string),
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
func (r *FileRegistry) Reload() error {
	r.mu.Lock()
	defer r.mu.Unlock()

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
		// Write back the orphan annotations. We do NOT hold the file lock here
		// because Reload is called from a signal handler context where we already
		// hold the write mutex; a deadlock would occur if withFileLock tried to
		// acquire the write mutex again. Instead we write directly.
		if err := r.writeManifestLocked(m); err != nil {
			// Non-fatal: cache will still reflect reality even if persist fails.
			_ = err
		}
	}

	r.populateCache(m)
	return nil
}

// ---- Internal helpers -------------------------------------------------------

// withFileLock acquires the advisory flock on repos.json, reads the current
// manifest, calls fn with it, and on nil return writes the updated manifest
// atomically. It then refreshes the in-memory cache.
func (r *FileRegistry) withFileLock(fn func(m *manifest) error) error {
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
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

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
	return nil
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
