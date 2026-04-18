package api

// Direct handler tests for handleGitStatus (GET /api/projects/{project}/git/status).
//
// The handler is explicitly best-effort per its doc comment: transient git
// errors and "not a git repo" are surfaced to the frontend as 200 OK with
// sentinel Branch="(no git)" so the editor chrome does not break. These tests
// lock in that contract against real ephemeral repos built with
// testutil.NewTestRepo, driving the handler via httptest.NewRecorder so the
// router and middleware stay out of the assertion surface.
//
// Test inventory:
//   TestHandleGitStatus_NotAGitRepo        — non-git workspace → 200 "(no git)"
//   TestHandleGitStatus_UnknownProject     — unknown project name → 200 "(no git)"
//   TestHandleGitStatus_DetachedHEAD       — detached HEAD → Branch="HEAD"
//   TestHandleGitStatus_DirtyWorkingTree   — uncommitted change → Dirty=true
//   TestHandleGitStatus_AheadBehindRemote  — upstream divergence → Ahead/Behind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
	"github.com/vedox/vedox/internal/testutil"
)

// gitIsolatedEnv returns an environment slice that neutralises host git
// configuration, mirroring testutil.TestRepo's isolation so ad-hoc commands
// run inside ahead/behind test clones cannot pick up the caller's ~/.gitconfig.
func gitIsolatedEnv(gitHome string) []string {
	base := os.Environ()
	return append(base,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"HOME="+gitHome,
		"GIT_AUTHOR_NAME=Test Author",
		"GIT_AUTHOR_EMAIL=test@vedox.local",
		"GIT_COMMITTER_NAME=Test Author",
		"GIT_COMMITTER_EMAIL=test@vedox.local",
	)
}

// runGitIn executes a git command in dir with the isolated env and fails the
// test on non-zero exit. Returns trimmed combined stdout+stderr.
func runGitIn(t *testing.T, dir, gitHome string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = gitIsolatedEnv(gitHome)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s): %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// callGitStatus invokes handleGitStatus directly with chi URL params attached.
// workspaceRoot is the Server's workspaceRoot; projectName is the {project}
// URL param. An empty projectName exercises the missing-param branch.
func callGitStatus(t *testing.T, workspaceRoot, projectName string) *httptest.ResponseRecorder {
	t.Helper()
	s := &Server{workspaceRoot: workspaceRoot}

	target := "/api/projects/" + projectName + "/git/status"
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rctx := chi.NewRouteContext()
	if projectName != "" {
		rctx.URLParams.Add("project", projectName)
	}
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	s.handleGitStatus(rec, req)
	return rec
}

// decodeGitStatus decodes rec.Body into a gitStatusResponse, failing the test
// on decode error. Also asserts a 200 status — every non-error path in the
// handler returns 200, so callers that expect a different code should inspect
// rec.Code directly instead.
func decodeGitStatus(t *testing.T, rec *httptest.ResponseRecorder) gitStatusResponse {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var got gitStatusResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v (body=%s)", err, rec.Body.String())
	}
	return got
}

// TestHandleGitStatus_NotAGitRepo verifies that pointing at a directory that
// exists but is not a git repo returns the documented best-effort sentinel
// (Branch="(no git)", zero counters) with HTTP 200 — not 404.
func TestHandleGitStatus_NotAGitRepo(t *testing.T) {
	workspace := t.TempDir()
	resolved, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	// Project directory exists but has no .git — rev-parse --abbrev-ref fails.
	if err := os.MkdirAll(filepath.Join(resolved, "plain"), 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}

	rec := callGitStatus(t, resolved, "plain")
	got := decodeGitStatus(t, rec)

	if got.Branch != "(no git)" {
		t.Errorf("Branch = %q, want %q", got.Branch, "(no git)")
	}
	if got.Dirty {
		t.Errorf("Dirty = true, want false for non-git project")
	}
	if got.Ahead != 0 || got.Behind != 0 {
		t.Errorf("Ahead/Behind = %d/%d, want 0/0", got.Ahead, got.Behind)
	}
}

