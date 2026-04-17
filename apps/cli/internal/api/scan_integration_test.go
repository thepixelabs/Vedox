package api_test

// Integration tests for POST /api/scan and GET /api/scan/:jobId.
//
// handleGetScanJob now json-encodes via JobStore.Snapshot, which returns a
// value copy taken under the store mutex (see WS-Q-17 for the original race
// write-up). Tests still wait for completion before asserting Projects
// content (the scan must actually finish to populate the field), but the
// HTTP path itself is race-free under -race regardless of timing.

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

// scanJob mirrors the API response shape for GET /api/scan/:jobId. Defined
// here so the test stays insulated from internal renames.
type scanJob struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Projects []struct {
		Name    string `json:"name"`
		AbsPath string `json:"absPath"`
		RelPath string `json:"relPath"`
	} `json:"projects"`
}

// startScanReq mirrors the API request body for POST /api/scan.
type startScanReq struct {
	WorkspaceRoot string `json:"workspaceRoot"`
}

// waitForScanCompletion blocks until the JobStore reports a completed scan
// for the fixture's workspaceRoot, or until the deadline passes.
func waitForScanCompletion(t *testing.T, f *testFixture) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if f.jobStore.LastCompleted(f.workspaceRoot) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("scan never completed for %s", f.workspaceRoot)
}

// fetchScanJob GETs /api/scan/:jobID and decodes the result. Safe to call
// only after waitForScanCompletion has returned (otherwise it races runScan).
func fetchScanJob(t *testing.T, f *testFixture, jobID string) scanJob {
	t.Helper()
	resp := f.do(t, http.MethodGet, "/api/scan/"+jobID, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/scan/%s = %d (body=%s)", jobID, resp.StatusCode, readBody(t, resp))
	}
	var job scanJob
	decodeJSON(t, resp, &job)
	return job
}

// startScan posts to /api/scan with the given root and returns the new job ID.
func startScan(t *testing.T, f *testFixture, root string) string {
	t.Helper()
	resp := f.do(t, http.MethodPost, "/api/scan", startScanReq{WorkspaceRoot: root})
	// handleStartScan returns 202 Accepted, not 200 — the scan is async.
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST /api/scan = %d, want 202 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	var body struct {
		JobID string `json:"jobId"`
	}
	decodeJSON(t, resp, &body)
	if body.JobID == "" {
		t.Fatalf("response missing jobId")
	}
	return body.JobID
}

// TestStartScan_ReturnsJobId issues a POST against an empty workspace and
// asserts the response is a valid job id. We wait for completion before the
// test exits so the spawned goroutine doesn't outlive the fixture (which would
// race against the dbStore Close cleanup).
func TestStartScan_ReturnsJobId(t *testing.T) {
	f := newTestServer(t)
	id := startScan(t, f, f.workspaceRoot)
	if len(id) < 16 {
		t.Errorf("jobID looks too short: %q", id)
	}
	waitForScanCompletion(t, f)
}

// TestGetScanJob_NotFound verifies the documented 404 response (with VDX-101)
// for a job id that was never created.
func TestGetScanJob_NotFound(t *testing.T) {
	f := newTestServer(t)
	resp := f.do(t, http.MethodGet, "/api/scan/does-not-exist", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// TestScanFindsGitProject seeds a workspace with one project root and asserts
// the scanner discovers it.
func TestScanFindsGitProject(t *testing.T) {
	f := newTestServer(t)

	projectDir := filepath.Join(f.workspaceRoot, "alpha")
	if err := mkdirAll(filepath.Join(projectDir, ".git")); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	id := startScan(t, f, f.workspaceRoot)
	waitForScanCompletion(t, f)
	job := fetchScanJob(t, f, id)

	if job.Status != "done" {
		t.Fatalf("scan status = %q, want done", job.Status)
	}
	found := false
	for _, p := range job.Projects {
		if p.Name == "alpha" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find 'alpha' in scan results, got %+v", job.Projects)
	}
}

// TestScanDepthLimit asserts the scanner stops descending below maxDepth (5).
//
// Detail: walkDir starts at depth 0 (workspace root), visits each child with
// depth+1, and refuses to recurse INTO a directory once depth exceeds 5. That
// means depth-6 entries are still VISITED (and discoverable as projects),
// while depth-7+ entries are unreachable. To prove the limit we place .git at
// depth 7.
func TestScanDepthLimit(t *testing.T) {
	f := newTestServer(t)

	// Control: project at depth 1 — must be found.
	if err := mkdirAll(filepath.Join(f.workspaceRoot, "shallow", ".git")); err != nil {
		t.Fatalf("mkdir shallow: %v", err)
	}

	// Buried: 7 levels deep — must NOT be found.
	deep := filepath.Join(f.workspaceRoot, "a", "b", "c", "d", "e", "f", "buried")
	if err := mkdirAll(filepath.Join(deep, ".git")); err != nil {
		t.Fatalf("mkdir deep: %v", err)
	}

	id := startScan(t, f, f.workspaceRoot)
	waitForScanCompletion(t, f)
	job := fetchScanJob(t, f, id)

	for _, p := range job.Projects {
		if p.Name == "buried" {
			t.Errorf("scanner returned project 'buried' from below max depth")
		}
	}
	foundShallow := false
	for _, p := range job.Projects {
		if p.Name == "shallow" {
			foundShallow = true
		}
	}
	if !foundShallow {
		t.Errorf("expected to find shallow control project, got %+v", job.Projects)
	}
}

// TestScanExcludesNodeModules asserts that a .git directory nested inside
// node_modules is invisible to the scanner.
func TestScanExcludesNodeModules(t *testing.T) {
	f := newTestServer(t)

	if err := mkdirAll(filepath.Join(f.workspaceRoot, "node_modules", "vendored", ".git")); err != nil {
		t.Fatalf("mkdir vendored: %v", err)
	}

	id := startScan(t, f, f.workspaceRoot)
	waitForScanCompletion(t, f)
	job := fetchScanJob(t, f, id)

	for _, p := range job.Projects {
		if p.Name == "vendored" {
			t.Errorf("scanner returned project from inside node_modules: %+v", p)
		}
	}
}
