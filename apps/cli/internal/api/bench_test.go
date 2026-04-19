package api

// bench_test.go — hot-path benchmarks for the API handler layer.
//
// Two handlers are measured:
//
//  1. handleHealth — the minimal JSON status endpoint. This is the lower bound
//     for any HTTP handler in the package; regressions here indicate framework
//     overhead (middleware, routing, encoding) rather than business logic.
//
//  2. handleGraph — the doc-reference graph endpoint on an empty project.
//     An empty graph exercises the query path and the non-null array guarantee
//     without the alloc cost of populating Cytoscape node/edge slices.
//
// Both benchmarks use net/http/httptest.ResponseRecorder to avoid network I/O —
// the measurement is pure handler execution time.
//
// Run with:
//
//	go test -bench=. -benchmem -run=^$ ./internal/api/...
import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/docgraph"
)

// benchOpenDB opens a real db.Store in b.TempDir() and registers a Cleanup to
// close it. The store has the full migration set applied (including the graph
// schema), mirroring the production daemon startup sequence.
func benchOpenDB(b *testing.B) *db.Store {
	b.Helper()
	raw := b.TempDir()
	resolved, err := filepath.EvalSymlinks(raw)
	if err != nil {
		b.Fatalf("EvalSymlinks: %v", err)
	}
	s, err := db.Open(db.Options{WorkspaceRoot: resolved})
	if err != nil {
		b.Fatalf("db.Open: %v", err)
	}
	b.Cleanup(func() { _ = s.Close() })
	return s
}

// BenchmarkHandleHealthz measures the handleHealth handler (mounted at
// /api/health). The handler writes {"status":"ok"} — the test verifies no
// error is introduced by the response recorder on each iteration.
func BenchmarkHandleHealthz(b *testing.B) {
	// A zero-value Server is sufficient for handleHealth: it has no dependencies.
	srv := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		srv.handleHealth(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("expected 200, got %d", w.Code)
		}
	}
}

// BenchmarkHandleGraph_Empty measures handleGraph on an empty project — zero
// stored references, so GetAllRefsForPrefix returns immediately after the
// index seek. The result must be {nodes:[], edges:[]} (non-null arrays).
//
// The GraphStore is backed by a real db.Store opened in b.TempDir() so WAL
// mode and FK constraints are active — this is a faithful in-process
// reflection of the production path without network I/O.
func BenchmarkHandleGraph_Empty(b *testing.B) {
	dbStore := benchOpenDB(b)
	gs := docgraph.NewGraphStore(dbStore)
	srv := &Server{
		db:         dbStore,
		graphStore: gs,
	}
	req := httptest.NewRequest(http.MethodGet, "/api/graph?project=bench-empty", nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		srv.handleGraph(w, req)
		if w.Code != http.StatusOK {
			b.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
		}
	}
}
