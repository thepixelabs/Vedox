package cmd

// Wiring tests for buildDevMux — the dev server's HTTP mux construction path.
//
// These tests mirror the pattern in server_wiring_test.go but target the dev
// code path (buildDevMux). They catch regressions where a new Set* injection
// is wired in the daemon (server.go) but missed in dev.go — the exact class
// of bug fixed by this PR (SetGlobalDB was called in the daemon but never in
// buildDevMux, causing /api/analytics/summary to always 503 in dev mode).
//
// Nothing here forks, binds, or touches ~/.vedox. Databases are backed by
// t.TempDir() and discarded after each test.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/config"
	globaldb "github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// newDevMuxForTest builds a fully-wired dev mux backed by temporary databases.
// It returns the test server and a cleanup function that closes all DB handles.
func newDevMuxForTest(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	wsRoot := t.TempDir()
	adapter, err := store.NewLocalAdapter(wsRoot, nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}

	wsDB, err := globaldb.Open(globaldb.Options{WorkspaceRoot: wsRoot})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}

	gdbPath := filepath.Join(t.TempDir(), "global.db")
	gdb, err := globaldb.OpenGlobalDB(gdbPath)
	if err != nil {
		_ = wsDB.Close()
		t.Fatalf("OpenGlobalDB: %v", err)
	}

	cfg := &config.Config{
		Workspace: wsRoot,
		Port:      5150,
	}
	registry := store.NewProjectRegistry()
	jobStore := scanner.NewJobStore()
	aiJobStore := ai.NewJobStore(3)

	// PassthroughAuth is tests-only — it lets all agent routes through without
	// a real HMAC key, so we can probe endpoints without loading a keychain.
	requireAgent := agentauth.PassthroughAuth()

	mux := buildDevMux(cfg, adapter, wsDB, gdb, nil /* keyStore */, jobStore, aiJobStore, registry, requireAgent)
	ts := httptest.NewServer(mux)

	cleanup := func() {
		ts.Close()
		_ = wsDB.Close()
		_ = gdb.Close()
	}
	return ts, cleanup
}

// TestBuildDevMux_WiresGlobalDB asserts that /api/analytics/summary returns
// 200 (not 503) when buildDevMux is given a non-nil globalDB. This is the
// regression guard for the VDX-503 bug: SetGlobalDB was called in the daemon
// path (server.go) but missing from buildDevMux.
func TestBuildDevMux_WiresGlobalDB(t *testing.T) {
	ts, cleanup := newDevMuxForTest(t)
	defer cleanup()

	resp, err := ts.Client().Get(ts.URL + "/api/analytics/summary")
	if err != nil {
		t.Fatalf("GET /api/analytics/summary: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/analytics/summary: status = %d, want 200 — SetGlobalDB not wired in buildDevMux",
			resp.StatusCode)
	}

	// Confirm the response body is well-formed JSON with the expected field.
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode analytics response: %v", err)
	}
	if _, ok := body["pipeline_ready"]; !ok {
		t.Error("analytics summary missing pipeline_ready field")
	}
}

// TestBuildDevMux_WiresGraphStore asserts that /api/graph returns 200 (not
// 503) for an empty project — confirming SetGraphStore was called.
func TestBuildDevMux_WiresGraphStore(t *testing.T) {
	ts, cleanup := newDevMuxForTest(t)
	defer cleanup()

	resp, err := ts.Client().Get(ts.URL + "/api/graph?project=myproject")
	if err != nil {
		t.Fatalf("GET /api/graph: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/graph: status = %d, want 200 — SetGraphStore not wired in buildDevMux",
			resp.StatusCode)
	}
}

// TestBuildDevMux_DegradesWithoutGlobalDB confirms that passing nil for
// globalDB results in a 503 (graceful degradation) rather than a panic.
func TestBuildDevMux_DegradesWithoutGlobalDB(t *testing.T) {
	wsRoot := t.TempDir()
	adapter, err := store.NewLocalAdapter(wsRoot, nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}

	wsDB, err := globaldb.Open(globaldb.Options{WorkspaceRoot: wsRoot})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = wsDB.Close() })

	cfg := &config.Config{Workspace: wsRoot, Port: 5150}
	mux := buildDevMux(
		cfg, adapter, wsDB,
		nil, /* globalDB — intentionally absent */
		nil, /* keyStore */
		scanner.NewJobStore(),
		ai.NewJobStore(3),
		store.NewProjectRegistry(),
		agentauth.PassthroughAuth(),
	)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/analytics/summary")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("/api/analytics/summary without globalDB: status = %d, want 503",
			resp.StatusCode)
	}
}
