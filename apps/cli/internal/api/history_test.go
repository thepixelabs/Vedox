package api

// Direct handler tests for handleDocHistory (GET /api/projects/{project}/docs/*/history).
//
// These tests drive the handler via httptest.NewRecorder + chi.NewRouteContext
// so the router and middleware stay out of the assertion surface. Real git
// repositories are constructed with testutil.NewTestRepo so the assertions
// reflect what an end-user with a real workspace would observe.
//
// Test inventory:
//   TestHandleDocHistory_HappyPath           — 3 commits in registered project → 3 entries
//   TestHandleDocHistory_MissingProjectID    — empty {project} URL param → 400 VDX-005
//   TestHandleDocHistory_ProjectNotRegistered — unknown project name → see comment for actual behavior
//   TestHandleDocHistory_FileMissingInRepo   — file never tracked → 200 with []entries (NOT 404)
//   TestHandleDocHistory_FollowAcrossRename  — git mv preserved by --follow → entries span both names

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/vedox/vedox/internal/testutil"
)

// callDocHistory invokes handleDocHistory directly with the chi URL params
// attached. workspaceRoot is the Server's workspaceRoot. project and wildcard
// are the {project} and {*} URL params; pass them exactly as chi would have
// extracted them — that is, wildcard MUST end in "/history" (or be the literal
// "history") for the handler to enter its happy path.
func callDocHistory(t *testing.T, workspaceRoot, project, wildcard string) *httptest.ResponseRecorder {
	t.Helper()
	s := &Server{workspaceRoot: workspaceRoot}

	target := "/api/projects/" + project + "/docs/" + wildcard
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rctx := chi.NewRouteContext()
	if project != "" {
		rctx.URLParams.Add("project", project)
	}
	rctx.URLParams.Add("*", wildcard)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	s.handleDocHistory(rec, req)
	return rec
}

// decodeHistory decodes rec.Body into a historyResponse, failing the test on
// any decode error. Callers that expect a non-200 status check rec.Code first
// and use rec.Body.String() to assert on the error code.
func decodeHistory(t *testing.T, rec *httptest.ResponseRecorder) historyResponse {
	t.Helper()
	var got historyResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v (body=%s)", err, rec.Body.String())
	}
	return got
}

// TestHandleDocHistory_HappyPath uses the "project is its own git repo" layout
// (Layout A in the handler): workspaceRoot is the parent of a TestRepo and
// projectName is the repo's directory name, so filepath.Join(workspaceRoot, projectName)
// is exactly a git repo. The file is committed at the repo-relative tree path
// "<docRel>" (no project prefix) because the handler passes docPath — not
// relPath — to FileHistoryContext in Layout A.
func TestHandleDocHistory_HappyPath(t *testing.T) {
	t.Parallel()
	repo := testutil.NewTestRepo(t)

	workspaceRoot := filepath.Dir(repo.Path())
	projectName := filepath.Base(repo.Path())

	const docRel = "guide.md"

	// Commit at repo-relative path "guide.md" (no project prefix). The handler
	// strips the project prefix when it detects Layout A via the inner .git.
	repo.CommitFile(docRel, "# v1\n", "docs: initial guide")
	repo.CommitFile(docRel, "# v2\n", "docs: revise guide")
	repo.CommitFile(docRel, "# v3\n", "docs: finalise guide")

	rec := callDocHistory(t, workspaceRoot, projectName, docRel+"/history")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	got := decodeHistory(t, rec)
	wantDocPath := filepath.Join(projectName, docRel)
	if got.DocPath != wantDocPath {
		t.Errorf("docPath = %q, want %q", got.DocPath, wantDocPath)
	}
	if len(got.Entries) != 3 {
		t.Fatalf("entries len = %d, want 3 (entries=%+v)", len(got.Entries), got.Entries)
	}

	// Ordering contract: entries are most-recent first.
	wantSubjectsNewestFirst := []string{
		"docs: finalise guide",
		"docs: revise guide",
		"docs: initial guide",
	}
	for i, want := range wantSubjectsNewestFirst {
		if got.Entries[i].Message != want {
			t.Errorf("entries[%d].message = %q, want %q", i, got.Entries[i].Message, want)
		}
		if len(got.Entries[i].CommitHash) != 40 {
			t.Errorf("entries[%d].commitHash %q is not a 40-char SHA-1",
				i, got.Entries[i].CommitHash)
		}
	}
}

// TestHandleDocHistory_MissingProjectID asserts the handler returns 400 with
// VDX-005 when the {project} URL param is empty. validateDocPath rejects an
// empty project component before any git work is attempted.
func TestHandleDocHistory_MissingProjectID(t *testing.T) {
	t.Parallel()
	repo := testutil.NewTestRepo(t)

	rec := callDocHistory(t, repo.Path(), "", "guide.md/history")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "VDX-005") {
		t.Errorf("expected VDX-005 in body, got %s", rec.Body.String())
	}
}

