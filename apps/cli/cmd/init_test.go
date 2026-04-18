package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// execInit is a test helper that runs initCmd with args, capturing stdout.
// It resets initFlags.force before every call so tests are independent.
func execInit(t *testing.T, args ...string) (string, error) {
	t.Helper()
	initFlags.force = false
	buf := &bytes.Buffer{}
	initCmd.SetOut(buf)
	initCmd.SetErr(buf)
	initCmd.SetArgs(args)
	err := initCmd.RunE(initCmd, args)
	return buf.String(), err
}

// --- System-level init tests -------------------------------------------------

func TestSystemInit_CreatesVedoxHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	out, err := execInit(t) // no args → system init
	if err != nil {
		t.Fatalf("system init failed: %v", err)
	}

	// ~/.vedox/ must exist
	vedoxDir := filepath.Join(home, ".vedox")
	if fi, statErr := os.Stat(vedoxDir); statErr != nil || !fi.IsDir() {
		t.Errorf("expected %s to be a directory", vedoxDir)
	}

	// repos.json must exist
	reposJSON := filepath.Join(vedoxDir, "repos.json")
	if _, statErr := os.Stat(reposJSON); statErr != nil {
		t.Errorf("expected repos.json at %s", reposJSON)
	}

	// global.db must exist
	globalDB := filepath.Join(vedoxDir, "global.db")
	if _, statErr := os.Stat(globalDB); statErr != nil {
		t.Errorf("expected global.db at %s", globalDB)
	}

	if !strings.Contains(out, "vedox initialized") {
		t.Errorf("expected 'vedox initialized' in output, got: %q", out)
	}
}

func TestSystemInit_Idempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// First run
	if _, err := execInit(t); err != nil {
		t.Fatalf("first system init failed: %v", err)
	}

	// Second run — must succeed and report already initialized
	out, err := execInit(t)
	if err != nil {
		t.Fatalf("second system init failed: %v", err)
	}
	if !strings.Contains(out, "already initialized") {
		t.Errorf("expected 'already initialized' on second run, got: %q", out)
	}
}

func TestSystemInit_ForceReinitializes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := execInit(t); err != nil {
		t.Fatalf("first system init failed: %v", err)
	}

	// --force on second run should NOT say "already initialized"
	initFlags.force = true
	buf := &bytes.Buffer{}
	initCmd.SetOut(buf)
	err := initCmd.RunE(initCmd, []string{})
	if err != nil {
		t.Fatalf("force system init failed: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "already initialized") {
		t.Errorf("--force should suppress 'already initialized', got: %q", out)
	}
	if !strings.Contains(out, "vedox initialized") {
		t.Errorf("expected 'vedox initialized' after --force, got: %q", out)
	}
}

// --- Project-level init tests ------------------------------------------------

