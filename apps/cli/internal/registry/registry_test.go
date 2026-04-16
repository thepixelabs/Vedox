package registry_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/vedox/vedox/internal/registry"
)

// newTempRegistry creates a FileRegistry backed by a temp file.
// The returned cleanup function removes the temp directory.
func newTempRegistry(t *testing.T) (*registry.FileRegistry, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.json")
	reg, err := registry.NewFileRegistry(path, nil)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	return reg, func() { os.RemoveAll(dir) }
}

// makeRepo returns a minimal Repo with the given name, type, and path.
func makeRepo(name string, repoType registry.RepoType, rootPath string) registry.Repo {
	return registry.Repo{
		Name:     name,
		Type:     repoType,
		RootPath: rootPath,
		Status:   registry.StatusActive,
	}
}

// ---- TestAdd -----------------------------------------------------------------

func TestAdd(t *testing.T) {
	reg, cleanup := newTempRegistry(t)
	defer cleanup()

	repo := makeRepo("docs-private", registry.RepoTypePrivate, "/tmp/docs-private")
	if err := reg.Add(repo); err != nil {
		t.Fatalf("Add: %v", err)
	}

	repos, err := reg.List()
	if err != nil {
		t.Fatalf("List after Add: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].Name != "docs-private" {
		t.Errorf("expected name docs-private, got %s", repos[0].Name)
	}
	if repos[0].ID == "" {
		t.Error("expected non-empty ID to be assigned")
	}
	if repos[0].CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestAdd_NameConflict(t *testing.T) {
	reg, cleanup := newTempRegistry(t)
	defer cleanup()

	repo := makeRepo("my-docs", registry.RepoTypeBareLocal, "/tmp/my-docs")
	if err := reg.Add(repo); err != nil {
		t.Fatalf("first Add: %v", err)
	}

	// Second add with the same name must fail.
	err := reg.Add(repo)
	if err == nil {
		t.Fatal("expected ErrNameConflict, got nil")
	}
	if !isNameConflict(err) {
		t.Errorf("expected ErrNameConflict, got %v", err)
	}
}

func TestAdd_EmptyName(t *testing.T) {
	reg, cleanup := newTempRegistry(t)
	defer cleanup()

	err := reg.Add(registry.Repo{Type: registry.RepoTypeBareLocal, RootPath: "/tmp/x"})
	if err == nil {
		t.Fatal("expected error for empty name, got nil")
	}
}

func TestAdd_DefaultsAssigned(t *testing.T) {
	reg, cleanup := newTempRegistry(t)
	defer cleanup()

	// No type, no status, no ID → should get defaults.
	err := reg.Add(registry.Repo{Name: "no-type", RootPath: "/tmp/no-type"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	repos, _ := reg.List()
	if repos[0].Type != registry.RepoTypeBareLocal {
		t.Errorf("expected default type bare-local, got %s", repos[0].Type)
	}
	if repos[0].Status != registry.StatusActive {
		t.Errorf("expected default status active, got %s", repos[0].Status)
	}
}

// ---- TestRemove -------------------------------------------------------------

func TestRemove(t *testing.T) {
	reg, cleanup := newTempRegistry(t)
	defer cleanup()

	if err := reg.Add(makeRepo("del-me", registry.RepoTypeBareLocal, "/tmp/del-me")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	repos, _ := reg.List()
	id := repos[0].ID

	if err := reg.Remove(id); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	repos, _ = reg.List()
	if len(repos) != 0 {
		t.Errorf("expected 0 repos after Remove, got %d", len(repos))
	}
}

func TestRemove_NotFound(t *testing.T) {
	reg, cleanup := newTempRegistry(t)
	defer cleanup()

	err := reg.Remove("nonexistent-id")
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if !isNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestRemove_DoesNotDeleteDisk(t *testing.T) {
	dir := t.TempDir()
	reg, cleanup := newTempRegistry(t)
	defer cleanup()

	if err := reg.Add(makeRepo("keep-on-disk", registry.RepoTypeBareLocal, dir)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	repos, _ := reg.List()
	_ = reg.Remove(repos[0].ID)

	// Directory must still exist after Remove.
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("expected directory to still exist after Remove, got stat error: %v", err)
	}
}

// ---- TestSetDefault ---------------------------------------------------------

func TestSetDefault(t *testing.T) {
	reg, cleanup := newTempRegistry(t)
	defer cleanup()

	if err := reg.Add(makeRepo("r1", registry.RepoTypePrivate, "/tmp/r1")); err != nil {
		t.Fatalf("Add r1: %v", err)
	}
	if err := reg.Add(makeRepo("r2", registry.RepoTypePrivate, "/tmp/r2")); err != nil {
		t.Fatalf("Add r2: %v", err)
	}

	repos, _ := reg.List()
	var idR1, idR2 string
	for _, r := range repos {
		if r.Name == "r1" {
			idR1 = r.ID
		}
		if r.Name == "r2" {
			idR2 = r.ID
		}
	}

	if err := reg.SetDefault(idR1); err != nil {
		t.Fatalf("SetDefault r1: %v", err)
	}

	def, err := reg.Default()
	if err != nil {
		t.Fatalf("Default after SetDefault: %v", err)
	}
	if def.ID != idR1 {
		t.Errorf("expected default to be r1, got %s", def.Name)
	}

	// Switch default to r2 — r1 must be cleared.
	if err := reg.SetDefault(idR2); err != nil {
		t.Fatalf("SetDefault r2: %v", err)
	}

	def, _ = reg.Default()
	if def.ID != idR2 {
		t.Errorf("expected default to be r2 after switch, got %s", def.Name)
	}

	repos, _ = reg.List()
	for _, r := range repos {
		if r.Name == "r1" && r.IsDefault {
			t.Error("r1 should no longer be default after switching to r2")
		}
	}
}

func TestSetDefault_NotFound(t *testing.T) {
	reg, cleanup := newTempRegistry(t)
	defer cleanup()

	err := reg.SetDefault("ghost-id")
	if err == nil {
		t.Fatal("expected error for non-existent ID, got nil")
	}
	if !isNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDefault_NoneSet(t *testing.T) {
	reg, cleanup := newTempRegistry(t)
	defer cleanup()

	if err := reg.Add(makeRepo("solo", registry.RepoTypeBareLocal, "/tmp/solo")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	_, err := reg.Default()
	if err == nil {
		t.Fatal("expected ErrNotFound when no default set, got nil")
	}
}

// ---- TestList ---------------------------------------------------------------

func TestList(t *testing.T) {
	reg, cleanup := newTempRegistry(t)
	defer cleanup()

	names := []string{"charlie", "alpha", "bravo"}
	for _, n := range names {
		if err := reg.Add(makeRepo(n, registry.RepoTypeBareLocal, "/tmp/"+n)); err != nil {
			t.Fatalf("Add %s: %v", n, err)
		}
	}

	repos, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(repos) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(repos))
	}

	// Must be sorted by name.
	if repos[0].Name != "alpha" || repos[1].Name != "bravo" || repos[2].Name != "charlie" {
		t.Errorf("List not sorted: %v %v %v", repos[0].Name, repos[1].Name, repos[2].Name)
	}
}

func TestList_Empty(t *testing.T) {
	reg, cleanup := newTempRegistry(t)
	defer cleanup()

	repos, err := reg.List()
	if err != nil {
		t.Fatalf("List on empty registry: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("expected empty slice, got %d repos", len(repos))
	}
}

// ---- TestGet ----------------------------------------------------------------

func TestGet(t *testing.T) {
	reg, cleanup := newTempRegistry(t)
	defer cleanup()

	if err := reg.Add(makeRepo("target", registry.RepoTypeProjectPublic, "/tmp/target")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	repos, _ := reg.List()
	id := repos[0].ID

	got, err := reg.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "target" {
		t.Errorf("expected name target, got %s", got.Name)
	}
	if got.Type != registry.RepoTypeProjectPublic {
		t.Errorf("expected type project-public, got %s", got.Type)
	}
}

func TestGet_NotFound(t *testing.T) {
	reg, cleanup := newTempRegistry(t)
	defer cleanup()

	_, err := reg.Get("no-such-id")
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
	if !isNotFound(err) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---- TestReload -------------------------------------------------------------

func TestReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.json")

	// Open registry A and add a repo.
	regA, err := registry.NewFileRegistry(path, nil)
	if err != nil {
		t.Fatalf("NewFileRegistry A: %v", err)
	}
	if err := regA.Add(makeRepo("original", registry.RepoTypeBareLocal, "/tmp/original")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Open a second registry instance B over the same file (simulates a second
	// process or a SIGHUP reload scenario).
	regB, err := registry.NewFileRegistry(path, nil)
	if err != nil {
		t.Fatalf("NewFileRegistry B: %v", err)
	}

	// B has the entry because it loaded on init.
	repos, _ := regB.List()
	if len(repos) != 1 {
		t.Fatalf("regB expected 1 repo, got %d", len(repos))
	}

	// A adds a second repo.
	if err := regA.Add(makeRepo("added-later", registry.RepoTypeBareLocal, "/tmp/added-later")); err != nil {
		t.Fatalf("Add second: %v", err)
	}

	// B has not reloaded yet — still sees 1.
	repos, _ = regB.List()
	if len(repos) != 1 {
		t.Errorf("regB should still see 1 repo before Reload, got %d", len(repos))
	}

	// After Reload, B sees 2.
	if err := regB.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	repos, _ = regB.List()
	if len(repos) != 2 {
		t.Errorf("regB should see 2 repos after Reload, got %d", len(repos))
	}
}

// ---- TestOrphanDetection ----------------------------------------------------

func TestOrphanDetection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.json")

	reg, err := registry.NewFileRegistry(path, nil)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}

	// Create a real directory so Add succeeds.
	realDir := filepath.Join(dir, "real-repo")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := reg.Add(makeRepo("real-repo", registry.RepoTypeBareLocal, realDir)); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := reg.Add(makeRepo("ghost-repo", registry.RepoTypeBareLocal, "/tmp/this-does-not-exist-vedox-test-12345")); err != nil {
		t.Fatalf("Add ghost: %v", err)
	}

	// Reload should detect the ghost as orphan.
	if err := reg.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	repos, _ := reg.List()
	statusByName := make(map[string]registry.RepoStatus)
	for _, r := range repos {
		statusByName[r.Name] = r.Status
	}

	if statusByName["real-repo"] != registry.StatusActive {
		t.Errorf("real-repo: expected active, got %s", statusByName["real-repo"])
	}
	if statusByName["ghost-repo"] != registry.StatusOrphan {
		t.Errorf("ghost-repo: expected orphan, got %s", statusByName["ghost-repo"])
	}
}

func TestOrphanRestoration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.json")

	reg, err := registry.NewFileRegistry(path, nil)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}

	// Path does not exist yet — will be orphan.
	comingDir := filepath.Join(dir, "coming-back")
	if err := reg.Add(makeRepo("coming-back", registry.RepoTypeBareLocal, comingDir)); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := reg.Reload(); err != nil {
		t.Fatalf("first Reload: %v", err)
	}

	repos, _ := reg.List()
	if repos[0].Status != registry.StatusOrphan {
		t.Fatalf("expected orphan status after first Reload, got %s", repos[0].Status)
	}

	// Create the directory — next Reload should restore to active.
	if err := os.MkdirAll(comingDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := reg.Reload(); err != nil {
		t.Fatalf("second Reload: %v", err)
	}

	repos, _ = reg.List()
	if repos[0].Status != registry.StatusActive {
		t.Errorf("expected active status after directory reappeared, got %s", repos[0].Status)
	}
}

// ---- TestConcurrentAccess ---------------------------------------------------

func TestConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.json")

	reg, err := registry.NewFileRegistry(path, nil)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}

	// Seed one repo to ensure default path is settable.
	if err := reg.Add(makeRepo("seed", registry.RepoTypeBareLocal, "/tmp/seed")); err != nil {
		t.Fatalf("Add seed: %v", err)
	}
	repos, _ := reg.List()
	seedID := repos[0].ID

	const workers = 10
	var wg sync.WaitGroup
	errCh := make(chan error, workers*3)

	// Mix of concurrent Adds, Lists, Gets, and SetDefaults.
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			name := fmt.Sprintf("concurrent-%d-%d", n, time.Now().UnixNano())
			if err := reg.Add(makeRepo(name, registry.RepoTypeBareLocal, "/tmp/"+name)); err != nil {
				// Name conflicts are acceptable under concurrent load.
				if !isNameConflict(err) {
					errCh <- err
				}
			}
			if _, err := reg.List(); err != nil {
				errCh <- err
			}
			if _, err := reg.Get(seedID); err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Errorf("concurrent error: %v", err)
		}
	}
}

// ---- TestPersistence (cross-instance) ----------------------------------------

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repos.json")

	reg1, err := registry.NewFileRegistry(path, nil)
	if err != nil {
		t.Fatalf("reg1 open: %v", err)
	}
	if err := reg1.Add(makeRepo("persistent", registry.RepoTypePrivate, "/tmp/persistent")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Open a second independent instance — must see the persisted data.
	reg2, err := registry.NewFileRegistry(path, nil)
	if err != nil {
		t.Fatalf("reg2 open: %v", err)
	}
	repos, err := reg2.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 persisted repo, got %d", len(repos))
	}
	if repos[0].Name != "persistent" {
		t.Errorf("expected name persistent, got %s", repos[0].Name)
	}
}

// ---- helpers ----------------------------------------------------------------

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "not found")
}

func isNameConflict(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "already registered")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

