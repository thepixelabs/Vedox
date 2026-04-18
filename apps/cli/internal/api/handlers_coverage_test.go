package api

// Coverage tests for previously-uncovered API handlers:
//   - handleBrowse
//   - handleListDocs
//   - handleDocMetadata
//   - handleCreateProject
//   - handleLinkProject
//
// Each test uses a real httptest.Server built from the same stack as the
// integration tests (NewServer + real LocalAdapter + real SQLite store).

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// coverageFixture is the local equivalent of the external api_test.testFixture.
// It is defined here so package-api tests can use it without importing api_test.
type coverageFixture struct {
	server        *httptest.Server
	workspaceRoot string
	dbStore       *db.Store
}

// coverageBrowseToken is the bootstrap token injected into coverage-test servers
// so that /api/browse auth tests can supply a valid credential.
const coverageBrowseToken = "c0ffee00c0ffee00c0ffee00c0ffee00c0ffee00c0ffee00c0ffee00c0ffee00"

// newCoverageServer mirrors newTestServer from api_integration_test.go but lives
// in package api so unexported helpers remain accessible.
func newCoverageServer(t *testing.T) *coverageFixture {
	t.Helper()

	raw := t.TempDir()
	resolved, err := filepath.EvalSymlinks(raw)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", raw, err)
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

	jobStore := scanner.NewJobStore()
	srv := NewServer(
		adapter,
		dbStore,
		resolved,
		jobStore,
		ai.NewJobStore(3),
		store.NewProjectRegistry(),
		agentauth.PassthroughAuth(),
	)
	// FIX-SEC-01: wire the bootstrap token so /api/browse is properly guarded
	// even in coverage tests. Tests that call getWithToken supply this value.
	srv.SetBootstrapToken(coverageBrowseToken)

	mux := http.NewServeMux()
	srv.Mount(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return &coverageFixture{
		server:        ts,
		workspaceRoot: resolved,
		dbStore:       dbStore,
	}
}

// get issues a GET request and returns the response. The body is auto-closed
// on test cleanup.
func (f *coverageFixture) get(t *testing.T, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.server.URL+path, nil)
	if err != nil {
		t.Fatalf("NewRequest GET %s: %v", path, err)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// getWithToken issues a GET request with the bootstrap Bearer token set.
// Use this for /api/browse tests (FIX-SEC-01).
func (f *coverageFixture) getWithToken(t *testing.T, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.server.URL+path, nil)
	if err != nil {
		t.Fatalf("NewRequest GET %s: %v", path, err)
	}
	req.Header.Set("Authorization", "Bearer "+coverageBrowseToken)
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// post issues a POST request with a JSON body and the CSRF-accepted Origin.
func (f *coverageFixture) post(t *testing.T, path string, body interface{}) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, f.server.URL+path, strings.NewReader(string(b)))
	if err != nil {
		t.Fatalf("NewRequest POST %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// bodyStr reads the full response body as a string.
func bodyStr(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

// ── handleBrowse ──────────────────────────────────────────────────────────────

// TestBrowse_ValidDir lists the user's home directory and checks the response
// shape. The workspaceRoot (a t.TempDir) lives outside $HOME on most platforms,
// so we browse $HOME directly — which is always within the boundary.
func TestBrowse_ValidDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("os.UserHomeDir unavailable")
	}
	f := newCoverageServer(t)

	resp := f.getWithToken(t, "/api/browse?path="+home)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}

	var got browseResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if got.Path == "" {
		t.Errorf("path field is empty")
	}
	// directories must be a non-null array (may legitimately be empty).
	if got.Directories == nil {
		t.Error("directories must be a non-null array")
	}
}

// TestBrowse_OutsideHome expects a 403 for any path outside $HOME (FIX-SEC-01).
// We use / (filesystem root) as a canonical out-of-boundary path.
func TestBrowse_InvalidDir(t *testing.T) {
	f := newCoverageServer(t)

	// / is always outside $HOME; the boundary check fires before any filesystem
	// access, so no real I/O occurs and the test is hermetic.
	resp := f.getWithToken(t, "/api/browse?path=/")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
}

// TestBrowse_DefaultsToHome checks that omitting ?path returns 200 and a valid
// browseResponse (the default is the user's home directory).
func TestBrowse_DefaultsToHome(t *testing.T) {
	if _, err := os.UserHomeDir(); err != nil {
		t.Skip("os.UserHomeDir unavailable")
	}
	f := newCoverageServer(t)

	resp := f.getWithToken(t, "/api/browse")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
	var got browseResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if got.Path == "" {
		t.Error("expected non-empty path in default browse response")
	}
	if got.Directories == nil {
		// Directories may be empty but must not be Go nil (would JSON-encode as null).
		t.Error("directories must be a non-null array")
	}
}

// ── handleListDocs ────────────────────────────────────────────────────────────

// TestListDocs_WithFiles seeds a project directory with .md files and checks
// that GET /api/projects/{project}/docs/ returns them.
func TestListDocs_WithFiles(t *testing.T) {
	f := newCoverageServer(t)

	// Seed files.
	projDir := filepath.Join(f.workspaceRoot, "testproj")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for _, name := range []string{"readme.md", "guide.md"} {
		if err := os.WriteFile(filepath.Join(projDir, name), []byte("# "+name), 0o644); err != nil {
			t.Fatalf("writeFile %s: %v", name, err)
		}
	}

	resp := f.get(t, "/api/projects/testproj/docs/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}

	var docs []docResponse
	if err := json.NewDecoder(resp.Body).Decode(&docs); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if len(docs) < 2 {
		t.Errorf("expected at least 2 docs, got %d", len(docs))
	}
}

// TestListDocs_EmptyProject checks that a project with no .md files returns []
// rather than null.
func TestListDocs_EmptyProject(t *testing.T) {
	f := newCoverageServer(t)

	projDir := filepath.Join(f.workspaceRoot, "emptyproj")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	resp := f.get(t, "/api/projects/emptyproj/docs/")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
	body := strings.TrimSpace(bodyStr(t, resp))
	if body == "null" {
		t.Error("body is null — empty doc list must be [] not null")
	}
	if !strings.HasPrefix(body, "[") {
		t.Errorf("body = %q, want JSON array", body)
	}
}

// ── handleDocMetadata ─────────────────────────────────────────────────────────

// TestDocMetadata_ReturnsOK seeds a file in a non-git workspace and verifies
// the endpoint returns 200 with the expected JSON structure. The git fields
// will be empty strings / empty arrays because the workspace is not a git repo,
// but the handler should not error out.
func TestDocMetadata_ReturnsOK(t *testing.T) {
	f := newCoverageServer(t)

	projDir := filepath.Join(f.workspaceRoot, "metaproj")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "doc.md"), []byte("# doc"), 0o644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}

	resp := f.get(t, "/api/projects/metaproj/docs/doc.md/metadata")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}

	var meta DocMetadataResponse
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	// Contributors must be a non-null array even when git has no history.
	if meta.Contributors == nil {
		t.Error("contributors must be a non-null array")
	}
}