func TestProjectInit_BasicGitRepo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Ensure system is initialized first so the registry exists.
	if _, err := execInit(t); err != nil {
		t.Fatalf("system init failed: %v", err)
	}

	// Create a fake project with a .git directory.
	projectDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Add a couple of markdown files.
	if err := os.WriteFile(filepath.Join(projectDir, "README.md"), []byte("# hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "docs.md"), []byte("# docs"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := execInit(t, projectDir)
	if err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	// .vedox/ must be created inside the project.
	dotVedox := filepath.Join(projectDir, ".vedox")
	if fi, statErr := os.Stat(dotVedox); statErr != nil || !fi.IsDir() {
		t.Errorf("expected .vedox/ dir at %s", dotVedox)
	}

	// Git detection should be surfaced in output.
	if !strings.Contains(out, "git repo detected") {
		t.Errorf("expected 'git repo detected' in output, got: %q", out)
	}

	// Markdown count should reflect exactly 2 files.
	if !strings.Contains(out, "2 markdown file(s)") {
		t.Errorf("expected '2 markdown file(s)' in output, got: %q", out)
	}

	// Next-step hint should be present.
	if !strings.Contains(out, "vedox server start") {
		t.Errorf("expected 'vedox server start' hint in output, got: %q", out)
	}
}

func TestProjectInit_NonGitRepo(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := execInit(t); err != nil {
		t.Fatalf("system init failed: %v", err)
	}

	projectDir := t.TempDir()
	// No .git directory — plain directory.

	out, err := execInit(t, projectDir)
	if err != nil {
		t.Fatalf("project init failed: %v", err)
	}

	if strings.Contains(out, "git repo detected") {
		t.Errorf("should NOT report git repo for plain directory, got: %q", out)
	}
	if !strings.Contains(out, "project initialized") {
		t.Errorf("expected 'project initialized' in output, got: %q", out)
	}
}

func TestProjectInit_Idempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := execInit(t); err != nil {
		t.Fatalf("system init failed: %v", err)
	}

	projectDir := t.TempDir()
	if _, err := execInit(t, projectDir); err != nil {
		t.Fatalf("first project init failed: %v", err)
	}

	// Second run must not fail and must report already initialized.
	out, err := execInit(t, projectDir)
	if err != nil {
		t.Fatalf("second project init failed: %v", err)
	}
	if !strings.Contains(out, "already initialized") {
		t.Errorf("expected 'already initialized' on second project init, got: %q", out)
	}
}

func TestProjectInit_ForceReinitializes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := execInit(t); err != nil {
		t.Fatalf("system init failed: %v", err)
	}

	projectDir := t.TempDir()
	if _, err := execInit(t, projectDir); err != nil {
		t.Fatalf("first project init failed: %v", err)
	}

	// --force must succeed without "already initialized".
	initFlags.force = true
	buf := &bytes.Buffer{}
	initCmd.SetOut(buf)
	err := initCmd.RunE(initCmd, []string{projectDir})
	if err != nil {
		t.Fatalf("force project init failed: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "already initialized") {
		t.Errorf("--force should suppress 'already initialized', got: %q", out)
	}
}

func TestProjectInit_PathDoesNotExist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := execInit(t); err != nil {
		t.Fatalf("system init failed: %v", err)
	}

	_, err := execInit(t, "/this/path/does/not/exist/at/all")
	if err == nil {
		t.Fatal("expected error for non-existent path, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' in error, got: %v", err)
	}
}

func TestProjectInit_PathIsFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if _, err := execInit(t); err != nil {
		t.Fatalf("system init failed: %v", err)
	}

	f, err := os.CreateTemp(t.TempDir(), "vedox-test-*.md")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	_, initErr := execInit(t, f.Name())
	if initErr == nil {
		t.Fatal("expected error when path is a file, got nil")
	}
	if !strings.Contains(initErr.Error(), "not a directory") {
		t.Errorf("expected 'not a directory' in error, got: %v", initErr)
	}
}

// --- countMDFiles helper tests -----------------------------------------------

func TestCountMDFiles(t *testing.T) {
	root := t.TempDir()

	files := []string{
		filepath.Join(root, "README.md"),
		filepath.Join(root, "docs", "guide.md"),
		filepath.Join(root, "docs", "ref.md"),
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("# test"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// node_modules/.md should be excluded.
	nmMD := filepath.Join(root, "node_modules", "something.md")
	if err := os.MkdirAll(filepath.Dir(nmMD), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nmMD, []byte("# ignored"), 0o644); err != nil {
		t.Fatal(err)
	}

	// .hidden/.md should also be excluded.
	hiddenMD := filepath.Join(root, ".hidden", "secret.md")
	if err := os.MkdirAll(filepath.Dir(hiddenMD), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hiddenMD, []byte("# also ignored"), 0o644); err != nil {
		t.Fatal(err)
	}

	count := countMDFiles(root)
	if count != 3 {
		t.Errorf("expected 3 markdown files, got %d", count)
	}
}
