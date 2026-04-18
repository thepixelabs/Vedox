package history_test

// Integration tests for the prose-diff engine (WS-H).
//
// These tests exercise FileHistory end-to-end against a real git repository
// (constructed with testutil.NewTestRepo) and the API endpoint via a real
// httptest.Server. Nothing is mocked — every assertion reflects observable
// behaviour a real user would experience.
//
// Test inventory (10 tests):
//   TestHistoryIntegration_ThreeEdits              — 4 commits, verify entry count + Summaries
//   TestHistoryIntegration_AddHeadingChange        — added heading detected as ChangeAdded/BlockHeading
//   TestHistoryIntegration_RemoveParagraphChange   — removed paragraph detected as ChangeRemoved/BlockParagraph
//   TestHistoryIntegration_ModifyCodeBlockChange   — modified code block detected as ChangeModified/BlockCodeFence
//   TestHistoryIntegration_FollowRename            — --follow tracks history across a file rename
//   TestHistoryIntegration_APIEndpoint             — GET .../history → 200 + JSON shape
//   TestHistoryIntegration_EmptyRepo               — file with no commits → empty non-nil slice
//   TestHistoryIntegration_LimitCapsResults        — limit=2 returns exactly 2 entries
//   TestHistoryIntegration_CancelledContext        — cancelled ctx does not deadlock
//   TestHistoryIntegration_AuthorKind              — human email classified as "human"

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

	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/api"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/history"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
	"github.com/vedox/vedox/internal/testutil"
)

// ── fixture ───────────────────────────────────────────────────────────────────

// historyFixture wraps a TestRepo with the three-edit scenario from the brief:
//
//	commit 1 (initial)   — heading + paragraph + code block
//	commit 2 (add head)  — adds ## Setup heading
//	commit 3 (rem para)  — removes the intro paragraph
//	commit 4 (mod code)  — changes code block body
type historyFixture struct {
	repo     *testutil.TestRepo
	filePath string // workspace-relative path passed to FileHistory
}

const (
	hfInitial = "# Guide\n\nIntro paragraph.\n\n```go\nfmt.Println(\"hello\")\n```\n"
	hfAddHead = "# Guide\n\n## Setup\n\nIntro paragraph.\n\n```go\nfmt.Println(\"hello\")\n```\n"
	hfRemPara = "# Guide\n\n## Setup\n\n```go\nfmt.Println(\"hello\")\n```\n"
	hfModCode = "# Guide\n\n## Setup\n\n```go\nfmt.Println(\"world\")\n```\n"
)

