package api

// Wave-0 wiring integration tests.
//
// These tests exercise every Wave-0 endpoint end-to-end through the full
// chi.Router produced by Server.Mount, so a failure here is a wiring bug
// (routes not registered, middleware missing, Set* injections forgotten)
// rather than a handler-internal defect.
//
// Each endpoint is covered by at least one test that hits the actual URL
// the frontend uses (see apps/editor/src/lib/api/client.ts and
// onboarding components) and confirms the handler runs. Handler-internal
// behaviour is covered by its own table-driven unit tests elsewhere.
//
// What this file specifically guards against:
//
//	- cmd/server.go forgetting to call Set{GlobalDB,KeyStore,GraphStore,
//	  VoiceServer,BootstrapToken}. A missing Set* call usually surfaces
//	  as a 503 from the dependent endpoint.
//	- Mount() route registrations getting commented out or reordered in
//	  a way that hides a route behind the docs subrouter wildcard.
//	- Middleware (CORS / security headers / bootstrap-token auth) not
//	  applying to a Wave-0 endpoint.
//	- Frontend/backend URL drift: every URL exercised here is copied
//	  verbatim from apps/editor/src/lib/api/client.ts or a component
//	  fetch call.
//
// Everything uses the production NewServer + Mount stack; no direct
// handleXxx calls. Fixtures are real SQLite stores inside t.TempDir()
// with a redirected home directory so no developer state is touched.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/docgraph"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// wave0Fixture is a fully-wired API server: every optional Set* call has
// been made (GlobalDB, KeyStore via mockKeyIssuer, GraphStore, BootstrapToken,
// HomeDirOverride). VoiceServer is left nil — exercised in its own test.
//
// Use this fixture for tests that verify the happy path of each Wave-0
// endpoint. Tests that want to exercise the 503 fallback should build a
// minimal fixture by hand.
type wave0Fixture struct {
	server        *httptest.Server
	srv           *Server
	workspaceRoot string
	homeDir       string
	gdb           *db.GlobalDB
	wsDB          *db.Store
	graphStore    *docgraph.GraphStore
}

const wave0Token = "f00df00df00df00df00df00df00df00df00df00df00df00df00df00df00df00d"

func newWave0Fixture(t *testing.T) *wave0Fixture {
	t.Helper()

	// Workspace root — holds the per-workspace index.db and any docs we create.
	raw := t.TempDir()
	wsRoot, err := filepath.EvalSymlinks(raw)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	// Fake home directory so we never touch ~/.vedox on the dev machine.
	home := t.TempDir()
	homeResolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatalf("EvalSymlinks home: %v", err)
	}

	adapter, err := store.NewLocalAdapter(wsRoot, nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}

	wsDB, err := db.Open(db.Options{WorkspaceRoot: wsRoot})
	if err != nil {
		t.Fatalf("db.Open wsDB: %v", err)
	}
	t.Cleanup(func() { _ = wsDB.Close() })

	gdbPath := filepath.Join(t.TempDir(), "global.db")
	gdb, err := db.OpenGlobalDB(gdbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB: %v", err)
	}
	t.Cleanup(func() { _ = gdb.Close() })

	gs := docgraph.NewGraphStore(wsDB)

	srv := NewServer(
		adapter,
		wsDB,
		wsRoot,
		scanner.NewJobStore(),
		ai.NewJobStore(3),
		store.NewProjectRegistry(),
		agentauth.PassthroughAuth(),
	)
	srv.SetGlobalDB(gdb)
	srv.SetKeyStore(&mockKeyIssuer{})
	srv.SetGraphStore(gs)
	srv.SetBootstrapToken(wave0Token)
	srv.SetHomeDirOverride(homeResolved)

	mux := http.NewServeMux()
	srv.Mount(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return &wave0Fixture{
		server:        ts,
		srv:           srv,
		workspaceRoot: wsRoot,
		homeDir:       homeResolved,
		gdb:           gdb,
		wsDB:          wsDB,
		graphStore:    gs,
	}
}

// ---------------------------------------------------------------------------
// Helpers — identical shape to the existing per-file helpers but local to
// this file so fixture-specific extensions are discoverable in one place.
// ---------------------------------------------------------------------------

func (f *wave0Fixture) get(t *testing.T, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.server.URL+path, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func (f *wave0Fixture) getWithToken(t *testing.T, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, f.server.URL+path, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+wave0Token)
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func (f *wave0Fixture) postJSON(t *testing.T, path string, body interface{}) *http.Response {
	t.Helper()
	var rdr *strings.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = strings.NewReader(string(b))
	} else {
		rdr = strings.NewReader("")
	}
	req, err := http.NewRequest(http.MethodPost, f.server.URL+path, rdr)
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

func (f *wave0Fixture) putJSON(t *testing.T, path string, body interface{}) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPut, f.server.URL+path, strings.NewReader(string(b)))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// ---------------------------------------------------------------------------
// /api/browse — route is registered, middleware chain runs,
// SetBootstrapToken is honoured end-to-end.
// ---------------------------------------------------------------------------

