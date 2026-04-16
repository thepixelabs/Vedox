package registry_test

// Integration tests verifying that FileRegistry mirrors repo records into
// GlobalDB on every manifest write.  These tests exercise the FIX-ARCH-06
// wiring: Add/Remove/SetDefault mutations must produce matching rows in
// global.db so the analytics FK chain stays consistent.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/registry"
)

// openTestGlobalDB opens an in-memory GlobalDB backed by a temp file and
// registers cleanup.
func openTestGlobalDB(t *testing.T) *db.GlobalDB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "global.db")
	g, err := db.OpenGlobalDB(path)
	if err != nil {
		t.Fatalf("OpenGlobalDB: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })
	return g
}

// newRegistryWithGlobalDB creates a FileRegistry wired to a real GlobalDB.
func newRegistryWithGlobalDB(t *testing.T, g *db.GlobalDB) *registry.FileRegistry {
	t.Helper()
	path := filepath.Join(t.TempDir(), "repos.json")
	reg, err := registry.NewFileRegistry(path, g)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	return reg
}

// ---------------------------------------------------------------------------
// Add mirrors into global.db
// ---------------------------------------------------------------------------

// TestRegistryAdd_MirrorsToGlobalDB verifies that adding a repo via the
// registry also creates a row in global.db with a matching ID and path.
func TestRegistryAdd_MirrorsToGlobalDB(t *testing.T) {
	ctx := context.Background()
	g := openTestGlobalDB(t)
	reg := newRegistryWithGlobalDB(t, g)

	if err := reg.Add(makeRepo("private-docs", registry.RepoTypePrivate, "/tmp/private-docs")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Retrieve the repo ID from the registry so we can look it up in global.db.
	repos, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	repoID := repos[0].ID

	// The row must exist in global.db.
	dbRepo, err := g.GetRepo(ctx, repoID)
	if err != nil {
		t.Fatalf("GetRepo from global.db: %v", err)
	}
	if dbRepo == nil {
		t.Fatal("expected repo in global.db, got nil")
	}
	if dbRepo.RootPath != "/tmp/private-docs" {
		t.Errorf("global.db RootPath = %q, want /tmp/private-docs", dbRepo.RootPath)
	}
}

// TestRegistryAdd_MultipleRepos_AllMirrored verifies that adding three repos
// results in three rows in global.db.
func TestRegistryAdd_MultipleRepos_AllMirrored(t *testing.T) {
	ctx := context.Background()
	g := openTestGlobalDB(t)
	reg := newRegistryWithGlobalDB(t, g)

	for _, r := range []registry.Repo{
		makeRepo("alpha", registry.RepoTypePrivate, "/tmp/alpha"),
		makeRepo("beta", registry.RepoTypeProjectPublic, "/tmp/beta"),
		makeRepo("gamma", registry.RepoTypeBareLocal, "/tmp/gamma"),
	} {
		if err := reg.Add(r); err != nil {
			t.Fatalf("Add %s: %v", r.Name, err)
		}
	}

	dbRepos, err := g.ListRepos(ctx, "")
	if err != nil {
		t.Fatalf("global.db ListRepos: %v", err)
	}
	if len(dbRepos) != 3 {
		t.Errorf("expected 3 repos in global.db, got %d", len(dbRepos))
	}
}

// ---------------------------------------------------------------------------
// SetDefault mirrors updated is_default semantics
// ---------------------------------------------------------------------------

// TestRegistrySetDefault_MirrorsToGlobalDB verifies that after SetDefault the
// global.db row for the affected repo can still be retrieved (the upsert
// correctly handles repeated writes without violating uniqueness constraints).
func TestRegistrySetDefault_MirrorsToGlobalDB(t *testing.T) {
	ctx := context.Background()
	g := openTestGlobalDB(t)
	reg := newRegistryWithGlobalDB(t, g)

	if err := reg.Add(makeRepo("default-candidate", registry.RepoTypePrivate, "/tmp/default")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	repos, _ := reg.List()
	id := repos[0].ID

	if err := reg.SetDefault(id); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	// The row must still exist in global.db after the SetDefault write.
	dbRepo, err := g.GetRepo(ctx, id)
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if dbRepo == nil {
		t.Fatal("repo must remain in global.db after SetDefault")
	}
}

// ---------------------------------------------------------------------------
// Remove mirrors deletion out of global.db
// ---------------------------------------------------------------------------

// TestRegistryRemove_GlobalDBRowPersists verifies the sync-only (upsert-only)
// design: Remove deletes the repo from the JSON manifest, but global.db is a
// denormalised read-side mirror that only receives UPSERTs, not DELETEs, from
// the registry.  The row therefore persists in global.db until an explicit
// GlobalDB.DeleteRepo call is made by a higher-level operator.
//
// This is intentional: analytics time-series data that references an old
// repo_id remains queryable even after the repo is de-registered.
func TestRegistryRemove_GlobalDBRowPersists(t *testing.T) {
	ctx := context.Background()
	g := openTestGlobalDB(t)
	reg := newRegistryWithGlobalDB(t, g)

	if err := reg.Add(makeRepo("to-be-removed", registry.RepoTypePrivate, "/tmp/removed")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	repos, _ := reg.List()
	id := repos[0].ID

	// Confirm the row exists before removal.
	before, _ := g.GetRepo(ctx, id)
	if before == nil {
		t.Fatal("expected row in global.db before Remove")
	}

	if err := reg.Remove(id); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// The repo must be gone from the JSON registry.
	regRepos, _ := reg.List()
	if len(regRepos) != 0 {
		t.Errorf("expected 0 repos in registry after Remove, got %d", len(regRepos))
	}

	// The row must still exist in global.db (upsert-only sync; no delete propagated).
	dbRepo, err := g.GetRepo(ctx, id)
	if err != nil {
		t.Fatalf("GetRepo after Remove: %v", err)
	}
	if dbRepo == nil {
		t.Error("global.db row must persist after registry Remove (upsert-only mirror design)")
	}

	// An explicit DeleteRepo on global.db does remove it.
	if err := g.DeleteRepo(ctx, id); err != nil {
		t.Fatalf("explicit DeleteRepo: %v", err)
	}
	after, _ := g.GetRepo(ctx, id)
	if after != nil {
		t.Error("expected nil after explicit global.db DeleteRepo")
	}
}

// ---------------------------------------------------------------------------
// Nil globalDB — standalone mode unchanged
// ---------------------------------------------------------------------------

// TestRegistry_NilGlobalDB verifies that a FileRegistry constructed with a nil
// GlobalDB works exactly as before — Add/Remove/List succeed without any DB
// interaction.
func TestRegistry_NilGlobalDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repos.json")
	reg, err := registry.NewFileRegistry(path, nil)
	if err != nil {
		t.Fatalf("NewFileRegistry(nil globalDB): %v", err)
	}

	if err := reg.Add(makeRepo("standalone", registry.RepoTypeBareLocal, "/tmp/standalone")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	repos, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("expected 1 repo, got %d", len(repos))
	}
}

// ---------------------------------------------------------------------------
// SetGlobalDB — post-construction injection
// ---------------------------------------------------------------------------

// TestSetGlobalDB_InjectAfterConstruction verifies that calling SetGlobalDB
// after construction and then adding a repo mirrors the record into global.db.
func TestSetGlobalDB_InjectAfterConstruction(t *testing.T) {
	ctx := context.Background()
	g := openTestGlobalDB(t)

	// Construct without globalDB.
	path := filepath.Join(t.TempDir(), "repos.json")
	reg, err := registry.NewFileRegistry(path, nil)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}

	// Inject after construction.
	reg.SetGlobalDB(g)

	if err := reg.Add(makeRepo("injected", registry.RepoTypePrivate, "/tmp/injected")); err != nil {
		t.Fatalf("Add after SetGlobalDB: %v", err)
	}

	repos, _ := reg.List()
	id := repos[0].ID

	dbRepo, err := g.GetRepo(ctx, id)
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if dbRepo == nil {
		t.Fatal("expected mirrored row in global.db after SetGlobalDB injection")
	}
	if dbRepo.Name != "injected" {
		t.Errorf("Name = %q, want injected", dbRepo.Name)
	}
}