// TestDocMetadata_MissingDocPath checks that calling the metadata endpoint
// without a real file path below the project returns 400.
func TestDocMetadata_MissingDocPath(t *testing.T) {
	f := newCoverageServer(t)

	// Request path ends in /metadata but there is no doc path segment before it,
	// so chi will route to handleDocMetadata with docPath == "metadata". The
	// handler then strips the "/metadata" suffix, leaving an empty doc path which
	// fails validateDocPath.
	resp := f.get(t, "/api/projects/metaproj/docs/metadata")
	// The handler returns 400 when the doc path is invalid.
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
}

// ── handleCreateProject ───────────────────────────────────────────────────────

// TestCreateProject_ValidName creates a project and checks the directory is
// created on disk and the JSON response is correct.
func TestCreateProject_ValidName(t *testing.T) {
	f := newCoverageServer(t)

	resp := f.post(t, "/api/projects", map[string]string{"name": "newproj"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}

	var got createProjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if got.Name != "newproj" {
		t.Errorf("name = %q, want %q", got.Name, "newproj")
	}

	// Confirm the directory was actually created.
	if _, err := os.Stat(filepath.Join(f.workspaceRoot, "newproj")); os.IsNotExist(err) {
		t.Errorf("project directory was not created on disk")
	}
}

// TestCreateProject_EmptyName rejects an empty name with 400 VDX-300.
func TestCreateProject_EmptyName(t *testing.T) {
	f := newCoverageServer(t)

	resp := f.post(t, "/api/projects", map[string]string{"name": ""})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
	body := bodyStr(t, resp)
	if !strings.Contains(body, "VDX-300") {
		t.Errorf("expected VDX-300 in body, got %s", body)
	}
}

// TestCreateProject_PathSeparatorRejected ensures a name containing "/" is
// rejected with 400 VDX-300.
func TestCreateProject_PathSeparatorRejected(t *testing.T) {
	f := newCoverageServer(t)

	resp := f.post(t, "/api/projects", map[string]string{"name": "evil/traversal"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
	body := bodyStr(t, resp)
	if !strings.Contains(body, "VDX-300") {
		t.Errorf("expected VDX-300 in body, got %s", body)
	}
}

// TestCreateProject_Duplicate returns 409 when the directory already exists.
func TestCreateProject_Duplicate(t *testing.T) {
	f := newCoverageServer(t)

	// Pre-create the directory.
	if err := os.MkdirAll(filepath.Join(f.workspaceRoot, "existing"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	resp := f.post(t, "/api/projects", map[string]string{"name": "existing"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
}

// ── handleLinkProject ─────────────────────────────────────────────────────────

// TestLinkProject_MissingFields returns 400 when required fields are absent.
func TestLinkProject_MissingFields(t *testing.T) {
	f := newCoverageServer(t)

	// Empty body — both fields missing.
	resp := f.post(t, "/api/link", map[string]string{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
}

// TestLinkProject_MissingProjectName returns 400 when projectName is empty.
func TestLinkProject_MissingProjectName(t *testing.T) {
	f := newCoverageServer(t)

	ext := t.TempDir()
	resp := f.post(t, "/api/link", map[string]string{
		"externalRoot": ext,
		"projectName":  "",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
}

// TestLinkProject_MissingExternalRoot returns 400 when externalRoot is empty.
func TestLinkProject_MissingExternalRoot(t *testing.T) {
	f := newCoverageServer(t)

	resp := f.post(t, "/api/link", map[string]string{
		"externalRoot": "",
		"projectName":  "someproj",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
}

// TestLinkProject_HappyPath links a real temporary directory as an external
// project and checks the 200 response.
func TestLinkProject_HappyPath(t *testing.T) {
	f := newCoverageServer(t)

	// External directory must be outside the workspace root.
	ext := t.TempDir()
	extResolved, err := filepath.EvalSymlinks(ext)
	if err != nil {
		t.Fatalf("EvalSymlinks ext: %v", err)
	}
	// Write a markdown file so docCount > 0.
	if err := os.WriteFile(filepath.Join(extResolved, "README.md"), []byte("# ext"), 0o644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}

	resp := f.post(t, "/api/link", map[string]string{
		"externalRoot": extResolved,
		"projectName":  "linked-proj",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}

	var got linkResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if got.ProjectName != "linked-proj" {
		t.Errorf("projectName = %q, want %q", got.ProjectName, "linked-proj")
	}
	if got.DocCount < 1 {
		t.Errorf("docCount = %d, want >= 1", got.DocCount)
	}
}