// TestHandleDocHistory_ProjectNotRegistered documents the handler's response
// when {project} names a directory that does not exist under the workspace
// root. The handler falls back to Layout B (workspace-as-repo) because the
// project directory has no inner .git, and `git log -- <project>/<doc>` in
// the workspace repo matches no commits — surfacing as 200 with an empty
// entries array.
//
// SPEC GAP: the originally-requested contract was 404 for an unregistered
// project so the frontend can distinguish "wrong project" from "valid project,
// no history yet". The current handler does NOT differentiate these — it
// returns 200 + [] in both cases. Fixing that requires deciding what counts
// as "registered" (LocalAdapter projects are not explicitly registered
// anywhere today) and is out of scope for this change set. The back-report
// flags the gap.
func TestHandleDocHistory_ProjectNotRegistered(t *testing.T) {
	t.Parallel()
	repo := testutil.NewTestRepo(t)

	// Seed an unrelated commit so the workspace-root fallback finds a HEAD.
	repo.CommitFile("other/readme.md", "# unrelated\n", "chore: seed unrelated commit")

	rec := callDocHistory(t, repo.Path(), "ghost-project", "guide.md/history")

	// Actual behavior is 200 with empty entries — test locks that in so a
	// future 404 fix updates this assertion deliberately.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (current behavior; spec wants 404 — see comment) body=%s",
			rec.Code, rec.Body.String())
	}
	got := decodeHistory(t, rec)
	if got.Entries == nil {
		t.Error("entries must be a non-null slice — JSON contract is [] not null")
	}
	if len(got.Entries) != 0 {
		t.Errorf("entries len = %d, want 0 for unregistered project", len(got.Entries))
	}
}

// TestHandleDocHistory_FileMissingInRepo verifies the documented contract: a
// file that exists in a registered project but has never been committed (and
// therefore has no git history) returns 200 with an empty entries array. This
// is intentional — a missing-file 404 here would force the frontend to
// special-case the empty-history state.
func TestHandleDocHistory_FileMissingInRepo(t *testing.T) {
	t.Parallel()
	repo := testutil.NewTestRepo(t)

	const project = "myproject"
	// Commit at least one unrelated file so the repo has a HEAD; otherwise
	// `git log` exits 128 and we are exercising "empty repo" rather than
	// "file missing in repo".
	repo.CommitFile(filepath.Join(project, "other.md"), "# other\n", "docs: seed other")

	rec := callDocHistory(t, repo.Path(), project, "missing.md/history")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	got := decodeHistory(t, rec)
	if got.DocPath != filepath.Join(project, "missing.md") {
		t.Errorf("docPath = %q, want %q", got.DocPath, filepath.Join(project, "missing.md"))
	}
	if got.Entries == nil {
		t.Fatal("entries must be a non-null slice — JSON contract is [] not null")
	}
	if len(got.Entries) != 0 {
		t.Errorf("entries len = %d, want 0 for an untracked file", len(got.Entries))
	}
}

// TestHandleDocHistory_FollowAcrossRename verifies that --follow (set inside
// history.FileHistoryContext) preserves history across a rename. The handler
// is queried with the post-rename path and must return entries that include
// the pre-rename commits.
func TestHandleDocHistory_FollowAcrossRename(t *testing.T) {
	t.Parallel()
	repo := testutil.NewTestRepo(t)

	const project = "myproject"
	origRel := filepath.Join(project, "old-name.md")
	newRel := filepath.Join(project, "new-name.md")

	// Two commits under the original name.
	repo.CommitFile(origRel, "# Original\n\nFirst version.\n", "docs: initial old-name")
	repo.CommitFile(origRel, "# Original\n\nSecond version.\n", "docs: update old-name")

	// Rename then commit the rename, then a post-rename modification.
	repo.MustOutput("git", "mv", origRel, newRel)
	repo.CommitAll("docs: rename old-name to new-name")
	repo.CommitFile(newRel, "# Original\n\nThird version.\n", "docs: update new-name")

	rec := callDocHistory(t, repo.Path(), project, "new-name.md/history")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}

	got := decodeHistory(t, rec)
	// --follow must surface at least the 2 pre-rename commits in addition to
	// the rename and the post-rename edit. The minimum we assert is 3 entries
	// so a future tweak to git's rename-detection threshold doesn't make the
	// test brittle while still proving --follow crossed the rename boundary.
	if len(got.Entries) < 3 {
		t.Fatalf("entries len = %d, want >= 3 (follow across rename) entries=%+v",
			len(got.Entries), got.Entries)
	}

	// The oldest entry must be the initial commit made under the OLD name —
	// that is the load-bearing observation for --follow.
	oldest := got.Entries[len(got.Entries)-1]
	if !strings.Contains(oldest.Message, "initial old-name") {
		t.Errorf("oldest entry message = %q, want it to reference the pre-rename initial commit",
			oldest.Message)
	}
}
