package api

// Tests for GET /api/scan (handleGetScanSummary).
//
// The production frontend's onboarding step hits this synchronous endpoint
// before doing anything else, so its behaviour matters more than most
// GETs: an empty {projects:[]} with status 200 is the happy path, and a
// 405/404 (which is what the old router returned) broke first-run.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// scanSummaryFixture is a thin test server wired with a real scanner JobStore
// so we can seed .git directories on disk and exercise both the fast path
// (cached job) and the slow path (synchronous scan).
type scanSummaryFixture struct {
	server        *httptest.Server
	workspaceRoot string
	jobStore      *scanner.JobStore
}

func newScanSummaryFixture(t *testing.T) *scanSummaryFixture {
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
	wsDB, err := db.Open(db.Options{WorkspaceRoot: resolved})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = wsDB.Close() })

	js := scanner.NewJobStore()
	srv := NewServer(adapter, wsDB, resolved, js, ai.NewJobStore(3), store.NewProjectRegistry(), agentauth.PassthroughAuth())

	mux := http.NewServeMux()
	srv.Mount(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return &scanSummaryFixture{server: ts, workspaceRoot: resolved, jobStore: js}
}

// TestScanSummary_EmptyWorkspace confirms 200 + {"projects":[]} on an empty
// workspace. The test also asserts that the synchronous fallback path runs
// (since no scan job was ever started) without panicking or returning null.
func TestScanSummary_EmptyWorkspace(t *testing.T) {
	f := newScanSummaryFixture(t)

	resp, err := f.server.Client().Get(f.server.URL + "/api/scan")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got scanSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Nil is acceptable here (the slice was allocated with len=0 inside the
	// handler, but json.Decoder may still round-trip to a nil slice if the
	// server emitted []). We care that it's never the literal JSON null.
	if got.Projects == nil {
		// The handler allocates out := make([]detectedProject, 0, len(scanned))
		// which encodes to [] — a nil slice here would indicate the server
		// emitted "null", which breaks the frontend .length access.
		t.Error("projects field is nil, expected an empty slice (encoded []) not null")
	}
	if len(got.Projects) != 0 {
		t.Errorf("expected 0 projects, got %d: %+v", len(got.Projects), got.Projects)
	}
}

// TestScanSummary_WithGitProject seeds one git root and asserts the GET
// surfaces it in the DetectedProject shape the frontend expects.
func TestScanSummary_WithGitProject(t *testing.T) {
	f := newScanSummaryFixture(t)

	projectDir := filepath.Join(f.workspaceRoot, "alpha")
	if err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	resp, err := f.server.Client().Get(f.server.URL + "/api/scan")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got scanSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(got.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d: %+v", len(got.Projects), got.Projects)
	}
	p := got.Projects[0]
	if p.Name != "alpha" {
		t.Errorf("project.name = %q, want %q", p.Name, "alpha")
	}
	if !p.HasGit {
		t.Error("project.hasGit = false, want true (scanner only yields .git roots)")
	}
	if p.Path != projectDir {
		t.Errorf("project.path = %q, want %q", p.Path, projectDir)
	}
}

// TestScanSummary_UsesCache seeds a completed scan in the JobStore directly
// and asserts the handler returns those results without re-scanning. We
// verify this by placing a second .git directory on disk AFTER seeding the
// cache — the cached result must not include it.
func TestScanSummary_UsesCache(t *testing.T) {
	f := newScanSummaryFixture(t)

	// Seed the cache by running the first scan through the async JobStore.
	// This is the same path the production daemon follows after a POST /api/scan.
	early := filepath.Join(f.workspaceRoot, "beta")
	if err := os.MkdirAll(filepath.Join(early, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir early: %v", err)
	}
	// Prime LastCompleted by running a synchronous scan and dropping the
	// results into the JobStore. We cannot reach runScan directly, so we
	// use Scanner().Scan then manually drive a StartScan.
	job := f.jobStore.StartScan(f.workspaceRoot)
	// Wait for completion — avoid a polling loop by blocking on
	// LastCompletedSnapshot; production code never uses the pointer accessor
	// concurrently with runScan under -race.
	for {
		if _, ok := f.jobStore.LastCompletedSnapshot(f.workspaceRoot); ok {
			break
		}
	}
	_ = job // silence unused warning

	// Now add a second project AFTER the cache is populated.
	late := filepath.Join(f.workspaceRoot, "gamma")
	if err := os.MkdirAll(filepath.Join(late, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir late: %v", err)
	}

	resp, err := f.server.Client().Get(f.server.URL + "/api/scan")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var got scanSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// The handler must have used the cache, so only "beta" should show up.
	// "gamma" exists on disk but was added after the cached scan ran.
	foundBeta := false
	for _, p := range got.Projects {
		if p.Name == "beta" {
			foundBeta = true
		}
		if p.Name == "gamma" {
			t.Errorf("scan summary returned 'gamma' — cache was not used")
		}
	}
	if !foundBeta {
		t.Errorf("scan summary did not include cached 'beta': %+v", got.Projects)
	}
}
