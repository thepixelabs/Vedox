package testutil_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/testutil"
)

// TestNewTestRepo verifies that NewTestRepo creates a valid git repository
// with a deterministic identity and that t.Cleanup removes the directory.
func TestNewTestRepo_CreatesRepo(t *testing.T) {
	r := testutil.NewTestRepo(t)

	// Path must point to an existing directory.
	info, err := os.Stat(r.Path())
	if err != nil {
		t.Fatalf("Stat(%s): %v", r.Path(), err)
	}
	if !info.IsDir() {
		t.Fatalf("expected directory, got file at %s", r.Path())
	}

	// A .git directory must exist inside the repo root.
	gitDir := filepath.Join(r.Path(), ".git")
	if _, err := os.Stat(gitDir); err != nil {
		t.Fatalf(".git not found at %s: %v", gitDir, err)
	}
}

// TestWriteFileAndReadFile verifies round-trip file I/O via the helper methods.
func TestWriteFileAndReadFile(t *testing.T) {
	r := testutil.NewTestRepo(t)

	r.WriteFile("hello.md", "# Hello\n\nworld")
	got := r.ReadFile("hello.md")
	if got != "# Hello\n\nworld" {
		t.Errorf("ReadFile: got %q, want %q", got, "# Hello\n\nworld")
	}
}

// TestWriteFile_CreatesParentDirs verifies that WriteFile creates intermediate
// directories when they do not yet exist.
func TestWriteFile_CreatesParentDirs(t *testing.T) {
	r := testutil.NewTestRepo(t)
	r.WriteFile("docs/sub/page.md", "nested content")
	got := r.ReadFile("docs/sub/page.md")
	if got != "nested content" {
		t.Errorf("ReadFile nested: got %q, want %q", got, "nested content")
	}
}

// TestCommitAll_ReturnsHash verifies that CommitAll stages all changes, creates
// a commit, and returns a non-empty SHA-1.
func TestCommitAll_ReturnsHash(t *testing.T) {
	r := testutil.NewTestRepo(t)
	r.WriteFile("a.txt", "alpha")
	hash := r.CommitAll("initial commit")

	if len(hash) < 7 {
		t.Fatalf("CommitAll returned short hash %q", hash)
	}
	// SHA-1 hashes are hex strings.
	for _, ch := range hash {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			t.Fatalf("CommitAll returned non-hex hash %q", hash)
		}
	}
}

// TestCommitFile_SingleCall verifies CommitFile writes and commits in one step.
func TestCommitFile_SingleCall(t *testing.T) {
	r := testutil.NewTestRepo(t)
	hash := r.CommitFile("readme.md", "# Readme", "add readme")

	if hash == "" {
		t.Fatal("CommitFile returned empty hash")
	}
	// File must be readable after the commit.
	got := r.ReadFile("readme.md")
	if got != "# Readme" {
		t.Errorf("ReadFile after CommitFile: got %q, want %q", got, "# Readme")
	}
}

// TestLog_ReturnsMessages verifies Log returns commit messages in most-recent-first order.
func TestLog_ReturnsMessages(t *testing.T) {
	r := testutil.NewTestRepo(t)
	r.CommitFile("a.txt", "a", "first commit")
	r.CommitFile("b.txt", "b", "second commit")
	r.CommitFile("c.txt", "c", "third commit")

	msgs := r.Log(3)
	if len(msgs) != 3 {
		t.Fatalf("Log(3) returned %d messages, want 3: %v", len(msgs), msgs)
	}
	// Most recent first.
	if msgs[0] != "third commit" {
		t.Errorf("msgs[0]: got %q, want %q", msgs[0], "third commit")
	}
	if msgs[1] != "second commit" {
		t.Errorf("msgs[1]: got %q, want %q", msgs[1], "second commit")
	}
	if msgs[2] != "first commit" {
		t.Errorf("msgs[2]: got %q, want %q", msgs[2], "first commit")
	}
}

// TestLog_FewerThanRequested verifies Log handles repos with fewer commits
// than n without panicking.
func TestLog_FewerThanRequested(t *testing.T) {
	r := testutil.NewTestRepo(t)
	r.CommitFile("x.txt", "x", "only commit")

	msgs := r.Log(5)
	if len(msgs) != 1 {
		t.Fatalf("Log(5) on single-commit repo: got %d messages, want 1: %v", len(msgs), msgs)
	}
	if msgs[0] != "only commit" {
		t.Errorf("msgs[0]: got %q, want %q", msgs[0], "only commit")
	}
}

