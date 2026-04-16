package registry_test

// Integration tests for the FileRegistry package exercising real git
// repositories via testutil.NewTestRepo. These tests verify cross-package
// behaviour that cannot be observed through unit-level mocks: real directory
// paths, orphan detection on deleted trees, and SIGHUP-style Reload().

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/vedox/vedox/internal/registry"
	"github.com/vedox/vedox/internal/testutil"
)

// ---- helpers ----------------------------------------------------------------

// newIntegrationRegistry creates a FileRegistry backed by a temp directory.
// The registry file itself lives in a sub-directory so that the temp root can
// also hold git repos without path collisions.
func newIntegrationRegistry(t *testing.T) (*registry.FileRegistry, string) {
	t.Helper()
	dir := testutil.TempDir(t)
	regDir := filepath.Join(dir, "registry")
	if err := os.MkdirAll(regDir, 0o700); err != nil {
		t.Fatalf("mkdir registry dir: %v", err)
	}
	path := filepath.Join(regDir, "repos.json")
	reg, err := registry.NewFileRegistry(path)
	if err != nil {
		t.Fatalf("NewFileRegistry: %v", err)
	}
	return reg, path
}

// repoWithPath builds a minimal Repo record pointing at path.
func repoWithPath(name string, typ registry.RepoType, path string) registry.Repo {
	return registry.Repo{
		Name:     name,
		Type:     typ,
		RootPath: path,
		Status:   registry.StatusActive,
	}
}

// ---- Test: register three real git repos, list/get/default ------------------

// TestIntegration_RegisterThreeRepos creates three real git repos (private,
// project-public, bare-local), registers all three in a FileRegistry, then
// verifies list ordering, individual Get, and default-repo routing.
func TestIntegration_RegisterThreeRepos(t *testing.T) {
	reg, _ := newIntegrationRegistry(t)

	// Create three real git repos via the ephemeral-git helper.
	private := testutil.NewTestRepo(t)
	private.CommitFile("README.md", "private docs", "initial commit")

	projectPublic := testutil.NewTestRepo(t)
	projectPublic.CommitFile("docs/index.md", "# Public docs", "initial commit")

	bareLocal := testutil.NewTestRepo(t)
	bareLocal.CommitFile("notes.md", "local only", "initial commit")

	// Register all three.
	repos := []registry.Repo{
		repoWithPath("docs-private", registry.RepoTypePrivate, private.Path()),
		repoWithPath("docs-project", registry.RepoTypeProjectPublic, projectPublic.Path()),
		repoWithPath("notes-local", registry.RepoTypeBareLocal, bareLocal.Path()),
	}
	for _, r := range repos {
		if err := reg.Add(r); err != nil {
			t.Fatalf("Add %s: %v", r.Name, err)
		}
	}

	// List returns all three sorted alphabetically.
	listed, err := reg.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("expected 3 repos, got %d", len(listed))
	}
	// Alphabetical: docs-private, docs-project, notes-local
	wantOrder := []string{"docs-private", "docs-project", "notes-local"}
	for i, want := range wantOrder {
		if listed[i].Name != want {
			t.Errorf("List[%d]: got %q, want %q", i, listed[i].Name, want)
		}
	}

	// Get each repo by ID and verify its Type and RootPath.
	for _, r := range listed {
		got, err := reg.Get(r.ID)
		if err != nil {
			t.Errorf("Get(%s): %v", r.ID, err)
			continue
		}
		if got.Name != r.Name {
			t.Errorf("Get: name mismatch: got %q, want %q", got.Name, r.Name)
		}
		if got.RootPath == "" {
			t.Errorf("Get: RootPath must not be empty for %s", r.Name)
		}
	}

	// SetDefault to the private repo; Default() must return it.
	var privateID string
	for _, r := range listed {
		if r.Name == "docs-private" {
			privateID = r.ID
		}
	}
	if err := reg.SetDefault(privateID); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	def, err := reg.Default()
	if err != nil {
		t.Fatalf("Default: %v", err)
	}
	if def.ID != privateID {
		t.Errorf("Default: got %q, want %q", def.Name, "docs-private")
	}

	// The other two repos must NOT be flagged as default.
	for _, r := range listed {
		if r.ID == privateID {
			continue
		}
		got, _ := reg.Get(r.ID)
		if got.IsDefault {
			t.Errorf("%s should not be default after setting docs-private as default", got.Name)
		}
	}
}