// TestHandleGitStatus_UnknownProject verifies that an unknown project name
// (directory does not exist on disk, not registered in links.json) returns
// the same best-effort sentinel as a non-git project. Per the handler's
// contract this is intentional: the endpoint must never break the status
// bar, so missing-project is indistinguishable from missing-git.
func TestHandleGitStatus_UnknownProject(t *testing.T) {
	workspace := t.TempDir()
	resolved, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	rec := callGitStatus(t, resolved, "does-not-exist")
	got := decodeGitStatus(t, rec)

	if got.Branch != "(no git)" {
		t.Errorf("Branch = %q, want %q (handler is best-effort, never 404)", got.Branch, "(no git)")
	}
}

// TestHandleGitStatus_DetachedHEAD verifies the detached-HEAD surface. Git's
// `rev-parse --abbrev-ref HEAD` returns the literal string "HEAD" when the
// working tree points directly at a commit SHA rather than a branch ref.
func TestHandleGitStatus_DetachedHEAD(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	// Need two commits so we have a SHA to detach onto that isn't main@{0}.
	first := repo.CommitFile("a.md", "a", "first")
	repo.CommitFile("b.md", "b", "second")
	// Detach onto the first commit's SHA — git moves HEAD but sets no branch.
	repo.Checkout(first)

	// Point workspaceRoot at repo's parent and use the repo dir name as project.
	workspace := filepath.Dir(repo.Path())
	project := filepath.Base(repo.Path())

	rec := callGitStatus(t, workspace, project)
	got := decodeGitStatus(t, rec)

	if got.Branch != "HEAD" {
		t.Errorf("Branch = %q, want %q for detached HEAD", got.Branch, "HEAD")
	}
	if got.Dirty {
		t.Errorf("Dirty = true, want false (no uncommitted changes)")
	}
}

// TestHandleGitStatus_DirtyWorkingTree verifies that an uncommitted change
// flips Dirty=true. The branch remains the current branch name — dirty state
// is orthogonal to branch state.
func TestHandleGitStatus_DirtyWorkingTree(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	repo.CommitFile("tracked.md", "original\n", "initial")
	// Modify a tracked file without staging — porcelain output is non-empty.
	repo.WriteFile("tracked.md", "modified\n")

	workspace := filepath.Dir(repo.Path())
	project := filepath.Base(repo.Path())

	rec := callGitStatus(t, workspace, project)
	got := decodeGitStatus(t, rec)

	if got.Branch != "main" {
		t.Errorf("Branch = %q, want %q", got.Branch, "main")
	}
	if !got.Dirty {
		t.Errorf("Dirty = false, want true (tracked file was modified)")
	}
}

