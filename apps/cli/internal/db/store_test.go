package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fsStore is a tiny DocStore that walks *.md files on disk. It lives
// in the test file so the production db package stays free of file
// I/O; the real LocalAdapter will land in the docstore package.
type fsStore struct{}

func (fsStore) WalkDocs(root string, fn func(*Doc) error) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".vedox" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".md") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		sum := sha256.Sum256(b)
		d := &Doc{
			ID:          rel,
			Project:     "test",
			Title:       strings.TrimSuffix(filepath.Base(path), ".md"),
			Type:        "how-to",
			Status:      "published",
			Date:        "2026-04-07",
			Tags:        []string{"alpha", "beta"},
			Author:      "tester",
			ContentHash: hex.EncodeToString(sum[:]),
			ModTime:     info.ModTime().UTC().Format(time.RFC3339),
			Size:        info.Size(),
			Body:        string(b),
		}
		return fn(d)
	})
}

func newTestWorkspace(t *testing.T, n int) string {
	t.Helper()
	root := t.TempDir()
	for i := 0; i < n; i++ {
		p := filepath.Join(root, fmt.Sprintf("doc-%02d.md", i))
		body := fmt.Sprintf("# Document %02d\n\nQuantum flux and reticulated splines — token-%02d.\n", i, i)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	return root
}

func openStore(t *testing.T, root string) *Store {
	t.Helper()
	s, err := Open(Options{WorkspaceRoot: root})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestMigrationsApplyOnce(t *testing.T) {
	root := t.TempDir()
	s := openStore(t, root)
	var v int
	if err := s.readDB.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&v); err != nil {
		t.Fatalf("read schema_version: %v", err)
	}
	if v < 1 {
		t.Fatalf("expected schema_version >= 1, got %d", v)
	}
	// Re-open: must not re-apply or error.
	_ = s.Close()
	s2 := openStore(t, root)
	if _, err := s2.CountDocs(context.Background()); err != nil {
		t.Fatalf("count after reopen: %v", err)
	}
}

func TestUpsertAndSearch(t *testing.T) {
	ctx := context.Background()
	root := newTestWorkspace(t, 3)
	s := openStore(t, root)
	if err := s.Reindex(ctx, fsStore{}, root); err != nil {
		t.Fatalf("reindex: %v", err)
	}
	hits, err := s.Search(ctx, "reticulated", SearchFilters{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 3 {
		t.Fatalf("expected 3 hits, got %d", len(hits))
	}
	// Exact-token search should narrow to one doc.
	hits, err = s.Search(ctx, "token-01", SearchFilters{})
	if err != nil {
		t.Fatalf("search token: %v", err)
	}
	if len(hits) != 1 || !strings.Contains(hits[0].ID, "doc-01") {
		t.Fatalf("expected single hit for token-01, got %+v", hits)
	}
}

func TestDeleteDoc(t *testing.T) {
	ctx := context.Background()
	root := newTestWorkspace(t, 2)
	s := openStore(t, root)
	if err := s.Reindex(ctx, fsStore{}, root); err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if err := s.DeleteDoc(ctx, "doc-00.md"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	n, _ := s.CountDocs(ctx)
	if n != 1 {
		t.Fatalf("expected 1 doc after delete, got %d", n)
	}
	hits, _ := s.Search(ctx, "token-00", SearchFilters{})
	if len(hits) != 0 {
		t.Fatalf("deleted doc should not be searchable, got %d hits", len(hits))
	}
}

func TestProjectFilter(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	s := openStore(t, root)
	mk := func(id, proj, body string) {
		if err := s.UpsertDoc(ctx, &Doc{
			ID: id, Project: proj, Title: id, Type: "how-to",
			Status: "published", ContentHash: "x", ModTime: "t", Size: 1, Body: body,
		}); err != nil {
			t.Fatal(err)
		}
	}
	mk("a.md", "alpha", "shared keyword zebra")
	mk("b.md", "beta", "shared keyword zebra")
	hits, err := s.Search(ctx, "zebra", SearchFilters{Project: "alpha"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].Project != "alpha" {
		t.Fatalf("project filter failed: %+v", hits)
	}
}

// TestDisasterRecovery is the DoD test from the Epic:
// `rm .vedox/index.db && vedox reindex` restores a fully searchable
// workspace with zero data loss.
func TestDisasterRecovery(t *testing.T) {
	ctx := context.Background()
	root := newTestWorkspace(t, 10)

	// Initial index.
	s := openStore(t, root)
	if err := s.Reindex(ctx, fsStore{}, root); err != nil {
		t.Fatalf("initial reindex: %v", err)
	}
	n, _ := s.CountDocs(ctx)
	if n != 10 {
		t.Fatalf("expected 10 docs pre-disaster, got %d", n)
	}
	_ = s.Close()

	// Simulate disaster: wipe the entire .vedox directory (db, WAL,
	// and SHM files all at once).
	if err := os.RemoveAll(filepath.Join(root, ".vedox")); err != nil {
		t.Fatalf("rm .vedox: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, IndexDBRelPath)); !os.IsNotExist(err) {
		t.Fatalf("expected index.db to be gone, err=%v", err)
	}

	// Recovery: re-open (runs migrations fresh) and reindex.
	s2 := openStore(t, root)
	if n, _ := s2.CountDocs(ctx); n != 0 {
		t.Fatalf("expected empty db after rm, got %d rows", n)
	}
	if err := s2.Reindex(ctx, fsStore{}, root); err != nil {
		t.Fatalf("recovery reindex: %v", err)
	}

	// Assert all 10 documents are searchable again.
	if n, _ := s2.CountDocs(ctx); n != 10 {
		t.Fatalf("expected 10 docs post-recovery, got %d", n)
	}
	for i := 0; i < 10; i++ {
		q := fmt.Sprintf("token-%02d", i)
		hits, err := s2.Search(ctx, q, SearchFilters{})
		if err != nil {
			t.Fatalf("search %s: %v", q, err)
		}
		if len(hits) != 1 {
			t.Fatalf("expected 1 hit for %s after recovery, got %d", q, len(hits))
		}
	}
}

// TestConcurrentWrites asserts the writer funnel serialises many
// concurrent submitters without data loss or deadlock.
func TestConcurrentWrites(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	s := openStore(t, root)
	const N = 50
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		go func(i int) {
			errs <- s.UpsertDoc(ctx, &Doc{
				ID: fmt.Sprintf("c-%03d.md", i), Project: "p", Title: fmt.Sprintf("t%d", i),
				Type: "how-to", Status: "draft", ContentHash: "h", ModTime: "t", Size: 1,
				Body: fmt.Sprintf("concurrent body %d", i),
			})
		}(i)
	}
	for i := 0; i < N; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent upsert: %v", err)
		}
	}
	if n, _ := s.CountDocs(ctx); n != N {
		t.Fatalf("expected %d rows, got %d", N, n)
	}
}
