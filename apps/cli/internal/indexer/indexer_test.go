package indexer_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
