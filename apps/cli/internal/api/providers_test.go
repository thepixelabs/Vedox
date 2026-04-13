package api

// Direct handler tests for the Claude provider config endpoints. These tests
// drive Server methods directly via httptest.NewRecorder so they bypass the
// CSRF/Origin middleware and exercise pure handler logic.

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

	"github.com/go-chi/chi/v5"
)

// newProviderTestServer builds a minimal Server suitable for provider handler
// tests. workspaceRoot is a fresh tempdir; homeDirOverride is set to a
// separate tempdir so Codex tests cannot reach the real home dir.
func newProviderTestServer(t *testing.T) (*Server, string, string) {
	t.Helper()
	root := t.TempDir()
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	home := t.TempDir()
	resolvedHome, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatalf("EvalSymlinks home: %v", err)
	}
	s := &Server{workspaceRoot: resolved, homeDirOverride: resolvedHome}
	if err := os.MkdirAll(filepath.Join(resolved, "myproject"), 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	return s, resolved, resolvedHome
}

// callHandler builds a request with chi route params attached and dispatches
// it directly to the handler. This bypasses the router and middleware so the
// test can exercise pure handler logic.
func callHandler(
	t *testing.T,
	h http.HandlerFunc,
	method, target string,
	body []byte,
	params map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, target, rdr)
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func decodeRec(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode body: %v (body=%s)", err, rec.Body.String())
	}
}

// ── assertNoSymlinkAncestor ──────────────────────────────────────────────────

func TestAssertNoSymlinkAncestor_RejectsSymlinkAncestor(t *testing.T) {
	root := t.TempDir()
	resolved, _ := filepath.EvalSymlinks(root)

	// Create a real directory and a symlink pointing at it; the target file
	// path goes through the symlink.
	realDir := filepath.Join(resolved, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(resolved, "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(link, "file.txt")

	if err := assertNoSymlinkAncestor(resolved, target); err == nil {
		t.Errorf("expected symlink ancestor to be rejected, got nil")
	}
}

func TestAssertNoSymlinkAncestor_AllowsRegularPath(t *testing.T) {
	root := t.TempDir()
	resolved, _ := filepath.EvalSymlinks(root)
	target := filepath.Join(resolved, "a", "b", "c.txt")
	if err := assertNoSymlinkAncestor(resolved, target); err != nil {
		t.Errorf("regular path rejected: %v", err)
	}
}

// ── Claude memory + permissions ──────────────────────────────────────────────

func TestClaudeMemory_RoundTrip(t *testing.T) {
	s, _, _ := newProviderTestServer(t)

	// GET on empty project → empty memory + empty etag.
	rec := callHandler(t, s.handleGetClaudeConfig, "GET", "/", nil,
		map[string]string{"project": "myproject"})
	if rec.Code != 200 {
		t.Fatalf("GET status %d (body=%s)", rec.Code, rec.Body.String())
	}
	var got claudeGetResponse
	decodeRec(t, rec, &got)
	if got.Memory.Content != "" || got.Memory.Etag != "" {
		t.Errorf("expected empty memory, got %+v", got.Memory)
	}

	// PUT with empty etag (initial write) → 200.
	body := mustJSON(t, putMemoryRequest{Content: "# Hello\n", Etag: ""})
	rec = callHandler(t, s.handlePutClaudeMemory, "PUT", "/", body,
		map[string]string{"project": "myproject"})
	if rec.Code != 200 {
		t.Fatalf("PUT status %d (body=%s)", rec.Code, rec.Body.String())
	}
	var put1 etagOnlyResponse
	decodeRec(t, rec, &put1)
	if put1.Etag == "" {
		t.Error("expected non-empty etag")
	}

	// GET → content matches.
	rec = callHandler(t, s.handleGetClaudeConfig, "GET", "/", nil,
		map[string]string{"project": "myproject"})
	decodeRec(t, rec, &got)
	if got.Memory.Content != "# Hello\n" {
		t.Errorf("memory content = %q", got.Memory.Content)
	}
	if got.Memory.Etag != put1.Etag {
		t.Errorf("etag mismatch %q vs %q", got.Memory.Etag, put1.Etag)
	}

	// PUT with stale etag → 409.
	body = mustJSON(t, putMemoryRequest{Content: "# Updated\n", Etag: "deadbeef"})
	rec = callHandler(t, s.handlePutClaudeMemory, "PUT", "/", body,
		map[string]string{"project": "myproject"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	var conflict conflictResponse
	decodeRec(t, rec, &conflict)
	if conflict.Error != "conflict" || conflict.CurrentEtag != put1.Etag {
		t.Errorf("conflict body wrong: %+v", conflict)
	}
}

func TestClaudePermissions_PreservesUnknownKeys(t *testing.T) {
	s, root, _ := newProviderTestServer(t)

	// Seed an existing settings.json with unknown keys.
	settingsPath := filepath.Join(root, "myproject", ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := `{"theme":"dark","permissions":{"allow":["Bash"]}}`
	if err := os.WriteFile(settingsPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	// GET → grab the etag.
	rec := callHandler(t, s.handleGetClaudeConfig, "GET", "/", nil,
		map[string]string{"project": "myproject"})
	var got claudeGetResponse
	decodeRec(t, rec, &got)
	etag := got.Permissions.Etag
	if etag == "" {
		t.Fatal("expected non-empty etag for seeded file")
	}

	// PUT new permissions.
	body := mustJSON(t, putPermissionsRequest{
		Permissions: map[string]any{"allow": []any{"Bash", "Read"}},
		Etag:        etag,
	})
	rec = callHandler(t, s.handlePutClaudePermissions, "PUT", "/", body,
		map[string]string{"project": "myproject"})
	if rec.Code != 200 {
		t.Fatalf("PUT status %d (body=%s)", rec.Code, rec.Body.String())
	}

	// Read raw file → "theme":"dark" must still be there.
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"theme"`) {
		t.Errorf("unknown key 'theme' was stripped: %s", string(raw))
	}
	if !strings.Contains(string(raw), `"Read"`) {
		t.Errorf("new permission not written: %s", string(raw))
	}
}

func TestClaudeMemory_RejectsSymlinkAncestor(t *testing.T) {
	s, root, _ := newProviderTestServer(t)

	// Replace the project's .claude with a symlink pointing somewhere else.
	other := t.TempDir()
	claudeDirPath := filepath.Join(root, "myproject", ".claude")
	if err := os.Symlink(other, claudeDirPath); err != nil {
		t.Fatal(err)
	}

	body := mustJSON(t, putMemoryRequest{Content: "# evil\n", Etag: ""})
	rec := callHandler(t, s.handlePutClaudeMemory, "PUT", "/", body,
		map[string]string{"project": "myproject"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "VDX-005") {
		t.Errorf("expected VDX-005 in body, got %s", rec.Body.String())
	}
}
