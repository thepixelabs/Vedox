// Package testutil provides shared test infrastructure for the Vedox CLI.
//
// All helpers are test-only. None of the types or functions in this package
// are imported by production code.
package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepo is an ephemeral git repository created inside a t.TempDir().
// It is fully isolated from the host machine's git configuration: the system
// config, global config, and HOME are all redirected to a temp directory so
// no ~/.gitconfig values bleed into test runs.
//
// Create one with NewTestRepo(t). Cleanup is registered automatically via
// t.Cleanup — callers do not need to call any Close or Cleanup method.
type TestRepo struct {
	root    string // absolute path to the working tree
	gitHome string // temp HOME used to isolate git identity
	t       *testing.T
}

// NewTestRepo creates a new isolated git repository in a temp directory,
// configures a deterministic user identity, and sets init.defaultBranch=main.
// The repository and its HOME directory are cleaned up automatically when the
// test ends.
func NewTestRepo(t *testing.T) *TestRepo {
	t.Helper()

	root := t.TempDir()
	gitHome := t.TempDir()

	r := &TestRepo{
		root:    root,
		gitHome: gitHome,
		t:       t,
	}

	// Resolve symlinks so paths are stable on macOS (/private/var/folders …).
	resolved, err := filepath.EvalSymlinks(root)
	if err == nil {
		r.root = resolved
	}

	r.mustRun("git", "init", "-b", "main")
	r.mustRun("git", "config", "user.name", "test")
	r.mustRun("git", "config", "user.email", "test@test.com")
	r.mustRun("git", "config", "init.defaultBranch", "main")

	return r
}

// Path returns the absolute path to the repository root (working tree).
func (r *TestRepo) Path() string {
	return r.root
}

// WriteFile writes content to path (relative to the repo root), creating any
// necessary parent directories.
func (r *TestRepo) WriteFile(path, content string) {
	r.t.Helper()
	abs := filepath.Join(r.root, path)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		r.t.Fatalf("testutil.WriteFile: mkdir %s: %v", filepath.Dir(abs), err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		r.t.Fatalf("testutil.WriteFile: write %s: %v", abs, err)
	}
}

// ReadFile reads and returns the content of path (relative to the repo root).
func (r *TestRepo) ReadFile(path string) string {
	r.t.Helper()
	abs := filepath.Join(r.root, path)
	b, err := os.ReadFile(abs)
	if err != nil {
		r.t.Fatalf("testutil.ReadFile: %s: %v", abs, err)
	}
	return string(b)
}

// CommitAll stages all changes (git add -A) and commits with message.
// It returns the full SHA-1 of the resulting commit.
func (r *TestRepo) CommitAll(message string) string {
	r.t.Helper()
	r.mustRun("git", "add", "-A")
	r.mustRun("git", "commit", "--allow-empty", "-m", message)
	return r.revParse("HEAD")
}

// CommitFile writes content to path (relative to repo root) and commits it in
// a single call. Returns the full SHA-1 of the resulting commit.
func (r *TestRepo) CommitFile(path, content, message string) string {
	r.t.Helper()
	r.WriteFile(path, content)
	return r.CommitAll(message)
}

// Log returns the commit messages of the last n commits, most-recent first.
// If the repo has fewer than n commits, all available messages are returned.
// An empty repository (no commits) returns nil without failing the test.
func (r *TestRepo) Log(n int) []string {
	r.t.Helper()
	cmd := exec.Command("git", "log", fmt.Sprintf("--max-count=%d", n), "--pretty=%s")
	cmd.Dir = r.root
	cmd.Env = r.gitEnv()
	out, err := cmd.Output()
	if err != nil {
		// git log exits 128 on a repo with no commits yet — treat as empty.
		return nil
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// Branch creates a new branch named name and checks it out.
func (r *TestRepo) Branch(name string) {
	r.t.Helper()
	r.mustRun("git", "checkout", "-b", name)
}

// Checkout switches the working tree to ref (branch name, tag, or SHA).
func (r *TestRepo) Checkout(ref string) {
	r.t.Helper()
	r.mustRun("git", "checkout", ref)
}

// MustOutput executes an arbitrary command in the repo root and returns
// combined stdout+stderr, failing the test on non-zero exit. It is exported so
// tests in external packages can inspect git state without needing to
// construct their own exec.Command.
func (r *TestRepo) MustOutput(name string, args ...string) string {
	r.t.Helper()
	return r.mustOutput(name, args...)
}

// revParse resolves ref to a full SHA-1 and returns it.
func (r *TestRepo) revParse(ref string) string {
	r.t.Helper()
	return strings.TrimSpace(r.mustOutput("git", "rev-parse", ref))
}

// gitEnv returns the environment slice that neutralises host git configuration
// for every command run inside the repository.
//
// Variables set:
//   - GIT_CONFIG_NOSYSTEM=1   — ignore /etc/gitconfig
//   - GIT_CONFIG_SYSTEM=/dev/null — belt-and-suspenders for older git versions
//   - GIT_CONFIG_GLOBAL=/dev/null — skip ~/.gitconfig on the host
//   - HOME=<gitHome>          — point git at our empty temp home
//   - GIT_AUTHOR_NAME / GIT_AUTHOR_EMAIL — deterministic identity
//   - GIT_COMMITTER_NAME / GIT_COMMITTER_EMAIL — same
func (r *TestRepo) gitEnv() []string {
	base := os.Environ()
	overrides := []string{
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"HOME=" + r.gitHome,
		"GIT_AUTHOR_NAME=Test Author",
		"GIT_AUTHOR_EMAIL=test@vedox.local",
		"GIT_COMMITTER_NAME=Test Author",
		"GIT_COMMITTER_EMAIL=test@vedox.local",
	}
	// Append overrides after base so they shadow anything inherited from the
	// host. Git (and most POSIX programs) use the last occurrence of a
	// duplicate variable.
	return append(base, overrides...)
}

// mustRun executes a git command in the repo root, failing the test if it
// exits non-zero. Stdout and stderr are captured and included in the failure
// message.
func (r *TestRepo) mustRun(name string, args ...string) {
	r.t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = r.root
	cmd.Env = r.gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("testutil: %s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

// mustOutput executes a git command and returns combined stdout, failing the
// test on non-zero exit.
func (r *TestRepo) mustOutput(name string, args ...string) string {
	r.t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = r.root
	cmd.Env = r.gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("testutil: %s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}
