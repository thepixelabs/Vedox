package api_test

// Integration tests for the Vedox HTTP API.
//
// These tests exercise the real stack end-to-end: real LocalAdapter, real
// SQLite store, real chi router, real httptest.Server. Nothing is mocked —
// a test that passes here is a test that would pass against a running
// `vedox dev` binary.
//
// Every test is fully isolated: each helper call builds a brand-new workspace
// inside t.TempDir() and spins up its own httptest.Server. There is no shared
// state between tests and no reliance on execution order, so they are safe to
// run with -race and -parallel.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

// allowedOrigin is the only Origin the API accepts for mutating verbs.
// Mirrors the allowlist in api/middleware.go — keep in sync if that list
// ever grows.
const allowedOrigin = "http://localhost:5151"

// testFixture bundles everything a test needs to talk to the API and to
// make assertions about the filesystem behind it.
type testFixture struct {
	server        *httptest.Server
	workspaceRoot string
	dbStore       *db.Store
	jobStore      *scanner.JobStore
	// apiHandler is the chi router that Mount registers under "/api/". We
	// extract it so traversal tests can bypass http.ServeMux's cleanPath
	// normalisation (which would strip "..").
	apiHandler http.Handler
}

// newTestServer spins up a full API server backed by a fresh LocalAdapter and
// SQLite store inside t.TempDir(). It returns a testFixture whose server is
// automatically closed on test cleanup.
//
// The workspace root is EvalSymlinks-resolved before being handed to the
// server so path-prefix checks inside validateDocPath match the LocalAdapter's
// own resolution of /var/folders → /private/var/folders on macOS.
func newTestServer(t *testing.T) *testFixture {
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
	srv := api.NewServer(
		adapter,
		dbStore,
		resolved,
		jobStore,
		ai.NewJobStore(3),
		store.NewProjectRegistry(),
		nil, // agentauth.PassthroughAuth — tests don't exercise agent auth
	)

	mux := http.NewServeMux()
	srv.Mount(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	// Fish the chi router back out of the mux. We use a harmless path that
	// does not trigger ServeMux path cleaning so the returned handler is the
	// chi router Mount registered at "/api/". Tests that need to exercise
	// path-traversal payloads call apiHandler.ServeHTTP directly.
	probe := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	apiHandler, _ := mux.Handler(probe)

	return &testFixture{
		server:        ts,
		workspaceRoot: resolved,
		dbStore:       dbStore,
		jobStore:      jobStore,
		apiHandler:    apiHandler,
	}
}

// do issues an HTTP request against the fixture server. For mutating verbs it
// sets the allowed Origin header so the CSRF middleware accepts the request;
// callers that want to exercise the block-foreign-origin path should use
// doRaw and set Origin themselves.
func (f *testFixture) do(t *testing.T, method, path string, body interface{}) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, f.server.URL+path, rdr)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if method != http.MethodGet && method != http.MethodHead {
		req.Header.Set("Origin", allowedOrigin)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// doRaw is like do but gives the caller full control over every header.
// No body marshalling and no default Origin — used by the CORS/CSRF tests.
func (f *testFixture) doRaw(t *testing.T, method, path string, body []byte, headers map[string]string) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, f.server.URL+path, rdr)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// decodeJSON reads resp.Body into v. The body is fully consumed so the caller
// does not have to worry about leaks; do/doRaw already arrange for Close.
func decodeJSON(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
}

// readBody returns resp.Body as a string — handy for error-path assertions
// where the exact JSON shape doesn't matter, only the VDX code substring.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

// ── Health ────────────────────────────────────────────────────────────────────

func TestHealth(t *testing.T) {
	f := newTestServer(t)
	resp := f.do(t, http.MethodGet, "/api/health", nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]string
	decodeJSON(t, resp, &body)
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want %q", body["status"], "ok")
	}
}

// ── Projects listing ──────────────────────────────────────────────────────────

