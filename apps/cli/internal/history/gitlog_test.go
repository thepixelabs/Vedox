package history

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeTempRepo creates a temporary git repository with three commits that each
// modify the same file. Returns the repo root and the relative file path.
//
// Commit sequence:
//   commit 1 — initial: "# Doc\n\nFirst paragraph.\n"
//   commit 2 — add section: adds "## Setup\n\nInstall dependencies.\n"
//   commit 3 — modify paragraph: changes "First paragraph." → "Updated first paragraph."
func makeTempRepo(t *testing.T) (repoRoot, filePath string) {
	t.Helper()

	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		// Supply a minimal git config so tests run cleanly in CI.
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test User",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test User",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("command %v failed: %v\n%s", args, err, out)
		}
	}

	run("git", "init", "-b", "main")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "Test User")

	docFile := filepath.Join(dir, "docs", "guide.md")
	if err := os.MkdirAll(filepath.Dir(docFile), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Commit 1: initial content.
	write(t, docFile, "# Doc\n\nFirst paragraph.\n")
	run("git", "add", ".")
	run("git", "commit", "--no-gpg-sign", "-m", "docs: initial guide")

	// Commit 2: add section.
	write(t, docFile, "# Doc\n\nFirst paragraph.\n\n## Setup\n\nInstall dependencies.\n")
	run("git", "add", ".")
	run("git", "commit", "--no-gpg-sign", "-m", "docs: add Setup section")

	// Commit 3: modify opening paragraph.
	write(t, docFile, "# Doc\n\nUpdated first paragraph.\n\n## Setup\n\nInstall dependencies.\n")
	run("git", "add", ".")
	run("git", "commit", "--no-gpg-sign", "-m", "docs: update introduction")

	return dir, filepath.Join("docs", "guide.md")
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// ── FileHistory tests ─────────────────────────────────────────────────────────

func TestFileHistory_ReturnsThreeEntries(t *testing.T) {
	repoRoot, filePath := makeTempRepo(t)

	entries, err := FileHistory(repoRoot, filePath, 0)
	if err != nil {
		t.Fatalf("FileHistory: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(entries))
	}
}

func TestFileHistory_OrderedMostRecentFirst(t *testing.T) {
	repoRoot, filePath := makeTempRepo(t)

	entries, err := FileHistory(repoRoot, filePath, 0)
	if err != nil {
		t.Fatalf("FileHistory: %v", err)
	}
	if len(entries) < 2 {
		t.Fatalf("need at least 2 entries, got %d", len(entries))
	}
	// Most recent commit message should be "docs: update introduction".
	if !strings.Contains(entries[0].Message, "update introduction") {
		t.Errorf("expected most-recent commit first, got %q", entries[0].Message)
	}
}

func TestFileHistory_EntriesHaveRequiredFields(t *testing.T) {
	repoRoot, filePath := makeTempRepo(t)

	entries, err := FileHistory(repoRoot, filePath, 0)
	if err != nil {
		t.Fatalf("FileHistory: %v", err)
	}
	for i, e := range entries {
		if e.CommitHash == "" {
			t.Errorf("entry %d: empty CommitHash", i)
		}
		if len(e.CommitHash) != 40 {
			t.Errorf("entry %d: CommitHash length %d, want 40", i, len(e.CommitHash))
		}
		if e.Author == "" {
			t.Errorf("entry %d: empty Author", i)
		}
		if e.Date == "" {
			t.Errorf("entry %d: empty Date", i)
		}
		if e.AuthorKind == "" {
			t.Errorf("entry %d: empty AuthorKind", i)
		}
	}
}

func TestFileHistory_AuthorKindHuman(t *testing.T) {
	repoRoot, filePath := makeTempRepo(t)

	entries, err := FileHistory(repoRoot, filePath, 0)
	if err != nil {
		t.Fatalf("FileHistory: %v", err)
	}
	for i, e := range entries {
		if e.AuthorKind != "human" {
			t.Errorf("entry %d: expected AuthorKind 'human', got %q", i, e.AuthorKind)
		}
	}
}

func TestFileHistory_ChangesPopulated(t *testing.T) {
	repoRoot, filePath := makeTempRepo(t)

	entries, err := FileHistory(repoRoot, filePath, 0)
	if err != nil {
		t.Fatalf("FileHistory: %v", err)
	}
	// Every entry except the initial commit (last) should have Changes.
	// Commits 0 and 1 (newest) changed the file relative to a prior version.
	if len(entries) < 2 {
		t.Fatal("need at least 2 entries")
	}
	if len(entries[0].Changes) == 0 {
		t.Error("most-recent entry (modify introduction) should have changes")
	}
	if len(entries[1].Changes) == 0 {
		t.Error("second entry (add Setup section) should have changes")
	}
}

func TestFileHistory_SummaryNonEmpty(t *testing.T) {
	repoRoot, filePath := makeTempRepo(t)

	entries, err := FileHistory(repoRoot, filePath, 0)
	if err != nil {
		t.Fatalf("FileHistory: %v", err)
	}
	for i, e := range entries {
		if e.Summary == "" {
			t.Errorf("entry %d: empty Summary", i)
		}
	}
}

func TestFileHistory_LimitRespected(t *testing.T) {
	repoRoot, filePath := makeTempRepo(t)

	entries, err := FileHistory(repoRoot, filePath, 2)
	if err != nil {
		t.Fatalf("FileHistory: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries with limit=2, got %d", len(entries))
	}
}

func TestFileHistory_NonExistentFile(t *testing.T) {
	repoRoot, _ := makeTempRepo(t)

	entries, err := FileHistory(repoRoot, "nonexistent/file.md", 0)
	if err != nil {
		t.Fatalf("expected no error for non-existent file, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for non-existent file, got %d", len(entries))
	}
}

func TestFileHistory_NonExistentRepo(t *testing.T) {
	_, err := FileHistory("/tmp/definitely-does-not-exist-vedox-test", "file.md", 0)
	// We expect either an error or empty entries — either is acceptable; we should
	// never panic.
	_ = err
}

// ── classifyAuthor tests ───────────────────────────────────────────────────────

func TestClassifyAuthor(t *testing.T) {
	tests := []struct {
		email string
		body  string
		want  string
	}{
		{"alice@example.com", "", "human"},
		{"bot@anthropic.com", "", "claude-code"},
		{"claude@users.noreply.github.com", "", "claude-code"},
		{"copilot@github.com", "", "copilot"},
		{"codex@openai.com", "", "codex"},
		{"gemini@google.com", "", "gemini"},
		{"unknown@bot.io", "Co-Authored-By: Vedox Doc Agent <agent@vedox.dev>", "vedox-agent"},
		{"vedox-doc-agent@vedox.dev", "", "vedox-agent"},
		{"noreply+vedox@users.noreply.github.com", "", "vedox-agent"},
		{"vedox@example.com", "", "human"}, // bare "vedox" in email should NOT match — prevents false positives
	}
	for _, tt := range tests {
		got := classifyAuthor(tt.email, tt.body)
		if got != tt.want {
			t.Errorf("classifyAuthor(%q, %q) = %q, want %q", tt.email, tt.body, got, tt.want)
		}
	}
}

// ── gitVersion smoke test ─────────────────────────────────────────────────────

func TestGitVersion(t *testing.T) {
	ver, err := gitVersion(context.Background())
	if err != nil {
		t.Skipf("git not available in test environment: %v", err)
	}
	if !strings.HasPrefix(ver, "git version") {
		t.Errorf("unexpected git version output: %q", ver)
	}
}

// ── parseISO tests ────────────────────────────────────────────────────────────

func TestParseISO(t *testing.T) {
	tests := []struct {
		input    string
		wantSuffix string // RFC3339 ends in Z for UTC
	}{
		{"2026-04-15T10:22:00+00:00", "Z"},
		{"2026-04-15T10:22:00+02:00", "Z"},
		{"invalid", ""},
	}
	for _, tt := range tests {
		got := parseISO(tt.input)
		if tt.wantSuffix == "Z" && !strings.HasSuffix(got, "Z") {
			t.Errorf("parseISO(%q) = %q, want UTC RFC3339 (ending in Z)", tt.input, got)
		}
		if tt.wantSuffix == "" && got != tt.input {
			// For invalid input we return the original string unchanged.
			t.Errorf("parseISO(%q) = %q, want original string", tt.input, got)
		}
	}
}
