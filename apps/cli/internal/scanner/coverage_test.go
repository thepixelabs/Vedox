package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// JobStore / progress.go
// ---------------------------------------------------------------------------

// TestNewJobStore_IsUsable verifies that NewJobStore returns a store ready
// for use without further initialisation.
func TestNewJobStore_IsUsable(t *testing.T) {
	js := NewJobStore()
	if js == nil {
		t.Fatal("NewJobStore returned nil")
	}
	if js.Scanner() == nil {
		t.Fatal("Scanner() returned nil")
	}
}

// TestJobStore_Get_UnknownID returns nil for an ID that was never registered.
func TestJobStore_Get_UnknownID(t *testing.T) {
	js := NewJobStore()
	if got := js.Get("nonexistent"); got != nil {
		t.Errorf("expected nil for unknown id, got %+v", got)
	}
}

// TestJobStore_LastCompleted_NoScanYet returns nil when no scan has finished.
func TestJobStore_LastCompleted_NoScanYet(t *testing.T) {
	js := NewJobStore()
	if got := js.LastCompleted("/some/root"); got != nil {
		t.Errorf("expected nil before any scan, got %+v", got)
	}
}

// TestStartScan_ReturnsJobImmediately verifies that StartScan returns a job
// with a populated ID and correct WorkspaceRoot, then waits for terminal state
// so the background goroutine has finished before t.TempDir is cleaned up.
func TestStartScan_ReturnsJobImmediately(t *testing.T) {
	ws := t.TempDir()
	js := NewJobStore()
	job := js.StartScan(ws)
	if job == nil {
		t.Fatal("StartScan returned nil")
	}
	// ID and WorkspaceRoot are written before the goroutine starts and never
	// mutated again — safe to read without the lock.
	if job.ID == "" {
		t.Error("job.ID is empty")
	}
	if job.WorkspaceRoot != ws {
		t.Errorf("WorkspaceRoot = %q, want %q", job.WorkspaceRoot, ws)
	}
	// Wait for the goroutine to reach a terminal state so the background
	// scan has finished writing to the tempdir before t.Cleanup removes it.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snap, ok := snapshotJob(js, job.ID)
		if !ok {
			t.Fatal("snapshotJob returned false for freshly started job")
		}
		switch snap.Status {
		case JobStatusDone, JobStatusError:
			return // terminal — test complete
		case JobStatusPending, JobStatusRunning:
			// still in flight — keep polling
		default:
			t.Errorf("unexpected job status %q", snap.Status)
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("job did not reach terminal state within 5s")
}

// TestStartScan_JobReachableByGet verifies that Get can find the job by ID,
// then waits for the goroutine to complete before the tempdir is cleaned up.
func TestStartScan_JobReachableByGet(t *testing.T) {
	ws := t.TempDir()
	js := NewJobStore()
	job := js.StartScan(ws)

	snap, ok := snapshotJob(js, job.ID)
	if !ok {
		t.Fatal("snapshotJob returned false for a job we just created")
	}
	// ID is set before the goroutine starts and never mutated — safe to compare.
	if snap.ID != job.ID {
		t.Errorf("snapshot returned wrong job: %q vs %q", snap.ID, job.ID)
	}
	// Wait for terminal state so the goroutine finishes before tempdir cleanup.
	waitTerminal(t, js, job.ID)
}

// snapshotJob returns a value copy of the ScanJob identified by id, holding
// the store's read lock for the duration of the copy. Returns (ScanJob{}, false)
// if the id is not found.
func snapshotJob(js *JobStore, id string) (ScanJob, bool) {
	js.mu.RLock()
	defer js.mu.RUnlock()
	p, ok := js.jobs[id]
	if !ok {
		return ScanJob{}, false
	}
	return *p, true
}

// waitTerminal polls snapshotJob until the job reaches a terminal state (done
// or error) or the 5-second deadline expires. This ensures the background
// goroutine has finished writing to the workspace before t.TempDir cleanup runs.
func waitTerminal(t *testing.T, js *JobStore, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		snap, ok := snapshotJob(js, id)
		if ok && (snap.Status == JobStatusDone || snap.Status == JobStatusError) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("job did not reach terminal state within 5s")
}

// TestStartScan_CompletesAndIsRetrievable waits for the scan to finish and
// asserts that LastCompleted returns it.
func TestStartScan_CompletesAndIsRetrievable(t *testing.T) {
	ws := t.TempDir()
	mkGitDir(t, filepath.Join(ws, "proj"))

	js := NewJobStore()
	job := js.StartScan(ws)

	// Poll until done or error (max 5 s), taking a race-safe snapshot each time.
	deadline := time.Now().Add(5 * time.Second)
	var snap ScanJob
	for time.Now().Before(deadline) {
		s, ok := snapshotJob(js, job.ID)
		if !ok {
			t.Fatal("job disappeared from store")
		}
		snap = s
		if snap.Status == JobStatusDone || snap.Status == JobStatusError {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if snap.Status != JobStatusDone {
		t.Fatalf("expected done status, got %q (error: %s)", snap.Status, snap.Error)
	}
	if snap.Total != 1 {
		t.Errorf("expected Total=1, got %d", snap.Total)
	}
	if snap.Scanned != 1 {
		t.Errorf("expected Scanned=1, got %d", snap.Scanned)
	}
	if snap.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}

	// Use the store lock to snapshot LastCompleted atomically so the race
	// detector sees a properly locked read of the job pointer's fields.
	js.mu.RLock()
	var lastID string
	if lj := js.lastDone[ws]; lj != nil {
		lastID = lj.ID
	}
	js.mu.RUnlock()

	if lastID == "" {
		t.Fatal("LastCompleted returned nil after successful scan")
	}
	if lastID != job.ID {
		t.Errorf("LastCompleted returned wrong job: %q vs %q", lastID, job.ID)
	}
}

// TestStartScan_ErrorPath triggers a scan on a path that does not exist so
// runScan hits the error branch.
func TestStartScan_ErrorPath(t *testing.T) {
	js := NewJobStore()
	job := js.StartScan("/this/path/does/not/exist/ever")

	deadline := time.Now().Add(5 * time.Second)
	var snap ScanJob
	for time.Now().Before(deadline) {
		s, ok := snapshotJob(js, job.ID)
		if !ok {
			t.Fatal("job disappeared from store")
		}
		snap = s
		if snap.Status == JobStatusDone || snap.Status == JobStatusError {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// walkDir logs and skips unreadable dirs rather than returning an error
	// (resilient design). So the scan completes with zero projects, not an
	// error job. Accept both outcomes; the important invariant is that the
	// job reaches a terminal state and is not stuck in running/pending.
	if snap.Status != JobStatusDone && snap.Status != JobStatusError {
		t.Errorf("expected terminal status, got %q", snap.Status)
	}
}

// TestInvalidateCache clears the lastDone entry so the next LastCompleted
// call returns nil.
func TestInvalidateCache(t *testing.T) {
	ws := t.TempDir()
	mkGitDir(t, filepath.Join(ws, "proj"))

	js := NewJobStore()
	job := js.StartScan(ws)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s, ok := snapshotJob(js, job.ID)
		if ok && (s.Status == JobStatusDone || s.Status == JobStatusError) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if js.LastCompleted(ws) == nil {
		t.Fatal("expected LastCompleted to have an entry before invalidation")
	}

	js.InvalidateCache(ws)

	if got := js.LastCompleted(ws); got != nil {
		t.Errorf("expected nil after InvalidateCache, got %+v", got)
	}
}

// TestNewJobID_Format verifies that newJobID returns a 32-character hex string.
func TestNewJobID_Format(t *testing.T) {
	id := newJobID()
	if len(id) != 32 {
		t.Errorf("expected 32-char hex id, got %q (len=%d)", id, len(id))
	}
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex char %q in id %q", c, id)
			break
		}
	}
}

// TestNewJobID_Unique verifies that two consecutive calls produce distinct IDs.
func TestNewJobID_Unique(t *testing.T) {
	a := newJobID()
	b := newJobID()
	if a == b {
		t.Errorf("two consecutive newJobID calls returned the same id: %q", a)
	}
}

// TestStartScan_Concurrent launches several concurrent scans to exercise the
// mutex paths in progress.go without data races.
func TestStartScan_Concurrent(t *testing.T) {
	const n = 5
	js := NewJobStore()

	var jobIDs [n]string
	var mu sync.Mutex

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ws := t.TempDir()
			job := js.StartScan(ws)
			if job == nil {
				t.Errorf("StartScan returned nil")
				return
			}
			mu.Lock()
			for j := range jobIDs {
				if jobIDs[j] == "" {
					jobIDs[j] = job.ID
					break
				}
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	// Wait for all scan goroutines to reach terminal state before the tempdir
	// cleanup fires.
	for _, id := range jobIDs {
		if id != "" {
			waitTerminal(t, js, id)
		}
	}
}

// ---------------------------------------------------------------------------
// scanner.go — uncovered branches in loadCache / saveCache
// ---------------------------------------------------------------------------

// TestLoadCache_CorruptJSON verifies that a corrupt cache file is treated as
// a cache miss (starts fresh) rather than returning an error.
func TestLoadCache_CorruptJSON(t *testing.T) {
	ws := t.TempDir()
	cacheDir := filepath.Join(ws, ".vedox")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write syntactically invalid JSON.
	if err := os.WriteFile(filepath.Join(cacheDir, "scan-cache.json"), []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	s := NewScanner()
	// Scan should succeed and return an empty result (corrupt cache = fresh start).
	projects, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("Scan with corrupt cache: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects from empty workspace, got %d", len(projects))
	}
}

// TestLoadCache_ValidDiskCache verifies that loadCache reads a valid JSON cache
// from disk when no in-memory copy exists yet.
func TestLoadCache_ValidDiskCache(t *testing.T) {
	ws := t.TempDir()
	projDir := filepath.Join(ws, "myproject")
	mkGitDir(t, projDir)

	// First scanner populates the disk cache.
	s1 := NewScanner()
	first, err := s1.Scan(ws)
	if err != nil || len(first) != 1 {
		t.Fatalf("first scan: err=%v count=%d", err, len(first))
	}
	firstScanned := first[0].LastScanned

	// Second scanner has no in-memory cache; it must read from disk.
	s2 := NewScanner()
	second, err := s2.Scan(ws)
	if err != nil || len(second) != 1 {
		t.Fatalf("second scan: err=%v count=%d", err, len(second))
	}
	// Disk cache hit: LastScanned must equal the first scan's timestamp.
	if !second[0].LastScanned.Equal(firstScanned) {
		t.Errorf("expected disk cache hit (same LastScanned), got first=%v second=%v",
			firstScanned, second[0].LastScanned)
	}
}

// TestSaveCache_PersistAndReload verifies that saveCache writes JSON that
// loadCache can parse back.
func TestSaveCache_PersistAndReload(t *testing.T) {
	ws := t.TempDir()
	mkGitDir(t, filepath.Join(ws, "project-a"))

	s := NewScanner()
	if _, err := s.Scan(ws); err != nil {
		t.Fatalf("scan: %v", err)
	}

	// Read back the raw JSON to ensure it is syntactically valid and contains
	// the expected project name.
	data, err := os.ReadFile(filepath.Join(ws, cacheFile))
	if err != nil {
		t.Fatalf("read cache file: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("invalid cache JSON: %v", err)
	}
	if len(raw) != 1 {
		t.Errorf("expected 1 entry in cache, got %d", len(raw))
	}
}

// TestScan_CacheMissAfterMtimeChange verifies that a project is re-scanned
// when its directory mtime changes (simulated by adding a file).
func TestScan_CacheMissAfterMtimeChange(t *testing.T) {
	ws := t.TempDir()
	proj := filepath.Join(ws, "myproject")
	mkGitDir(t, proj)

	s := NewScanner()
	first, err := s.Scan(ws)
	if err != nil || len(first) != 1 {
		t.Fatalf("first scan: err=%v count=%d", err, len(first))
	}

	// Force the project directory's mtime to change by writing a new file.
	// Sleep briefly to ensure a measurably different mtime on filesystems with
	// 1-second granularity (common on Linux tmpfs).
	time.Sleep(10 * time.Millisecond)
	mkFile(t, filepath.Join(proj, "new.md"))
	// Touch the project directory itself to guarantee mtime changes.
	now := time.Now()
	if err := os.Chtimes(proj, now, now); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	second, err := s.Scan(ws)
	if err != nil || len(second) != 1 {
		t.Fatalf("second scan: err=%v count=%d", err, len(second))
	}

	// After mtime change the cache entry is stale; doc count should include
	// the new .md file.
	if second[0].DocCount != 1 {
		t.Errorf("expected DocCount=1 after adding new.md, got %d", second[0].DocCount)
	}
}

// TestCountMarkdownFiles_SkipsNodeModules exercises the node_modules pruning
// branch inside countMarkdownFiles.
func TestCountMarkdownFiles_SkipsNodeModules(t *testing.T) {
	root := t.TempDir()
	// These should be counted.
	mkFile(t, filepath.Join(root, "README.md"))
	mkFile(t, filepath.Join(root, "docs", "guide.md"))
	// These should NOT be counted.
	mkFile(t, filepath.Join(root, "node_modules", "pkg", "README.md"))
	mkFile(t, filepath.Join(root, "vendor", "lib", "README.md"))

	count := countMarkdownFiles(root)
	if count != 2 {
		t.Errorf("expected 2 md files (excluding node_modules+vendor), got %d", count)
	}
}

// TestScan_DetectedFramework verifies that framework detection runs as part of
// the normal scan and the result is non-empty.
func TestScan_DetectedFramework(t *testing.T) {
	ws := t.TempDir()
	proj := filepath.Join(ws, "myproject")
	mkGitDir(t, proj)
	mkFile(t, filepath.Join(proj, "mkdocs.yml"))

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil || len(projects) != 1 {
		t.Fatalf("scan: err=%v count=%d", err, len(projects))
	}
	if projects[0].DetectedFramework != FrameworkMkDocs {
		t.Errorf("expected mkdocs framework, got %q", projects[0].DetectedFramework)
	}
}

// TestScan_AbsPath verifies that AbsPath is an absolute path to the project root.
func TestScan_AbsPath(t *testing.T) {
	ws := t.TempDir()
	mkGitDir(t, filepath.Join(ws, "proj"))

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil || len(projects) != 1 {
		t.Fatalf("scan: err=%v count=%d", err, len(projects))
	}
	if !filepath.IsAbs(projects[0].AbsPath) {
		t.Errorf("AbsPath %q is not absolute", projects[0].AbsPath)
	}
}

// TestScan_LastScannedIsSet verifies that LastScanned is populated on fresh scan.
func TestScan_LastScannedIsSet(t *testing.T) {
	ws := t.TempDir()
	mkGitDir(t, filepath.Join(ws, "proj"))

	before := time.Now().Add(-time.Second)
	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil || len(projects) != 1 {
		t.Fatalf("scan: err=%v count=%d", err, len(projects))
	}
	if !projects[0].LastScanned.After(before) {
		t.Errorf("LastScanned %v is not after start time %v", projects[0].LastScanned, before)
	}
}

// TestScan_GitWorktreeFile verifies that a plain .git file (git worktree)
// is accepted as a project root alongside a normal .git directory.
func TestScan_GitWorktreeFile(t *testing.T) {
	ws := t.TempDir()
	proj := filepath.Join(ws, "worktree-proj")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Write a plain .git file (the format used by `git worktree add`).
	if err := os.WriteFile(filepath.Join(proj, ".git"), []byte("gitdir: ../main/.git/worktrees/wt\n"), 0o644); err != nil {
		t.Fatalf("write .git file: %v", err)
	}

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project for git worktree, got %d", len(projects))
	}
	if projects[0].Name != "worktree-proj" {
		t.Errorf("expected name 'worktree-proj', got %q", projects[0].Name)
	}
}