// TestListProjects_Empty asserts that a fresh workspace with no .git roots and
// no registered projects returns `[]` rather than `null` or a 500. The empty
// case is a common failure mode for JSON APIs written in Go (nil slice → null).
func TestListProjects_Empty(t *testing.T) {
	f := newTestServer(t)
	resp := f.do(t, http.MethodGet, "/api/projects", nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := readBody(t, resp)
	trimmed := strings.TrimSpace(body)
	// Must be an array literal, never "null".
	if !strings.HasPrefix(trimmed, "[") {
		t.Errorf("body must start with '[', got %q", trimmed)
	}
	if trimmed == "null" {
		t.Errorf("body is null — nil slice not initialised before JSON encoding")
	}
}

// ── Doc write / read round trip ───────────────────────────────────────────────

func TestWriteDoc_CreatesFile(t *testing.T) {
	f := newTestServer(t)
	resp := f.do(t, http.MethodPost,
		"/api/projects/myproject/docs/test.md",
		map[string]string{"content": "# Test\n\nHello."},
	)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, readBody(t, resp))
	}

	// The write handler auto-saves to a draft, not to the committed path.
	// The draft lives at .vedox/drafts/myproject/test.md.draft.md relative
	// to the workspace root — verify it landed there.
	draft := filepath.Join(f.workspaceRoot, ".vedox", "drafts", "myproject", "test.md.draft.md")
	if _, err := readFile(draft); err != nil {
		t.Fatalf("expected draft at %s: %v", draft, err)
	}
}

func TestWriteDoc_ReturnsDraft(t *testing.T) {
	f := newTestServer(t)
	body := "# Draft\n\nBody content."
	f.do(t, http.MethodPost,
		"/api/projects/myproject/docs/test.md",
		map[string]string{"content": body},
	)

	resp := f.do(t, http.MethodGet, "/api/projects/myproject/docs/test.md", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	var doc struct {
		Content string `json:"content"`
		Path    string `json:"path"`
	}
	decodeJSON(t, resp, &doc)
	if doc.Content != body {
		t.Errorf("content mismatch: got %q, want %q", doc.Content, body)
	}
	// The returned path must be the canonical (non-draft) path so the frontend
	// can track identity even though the bytes came from the draft.
	if doc.Path != filepath.Join("myproject", "test.md") {
		t.Errorf("path = %q, want %q", doc.Path, filepath.Join("myproject", "test.md"))
	}
}

// TestPublish_NoGitConfig ensures Publish fails fast with VDX-003 when the
// git identity is unset. Isolating git config from the host environment is
// delicate: we redirect every config source (system, global, XDG) to /dev/null
// and unset the author/committer env vars so `git config user.name` returns
// empty regardless of the developer's real config.
func TestPublish_NoGitConfig(t *testing.T) {
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_COMMITTER_NAME", "")
	t.Setenv("GIT_COMMITTER_EMAIL", "")

	f := newTestServer(t)

	// Write a draft first so Publish has something to try to promote. Without
	// this the handler would still fail at gitIdentity, but the test intent
	// is clearer when the doc exists.
	f.do(t, http.MethodPost,
		"/api/projects/myproject/docs/test.md",
		map[string]string{"content": "# Test"},
	)

	resp := f.do(t, http.MethodPost,
		"/api/projects/myproject/docs/test.md/publish",
		map[string]string{"message": "initial"},
	)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "VDX-003") {
		t.Errorf("expected VDX-003 in body, got %s", body)
	}
}

// ── Search ────────────────────────────────────────────────────────────────────

