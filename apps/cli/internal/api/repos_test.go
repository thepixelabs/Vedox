package api

// Tests for the global repo registry HTTP endpoints:
//
//	GET  /api/repos   — handleListRepos
//	POST /api/repos   — handleCreateRepo
//
// Each sub-group tests both the happy path and error conditions.
// The fixture opens a real GlobalDB in a t.TempDir() for full integration
// coverage; no mocks are used.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// testReposToken is the bootstrap token the fixture installs on the server.
// All fixture-driven POSTs carry this value in Authorization: Bearer so the
// FIX-SEC-07 guard on /api/repos/create and /api/repos/register is satisfied.
// Tests that exercise the 401 path build their own request without going
// through the fixture helper.
const testReposToken = "b1c2d3e4f5a6b1c2d3e4f5a6b1c2d3e4f5a6b1c2d3e4f5a6b1c2d3e4f5a6b1c2"

// reposFixture builds an httptest.Server with a real GlobalDB injected.
type reposFixture struct {
	server *httptest.Server
	gdb    *db.GlobalDB
}

func newReposFixture(t *testing.T) *reposFixture {
	t.Helper()

	gdbPath := filepath.Join(t.TempDir(), "global.db")
	gdb, err := db.OpenGlobalDB(gdbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB: %v", err)
	}
	t.Cleanup(func() { _ = gdb.Close() })

	// We need a minimal workspace store for NewServer; reuse t.TempDir.
	wsRoot := t.TempDir()
	adapter, err := store.NewLocalAdapter(wsRoot, nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}
	wsDB, err := db.Open(db.Options{WorkspaceRoot: wsRoot})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = wsDB.Close() })

	srv := NewServer(adapter, wsDB, wsRoot, scanner.NewJobStore(), ai.NewJobStore(3), store.NewProjectRegistry(), agentauth.PassthroughAuth())
	srv.SetGlobalDB(gdb)
	srv.SetBootstrapToken(testReposToken)

	mux := http.NewServeMux()
	srv.Mount(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return &reposFixture{server: ts, gdb: gdb}
}

// get issues a GET to the test server.
func (f *reposFixture) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := f.server.Client().Get(f.server.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// post issues a POST with JSON body and the CORS-accepted Origin header.
// The bootstrap token is attached unconditionally so that tests hitting the
// FIX-SEC-07-protected endpoints (/api/repos/create, /api/repos/register)
// succeed. Endpoints that do not require the token simply ignore the header.
func (f *reposFixture) post(t *testing.T, path string, body interface{}) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, f.server.URL+path, strings.NewReader(string(b)))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	req.Header.Set("Authorization", "Bearer "+testReposToken)
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// ── GET /api/repos ────────────────────────────────────────────────────────────

// TestListRepos_Empty verifies that an empty registry returns [] not null.
func TestListRepos_Empty(t *testing.T) {
	f := newReposFixture(t)

	resp := f.get(t, "/api/repos")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got []repoResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got == nil {
		t.Error("body must be [] not null for empty registry")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 repos, got %d", len(got))
	}
}

// TestListRepos_WithData seeds the GlobalDB and verifies the list returns all rows.
func TestListRepos_WithData(t *testing.T) {
	f := newReposFixture(t)

	// Seed via POST so both handlers are exercised together.
	for _, body := range []map[string]string{
		{"name": "docs-private", "type": "private", "root_path": "/home/user/docs-private"},
		{"name": "docs-public", "type": "public", "root_path": "/home/user/docs-public"},
	} {
		resp := f.post(t, "/api/repos", body)
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("seed POST status = %d, want 201 for %v", resp.StatusCode, body)
		}
	}

	resp := f.get(t, "/api/repos")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got []repoResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 repos, got %d", len(got))
	}
}

// TestListRepos_StatusFilter verifies ?status narrows results.
func TestListRepos_StatusFilter(t *testing.T) {
	f := newReposFixture(t)

	// Create one active repo via POST.
	resp := f.post(t, "/api/repos", map[string]string{
		"name": "active-repo", "type": "private", "root_path": "/tmp/active",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed: status = %d", resp.StatusCode)
	}

	activeResp := f.get(t, "/api/repos?status=active")
	if activeResp.StatusCode != http.StatusOK {
		t.Fatalf("active filter status = %d", activeResp.StatusCode)
	}
	var active []repoResponse
	if err := json.NewDecoder(activeResp.Body).Decode(&active); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active repo, got %d", len(active))
	}

	archivedResp := f.get(t, "/api/repos?status=archived")
	if archivedResp.StatusCode != http.StatusOK {
		t.Fatalf("archived filter status = %d", archivedResp.StatusCode)
	}
	var archived []repoResponse
	if err := json.NewDecoder(archivedResp.Body).Decode(&archived); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(archived) != 0 {
		t.Errorf("expected 0 archived repos, got %d", len(archived))
	}
}

