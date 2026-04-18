package api

// Security re-audit tests for /api/repos/{create,register} — verify that
// wave-0 withinHomeDir fix defeats not only "literal outside home" but also
// the symlink-escape vector that filepath.Abs misses.

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCreateRepoWithInit_SymlinkEscape verifies that a user-owned symlink
// inside $HOME pointing outside $HOME cannot be used as the scaffold target.
// Without the resolveExistingAncestor guard the mkdir+git-init would happen
// on the symlink-resolved path (e.g. /tmp/evil-repo) — a CWE-73 escape.
func TestCreateRepoWithInit_SymlinkEscape(t *testing.T) {
	f := newOnboardingFixture(t)

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}

	// Create a trap directory inside $HOME. Inside it place a symlink
	// pointing to /tmp — which is outside $HOME on every supported OS.
	trap := homeTempDir(t)
	symlink := filepath.Join(trap, "escape")
	if err := os.Symlink("/tmp", symlink); err != nil {
		t.Skipf("symlink creation failed: %v", err)
	}

	// Request scaffolding under the symlink. The handler's withinHomeDir
	// check sees a path starting with $HOME (pass), but resolveExistingAncestor
	// then EvalSymlinks the parent (/tmp) and re-checks — this must fail.
	target := filepath.Join(symlink, "new-repo")
	_ = home // reference to quiet unused-var if build mode ever changes

	resp := f.post(t, "/api/repos/create", map[string]interface{}{
		"name": "escape",
		"path": target,
		"type": "private",
	})

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for symlink-escape target (body=%s)",
			resp.StatusCode, drainBody(t, resp))
	}
	body := drainBody(t, resp)
	if !strings.Contains(body, "VDX-400") {
		t.Errorf("expected VDX-400, got %s", body)
	}

	// Make absolutely sure no repo was scaffolded at the symlink target.
	realTarget := filepath.Join("/tmp", "new-repo")
	if _, err := os.Stat(realTarget); err == nil {
		_ = os.RemoveAll(realTarget)
		t.Fatalf("symlink-escape target /tmp/new-repo was created — wave-0 guard bypassed")
	}
}

// TestRegisterRepo_SymlinkEscape is the register-side counterpart. An
// existing git repo at /tmp/fake-repo is wrapped by a symlink inside $HOME.
// The register endpoint must refuse to add it to GlobalDB.
func TestRegisterRepo_SymlinkEscape(t *testing.T) {
	f := newOnboardingFixture(t)

	// Build a fake "git repo" at /tmp/vedox-sym-target (.git dir is enough
	// for the handler's is-repo check).
	externalRepo, err := os.MkdirTemp("", "vedox-reaudit-fake-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(externalRepo) })
	if err := os.Mkdir(filepath.Join(externalRepo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	// Symlink inside $HOME pointing to the external repo.
	symlink := filepath.Join(homeTempDir(t), "linked-repo")
	if err := os.Symlink(externalRepo, symlink); err != nil {
		t.Skipf("symlink creation failed: %v", err)
	}

	resp := f.post(t, "/api/repos/register", map[string]interface{}{
		"path": symlink,
		"name": "should-not-register",
		"type": "private",
	})

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for symlink-escape register (body=%s)",
			resp.StatusCode, drainBody(t, resp))
	}
	body := drainBody(t, resp)
	if !strings.Contains(body, "VDX-400") {
		t.Errorf("expected VDX-400, got %s", body)
	}
	if strings.Contains(body, externalRepo) {
		t.Errorf("response leaked external path %q: %s", externalRepo, body)
	}
}

// TestCreateRepoWithInit_BodyTooLarge verifies that the MaxBytesReader guard
// rejects a 1 MB garbage body with 400 or 413 — never OOMs the daemon.
func TestCreateRepoWithInit_BodyTooLarge(t *testing.T) {
	f := newOnboardingFixture(t)

	// 128 KB body — above the 64 KB ceiling.
	payload := `{"name":"a","path":"/tmp/x","type":"private","junk":"` +
		strings.Repeat("A", 128*1024) + `"}`
	req, _ := http.NewRequest(http.MethodPost, f.server.URL+"/api/repos/create",
		strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()

	// Accept 400 (malformed after truncation) or 413 (explicit too-large).
	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		t.Fatalf("oversized body: got %d, want 4xx", resp.StatusCode)
	}
}