// TestSearch_FindsIndexedDoc upserts a document directly into the SQLite FTS
// store (bypassing the background indexer goroutine, which we don't start in
// tests) and verifies the HTTP search endpoint surfaces the hit.
//
// We use a deliberately unique token — "zephyrquantum" — so the match cannot
// come from stray fixture data or a stemmer false positive.
func TestSearch_FindsIndexedDoc(t *testing.T) {
	f := newTestServer(t)

	doc := &db.Doc{
		ID:          "myproject/notes.md",
		Project:     "myproject",
		Title:       "Notes",
		Type:        "how-to",
		Status:      "published",
		ContentHash: "abc",
		ModTime:     "2026-04-13T00:00:00Z",
		Size:        42,
		Body:        "This contains the zephyrquantum token.",
	}
	if err := f.dbStore.UpsertDoc(context.Background(), doc); err != nil {
		t.Fatalf("UpsertDoc: %v", err)
	}

	resp := f.do(t, http.MethodGet, "/api/projects/myproject/search?q=zephyrquantum", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	var hits []map[string]interface{}
	decodeJSON(t, resp, &hits)
	if len(hits) == 0 {
		t.Fatalf("expected at least 1 hit for 'zephyrquantum', got 0")
	}
}

// TestSearch_EmptyQuery asserts the documented contract: an empty ?q returns
// 200 with an empty array, not a 400. The frontend relies on this to clear
// the results pane when the search box is emptied.
func TestSearch_EmptyQuery(t *testing.T) {
	f := newTestServer(t)
	resp := f.do(t, http.MethodGet, "/api/projects/myproject/search?q=", nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	body := strings.TrimSpace(readBody(t, resp))
	if body != "[]" {
		t.Errorf("body = %q, want %q", body, "[]")
	}
}

// ── Path traversal ────────────────────────────────────────────────────────────

// serveDirect dispatches a request straight to the chi router, bypassing
// http.ServeMux and its cleanPath normalisation. This is the only reliable
// way to send a literal ".." sequence through the URL and see how the
// application-level validateDocPath handles it.
func (f *testFixture) serveDirect(t *testing.T, method, rawPath string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, rawPath, rdr)
	if method != http.MethodGet && method != http.MethodHead {
		req.Header.Set("Origin", allowedOrigin)
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	f.apiHandler.ServeHTTP(rec, req)
	return rec
}

// TestPathTraversal_DocRead covers GET-side traversal: a wildcard doc path
// whose Clean'd form escapes the project directory must return 400 VDX-005
// without touching the filesystem.
func TestPathTraversal_DocRead(t *testing.T) {
	f := newTestServer(t)
	rec := f.serveDirect(t, http.MethodGet,
		"/api/projects/foo/docs/../../secrets.md", nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "VDX-005") {
		t.Errorf("expected VDX-005 in body, got %s", rec.Body.String())
	}
}

// TestPathTraversal_DocWrite covers POST-side traversal — same rule, same code.
func TestPathTraversal_DocWrite(t *testing.T) {
	f := newTestServer(t)
	rec := f.serveDirect(t, http.MethodPost,
		"/api/projects/foo/docs/../../secrets.md",
		[]byte(`{"content":"# evil"}`),
	)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "VDX-005") {
		t.Errorf("expected VDX-005 in body, got %s", rec.Body.String())
	}
}

// ── CORS / CSRF ───────────────────────────────────────────────────────────────

func TestCORS_AllowsLocalhost(t *testing.T) {
	f := newTestServer(t)
	resp := f.doRaw(t, http.MethodGet, "/api/health", nil, map[string]string{
		"Origin": allowedOrigin,
	})
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, allowedOrigin)
	}
}