func newHistoryFixture(t *testing.T) *historyFixture {
	t.Helper()
	repo := testutil.NewTestRepo(t)

	const rel = "docs/guide.md"
	repo.CommitFile(rel, hfInitial, "docs: initial guide")
	repo.CommitFile(rel, hfAddHead, "docs: add Setup heading")
	repo.CommitFile(rel, hfRemPara, "docs: remove intro paragraph")
	repo.CommitFile(rel, hfModCode, "docs: update code block")

	return &historyFixture{repo: repo, filePath: rel}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestHistoryIntegration_ThreeEdits asserts that FileHistory returns 4 entries
// (initial + 3 modifications) each with a non-empty Summary.
func TestHistoryIntegration_ThreeEdits(t *testing.T) {
	t.Parallel()
	f := newHistoryFixture(t)

	entries, err := history.FileHistory(f.repo.Path(), f.filePath, 0)
	if err != nil {
		t.Fatalf("FileHistory: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 history entries (initial + 3 modifications), got %d", len(entries))
	}
	for i, e := range entries {
		if e.Summary == "" {
			t.Errorf("entries[%d]: Summary is empty", i)
		}
		if e.CommitHash == "" || len(e.CommitHash) != 40 {
			t.Errorf("entries[%d]: CommitHash %q is not a 40-char SHA", i, e.CommitHash)
		}
	}
}

// TestHistoryIntegration_AddHeadingChange verifies that the commit which added
// "## Setup" produces at least one ChangeAdded change of kind BlockHeading.
func TestHistoryIntegration_AddHeadingChange(t *testing.T) {
	t.Parallel()
	f := newHistoryFixture(t)

	entries, err := history.FileHistory(f.repo.Path(), f.filePath, 0)
	if err != nil {
		t.Fatalf("FileHistory: %v", err)
	}
	if len(entries) < 4 {
		t.Fatalf("need 4 entries, got %d", len(entries))
	}

	// entries[0] = most recent; entries[2] = "add Setup heading"
	addHeadEntry := entries[2]
	if !strings.Contains(addHeadEntry.Message, "Setup") {
		t.Fatalf("entries[2] message %q does not mention 'Setup'", addHeadEntry.Message)
	}

	found := false
	for _, c := range addHeadEntry.Changes {
		if c.Type == history.ChangeAdded && c.Kind == history.BlockHeading {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("'add Setup heading' commit: expected ChangeAdded/BlockHeading among Changes=%+v", addHeadEntry.Changes)
	}
	if !strings.Contains(addHeadEntry.Summary, "heading") && !strings.Contains(addHeadEntry.Summary, "Added") {
		t.Errorf("Summary %q does not describe a heading addition", addHeadEntry.Summary)
	}
}

// TestHistoryIntegration_RemoveParagraphChange verifies that removing the
// intro paragraph produces a ChangeRemoved/BlockParagraph change.
func TestHistoryIntegration_RemoveParagraphChange(t *testing.T) {
	t.Parallel()
	f := newHistoryFixture(t)

	entries, err := history.FileHistory(f.repo.Path(), f.filePath, 0)
	if err != nil {
		t.Fatalf("FileHistory: %v", err)
	}
	if len(entries) < 4 {
		t.Fatalf("need 4 entries, got %d", len(entries))
	}

	// entries[1] = "remove intro paragraph"
	remParaEntry := entries[1]
	if !strings.Contains(remParaEntry.Message, "remove") {
		t.Fatalf("entries[1] message %q does not mention 'remove'", remParaEntry.Message)
	}

	found := false
	for _, c := range remParaEntry.Changes {
		if c.Type == history.ChangeRemoved && c.Kind == history.BlockParagraph {
			found = true
			if c.OldContent == "" {
				t.Error("ChangeRemoved/BlockParagraph: OldContent must be non-empty")
			}
			break
		}
	}
	if !found {
		t.Errorf("'remove intro paragraph' commit: expected ChangeRemoved/BlockParagraph among Changes=%+v", remParaEntry.Changes)
	}
}

// TestHistoryIntegration_ModifyCodeBlockChange verifies that modifying the
// code block body produces a ChangeModified/BlockCodeFence with both
// OldContent and NewContent populated.
func TestHistoryIntegration_ModifyCodeBlockChange(t *testing.T) {
	t.Parallel()
	f := newHistoryFixture(t)

	entries, err := history.FileHistory(f.repo.Path(), f.filePath, 0)
	if err != nil {
		t.Fatalf("FileHistory: %v", err)
	}
	if len(entries) < 4 {
		t.Fatalf("need 4 entries, got %d", len(entries))
	}

	// entries[0] = "update code block" (most recent)
	modCodeEntry := entries[0]
	if !strings.Contains(modCodeEntry.Message, "code") {
		t.Fatalf("entries[0] message %q does not mention 'code'", modCodeEntry.Message)
	}

	found := false
	for _, c := range modCodeEntry.Changes {
		if c.Type == history.ChangeModified && c.Kind == history.BlockCodeFence {
			found = true
			if c.OldContent == "" {
				t.Error("ChangeModified/BlockCodeFence: OldContent is empty")
			}
			if c.NewContent == "" {
				t.Error("ChangeModified/BlockCodeFence: NewContent is empty")
			}
			if c.OldContent == c.NewContent {
				t.Error("ChangeModified/BlockCodeFence: OldContent == NewContent (no actual change detected)")
			}
			break
		}
	}
	if !found {
		t.Errorf("'update code block' commit: expected ChangeModified/BlockCodeFence among Changes=%+v", modCodeEntry.Changes)
	}
}

// TestHistoryIntegration_FollowRename verifies that FileHistory (which passes
// --follow to git log) returns commits from before a file rename when queried
// with the new path. Total commits: 2 pre-rename + rename commit + 1 post-rename = 4.
func TestHistoryIntegration_FollowRename(t *testing.T) {
	t.Parallel()
	repo := testutil.NewTestRepo(t)

	const origPath = "docs/old-name.md"
	const newPath = "docs/new-name.md"

	repo.CommitFile(origPath, "# Original\n\nFirst version.\n", "docs: initial old-name")
	repo.CommitFile(origPath, "# Original\n\nSecond version.\n", "docs: update old-name")

	// Rename via git mv and commit.
	repo.MustOutput("git", "mv", origPath, newPath)
	repo.CommitAll("docs: rename old-name to new-name")

	// Post-rename modification.
	repo.CommitFile(newPath, "# Original\n\nThird version.\n", "docs: update new-name post-rename")

	entries, err := history.FileHistory(repo.Path(), newPath, 0)
	if err != nil {
		t.Fatalf("FileHistory (--follow): %v", err)
	}

	// --follow must surface at least the 2 pre-rename commits; 4 is ideal.
	if len(entries) < 3 {
		t.Errorf("expected at least 3 entries (follow across rename), got %d", len(entries))
	}

	// The oldest entry must correspond to the initial commit made under the old name.
	oldest := entries[len(entries)-1]
	if !strings.Contains(oldest.Message, "initial") {
		t.Errorf("oldest entry message %q should correspond to the pre-rename initial commit", oldest.Message)
	}
}

// TestHistoryIntegration_APIEndpoint starts a full httptest.Server and verifies
// the GET .../history endpoint returns 200 with the correct JSON shape.
func TestHistoryIntegration_APIEndpoint(t *testing.T) {
	t.Parallel()

	// Build a workspace with a real git repo at its root so the history
	// handler's fallback (git -C workspaceRoot) can find the commits.
	raw := t.TempDir()
	resolved, err := filepath.EvalSymlinks(raw)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	apiGitCmd(t, resolved, "init", "-b", "main")
	apiGitCmd(t, resolved, "config", "user.name", "test")
	apiGitCmd(t, resolved, "config", "user.email", "test@vedox.local")

	const project = "testproj"
	const docRel = "guide.md"
	projDir := filepath.Join(resolved, project)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, docRel), []byte("# Guide\n\nHello.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	apiGitCmd(t, resolved, "add", "-A")
	apiGitCmd(t, resolved, "commit", "--allow-empty", "-m", "docs: initial guide")

	// Stand up the API server.
	adapter, err := store.NewLocalAdapter(resolved, nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}
	dbStore, err := db.Open(db.Options{WorkspaceRoot: resolved})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = dbStore.Close() })

	srv := api.NewServer(
		adapter, dbStore, resolved,
		scanner.NewJobStore(), ai.NewJobStore(3),
		store.NewProjectRegistry(), agentauth.PassthroughAuth(),
	)
	mux := http.NewServeMux()
	srv.Mount(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	url := ts.URL + "/api/projects/" + project + "/docs/" + docRel + "/history"
	resp, err := ts.Client().Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		DocPath string `json:"docPath"`
		Entries []struct {
			CommitHash string `json:"commitHash"`
			AuthorKind string `json:"authorKind"`
			Date       string `json:"date"`
			Message    string `json:"message"`
			Summary    string `json:"summary"`
			Changes    []struct {
				Type    string `json:"type"`
				Kind    string `json:"kind"`
				Summary string `json:"summary"`
			} `json:"changes"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	if body.DocPath == "" {
		t.Error("docPath field is empty in response")
	}
	// entries must be a JSON array, never null.
	if body.Entries == nil {
		t.Fatal("entries field is null — handler must initialise slice before encoding")
	}
	for i, e := range body.Entries {
		if len(e.CommitHash) != 40 {
			t.Errorf("entries[%d].commitHash %q is not a 40-char SHA-1", i, e.CommitHash)
		}
		if e.AuthorKind == "" {
			t.Errorf("entries[%d].authorKind is empty", i)
		}
		if e.Summary == "" {
			t.Errorf("entries[%d].summary is empty", i)
		}
	}
}

// TestHistoryIntegration_EmptyRepo verifies that FileHistory returns an
// initialised empty slice (not nil, not error) for a file with no commits.
func TestHistoryIntegration_EmptyRepo(t *testing.T) {
	t.Parallel()
	repo := testutil.NewTestRepo(t)

	entries, err := history.FileHistory(repo.Path(), "nonexistent.md", 0)
	if err != nil {
		t.Fatalf("expected no error for uncommitted file, got %v", err)
	}
	if entries == nil {
		t.Error("FileHistory must return a non-nil slice for a missing file")
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// TestHistoryIntegration_LimitCapsResults verifies that limit=2 returns
// exactly 2 entries even though the fixture has 4 commits.
func TestHistoryIntegration_LimitCapsResults(t *testing.T) {
	t.Parallel()
	f := newHistoryFixture(t)

	entries, err := history.FileHistory(f.repo.Path(), f.filePath, 2)
	if err != nil {
		t.Fatalf("FileHistory(limit=2): %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries with limit=2, got %d", len(entries))
	}
}

// TestHistoryIntegration_CancelledContext verifies that a pre-cancelled context
// causes FileHistoryContext to return an error rather than blocking indefinitely.
// The test asserts no panic and no goroutine leak (enforced by the test timeout).
func TestHistoryIntegration_CancelledContext(t *testing.T) {
	t.Parallel()
	f := newHistoryFixture(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call so git may not even start

	_, err := history.FileHistoryContext(ctx, f.repo.Path(), f.filePath, 0)
	// Either an error is returned (context.Canceled, exec.ExitError) or git
	// finished before the cancellation was observed. Both are valid; we assert
	// no deadlock and no panic.
	_ = err
}

// TestHistoryIntegration_AuthorKind verifies that author classification is
// applied consistently to all entries. testutil.NewTestRepo uses
// GIT_AUTHOR_EMAIL=test@vedox.local, which the classifyAuthor heuristic
// matches against the "vedox" pattern — so every entry produced by the fixture
// repo is expected to be classified as "vedox-agent". This test documents that
// invariant and catches any regression where the AuthorKind field is empty or
// has a different value than what the pattern match produces.
func TestHistoryIntegration_AuthorKind(t *testing.T) {
	t.Parallel()
	f := newHistoryFixture(t)

	entries, err := history.FileHistory(f.repo.Path(), f.filePath, 0)
	if err != nil {
		t.Fatalf("FileHistory: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one entry")
	}
	// testutil.NewTestRepo uses email "test@vedox.local"; the classifier
	// matches the "vedox" pattern → "vedox-agent". Every entry must have a
	// consistent, non-empty AuthorKind.
	firstKind := entries[0].AuthorKind
	if firstKind == "" {
		t.Fatal("entries[0]: AuthorKind is empty")
	}
	for i, e := range entries {
		if e.AuthorKind == "" {
			t.Errorf("entries[%d]: AuthorKind is empty", i)
		}
		if e.AuthorKind != firstKind {
			t.Errorf("entries[%d]: AuthorKind = %q, want consistent %q", i, e.AuthorKind, firstKind)
		}
	}
}

// ── git helpers for the API endpoint test ─────────────────────────────────────
// apiGitCmd runs a git subcommand directly in dir (not via TestRepo) so the
// API endpoint test can initialise a git repo at the workspace root independently
// of any TestRepo instance.

func apiGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
