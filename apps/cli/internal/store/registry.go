// Package store — ProjectRegistry.
//
// ProjectRegistry holds multiple DocStore instances keyed by project name. It
// is the routing layer that lets the API server dispatch doc requests to the
// right store: LocalAdapter for imported projects, SymlinkAdapter for linked
// ones.
//
// All methods are safe for concurrent use; access to the internal map is
// protected by a sync.RWMutex.
package store

import (
	"fmt"
	"sort"
	"sync"
)

// ProjectRegistry maps project names to their backing DocStore. It is the
// single source of truth for which projects are registered and how their files
// should be read or written.
//
// Thread safety: all exported methods acquire the appropriate lock (read or
// write) before accessing the map. Callers must not mutate the returned
// DocStore slices.
type ProjectRegistry struct {
	mu     sync.RWMutex
	stores map[string]DocStore
}

// NewProjectRegistry returns an initialised, empty ProjectRegistry.
func NewProjectRegistry() *ProjectRegistry {
	return &ProjectRegistry{
		stores: make(map[string]DocStore),
	}
}

// Register adds or replaces the DocStore for the given project name. Replacing
// an existing registration is allowed — it is how a project is upgraded from a
// SymlinkAdapter to a LocalAdapter after Import & Migrate.
func (r *ProjectRegistry) Register(name string, store DocStore) {
	if name == "" {
		// Programming error — panic early rather than silently storing under
		// the empty key, which would be unroutable and confusing.
		panic("store.ProjectRegistry.Register: name must not be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stores[name] = store
}

// Get returns the DocStore for name and true, or nil and false if the project
// is not registered.
func (r *ProjectRegistry) Get(name string) (DocStore, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.stores[name]
	return s, ok
}

// List returns the registered project names in lexicographic order. The slice
// is a snapshot — subsequent Register calls do not affect it.
func (r *ProjectRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.stores))
	for name := range r.stores {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Unregister removes the DocStore for name. It is a no-op if name is not
// registered. Returns an error if name is empty for fail-fast diagnostics.
func (r *ProjectRegistry) Unregister(name string) error {
	if name == "" {
		return fmt.Errorf("store.ProjectRegistry.Unregister: name must not be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.stores, name)
	return nil
}

// Len returns the number of registered projects. Safe for concurrent use.
func (r *ProjectRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.stores)
}