// TestCORS_BlocksForeignOrigin asserts the CSRF fix: a mutating request whose
// Origin is not in the allowlist is rejected server-side with 403, not merely
// blocked by the browser on the response side. This is the hardening that
// landed in middleware.go.
func TestCORS_BlocksForeignOrigin(t *testing.T) {
	f := newTestServer(t)
	resp := f.doRaw(t, http.MethodPost,
		"/api/projects/foo/docs/test.md",
		[]byte(`{"content":"# evil"}`),
		map[string]string{
			"Origin":       "http://evil.com",
			"Content-Type": "application/json",
		},
	)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestDeleteDoc(t *testing.T) {
	f := newTestServer(t)

	// Seed a committed file directly on disk; writeDoc goes to drafts, which
	// is not what we want to test delete against.
	projectDir := filepath.Join(f.workspaceRoot, "myproject")
	_ = mkdirAll(projectDir)
	docAbs := filepath.Join(projectDir, "test.md")
	if err := writeFile(docAbs, "# Hello"); err != nil {
		t.Fatalf("writeFile: %v", err)
	}

	// Confirm it's readable before delete.
	resp := f.do(t, http.MethodGet, "/api/projects/myproject/docs/test.md", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pre-delete GET = %d, want 200", resp.StatusCode)
	}

	// Delete and expect 204.
	resp = f.do(t, http.MethodDelete, "/api/projects/myproject/docs/test.md", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE = %d, want 204 (body=%s)", resp.StatusCode, readBody(t, resp))
	}

	// Subsequent GET must 404.
	resp = f.do(t, http.MethodGet, "/api/projects/myproject/docs/test.md", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("post-delete GET = %d, want 404", resp.StatusCode)
	}
}

// ── Body size limit ───────────────────────────────────────────────────────────

// TestRequestBodySizeLimit sends a ~2 MB content payload — double the write
// handler's 1 MB MaxBytesReader ceiling — and expects 413 with VDX-007.
func TestRequestBodySizeLimit(t *testing.T) {
	f := newTestServer(t)

	// Build a 2 MB body by hand to avoid rounding surprises in json.Marshal.
	huge := strings.Repeat("a", 2<<20)
	body := []byte(`{"content":"` + huge + `"}`)

	resp := f.doRaw(t, http.MethodPost,
		"/api/projects/myproject/docs/big.md",
		body,
		map[string]string{
			"Origin":       allowedOrigin,
			"Content-Type": "application/json",
		},
	)
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
}

// ── Secret file blocklist ─────────────────────────────────────────────────────

// TestSecretFileBlocked exercises the HTTP surface of the secret-file
// blocklist. We pre-seed a .env file on disk (simulating one that was placed
// there outside of Vedox) and then attempt to DELETE it via the API. The
// LocalAdapter must refuse with VDX-006 → 403, ensuring the HTTP surface
// cannot be used to reach into an on-disk secret.
//
// NOTE (finding for follow-up): POST /api/projects/:project/docs/.env
// currently writes to .vedox/drafts/<project>/.env.draft.md and does NOT
// trip the secret-file blocklist because the blocklist matches the basename
// of the destination path, and the destination basename is ".env.draft.md",
// not ".env". That leak is orthogonal to this test; it should be tracked as
// its own bug and covered by a test of handleWriteDoc once fixed.
func TestSecretFileBlocked(t *testing.T) {
	f := newTestServer(t)

	// Seed a .env file on disk inside a project directory.
	projectDir := filepath.Join(f.workspaceRoot, "myproject")
	if err := mkdirAll(projectDir); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := writeFile(filepath.Join(projectDir, ".env"), "SECRET=abc"); err != nil {
		t.Fatalf("writeFile: %v", err)
	}

	resp := f.do(t, http.MethodDelete,
		"/api/projects/myproject/docs/.env", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	if !strings.Contains(readBody(t, resp), "VDX-006") {
		t.Errorf("expected VDX-006 in body")
	}
}

// ── Small filesystem helpers local to this test file ─────────────────────────
// We keep these as plain functions rather than importing os in every test so
// the intent at the call site is obvious and easy to grep for.

func readFile(path string) ([]byte, error) { return os.ReadFile(path) }
func writeFile(path, content string) error { return os.WriteFile(path, []byte(content), 0o644) }
func mkdirAll(path string) error           { return os.MkdirAll(path, 0o755) }
