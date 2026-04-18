package indexer_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/indexer"
	"github.com/vedox/vedox/internal/store"
)

// openTestDB opens a real SQLite store in a temp directory.
func openTestDB(t *testing.T, root string) *db.Store {
	t.Helper()
	s, err := db.Open(db.Options{WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// openTestStore opens a real LocalAdapter.
func openTestStore(t *testing.T, root string) store.DocStore {
	t.Helper()
	a, err := store.NewLocalAdapter(root, nil)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return a
}

// startIndexer creates and starts an Indexer; cancels it after the test.
func startIndexer(t *testing.T, s store.DocStore, d *db.Store, root string) {
	t.Helper()
	ix := indexer.New(s, d, root)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		// Give the indexer a moment to shut down cleanly.
		time.Sleep(50 * time.Millisecond)
	})
	go func() { _ = ix.Start(ctx) }()
	// Small pause to let fsnotify register watches before we write files.
	time.Sleep(100 * time.Millisecond)
}

// writeMD writes content to path (absolute) and returns the workspace-relative path.
func writeMD(t *testing.T, root, name, content string) string {
	t.Helper()
	abs := filepath.Join(root, name)
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return name
}

// pollSearch polls Search until it returns at least minHits results or deadline passes.
func pollSearch(t *testing.T, d *db.Store, query string, minHits int, deadline time.Duration) []*db.SearchResult {
	t.Helper()
	ctx := context.Background()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		hits, err := d.Search(ctx, query, db.SearchFilters{})
		if err != nil {
			t.Fatalf("search %q: %v", query, err)
		}
		if len(hits) >= minHits {
			return hits
		}
		time.Sleep(20 * time.Millisecond)
	}
	return nil
}

// pollCount polls CountDocs until it equals want or deadline passes.
func pollCount(t *testing.T, d *db.Store, want int, deadline time.Duration) int {
	t.Helper()
	ctx := context.Background()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		n, err := d.CountDocs(ctx)
		if err != nil {
			t.Fatalf("CountDocs: %v", err)
		}
		if n == want {
			return n
		}
		time.Sleep(20 * time.Millisecond)
	}
	n, _ := d.CountDocs(ctx)
	return n
}

// TestWriteAppearsInFTS verifies that writing a .md file causes it to become
// searchable within 500ms.
func TestWriteAppearsInFTS(t *testing.T) {
	root := t.TempDir()
	s := openTestStore(t, root)
	d := openTestDB(t, root)
	startIndexer(t, s, d, root)

	writeMD(t, root, "hello.md", "# Hello World\n\nquantum-zebra-token\n")

	hits := pollSearch(t, d, "quantum-zebra-token", 1, 500*time.Millisecond)
	if len(hits) == 0 {
		t.Fatal("expected hello.md to appear in FTS within 500ms, but got 0 hits")
	}
	if hits[0].ID != "hello.md" {
		t.Fatalf("expected ID=hello.md, got %q", hits[0].ID)
	}
}