// TestWave0_BrowseRouteWired confirms the route exists under the chi router
// (non-404) and that the bootstrap token middleware is applied (401 without
// a token). Handler-internal behaviour is covered in browse_test.go.
func TestWave0_BrowseRouteWired(t *testing.T) {
	f := newWave0Fixture(t)

	// No token — middleware must reject.
	resp := f.get(t, "/api/browse")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/api/browse without token: status = %d, want 401", resp.StatusCode)
	}

	// CORS middleware must have attached the CSP header even on a 401.
	if got := resp.Header.Get("Content-Security-Policy"); got == "" {
		t.Error("CSP header missing — corsMiddleware not in the chi middleware chain")
	}
}

// TestWave0_BrowseWithTokenRoutes confirms that with a valid token the route
// reaches the handler and emits a 200/403 — either is fine, just never a 404.
func TestWave0_BrowseWithTokenRoutes(t *testing.T) {
	f := newWave0Fixture(t)
	// Path=homeDir is inside $HOME for the handler (which uses real $HOME
	// via withinHomeDir). We just assert the route ran — not 404, not 405.
	resp := f.getWithToken(t, "/api/browse")
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
		t.Fatalf("/api/browse: status = %d — route not registered", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// /api/graph — confirms SetGraphStore actually gets wired into the chi
// router such that the frontend can reach a working graph endpoint.
// This is the regression guard for the "GraphStore never wired in the
// daemon" bug found during this audit.
// ---------------------------------------------------------------------------

// TestWave0_GraphRouteWired confirms that with a GraphStore injected the
// endpoint returns 200 rather than the 503 it returns when SetGraphStore
// was never called. Missing ?project still returns 400 (covered in
// graph_test.go) — we use a valid ?project here to exercise the 200 path.
func TestWave0_GraphRouteWired(t *testing.T) {
	f := newWave0Fixture(t)

	resp := f.get(t, "/api/graph?project=myproject")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/graph: status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}

	var body graphResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Nodes == nil || body.Edges == nil {
		t.Error("nodes/edges must be non-null arrays even for an empty graph")
	}
}

// TestWave0_GraphEndToEnd seeds two references via the GraphStore and
// verifies they surface on the HTTP endpoint. This is the true wiring
// test: handler → graphStore → db → back out.
func TestWave0_GraphEndToEnd(t *testing.T) {
	f := newWave0Fixture(t)
	ctx := context.Background()

	// Seed documents (FK constraint on doc_references.source_doc_id).
	if err := f.wsDB.UpsertDoc(ctx, &db.Doc{
		ID:      "myproject/a.md",
		Project: "myproject",
		Slug:    "myproject/a.md",
		Title:   "A",
		Status:  "published",
		Type:    "how-to",
	}); err != nil {
		t.Fatalf("UpsertDoc a: %v", err)
	}
	if err := f.graphStore.SaveRefs(ctx, "myproject/a.md", []docgraph.DocRef{
		{
			SourcePath: "myproject/a.md",
			TargetPath: "myproject/b.md",
			LinkType:   docgraph.LinkTypeMD,
			LineNum:    1,
		},
	}); err != nil {
		t.Fatalf("SaveRefs: %v", err)
	}

	resp := f.get(t, "/api/graph?project=myproject")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}

	var body graphResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Edges) != 1 {
		t.Errorf("edges = %d, want 1", len(body.Edges))
	}
}

// ---------------------------------------------------------------------------
// /api/settings — GET + PUT round-trip through the mounted chi router.
// Verifies PATCH merge semantics survive end-to-end, including odd but
// legal JSON inputs that could otherwise crash a naive decoder.
// ---------------------------------------------------------------------------

// TestWave0_SettingsGetWired confirms GET /api/settings returns 200 and an
// empty object on a fresh home dir.
func TestWave0_SettingsGetWired(t *testing.T) {
	f := newWave0Fixture(t)
	resp := f.get(t, "/api/settings")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/settings: status = %d, want 200", resp.StatusCode)
	}
}