// ── POST /api/repos ───────────────────────────────────────────────────────────

// TestCreateRepo_HappyPath creates a private repo and checks the JSON response.
func TestCreateRepo_HappyPath(t *testing.T) {
	f := newReposFixture(t)

	resp := f.post(t, "/api/repos", map[string]string{
		"name":       "my-docs",
		"type":       "private",
		"root_path":  "/home/user/my-docs",
		"remote_url": "https://github.com/user/my-docs",
	})
	if resp.StatusCode != http.StatusCreated {
		b, _ := json.Marshal(nil)
		_ = b
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var got repoResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID == "" {
		t.Error("id must not be empty")
	}
	if got.Name != "my-docs" {
		t.Errorf("name = %q, want my-docs", got.Name)
	}
	if got.Type != "private" {
		t.Errorf("type = %q, want private", got.Type)
	}
	if got.Status != "active" {
		t.Errorf("status = %q, want active", got.Status)
	}
	if got.RemoteURL != "https://github.com/user/my-docs" {
		t.Errorf("remote_url = %q, want https://github.com/user/my-docs", got.RemoteURL)
	}
}

// TestCreateRepo_InboxType creates an inbox repo (no remote required).
func TestCreateRepo_InboxType(t *testing.T) {
	f := newReposFixture(t)

	resp := f.post(t, "/api/repos", map[string]string{
		"name":      "inbox",
		"type":      "inbox",
		"root_path": "/home/user/inbox",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var got repoResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Type != "inbox" {
		t.Errorf("type = %q, want inbox", got.Type)
	}
}

// TestCreateRepo_MissingName returns 400.
func TestCreateRepo_MissingName(t *testing.T) {
	f := newReposFixture(t)

	resp := f.post(t, "/api/repos", map[string]string{
		"type": "private", "root_path": "/tmp/x",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestCreateRepo_MissingRootPath returns 400.
func TestCreateRepo_MissingRootPath(t *testing.T) {
	f := newReposFixture(t)

	resp := f.post(t, "/api/repos", map[string]string{
		"name": "x", "type": "private",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestCreateRepo_MissingType returns 400.
func TestCreateRepo_MissingType(t *testing.T) {
	f := newReposFixture(t)

	resp := f.post(t, "/api/repos", map[string]string{
		"name": "x", "root_path": "/tmp/x",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestCreateRepo_InvalidType returns 400 for an unrecognised type value.
func TestCreateRepo_InvalidType(t *testing.T) {
	f := newReposFixture(t)

	resp := f.post(t, "/api/repos", map[string]string{
		"name": "x", "type": "enterprise", "root_path": "/tmp/x",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestCreateRepo_InvalidJSON returns 400 for a malformed request body.
func TestCreateRepo_InvalidJSON(t *testing.T) {
	f := newReposFixture(t)

	req, _ := http.NewRequest(http.MethodPost, f.server.URL+"/api/repos",
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

// ── nil globalDB (dev server mode) ───────────────────────────────────────────

// TestRepos_NilGlobalDB verifies both endpoints return 503 when no GlobalDB
// is injected (dev server mode).
func TestRepos_NilGlobalDB(t *testing.T) {
	wsRoot := t.TempDir()
	adapter, _ := store.NewLocalAdapter(wsRoot, nil)
	wsDB, _ := db.Open(db.Options{WorkspaceRoot: wsRoot})
	t.Cleanup(func() { _ = wsDB.Close() })

	srv := NewServer(adapter, wsDB, wsRoot, scanner.NewJobStore(), ai.NewJobStore(3), store.NewProjectRegistry(), agentauth.PassthroughAuth())
	// Deliberately do NOT call srv.SetGlobalDB — simulate dev server.

	mux := http.NewServeMux()
	srv.Mount(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	// GET /api/repos
	resp, err := ts.Client().Get(ts.URL + "/api/repos")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("GET /api/repos without globalDB: status = %d, want 503", resp.StatusCode)
	}

	// POST /api/repos
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/repos",
		strings.NewReader(`{"name":"x","type":"private","root_path":"/tmp/x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp2, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("POST /api/repos without globalDB: status = %d, want 503", resp2.StatusCode)
	}
}
