package api

// Tests for the onboarding repo endpoints:
//
//	POST /api/repos/create   — handleCreateRepoWithInit
//	POST /api/repos/register — handleRegisterRepo
//
// All tests use a real GlobalDB in t.TempDir() — no mocks.
// The git binary must be available in PATH (standard CI requirement).

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// newOnboardingFixture returns a test server with a real GlobalDB attached.
func newOnboardingFixture(t *testing.T) *reposFixture {
	t.Helper()
	return newReposFixture(t)
}

// homeTempDir creates a temporary directory inside the user's home directory
// and registers it for cleanup. Required because t.TempDir() on macOS resolves
// to /var/folders/... which is outside $HOME, so the withinHomeDir guard would
// reject it.
func homeTempDir(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	dir, err := os.MkdirTemp(home, ".vedox-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp in $HOME: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// ---------------------------------------------------------------------------
// POST /api/repos/create
// ---------------------------------------------------------------------------

// TestCreateRepoWithInit_HappyPath creates a new directory, runs git init,
// and registers in GlobalDB.
func TestCreateRepoWithInit_HappyPath(t *testing.T) {
	f := newOnboardingFixture(t)

	newRepoPath := filepath.Join(homeTempDir(t), "my-new-docs")

	resp := f.post(t, "/api/repos/create", map[string]interface{}{
		"name": "my-new-docs",
		"path": newRepoPath,
		"type": "private",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body=%s)", resp.StatusCode, drainBody(t, resp))
	}

	var got repoResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID == "" {
		t.Error("id must not be empty")
	}
	if got.Name != "my-new-docs" {
		t.Errorf("name = %q, want my-new-docs", got.Name)
	}
	if got.Type != "private" {
		t.Errorf("type = %q, want private", got.Type)
	}
	if got.Status != "active" {
		t.Errorf("status = %q, want active", got.Status)
	}
	if got.RootPath != newRepoPath {
		t.Errorf("root_path = %q, want %q", got.RootPath, newRepoPath)
	}

	// Verify the directory and .git were created on disk.
	if _, err := os.Stat(newRepoPath); err != nil {
		t.Errorf("directory not created: %v", err)
	}
	gitDir := filepath.Join(newRepoPath, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		t.Errorf("git repo not initialised (.git not found): %v", err)
	}
}

// TestCreateRepoWithInit_DirectoryAlreadyExists verifies git init is idempotent
// on an existing directory.
func TestCreateRepoWithInit_DirectoryAlreadyExists(t *testing.T) {
	f := newOnboardingFixture(t)

	existingDir := homeTempDir(t)

	resp := f.post(t, "/api/repos/create", map[string]interface{}{
		"name": "existing-dir",
		"path": existingDir,
		"type": "public",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	gitDir := filepath.Join(existingDir, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		t.Errorf(".git not found after init: %v", err)
	}
}

// TestCreateRepoWithInit_PrivateBoolFallback verifies the legacy `private`
// boolean defaults type to "private" when the `type` field is omitted.
func TestCreateRepoWithInit_PrivateBoolFallback(t *testing.T) {
	f := newOnboardingFixture(t)

	newRepoPath := filepath.Join(homeTempDir(t), "bool-fallback")
	resp := f.post(t, "/api/repos/create", map[string]interface{}{
		"name":    "bool-fallback",
		"path":    newRepoPath,
		"private": true,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var got repoResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Type != "private" {
		t.Errorf("type = %q, want private", got.Type)
	}
}

// TestCreateRepoWithInit_MissingName returns 400.
func TestCreateRepoWithInit_MissingName(t *testing.T) {
	f := newOnboardingFixture(t)
	resp := f.post(t, "/api/repos/create", map[string]interface{}{
		"path": t.TempDir(),
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestCreateRepoWithInit_MissingPath returns 400.
func TestCreateRepoWithInit_MissingPath(t *testing.T) {
	f := newOnboardingFixture(t)
	resp := f.post(t, "/api/repos/create", map[string]interface{}{
		"name": "no-path",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestCreateRepoWithInit_InvalidType returns 400.
func TestCreateRepoWithInit_InvalidType(t *testing.T) {
	f := newOnboardingFixture(t)
	resp := f.post(t, "/api/repos/create", map[string]interface{}{
		"name": "x", "path": homeTempDir(t), "type": "enterprise",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestCreateRepoWithInit_InvalidJSON returns 400.
func TestCreateRepoWithInit_InvalidJSON(t *testing.T) {
	f := newOnboardingFixture(t)
	req, _ := http.NewRequest(http.MethodPost, f.server.URL+"/api/repos/create",
		strings.NewReader("{not json}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestCreateRepoWithInit_NoGlobalDB returns 503.
func TestCreateRepoWithInit_NoGlobalDB(t *testing.T) {
	wsRoot := t.TempDir()
	adapter, _ := store.NewLocalAdapter(wsRoot, nil)
	wsDB, _ := db.Open(db.Options{WorkspaceRoot: wsRoot})
	t.Cleanup(func() { _ = wsDB.Close() })

	srv := NewServer(adapter, wsDB, wsRoot, scanner.NewJobStore(), ai.NewJobStore(3), store.NewProjectRegistry(), nil)

	mux := http.NewServeMux()
	srv.Mount(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/repos/create",
		strings.NewReader(`{"name":"x","path":"/tmp/x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// POST /api/repos/register
// ---------------------------------------------------------------------------

// TestRegisterRepo_HappyPath registers an existing git repo by path.
func TestRegisterRepo_HappyPath(t *testing.T) {
	f := newOnboardingFixture(t)

	repoDir := homeTempDir(t)
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	resp := f.post(t, "/api/repos/register", map[string]interface{}{
		"path": repoDir,
		"name": "existing-docs",
		"type": "public",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body=%s)", resp.StatusCode, drainBody(t, resp))
	}

	var got repoResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID == "" {
		t.Error("id must not be empty")
	}
	if got.Name != "existing-docs" {
		t.Errorf("name = %q, want existing-docs", got.Name)
	}
	if got.Type != "public" {
		t.Errorf("type = %q, want public", got.Type)
	}
	if got.RootPath != repoDir {
		t.Errorf("root_path = %q, want %q", got.RootPath, repoDir)
	}
}

// TestRegisterRepo_NameDefaults verifies the directory basename is used as
// the name when none is supplied.
func TestRegisterRepo_NameDefaults(t *testing.T) {
	f := newOnboardingFixture(t)

	// Create a subdirectory with a known name so filepath.Base is predictable.
	parent := homeTempDir(t)
	repoDir := filepath.Join(parent, "inferred-name")
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	resp := f.post(t, "/api/repos/register", map[string]interface{}{
		"path": repoDir,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var got repoResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "inferred-name" {
		t.Errorf("name = %q, want inferred-name", got.Name)
	}
}

// TestRegisterRepo_DefaultTypeIsPrivate verifies type defaults to "private".
func TestRegisterRepo_DefaultTypeIsPrivate(t *testing.T) {
	f := newOnboardingFixture(t)

	repoDir := homeTempDir(t)
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	resp := f.post(t, "/api/repos/register", map[string]interface{}{
		"path": repoDir,
		"name": "default-type",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var got repoResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Type != "private" {
		t.Errorf("type = %q, want private", got.Type)
	}
}

// TestRegisterRepo_NotAGitRepo returns 400 when the path has no .git entry.
func TestRegisterRepo_NotAGitRepo(t *testing.T) {
	f := newOnboardingFixture(t)

	plainDir := homeTempDir(t)
	resp := f.post(t, "/api/repos/register", map[string]interface{}{
		"path": plainDir,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestRegisterRepo_PathDoesNotExist returns 400.
func TestRegisterRepo_PathDoesNotExist(t *testing.T) {
	f := newOnboardingFixture(t)

	resp := f.post(t, "/api/repos/register", map[string]interface{}{
		"path": "/vedox-test-does-not-exist-xyz123",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestRegisterRepo_MissingPath returns 400.
func TestRegisterRepo_MissingPath(t *testing.T) {
	f := newOnboardingFixture(t)

	resp := f.post(t, "/api/repos/register", map[string]interface{}{
		"name": "no-path",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestRegisterRepo_InvalidType returns 400.
func TestRegisterRepo_InvalidType(t *testing.T) {
	f := newOnboardingFixture(t)

	repoDir := homeTempDir(t)
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	resp := f.post(t, "/api/repos/register", map[string]interface{}{
		"path": repoDir, "type": "enterprise",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestRegisterRepo_NoGlobalDB returns 503.
func TestRegisterRepo_NoGlobalDB(t *testing.T) {
	wsRoot := t.TempDir()
	adapter, _ := store.NewLocalAdapter(wsRoot, nil)
	wsDB, _ := db.Open(db.Options{WorkspaceRoot: wsRoot})
	t.Cleanup(func() { _ = wsDB.Close() })

	srv := NewServer(adapter, wsDB, wsRoot, scanner.NewJobStore(), ai.NewJobStore(3), store.NewProjectRegistry(), nil)

	mux := http.NewServeMux()
	srv.Mount(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/repos/register",
		strings.NewReader(`{"path":"/tmp"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Security: home-directory boundary guard (HIGH-01 / HIGH-04 — FIX-SEC-03)
// ---------------------------------------------------------------------------

// TestCreateRepoWithInit_OutsideHome rejects a path outside $HOME with 400.
func TestCreateRepoWithInit_OutsideHome(t *testing.T) {
	f := newOnboardingFixture(t)

	// /tmp is virtually never inside $HOME on macOS or Linux.
	outsidePath := "/tmp/vedox-sec-test-outside-home"
	resp := f.post(t, "/api/repos/create", map[string]interface{}{
		"name": "evil",
		"path": outsidePath,
		"type": "private",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for path outside $HOME (body=%s)",
			resp.StatusCode, drainBody(t, resp))
	}
	body := drainBody(t, resp)
	if !strings.Contains(body, "VDX-400") {
		t.Errorf("expected VDX-400 in body, got: %s", body)
	}
}

// TestRegisterRepo_OutsideHome rejects a path outside $HOME with 400.
func TestRegisterRepo_OutsideHome(t *testing.T) {
	f := newOnboardingFixture(t)

	resp := f.post(t, "/api/repos/register", map[string]interface{}{
		"path": "/tmp/vedox-sec-test-outside-home",
		"name": "evil",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for path outside $HOME (body=%s)",
			resp.StatusCode, drainBody(t, resp))
	}
	body := drainBody(t, resp)
	if !strings.Contains(body, "VDX-400") {
		t.Errorf("expected VDX-400 in body, got: %s", body)
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// drainBody reads resp.Body to a string for use in failure messages when the
// caller has not yet consumed the body.
func drainBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "(read error)"
	}
	return string(b)
}
