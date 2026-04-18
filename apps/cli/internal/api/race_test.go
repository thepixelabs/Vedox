package api_test

// Race tests for HTTP handlers that read from JobStores.
//
// Prior to WS-Q-17, handleGetScanJob and handleGenerateNamesStatus called
// jobStore.Get(id) and passed the returned *ScanJob / *GenerationJob to
// writeJSON. The encode then read job fields without holding the store
// mutex, racing with the in-flight scan / generation goroutine that
// mutates the same struct under the write lock. These tests pin the fix
// in place by driving the HTTP handlers concurrently with scan progress
// updates and asserting the race detector stays quiet.
//
// Run with:
//
//	go test -race -short -count=1 ./internal/api/...

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/api"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// raceFixture is a minimal HTTP server purpose-built for the race tests in
// this file. It deliberately does NOT reuse newTestServer / testFixture
// because those are tuned for functional integration tests; here we want
// direct access to the JobStore handles so we can submit work to them
// without going through the full validation path of the public POST
// endpoints (which would require provider binaries on PATH for AI).
type raceFixture struct {
	server        *httptest.Server
	workspaceRoot string
	jobStore      *scanner.JobStore
	aiJobStore    *ai.JobStore
}

// newRaceFixture builds the smallest-possible api.Server wired to its own
// SQLite DB, JobStore, and AI JobStore so race tests are fully isolated
// from each other and from the broader integration suite.
func newRaceFixture(t *testing.T) *raceFixture {
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

	jobStore := scanner.NewJobStore()
	aiJobStore := ai.NewJobStore(8)

	srv := api.NewServer(
		adapter,
		dbStore,
		resolved,
		jobStore,
		aiJobStore,
		store.NewProjectRegistry(),
		agentauth.PassthroughAuth(),
	)

	mux := http.NewServeMux()
	srv.Mount(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return &raceFixture{
		server:        ts,
		workspaceRoot: resolved,
		jobStore:      jobStore,
		aiJobStore:    aiJobStore,
	}
}

// TestRaceScanHandlerVsRunScan starts real scans and pounds GET /api/scan/:id
// from many goroutines while runScan is mutating each job's fields under the
// store lock. With handleGetScanJob using Snapshot() the encode is race-free.
// With the prior Get() implementation the race detector flagged a write to
// job.Status during JSON encode.
func TestRaceScanHandlerVsRunScan(t *testing.T) {
	f := newRaceFixture(t)

	// Seed a workspace with several .git roots so the scan has actual work
	// to do, widening the window where readers can overlap with runScan's
	// terminal critical section.
	for _, name := range []string{"alpha", "bravo", "charlie", "delta", "echo"} {
		dir := filepath.Join(f.workspaceRoot, name, ".git")
		if err := mkdirAll(dir); err != nil {
			t.Fatalf("mkdirAll: %v", err)
		}
	}

	const scanCount = 8
	jobIDs := make([]string, 0, scanCount)
	for i := 0; i < scanCount; i++ {
		j := f.jobStore.StartScan(f.workspaceRoot)
		jobIDs = append(jobIDs, j.ID)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	const readers = 6
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := f.server.Client()
			for {
				select {
				case <-stop:
					return
				default:
				}
				for _, id := range jobIDs {
					req, _ := http.NewRequest(http.MethodGet,
						f.server.URL+"/api/scan/"+id, nil)
					resp, err := client.Do(req)
					if err != nil {
						return
					}
					_ = resp.Body.Close()
				}
			}
		}()
	}

	// Let readers and runScan overlap long enough that some GETs land
	// while runScan holds the write lock.
	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()

	// Drain to terminal state so background goroutines don't outlive the
	// fixture (which would race with dbStore Close on cleanup).
	deadline := time.Now().Add(5 * time.Second)
	for _, id := range jobIDs {
		for time.Now().Before(deadline) {
			snap, ok := f.jobStore.Snapshot(id)
			if ok && (snap.Status == scanner.JobStatusDone || snap.Status == scanner.JobStatusError) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// TestRaceAIStatusHandlerVsRun drives GET /api/ai/generate-names/{jobId}
// concurrently with the AI run() goroutine that mutates the same
// *GenerationJob under the store lock. We submit through the JobStore
// directly (not POST /api/ai/generate-names) so the test is independent
// of provider binary availability — run() will fail fast with
// "binary not found on PATH" but still mutates job state along the way.
//
// With handleGenerateNamesStatus using Snapshot() the encode is race-free.
// With the prior Get() implementation the race detector flagged writes to
// job.Status / job.Error during JSON encode.
func TestRaceAIStatusHandlerVsRun(t *testing.T) {
	f := newRaceFixture(t)

	const jobs = 8
	jobIDs := make([]string, 0, jobs)
	for i := 0; i < jobs; i++ {
		j := f.aiJobStore.Submit(ai.GenerationRequest{
			Provider: ai.ProviderID("__vedox_test_no_such_provider__"),
			Timeout:  300 * time.Millisecond,
			Count:    5,
		})
		jobIDs = append(jobIDs, j.ID)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	const readers = 6
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := f.server.Client()
			for {
				select {
				case <-stop:
					return
				default:
				}
				for _, id := range jobIDs {
					req, _ := http.NewRequest(http.MethodGet,
						f.server.URL+"/api/ai/generate-names/"+id, nil)
					resp, err := client.Do(req)
					if err != nil {
						return
					}
					_ = resp.Body.Close()
				}
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()

	// Drain to terminal state.
	deadline := time.Now().Add(5 * time.Second)
	for _, id := range jobIDs {
		for time.Now().Before(deadline) {
			snap, ok := f.aiJobStore.Snapshot(id)
			if ok && (snap.Status == ai.JobDone || snap.Status == ai.JobError) {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}