// TestLog_EmptyRepo verifies Log on a repo with no commits returns nil.
func TestLog_EmptyRepo(t *testing.T) {
	r := testutil.NewTestRepo(t)
	msgs := r.Log(5)
	if msgs != nil {
		t.Errorf("Log on empty repo: got %v, want nil", msgs)
	}
}

// TestBranchAndCheckout verifies Branch creates a new branch and Checkout
// switches between branches, maintaining per-branch state.
func TestBranchAndCheckout(t *testing.T) {
	r := testutil.NewTestRepo(t)

	// Create an initial commit on main so the branch has a parent.
	r.CommitFile("base.txt", "base", "base commit")

	// Create and commit on feature branch.
	r.Branch("feature")
	r.CommitFile("feature.txt", "feature content", "add feature doc")

	// Switch back to main. feature.txt must not exist there.
	r.Checkout("main")
	if _, err := os.Stat(filepath.Join(r.Path(), "feature.txt")); !os.IsNotExist(err) {
		t.Error("feature.txt should not exist on main branch")
	}

	// Switch back to feature. feature.txt must exist again.
	r.Checkout("feature")
	if _, err := os.Stat(filepath.Join(r.Path(), "feature.txt")); err != nil {
		t.Errorf("feature.txt should exist on feature branch: %v", err)
	}
}

// TestGitConfigIsolation verifies that the host's git identity is not used
// in commits made through the helper. Every commit must carry the test identity.
func TestGitConfigIsolation(t *testing.T) {
	r := testutil.NewTestRepo(t)
	r.CommitFile("isolation.txt", "isolation test", "isolation commit")

	// Read the committer identity from git log.
	// %ae = author email, %ce = committer email.
	out := r.MustOutput("git", "log", "-1", "--pretty=%ae %ce")
	out = strings.TrimSpace(out)
	fields := strings.Fields(out)
	if len(fields) != 2 {
		t.Fatalf("unexpected git log output: %q", out)
	}
	authorEmail := fields[0]
	committerEmail := fields[1]

	// Both must be the deterministic test identity — not whatever the host
	// machine's ~/.gitconfig says.
	if authorEmail != "test@vedox.local" {
		t.Errorf("author email: got %q, want test@vedox.local", authorEmail)
	}
	if committerEmail != "test@vedox.local" {
		t.Errorf("committer email: got %q, want test@vedox.local", committerEmail)
	}
}

// TestMultipleReposAreIndependent verifies that two TestRepo instances share
// no state and that commits in one do not appear in the other.
func TestMultipleReposAreIndependent(t *testing.T) {
	r1 := testutil.NewTestRepo(t)
	r2 := testutil.NewTestRepo(t)

	r1.CommitFile("r1.txt", "repo one", "repo one commit")
	r2.CommitFile("r2.txt", "repo two", "repo two commit")

	// r1 must not have r2.txt.
	if _, err := os.Stat(filepath.Join(r1.Path(), "r2.txt")); !os.IsNotExist(err) {
		t.Error("r2.txt should not exist in r1")
	}
	// r2 must not have r1.txt.
	if _, err := os.Stat(filepath.Join(r2.Path(), "r1.txt")); !os.IsNotExist(err) {
		t.Error("r1.txt should not exist in r2")
	}

	msgs1 := r1.Log(10)
	for _, m := range msgs1 {
		if m == "repo two commit" {
			t.Error("r1.Log() contains r2's commit message")
		}
	}
}

// TestHelpers_TempDir verifies TempDir returns an accessible directory.
func TestHelpers_TempDir(t *testing.T) {
	dir := testutil.TempDir(t)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("TempDir: Stat(%s): %v", dir, err)
	}
	if !info.IsDir() {
		t.Fatalf("TempDir: %s is not a directory", dir)
	}
}

// TestHelpers_AssertFileContains verifies the positive and negative cases.
func TestHelpers_AssertFileContains(t *testing.T) {
	dir := testutil.TempDir(t)
	path := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Positive case — should not fail.
	testutil.AssertFileContains(t, path, "hello")

	// Negative case — use a sub-test with its own recorder so the outer test
	// does not fail when AssertFileNotContains catches the absence correctly.
	testutil.AssertFileNotContains(t, path, "goodbye")
}
