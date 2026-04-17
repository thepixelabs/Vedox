package cmd

// Wiring tests for the daemon's *api.Server construction path.
//
// These tests mount the result of buildDaemonAPIServer on a real http.ServeMux
// and hit the Wave-0 endpoints to confirm every Set* injection was applied.
// They catch regressions of the class "future developer adds a new Set* method
// and forgets to call it in runForeground" — the exact failure mode that made
// this audit necessary (SetGraphStore was never called).
//
// Nothing here forks, binds, or touches ~/.vedox. The test replaces docStore /
// wsDB / globalDB with t.TempDir()-backed stubs and uses the production
// api.NewServer + Mount stack to verify routes respond.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	globaldb "github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// newDaemonDepsForTest builds a daemonAPIDeps with real-but-temporary
// databases. Every field is populated so the test exercises the full
// wiring path — nothing is left to the "skip if nil" fallback.
func newDaemonDepsForTest(t *testing.T) (daemonAPIDeps, func()) {
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

	deps := daemonAPIDeps{
		DocStore:        adapter,
		WorkspaceDB:     wsDB,
		WorkspaceRoot:   wsRoot,
		JobStore:        scanner.NewJobStore(),
		AIJobStore:      ai.NewJobStore(3),
		ProjectRegistry: store.NewProjectRegistry(),
		RequireAgent:    agentauth.PassthroughAuth(),
		GlobalDB:        gdb,
		KeyStore:        nil, // KeyStore requires keychain; covered separately.
		BootstrapToken:  "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
	}
	cleanup := func() {
		_ = wsDB.Close()
		_ = gdb.Close()
	}
	return deps, cleanup
}

// TestBuildDaemonAPIServer_ReturnsNilWithoutDocStore protects the guard
// clause that keeps the daemon from crashing when workspace initialisation
// fails (e.g. unwritable VedoxHome).
func TestBuildDaemonAPIServer_ReturnsNilWithoutDocStore(t *testing.T) {
	got := buildDaemonAPIServer(daemonAPIDeps{}) // everything zero
	if got != nil {
		t.Errorf("expected nil return when DocStore is missing, got %v", got)
	}
}

// TestBuildDaemonAPIServer_WiresGraphStore asserts that /api/graph returns
// 200 (not 503) after buildDaemonAPIServer — proving SetGraphStore was
// called. This is the regression guard for the bug found during the Wave-0
// integration audit.
func TestBuildDaemonAPIServer_WiresGraphStore(t *testing.T) {
	deps, cleanup := newDaemonDepsForTest(t)
	defer cleanup()

	apiServer := buildDaemonAPIServer(deps)
	if apiServer == nil {
		t.Fatal("expected non-nil api.Server")
	}

	mux := http.NewServeMux()
	apiServer.Mount(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/graph?project=myproject")
	if err != nil {
		t.Fatalf("GET /api/graph: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/graph: status = %d, want 200 — SetGraphStore was never called in the daemon wiring",
			resp.StatusCode)
	}
}

// TestBuildDaemonAPIServer_WiresGlobalDB asserts /api/analytics/summary
// returns 200 rather than the 503 it returns when globalDB is nil.
func TestBuildDaemonAPIServer_WiresGlobalDB(t *testing.T) {
	deps, cleanup := newDaemonDepsForTest(t)
	defer cleanup()

	apiServer := buildDaemonAPIServer(deps)
	mux := http.NewServeMux()
	apiServer.Mount(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/analytics/summary")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/analytics/summary: status = %d, want 200 — SetGlobalDB was never called",
			resp.StatusCode)
	}
	// sanity-check that the analytics JSON is well-formed
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["pipeline_ready"]; !ok {
		t.Error("analytics summary missing pipeline_ready field")
	}
}

// TestBuildDaemonAPIServer_WiresBootstrapToken asserts /api/browse returns
// 401 without the token and 200/403 (route reachable) with it — proving
// SetBootstrapToken was called.
func TestBuildDaemonAPIServer_WiresBootstrapToken(t *testing.T) {
	deps, cleanup := newDaemonDepsForTest(t)
	defer cleanup()

	apiServer := buildDaemonAPIServer(deps)
	mux := http.NewServeMux()
	apiServer.Mount(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// No token → must be 401.
	resp, err := ts.Client().Get(ts.URL + "/api/browse")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/api/browse without token: status = %d, want 401 — SetBootstrapToken not wired",
			resp.StatusCode)
	}

	// With token → route reached (not 404/405).
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/browse", nil)
	req.Header.Set("Authorization", "Bearer "+deps.BootstrapToken)
	resp2, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("GET with token: %v", err)
	}
	_ = resp2.Body.Close()
	if resp2.StatusCode == http.StatusNotFound || resp2.StatusCode == http.StatusMethodNotAllowed {
		t.Fatalf("/api/browse with token: status = %d — route not registered under chi",
			resp2.StatusCode)
	}
}

// TestBuildDaemonAPIServer_DegradesWithoutOptionalDeps confirms the
// builder does not panic when GlobalDB / KeyStore are nil. This is the
// graceful-degradation contract the daemon relies on when ~/.vedox is
// read-only or the keychain is unavailable.
func TestBuildDaemonAPIServer_DegradesWithoutOptionalDeps(t *testing.T) {
	deps, cleanup := newDaemonDepsForTest(t)
	defer cleanup()

	deps.GlobalDB = nil
	deps.KeyStore = nil
	deps.BootstrapToken = "" // fail-closed: every /api/browse is 401

	apiServer := buildDaemonAPIServer(deps)
	if apiServer == nil {
		t.Fatal("expected non-nil api.Server even without optional deps")
	}

	mux := http.NewServeMux()
	apiServer.Mount(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Endpoints that depend on GlobalDB should return 503 — not 500.
	resp, err := ts.Client().Get(ts.URL + "/api/analytics/summary")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("/api/analytics/summary: status = %d, want 503 without GlobalDB", resp.StatusCode)
	}
}