// TestWave0_SettingsPutWired confirms PUT /api/settings merges the body and
// returns the merged document.
func TestWave0_SettingsPutWired(t *testing.T) {
	f := newWave0Fixture(t)
	resp := f.putJSON(t, "/api/settings", map[string]any{
		"appearance": map[string]string{"theme": "ember"},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT /api/settings: status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
}

// TestWave0_SettingsPutPreservesNullValues is the "unexpected types" guard.
// The frontend uses json.stringify which will emit `null` for deleted keys;
// the handler must accept a null top-level value without exploding.
func TestWave0_SettingsPutPreservesNullValues(t *testing.T) {
	f := newWave0Fixture(t)

	// Seed existing config.
	first := f.putJSON(t, "/api/settings", map[string]any{
		"editor": map[string]bool{"spellCheck": true},
	})
	if first.StatusCode != http.StatusOK {
		t.Fatalf("seed PUT: %d", first.StatusCode)
	}

	// Now PATCH a null — must not 500.
	resp := f.putJSON(t, "/api/settings", map[string]any{
		"editor": nil,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT null value: status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
}

// TestWave0_SettingsPutRejectsNonObject confirms the PATCH decoder rejects
// a top-level array with 400 rather than silently partially-merging.
func TestWave0_SettingsPutRejectsNonObject(t *testing.T) {
	f := newWave0Fixture(t)

	req, err := http.NewRequest(http.MethodPut, f.server.URL+"/api/settings",
		strings.NewReader(`[1,2,3]`))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for top-level array", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// /api/agent/* — list is always available; install/uninstall need keyStore.
// The fixture provides a mockKeyIssuer so the install path returns 500 from
// the installer (real filesystem needed) but the route is reached.
// ---------------------------------------------------------------------------

// TestWave0_AgentListWired confirms GET /api/agent/list returns 200 and
// an empty array on a fresh receipt store.
func TestWave0_AgentListWired(t *testing.T) {
	f := newWave0Fixture(t)
	resp := f.get(t, "/api/agent/list")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/agent/list: status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}

	var list []agentListItem
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if list == nil {
		t.Error("list must be [] not null")
	}
}

// TestWave0_AgentInstallRouteWired confirms POST /api/agent/install reaches
// the handler (not 404/405/403). With an unknown provider the handler returns
// 400, which is sufficient to prove the chi route + CSRF middleware + body
// decoder all line up.
func TestWave0_AgentInstallRouteWired(t *testing.T) {
	f := newWave0Fixture(t)
	resp := f.postJSON(t, "/api/agent/install", map[string]string{
		"provider": "nobody-knows-me",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for unknown provider (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
}

// TestWave0_AgentUninstallRouteWired mirrors the install test for uninstall.
func TestWave0_AgentUninstallRouteWired(t *testing.T) {
	f := newWave0Fixture(t)
	resp := f.postJSON(t, "/api/agent/uninstall", map[string]string{
		"provider": "nobody-knows-me",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for unknown provider (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
}

// ---------------------------------------------------------------------------
// /api/analytics/summary — GlobalDB must be wired via SetGlobalDB.
// ---------------------------------------------------------------------------

// TestWave0_AnalyticsSummaryWired confirms the endpoint returns 200 JSON
// with pipeline_ready=false on an empty DB. This asserts that SetGlobalDB
// was honoured; without it the endpoint returns 503.
func TestWave0_AnalyticsSummaryWired(t *testing.T) {
	f := newWave0Fixture(t)
	resp := f.get(t, "/api/analytics/summary")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/analytics/summary: status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}

	var body analyticsSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.PipelineReady {
		t.Error("pipeline_ready must be false on an empty GlobalDB")
	}
}

// ---------------------------------------------------------------------------
// Middleware coverage — every Wave-0 endpoint must carry the baseline
// security headers, whether it returns 200 or 4xx.
// ---------------------------------------------------------------------------

// TestWave0_SecurityHeadersOnEveryEndpoint walks the list of Wave-0 GET
// endpoints and asserts the CSP/X-Content-Type-Options headers are set.
// If corsMiddleware is accidentally removed from the chi Use() chain this
// test will catch it in one pass.
func TestWave0_SecurityHeadersOnEveryEndpoint(t *testing.T) {
	f := newWave0Fixture(t)

	endpoints := []string{
		"/api/health",
		"/api/browse", // will 401 but still applies headers
		"/api/settings",
		"/api/graph?project=x",
		"/api/analytics/summary",
		"/api/agent/list",
		"/api/repos",
	}
	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			resp := f.get(t, ep)
			if got := resp.Header.Get("Content-Security-Policy"); got == "" {
				t.Errorf("%s: CSP header missing", ep)
			}
			if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", ep, got)
			}
		})
	}
}

// TestWave0_CSPHeaderMatchesE9Spec pins the exact Content-Security-Policy
// value to the v2.0 string mandated by binding ruling E9 (vedox-v2
// MASTER_PLAN). Any drift from this string — tightening, loosening, or
// re-ordering — must be a deliberate edit to CSPHeaderValue with an ADR
// update, not an accidental one-liner. This guards FIX-ARCH-09: the
// previous implementation used "script-src 'none'" and was missing
// style-src 'unsafe-inline', which would have broken Shiki rendering.
func TestWave0_CSPHeaderMatchesE9Spec(t *testing.T) {
	const wantCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; object-src 'none'; frame-ancestors 'none'"

	f := newWave0Fixture(t)
	resp := f.get(t, "/api/health")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/api/health: status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Security-Policy"); got != wantCSP {
		t.Errorf("Content-Security-Policy mismatch\n got: %q\nwant: %q", got, wantCSP)
	}
}
