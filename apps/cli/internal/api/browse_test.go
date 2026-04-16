package api_test

// Tests for GET /api/browse — authentication and home-directory boundary.
//
// FIX-SEC-01 / CRIT-02 acceptance criteria:
//   1. No token             → 401 VDX-401
//   2. Wrong token          → 401 VDX-401
//   3. Token + path=/       → 403 VDX-403 (outside $HOME)
//   4. Token + path=$HOME   → 200, valid JSON
//   5. Token + no path      → 200, defaults to $HOME
//   6. Absolute path in error message must not be leaked on a 403 boundary
//      rejection (MED-02).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/api"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

const testBrowseToken = "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"

// newBrowseFixture is like newTestServer but also calls SetBootstrapToken so
// /api/browse is properly guarded. A real home directory is needed for the
// boundary check; we point the fixture at $HOME directly (read-only, no writes).
func newBrowseFixture(t *testing.T) *testFixture {
	t.Helper()

	raw := t.TempDir()
	resolved, err := filepath.EvalSymlinks(raw)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
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

	srv := api.NewServer(
		adapter,
		dbStore,
		resolved,
		scanner.NewJobStore(),
		ai.NewJobStore(3),
		store.NewProjectRegistry(),
		nil,
	)
	srv.SetBootstrapToken(testBrowseToken)

	mux := http.NewServeMux()
	srv.Mount(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	probe := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	apiHandler, _ := mux.Handler(probe)

	return &testFixture{
		server:        ts,
		workspaceRoot: resolved,
		dbStore:       dbStore,
		jobStore:      scanner.NewJobStore(),
		apiHandler:    apiHandler,
	}
}

// browseGet issues a GET /api/browse request with the given path param and
// Authorization header. Pass an empty authHeader to omit it.
func browseGet(t *testing.T, f *testFixture, pathParam, authHeader string) *http.Response {
	t.Helper()
	url := f.server.URL + "/api/browse"
	if pathParam != "" {
		url += "?path=" + pathParam
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /api/browse: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// ── Authentication tests ──────────────────────────────────────────────────────

// TestBrowse_NoToken asserts that a request with no Authorization header is
// rejected with 401 VDX-401. This is the CRIT-02 baseline.
func TestBrowse_NoToken(t *testing.T) {
	f := newBrowseFixture(t)
	resp := browseGet(t, f, "", "")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "VDX-401") {
		t.Errorf("expected VDX-401 in body, got %s", body)
	}
}

// TestBrowse_WrongToken asserts that a plausible but incorrect token is also
// rejected with 401 — the constant-time comparison must not short-circuit.
func TestBrowse_WrongToken(t *testing.T) {
	f := newBrowseFixture(t)
	resp := browseGet(t, f, "", "Bearer wrongtoken")

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "VDX-401") {
		t.Errorf("expected VDX-401 in body, got %s", body)
	}
}

// TestBrowse_MalformedAuthScheme asserts that a non-Bearer Authorization value
// is rejected with 401.
func TestBrowse_MalformedAuthScheme(t *testing.T) {
	f := newBrowseFixture(t)
	resp := browseGet(t, f, "", "Basic "+testBrowseToken)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
}

// ── Home-directory boundary tests ────────────────────────────────────────────

// TestBrowse_PathOutsideHome asserts that path=/ (filesystem root) is rejected
// with 403 VDX-403 even with a valid token. This is the CRIT-02 acceptance
// criterion: curl with token but path=/ → 403.
func TestBrowse_PathOutsideHome(t *testing.T) {
	f := newBrowseFixture(t)
	resp := browseGet(t, f, "/", "Bearer "+testBrowseToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "VDX-403") {
		t.Errorf("expected VDX-403 in body, got %s", body)
	}
}

// TestBrowse_PathEtc asserts that /etc is also blocked (CWE-552 acceptance
// test from the spec: curl with token but path=/etc → 403).
func TestBrowse_PathEtc(t *testing.T) {
	f := newBrowseFixture(t)
	resp := browseGet(t, f, "/etc", "Bearer "+testBrowseToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "VDX-403") {
		t.Errorf("expected VDX-403 in body, got %s", body)
	}
	// MED-02: the absolute path must not appear in the error response.
	if strings.Contains(body, "/etc") {
		t.Errorf("absolute path /etc must not appear in the error response, got: %s", body)
	}
}

// ── Acceptance tests ─────────────────────────────────────────────────────────

// TestBrowse_HomeDir asserts that browsing $HOME with a valid token returns
// 200 and a well-formed JSON payload. We don't assert the exact contents
// because $HOME varies across environments.
func TestBrowse_HomeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("os.UserHomeDir unavailable in this environment")
	}

	f := newBrowseFixture(t)
	resp := browseGet(t, f, home, "Bearer "+testBrowseToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	var payload struct {
		Path        string      `json:"path"`
		Parent      string      `json:"parent"`
		Directories interface{} `json:"directories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if payload.Path == "" {
		t.Errorf("response.path is empty — home browse did not return a path")
	}
}

// TestBrowse_NoPathDefaultsToHome asserts that omitting ?path entirely returns
// 200 and the path field equals $HOME (or its symlink-resolved equivalent).
func TestBrowse_NoPathDefaultsToHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("os.UserHomeDir unavailable in this environment")
	}

	f := newBrowseFixture(t)
	resp := browseGet(t, f, "", "Bearer "+testBrowseToken)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	var payload struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	// The path must be the home directory or its canonical form.
	resolvedHome, _ := filepath.EvalSymlinks(home)
	if payload.Path != home && payload.Path != resolvedHome {
		t.Errorf("default path = %q, want %q or %q", payload.Path, home, resolvedHome)
	}
}

// TestBrowse_ErrorResponseNoAbsPath verifies MED-02: the error message returned
// by a 403 boundary rejection must not contain the requested absolute path.
// We use a deeply-nested fake path that os.ReadDir would reject too, but the
// home-dir check fires first and must not echo the path back.
func TestBrowse_ErrorResponseNoAbsPath(t *testing.T) {
	f := newBrowseFixture(t)
	// /tmp/secret is outside $HOME on all supported platforms.
	resp := browseGet(t, f, "/tmp/secret", "Bearer "+testBrowseToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	body := readBody(t, resp)
	if strings.Contains(body, "/tmp/secret") {
		t.Errorf("absolute path must not appear in the 403 response, got: %s", body)
	}
}