// ---- Test: remove one repo, verify remainder --------------------------------

// TestIntegration_RemoveOneRepo registers three repos then removes the middle
// one (by name) and verifies only two remain.
func TestIntegration_RemoveOneRepo(t *testing.T) {
	reg, _ := newIntegrationRegistry(t)

	r1 := testutil.NewTestRepo(t)
	r2 := testutil.NewTestRepo(t)
	r3 := testutil.NewTestRepo(t)

	for _, r := range []struct {
		name string
		repo *testutil.TestRepo
		typ  registry.RepoType
	}{
		{"alpha", r1, registry.RepoTypePrivate},
		{"beta", r2, registry.RepoTypeProjectPublic},
		{"gamma", r3, registry.RepoTypeBareLocal},
	} {
		if err := reg.Add(repoWithPath(r.name, r.typ, r.repo.Path())); err != nil {
			t.Fatalf("Add %s: %v", r.name, err)
		}
	}

	// Locate beta's ID.
	listed, _ := reg.List()
	var betaID string
	for _, r := range listed {
		if r.Name == "beta" {
			betaID = r.ID
		}
	}
	if betaID == "" {
		t.Fatal("beta not found in registry")
	}

	// Remove beta.
	if err := reg.Remove(betaID); err != nil {
		t.Fatalf("Remove beta: %v", err)
	}

	// Only alpha and gamma remain.
	listed, _ = reg.List()
	if len(listed) != 2 {
		t.Fatalf("expected 2 repos after Remove, got %d", len(listed))
	}
	for _, r := range listed {
		if r.Name == "beta" {
			t.Error("beta should not appear after Remove")
		}
	}

	// Get on removed ID returns ErrNotFound.
	_, err := reg.Get(betaID)
	if err == nil {
		t.Error("expected ErrNotFound for removed repo, got nil")
	}

	// The physical git directories must still exist (Remove is registry-only).
	if _, err := os.Stat(r2.Path()); err != nil {
		t.Errorf("git repo directory should survive Remove, got stat error: %v", err)
	}
}

// ---- Test: orphan detection when repo path is deleted -----------------------

// TestIntegration_OrphanDetectionOnDeletion registers a real git repo, then
// removes the directory from disk and calls Reload(). The repo must transition
// to StatusOrphan.
func TestIntegration_OrphanDetectionOnDeletion(t *testing.T) {
	reg, _ := newIntegrationRegistry(t)

	// Repo that stays alive.
	stable := testutil.NewTestRepo(t)
	stable.CommitFile("README.md", "stable", "init")

	// Repo that we will delete from disk.
	doomed := testutil.NewTestRepo(t)
	doomed.CommitFile("README.md", "doomed", "init")
	doomedPath := doomed.Path()

	for _, r := range []struct {
		name string
		path string
		typ  registry.RepoType
	}{
		{"stable-repo", stable.Path(), registry.RepoTypeBareLocal},
		{"doomed-repo", doomedPath, registry.RepoTypeBareLocal},
	} {
		if err := reg.Add(repoWithPath(r.name, r.typ, r.path)); err != nil {
			t.Fatalf("Add %s: %v", r.name, err)
		}
	}

	// Verify both are active before deletion.
	repos, _ := reg.List()
	for _, r := range repos {
		if r.Status != registry.StatusActive {
			t.Fatalf("pre-deletion: expected active, got %s for %s", r.Status, r.Name)
		}
	}

	// Delete the doomed repo's directory.
	if err := os.RemoveAll(doomedPath); err != nil {
		t.Fatalf("RemoveAll doomed repo: %v", err)
	}

	// Reload triggers orphan detection.
	if err := reg.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	repos, _ = reg.List()
	statusByName := make(map[string]registry.RepoStatus, len(repos))
	for _, r := range repos {
		statusByName[r.Name] = r.Status
	}

	if statusByName["stable-repo"] != registry.StatusActive {
		t.Errorf("stable-repo: expected active, got %s", statusByName["stable-repo"])
	}
	if statusByName["doomed-repo"] != registry.StatusOrphan {
		t.Errorf("doomed-repo: expected orphan after directory deletion, got %s", statusByName["doomed-repo"])
	}
}

