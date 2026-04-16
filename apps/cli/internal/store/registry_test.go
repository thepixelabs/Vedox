package store

import (
	"fmt"
	"sort"
	"sync"
	"testing"
)

// newTestRegistry creates a fresh, empty ProjectRegistry for test use.
func newTestRegistry(t *testing.T) *ProjectRegistry {
	t.Helper()
	return NewProjectRegistry()
}

// newTestStore creates a minimal DocStore (LocalAdapter) for use in registry tests.
// It is rooted at a temporary directory that is cleaned up when the test ends.
func newTestStore(t *testing.T) DocStore {
	t.Helper()
	root := t.TempDir()
	a, err := NewLocalAdapter(root, nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}
	return a
}

// TestRegistry_RegisterAndGet verifies that a registered store is returned by Get.
func TestRegistry_RegisterAndGet(t *testing.T) {
	r := newTestRegistry(t)
	s := newTestStore(t)

	r.Register("my-project", s)

	got, ok := r.Get("my-project")
	if !ok {
		t.Fatal("Get: expected true for registered project, got false")
	}
	if got != s {
		t.Error("Get: returned a different DocStore than what was registered")
	}
}

// TestRegistry_GetUnknown verifies that Get returns false for an unregistered name.
func TestRegistry_GetUnknown(t *testing.T) {
	r := newTestRegistry(t)

	got, ok := r.Get("does-not-exist")
	if ok {
		t.Error("Get: expected false for unknown project, got true")
	}
	if got != nil {
		t.Errorf("Get: expected nil store for unknown project, got %v", got)
	}
}

// TestRegistry_RegisterReplaces verifies that registering the same name twice
// replaces the first store — the documented upgrade path from SymlinkAdapter
// to LocalAdapter.
func TestRegistry_RegisterReplaces(t *testing.T) {
	r := newTestRegistry(t)
	first := newTestStore(t)
	second := newTestStore(t)

	r.Register("project", first)
	r.Register("project", second)

	got, ok := r.Get("project")
	if !ok {
		t.Fatal("Get: expected true after second Register, got false")
	}
	if got != second {
		t.Error("Get: expected second store after replacement, got first")
	}
}

// TestRegistry_RegisterEmptyNamePanics verifies that registering with an empty
// name panics immediately (documented programming-error guard).
func TestRegistry_RegisterEmptyNamePanics(t *testing.T) {
	r := newTestRegistry(t)
	s := newTestStore(t)

	defer func() {
		if recover() == nil {
			t.Error("Register(\"\") should have panicked but did not")
		}
	}()

	r.Register("", s)
}

// TestRegistry_List returns all registered names in lexicographic order.
func TestRegistry_List(t *testing.T) {
	r := newTestRegistry(t)

	names := []string{"zebra", "alpha", "mango", "banana"}
	for _, name := range names {
		r.Register(name, newTestStore(t))
	}

	got := r.List()

	want := make([]string, len(names))
	copy(want, names)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("List: got %d names, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("List[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestRegistry_ListEmpty verifies that List returns an empty slice (not nil)
// on a registry with no registrations.
func TestRegistry_ListEmpty(t *testing.T) {
	r := newTestRegistry(t)

	got := r.List()
	if got == nil {
		t.Error("List: expected non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("List: expected 0 names, got %d", len(got))
	}
}

// TestRegistry_ListIsSnapshot verifies that a List result is not affected by
// subsequent Register calls.
func TestRegistry_ListIsSnapshot(t *testing.T) {
	r := newTestRegistry(t)
	r.Register("first", newTestStore(t))

	snapshot := r.List()
	r.Register("second", newTestStore(t))

	if len(snapshot) != 1 {
		t.Errorf("snapshot was mutated after subsequent Register; got len=%d", len(snapshot))
	}
}

// TestRegistry_Unregister verifies that a registered project is removed and
// subsequent Get returns false.
func TestRegistry_Unregister(t *testing.T) {
	r := newTestRegistry(t)
	r.Register("target", newTestStore(t))

	if err := r.Unregister("target"); err != nil {
		t.Fatalf("Unregister: unexpected error: %v", err)
	}

	_, ok := r.Get("target")
	if ok {
		t.Error("Get: expected false after Unregister, got true")
	}
}

// TestRegistry_UnregisterUnknownIsNoOp verifies that Unregistering a name that
// was never registered succeeds without error.
func TestRegistry_UnregisterUnknownIsNoOp(t *testing.T) {
	r := newTestRegistry(t)

	if err := r.Unregister("never-existed"); err != nil {
		t.Errorf("Unregister of unknown name: expected nil error, got %v", err)
	}
}

// TestRegistry_UnregisterEmptyNameErrors verifies that Unregister returns an
// error when the name is empty.
func TestRegistry_UnregisterEmptyNameErrors(t *testing.T) {
	r := newTestRegistry(t)

	if err := r.Unregister(""); err == nil {
		t.Error("Unregister with empty name: expected error, got nil")
	}
}

// TestRegistry_Len verifies the count of registered projects.
func TestRegistry_Len(t *testing.T) {
	r := newTestRegistry(t)

	if r.Len() != 0 {
		t.Errorf("Len: expected 0 on empty registry, got %d", r.Len())
	}

	r.Register("a", newTestStore(t))
	r.Register("b", newTestStore(t))

	if r.Len() != 2 {
		t.Errorf("Len: expected 2 after two registers, got %d", r.Len())
	}

	_ = r.Unregister("a")

	if r.Len() != 1 {
		t.Errorf("Len: expected 1 after unregister, got %d", r.Len())
	}
}

// TestRegistry_ConcurrentAccess exercises Register and Get concurrently to
// confirm no data races under -race.
func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := newTestRegistry(t)
	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Writers: register unique project names.
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("project-%d", i)
			r.Register(name, newTestStore(t))
		}()
	}

	// Readers: concurrently call Get and List while writers are active.
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			name := fmt.Sprintf("project-%d", i)
			// Get may or may not find the project yet — that is fine.
			// What matters is that no race is detected.
			_, _ = r.Get(name)
			_ = r.List()
		}()
	}

	wg.Wait()

	// All writers have finished; every project must now be registered.
	if r.Len() != goroutines {
		t.Errorf("Len: expected %d, got %d", goroutines, r.Len())
	}
}

// TestRegistry_ConcurrentUnregister exercises concurrent Register and Unregister
// to verify mutex correctness on the write path.
func TestRegistry_ConcurrentUnregister(t *testing.T) {
	r := newTestRegistry(t)
	const n = 30

	// Pre-register all projects.
	for i := 0; i < n; i++ {
		r.Register(fmt.Sprintf("p-%d", i), newTestStore(t))
	}

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			_ = r.Unregister(fmt.Sprintf("p-%d", i))
		}()
	}
	wg.Wait()

	if r.Len() != 0 {
		t.Errorf("expected empty registry after concurrent unregisters, got Len=%d", r.Len())
	}
}