// TestDeleteDisappearsFromFTS verifies that removing a .md file removes it
// from the index within 500ms.
func TestDeleteDisappearsFromFTS(t *testing.T) {
	root := t.TempDir()
	s := openTestStore(t, root)
	d := openTestDB(t, root)

	// Write the file before starting the indexer so we can seed via UpsertDoc.
	abs := filepath.Join(root, "byebye.md")
	if err := os.WriteFile(abs, []byte("# Bye\n\nreticulated-spline-xyz\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	startIndexer(t, s, d, root)

	// Wait for the indexer to pick up the pre-existing file (Create event fires
	// on watch registration for existing entries on some platforms, but not all;
	// so we write it again to be sure).
	if err := os.WriteFile(abs, []byte("# Bye\n\nreticulated-spline-xyz\n"), 0o644); err != nil {
		t.Fatalf("re-write: %v", err)
	}
	hits := pollSearch(t, d, "reticulated-spline-xyz", 1, 500*time.Millisecond)
	if len(hits) == 0 {
		t.Fatal("file should be indexed before deletion")
	}

	// Now delete it.
	if err := os.Remove(abs); err != nil {
		t.Fatalf("remove: %v", err)
	}

	// Poll until the doc disappears.
	end := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(end) {
		hits2, _ := d.Search(context.Background(), "reticulated-spline-xyz", db.SearchFilters{})
		if len(hits2) == 0 {
			return // success
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("deleted file still appears in FTS after 500ms")
}

// TestDebounceFiresOnce verifies that 10 rapid writes to the same file result
// in exactly one FTS upsert (the debounce fires once, not 10 times).
//
// We measure this indirectly: write a file with a unique token in the last
// write, then verify the search returns exactly one hit for that token and
// the hit reflects the final content.
func TestDebounceFiresOnce(t *testing.T) {
	root := t.TempDir()
	s := openTestStore(t, root)
	d := openTestDB(t, root)
	startIndexer(t, s, d, root)

	abs := filepath.Join(root, "rapid.md")

	// 10 writes in ~100ms total (10ms apart).
	for i := 0; i < 10; i++ {
		content := fmt.Sprintf("# Rapid\n\nwrite-iteration-%d unique-debounce-final-token\n", i)
		if i == 9 {
			// Final write contains our sentinel.
			content = "# Rapid\n\ndebounce-sentinel-99 unique-debounce-final-token\n"
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("rapid write %d: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce + processing (300ms debounce + 200ms margin).
	hits := pollSearch(t, d, "debounce-sentinel-99", 1, 600*time.Millisecond)
	if len(hits) == 0 {
		t.Fatal("expected debounce-sentinel-99 in FTS after rapid writes")
	}

	// Exactly one row should match — debounce must not have written stale content.
	all, err := d.Search(context.Background(), "unique-debounce-final-token", db.SearchFilters{})
	if err != nil {
		t.Fatalf("search all: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected exactly 1 FTS row for rapid.md, got %d", len(all))
	}
}

// TestDraftFilesNotIndexed verifies that files under .vedox/drafts/ are never
// inserted into the FTS index.
func TestDraftFilesNotIndexed(t *testing.T) {
	root := t.TempDir()

	// Create the drafts directory.
	draftsDir := filepath.Join(root, ".vedox", "drafts")
	if err := os.MkdirAll(draftsDir, 0o755); err != nil {
		t.Fatalf("mkdir drafts: %v", err)
	}

	s := openTestStore(t, root)
	d := openTestDB(t, root)
	startIndexer(t, s, d, root)

	// Write a normal file (control) and a draft file.
	writeMD(t, root, "normal.md", "# Normal\n\nnormal-unique-alpha-token\n")

	draftPath := filepath.Join(draftsDir, "draft.md")
	if err := os.WriteFile(draftPath, []byte("# Draft\n\ndraft-unique-beta-token\n"), 0o644); err != nil {
		t.Fatalf("write draft: %v", err)
	}

	// Normal file should appear.
	hits := pollSearch(t, d, "normal-unique-alpha-token", 1, 500*time.Millisecond)
	if len(hits) == 0 {
		t.Fatal("normal.md should be indexed")
	}

	// Draft file must NOT appear — wait the full debounce window plus margin
	// to be certain the indexer had time to process it (and should have skipped it).
	time.Sleep(debounceDuration() + 100*time.Millisecond)

	draftHits, err := d.Search(context.Background(), "draft-unique-beta-token", db.SearchFilters{})
	if err != nil {
		t.Fatalf("search draft: %v", err)
	}
	if len(draftHits) != 0 {
		t.Fatalf("draft file should not be in FTS, got %d hits", len(draftHits))
	}
}

// debounceDuration exposes the package constant for tests without exporting it
// from the production code.
func debounceDuration() time.Duration {
	return 300 * time.Millisecond
}

// countIndexerGoroutines returns the number of live goroutines whose stack
// references the indexer package. We filter on package path rather than using
// runtime.NumGoroutine() because unrelated runtime goroutines (GC, netpoller,
// test harness) would otherwise produce noise. Used as a belt-and-braces check
// alongside the timing assertion in TestStopNoGoroutineLeak.
func countIndexerGoroutines() int {
	buf := make([]byte, 64*1024)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	count := 0
	for _, block := range strings.Split(string(buf), "\n\n") {
		if strings.Contains(block, "internal/indexer.") {
			count++
		}
	}
	return count
}

// TestStopNoGoroutineLeak is the QA-architect regression test for indexer.Stop.
//
// The bug before the fix:
//
//  1. scheduleDebounce armed a time.AfterFunc that, on firing, spawned a
//     goroutine calling processPath → ix.db.UpsertDoc.
//  2. Stop() only closed stopCh and returned — it did NOT wait for AfterFunc
//     callbacks that had already fired. drainTimers called timer.Stop() on
//     each pending timer, but the Go docs for Timer.Stop warn:
//     "if t.Stop returns false, then the timer has already expired and the
//     function f has been started in its own goroutine; Stop does not wait
//     for f to complete before returning."
//  3. So callbacks that had just fired (or were about to) kept running after
//     Stop returned. If the caller then closed ix.db, those callbacks raced
//     the close — classic use-after-free, caught by -race.
//
// We reproduce the window deterministically by waiting just past the debounce
// fire time before calling Stop, so the in-flight callbacks are running
// through processPath when Stop begins shutdown. The fix installs a WaitGroup
// that scheduleDebounce Adds to and each AfterFunc Dones; runLoop's defer
// waits on that WaitGroup before closing runDone (and before Stop returns).
//
// Assertions:
//
//   - After Stop returns, runtime.Stack must show zero goroutines whose stack
//     references the indexer package. Polling for 500ms covers any remaining
//     AfterFunc firings from the pre-fix buggy path.
//   - Start's goroutine must have exited (proves runLoop unwound, not just
//     stopCh closed and forgotten).
//
// Run with -race to additionally catch any data-race-after-close on ix.db
// when a buggy Stop lets a callback keep running past teardown.
func TestStopNoGoroutineLeak(t *testing.T) {
	root := t.TempDir()
	s := openTestStore(t, root)
	d := openTestDB(t, root)

	ix := indexer.New(s, d, root)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startExited := make(chan struct{})
	go func() {
		_ = ix.Start(ctx)
		close(startExited)
	}()

	// Give Start time to register fsnotify watches before we write files.
	time.Sleep(100 * time.Millisecond)

	// Warm-up: write a probe file and wait for it to land in the FTS index.
	// This is positive proof that the fsnotify → handleEvent → scheduleDebounce
	// → AfterFunc → upsertDoc pipeline is end-to-end functional on this host.
	// Without it, a test machine with slow fsnotify delivery might never arm
	// any debounce timers in the probe window below, producing a false PASS
	// (no callbacks to leak means no leak to catch).
	if err := os.WriteFile(filepath.Join(root, "warmup.md"),
		[]byte("# Warmup\n\nindexer-leak-warmup-token\n"), 0o644); err != nil {
		t.Fatalf("warmup write: %v", err)
	}
	if hits := pollSearch(t, d, "indexer-leak-warmup-token", 1, 2*time.Second); len(hits) == 0 {
		t.Skip("fsnotify did not deliver warmup event within 2s — host is too slow for this timing-sensitive test")
	}

	// Sanity check: runLoop is running, so we expect indexer goroutines > 0.
	if got := countIndexerGoroutines(); got < 1 {
		t.Fatalf("expected indexer goroutines > 0 while running, got %d", got)
	}

	// Arm several debounce timers via real file writes. Each scheduleDebounce
	// call registers an AfterFunc set to fire after debounceDuration (300ms).
	const n = 16
	for i := 0; i < n; i++ {
		p := filepath.Join(root, fmt.Sprintf("leak-probe-%d.md", i))
		if err := os.WriteFile(p, []byte("# probe\nleak-probe-body-token-zxyq\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	// Wait just PAST the debounce fire time so those AfterFuncs are actively
	// running processPath → ix.db.UpsertDoc when we call Stop. This is the
	// narrow window where the bug manifests: drainTimers' t.Stop() returns
	// false for a firing timer, so the pre-fix code cannot cancel or wait
	// for the in-flight callback.
	time.Sleep(debounceDuration() + 20*time.Millisecond)

	// Stop must wait for every in-flight AfterFunc goroutine to complete
	// before returning, otherwise callers who close ix.db after Stop will
	// race the callback's UpsertDoc.
	ix.Stop()

	// Start's goroutine must be gone — proves runLoop unwound fully.
	select {
	case <-startExited:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Start goroutine did not exit within 500ms of Stop returning — Stop did not wait for runLoop to unwind")
	}

	// Goroutine census: any indexer goroutine surviving Stop is a leak.
	// Poll for 500ms to catch any callback still running. Sample every 5ms so
	// we don't miss a short-lived goroutine.
	deadline := time.Now().Add(500 * time.Millisecond)
	maxSeen := 0
	for time.Now().Before(deadline) {
		if c := countIndexerGoroutines(); c > maxSeen {
			maxSeen = c
		}
		time.Sleep(5 * time.Millisecond)
	}
	if maxSeen > 0 {
		buf := make([]byte, 64*1024)
		m := runtime.Stack(buf, true)
		t.Fatalf("indexer goroutine leak after Stop: peak indexer goroutines in 500ms post-Stop window = %d (expected 0)\n\nstacks:\n%s",
			maxSeen, buf[:m])
	}
}

// TestStopWithoutStart verifies Stop is safe when Start was never called —
// a legitimate path if indexer setup short-circuits early. Without care
// wg.Wait could block forever if Add was called without a matching Done,
// and a second Stop could panic on close of a closed channel.
func TestStopWithoutStart(t *testing.T) {
	root := t.TempDir()
	s := openTestStore(t, root)
	d := openTestDB(t, root)

	ix := indexer.New(s, d, root)

	done := make(chan struct{})
	go func() {
		ix.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop without Start blocked — wg.Wait deadlocked")
	}

	// Idempotent: calling Stop again must not panic and must return promptly.
	done2 := make(chan struct{})
	go func() {
		ix.Stop()
		close(done2)
	}()
	select {
	case <-done2:
	case <-time.After(time.Second):
		t.Fatal("second Stop call blocked")
	}
}
