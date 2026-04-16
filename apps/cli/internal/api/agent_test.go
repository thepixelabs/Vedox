package api

// Tests for the Doc Agent HTTP endpoints:
//
//	POST /api/agent/install   — handleAgentInstall
//	POST /api/agent/uninstall — handleAgentUninstall
//	GET  /api/agent/list      — handleAgentList
//
// Design rationale: the ProviderInstaller adapters call the OS keychain and
// write to the filesystem under $HOME. In unit test mode we cannot exercise
// the full install path without those side-effects. We therefore test:
//
//   - 503 when the keyStore is nil (dev-server mode)
//   - 400 for unknown / missing provider values
//   - 400 for malformed JSON
//   - GET /api/agent/list returns [] when no receipts exist
//
// End-to-end install/uninstall is covered by the existing CLI integration
// tests in cmd/ (which run against a real keychain on macOS CI).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/providers"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// ---------------------------------------------------------------------------
// Fixture
// ---------------------------------------------------------------------------

// agentFixture builds a test server. keyStore controls whether the agent
// handlers are enabled (nil = 503, non-nil = enabled).
type agentFixture struct {
	server *httptest.Server
}

func newAgentFixture(t *testing.T, keyStore providers.KeyIssuer) *agentFixture {
	t.Helper()

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

	srv := NewServer(adapter, wsDB, wsRoot, scanner.NewJobStore(), ai.NewJobStore(3), store.NewProjectRegistry(), nil)
	if keyStore != nil {
		srv.SetKeyStore(keyStore)
	}
	// Redirect userHome to TempDir so any accidental receipt-store access
	// stays isolated from the developer's real ~/.vedox directory.
	srv.SetHomeDirOverride(wsRoot)

	mux := http.NewServeMux()
	srv.Mount(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return &agentFixture{server: ts}
}

// post issues a JSON POST to the fixture server.
func (f *agentFixture) post(t *testing.T, path string, body interface{}) *http.Response {
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
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// get issues a GET to the fixture server.
func (f *agentFixture) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := f.server.Client().Get(f.server.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// ---------------------------------------------------------------------------
// mockKeyIssuer — satisfies providers.KeyIssuer without touching the keychain.
// ---------------------------------------------------------------------------

type mockKeyIssuer struct{}

func (m *mockKeyIssuer) IssueKey(_, _, _ string) (string, string, error) {
	return "mock-key-id", "mock-secret", nil
}
func (m *mockKeyIssuer) RevokeKey(_ string) error { return nil }

// ---------------------------------------------------------------------------
// POST /api/agent/install — 503 path (no keyStore)
// ---------------------------------------------------------------------------

// TestAgentInstall_NoKeyStore returns 503 when the server has no keyStore.
func TestAgentInstall_NoKeyStore(t *testing.T) {
	f := newAgentFixture(t, nil)

	resp := f.post(t, "/api/agent/install", map[string]string{"provider": "claude"})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// POST /api/agent/install — 400 paths
// ---------------------------------------------------------------------------

// TestAgentInstall_UnknownProvider returns 400 for an unknown provider.
func TestAgentInstall_UnknownProvider(t *testing.T) {
	f := newAgentFixture(t, &mockKeyIssuer{})

	resp := f.post(t, "/api/agent/install", map[string]string{"provider": "openai-gpt-99"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestAgentInstall_EmptyProvider returns 400.
func TestAgentInstall_EmptyProvider(t *testing.T) {
	f := newAgentFixture(t, &mockKeyIssuer{})

	resp := f.post(t, "/api/agent/install", map[string]string{"provider": ""})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestAgentInstall_InvalidJSON returns 400.
func TestAgentInstall_InvalidJSON(t *testing.T) {
	f := newAgentFixture(t, &mockKeyIssuer{})

	req, _ := http.NewRequest(http.MethodPost, f.server.URL+"/api/agent/install",
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

// ---------------------------------------------------------------------------
// POST /api/agent/uninstall — 503 path
// ---------------------------------------------------------------------------

func TestAgentUninstall_NoKeyStore(t *testing.T) {
	f := newAgentFixture(t, nil)

	resp := f.post(t, "/api/agent/uninstall", map[string]string{"provider": "claude"})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

// TestAgentUninstall_UnknownProvider returns 400.
func TestAgentUninstall_UnknownProvider(t *testing.T) {
	f := newAgentFixture(t, &mockKeyIssuer{})

	resp := f.post(t, "/api/agent/uninstall", map[string]string{"provider": "unknown-bot"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// GET /api/agent/list
// ---------------------------------------------------------------------------

// TestAgentList_Empty returns [] when no receipts are on disk.
// The server's homeDirOverride points to a fresh TempDir so there are
// guaranteed to be no receipt files.
func TestAgentList_Empty(t *testing.T) {
	f := newAgentFixture(t, nil) // keyStore not required for list

	resp := f.get(t, "/api/agent/list")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got []agentListItem
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got == nil {
		t.Error("body must be [] not null for empty list")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 items, got %d", len(got))
	}
}