// TestHandleGitStatus_AheadBehindRemote verifies the ahead/behind counters
// when the local branch has diverged from its upstream. We set up a bare
// "remote" cloned from a seed repo, then clone that bare into a working copy.
// Commits added on both sides without pulling leave the working clone
// simultaneously ahead by 1 and behind by 1.
//
// Layout:
//
//	<seed>                 — the testutil.TestRepo (identity-isolated)
//	<parent>/remote.git    — bare clone used as upstream
//	<parent>/work          — working clone; main tracks remote.git/main
//	                         (1 commit ahead, 1 commit behind after divergence)
func TestHandleGitStatus_AheadBehindRemote(t *testing.T) {
	seed := testutil.NewTestRepo(t)
	seed.CommitFile("seed.md", "seed\n", "seed commit")

	// parent is the workspaceRoot the handler will see; gitHome is shared by
	// all ad-hoc commands so identity isolation matches the testutil helper.
	parent := t.TempDir()
	gitHome := t.TempDir()

	remote := filepath.Join(parent, "remote.git")
	runGitIn(t, parent, gitHome, "clone", "--bare", seed.Path(), remote)

	workPath := filepath.Join(parent, "work")
	runGitIn(t, parent, gitHome, "clone", remote, workPath)

	// Configure identity on the working clone (belt-and-suspenders — the env
	// vars above already supply identity, but some git subcommands still read
	// repo config for display purposes).
	runGitIn(t, workPath, gitHome, "config", "user.name", "test")
	runGitIn(t, workPath, gitHome, "config", "user.email", "test@test.com")

	// Push a new commit from seed to the remote. The working clone's main will
	// be one commit behind origin/main after the next fetch.
	seed.CommitFile("upstream.md", "upstream\n", "behind commit")
	seed.MustOutput("git", "push", remote, "main")

	// Add an ahead commit on the working clone without pulling.
	if err := os.WriteFile(filepath.Join(workPath, "local.md"), []byte("local\n"), 0o644); err != nil {
		t.Fatalf("writeFile local.md: %v", err)
	}
	runGitIn(t, workPath, gitHome, "add", "local.md")
	runGitIn(t, workPath, gitHome, "commit", "-m", "ahead commit")

	// Refresh the upstream ref cache so @{u} resolves to the latest remote tip.
	runGitIn(t, workPath, gitHome, "fetch", "origin")

	// Resolve parent to match filesystem reality on macOS (/var → /private/var)
	// so the handler's filepath.Join produces a path git can find.
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		t.Fatalf("EvalSymlinks parent: %v", err)
	}

	rec := callGitStatus(t, resolvedParent, "work")
	got := decodeGitStatus(t, rec)

	if got.Branch != "main" {
		t.Errorf("Branch = %q, want %q", got.Branch, "main")
	}
	if got.Ahead != 1 {
		t.Errorf("Ahead = %d, want 1", got.Ahead)
	}
	if got.Behind != 1 {
		t.Errorf("Behind = %d, want 1", got.Behind)
	}
	if got.Dirty {
		t.Errorf("Dirty = true, want false (no uncommitted changes)")
	}
}

// ── Integration test — route wiring through full chi stack ────────────────────

// newGitStatusServer spins up a full API server whose workspace root is set
// to the parent directory of the supplied repo. It is analogous to
// newTestServer in api_integration_test.go but accepts a pre-built TestRepo
// so the integration test controls git state before the server starts.
func newGitStatusServer(t *testing.T, workspaceRoot string) *httptest.Server {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	adapter, err := store.NewLocalAdapter(resolved, nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}

	dbStore, err := db.Open(db.Options{WorkspaceRoot: resolved})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = dbStore.Close() })

	srv := NewServer(
		adapter,
		dbStore,
		resolved,
		scanner.NewJobStore(),
		ai.NewJobStore(3),
		store.NewProjectRegistry(),
		agentauth.PassthroughAuth(),
	)

	mux := http.NewServeMux()
	srv.Mount(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// TestGitStatusRoute_Integration verifies that GET /api/projects/{project}/git/status
// is wired through the full chi router stack produced by Server.Mount and
// returns the expected JSON shape for a real git repository.
//
// This test guards against the dead-route failure mode: handleGitStatus
// existing in git_status.go but not registered in Mount(), which would cause
// the frontend status bar to receive 404s and silently degrade.
func TestGitStatusRoute_Integration(t *testing.T) {
	repo := testutil.NewTestRepo(t)
	repo.CommitFile("doc.md", "hello\n", "initial commit")

	// Dirty the working tree so we can assert Dirty=true and confirm the full
	// handler path executed (not just a stub 200).
	repo.WriteFile("doc.md", "modified\n")

	// workspaceRoot is the parent directory; the project name is the repo dir name.
	workspace := filepath.Dir(repo.Path())
	project := filepath.Base(repo.Path())

	ts := newGitStatusServer(t, workspace)

	url := ts.URL + "/api/projects/" + project + "/git/status"
	resp, err := ts.Client().Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got gitStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Branch != "main" {
		t.Errorf("Branch = %q, want %q", got.Branch, "main")
	}
	if !got.Dirty {
		t.Errorf("Dirty = false, want true (working tree was modified before request)")
	}
}