// ---- Test: SIGHUP reload simulation -----------------------------------------

// TestIntegration_SIGHUPReload simulates a SIGHUP-triggered config reload:
// an external process writes a new repos.json directly to disk, then the
// running registry calls Reload() and picks up the new state.
func TestIntegration_SIGHUPReload(t *testing.T) {
	reg, regPath := newIntegrationRegistry(t)

	// Add one repo via the registry API so the manifest has a valid structure.
	initial := testutil.NewTestRepo(t)
	initial.CommitFile("README.md", "initial", "init")
	if err := reg.Add(repoWithPath("initial-repo", registry.RepoTypeBareLocal, initial.Path())); err != nil {
		t.Fatalf("Add initial: %v", err)
	}

	listed, _ := reg.List()
	if len(listed) != 1 {
		t.Fatalf("expected 1 repo before SIGHUP, got %d", len(listed))
	}

	// Simulate an external process writing a new manifest directly to disk.
	// This is the SIGHUP scenario: another instance of vedox (or a CLI command)
	// mutates repos.json while the daemon is running.
	newRepo := testutil.NewTestRepo(t)
	newRepo.CommitFile("CONTRIBUTING.md", "welcome", "init")

	newManifest := struct {
		Version int `json:"version"`
		Repos   []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Type     string `json:"type"`
			RootPath string `json:"root_path"`
			Status   string `json:"status"`
		} `json:"repos"`
	}{
		Version: 1,
		Repos: []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Type     string `json:"type"`
			RootPath string `json:"root_path"`
			Status   string `json:"status"`
		}{
			{
				ID:       "external-uuid-001",
				Name:     "externally-added",
				Type:     "bare-local",
				RootPath: newRepo.Path(),
				Status:   "active",
			},
		},
	}

	b, err := json.MarshalIndent(newManifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal new manifest: %v", err)
	}
	if err := os.WriteFile(regPath, b, 0o600); err != nil {
		t.Fatalf("write new manifest to disk: %v", err)
	}

	// Reload picks up the externally-written manifest.
	if err := reg.Reload(); err != nil {
		t.Fatalf("Reload after SIGHUP simulation: %v", err)
	}

	listed, _ = reg.List()
	if len(listed) != 1 {
		t.Fatalf("expected 1 repo after SIGHUP Reload, got %d", len(listed))
	}
	if listed[0].Name != "externally-added" {
		t.Errorf("expected externally-added repo, got %q", listed[0].Name)
	}

	// Initial repo must no longer be present (it was replaced on disk).
	for _, r := range listed {
		if r.Name == "initial-repo" {
			t.Error("initial-repo should be gone after SIGHUP reload replaced the manifest")
		}
	}
}

// ---- Test: paused repos are exempt from orphan detection --------------------

// TestIntegration_PausedRepoNotOrphaned verifies that a repo with
// StatusPaused is never reclassified as StatusOrphan by Reload, even when
// its RootPath no longer exists on disk.
func TestIntegration_PausedRepoNotOrphaned(t *testing.T) {
	_, _ = newIntegrationRegistry(t)

	// Create then immediately delete a directory.
	gone := testutil.TempDir(t)
	if err := os.RemoveAll(gone); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	// Use a second independent FileRegistry over the same file path to add
	// the paused repo and then reload the first.
	reg2path := filepath.Join(testutil.TempDir(t), "repos.json")
	reg2, err := registry.NewFileRegistry(reg2path)
	if err != nil {
		t.Fatalf("reg2: %v", err)
	}

	if err := reg2.Add(registry.Repo{
		Name:     "paused-repo",
		Type:     registry.RepoTypeBareLocal,
		RootPath: gone, // path no longer exists
		Status:   registry.StatusPaused,
	}); err != nil {
		t.Fatalf("Add paused-repo: %v", err)
	}

	if err := reg2.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	repos, _ := reg2.List()
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}
	if repos[0].Status != registry.StatusPaused {
		t.Errorf("paused repo must not be reclassified: got %s, want paused", repos[0].Status)
	}
}
