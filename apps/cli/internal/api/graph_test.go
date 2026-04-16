package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/docgraph"
)

// seedDoc inserts a minimal document row into the db so that the FK constraint
// on doc_references.source_doc_id is satisfied when SaveRefs is called.
func seedDoc(t *testing.T, dbStore *db.Store, id, project string) {
	t.Helper()
	err := dbStore.UpsertDoc(context.Background(), &db.Doc{
		ID:      id,
		Project: project,
		Slug:    id,
		Title:   id,
		Status:  "published",
		Type:    "how-to",
	})
	if err != nil {
		t.Fatalf("seedDoc(%q): %v", id, err)
	}
}

// fakeGraphSubmitter is a minimal docgraph.Submitter backed by an in-memory
// slice of refs inserted via SaveRefs. It is used to construct a real
// *docgraph.GraphStore without opening a SQLite file, so tests remain fast
// and hermetic.
//
// It satisfies the docgraph.Submitter interface by delegating SubmitWrite to
// the underlying SQLite-backed store opened in a temp dir.
//
// Because we need a real *docgraph.GraphStore (not an interface mock) to call
// SetGraphStore, we open a real db.Store pointed at a t.TempDir() and wire
// it through docgraph.NewGraphStore. The graph tables are created by the
// migration that db.Open runs automatically.

// TestHandleGraph_MissingProject verifies that omitting the ?project= query
// parameter returns 400 VDX-400.
func TestHandleGraph_MissingProject(t *testing.T) {
	f := newCoverageServer(t)

	resp := f.get(t, "/api/graph")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != "VDX-400" {
		t.Errorf("expected code VDX-400, got %q", body["code"])
	}
}

// TestHandleGraph_NilStore verifies that a server without a GraphStore
// injected returns 503 VDX-503.
func TestHandleGraph_NilStore(t *testing.T) {
	f := newCoverageServer(t)
	// newCoverageServer does not inject a GraphStore, so s.graphStore is nil.

	resp := f.get(t, "/api/graph?project=myproject")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["code"] != "VDX-503" {
		t.Errorf("expected code VDX-503, got %q", body["code"])
	}
}

// TestHandleGraph_EmptyProject verifies that a valid project with no indexed
// docs returns 200 with empty nodes and edges arrays (not null).
func TestHandleGraph_EmptyProject(t *testing.T) {
	f := newCoverageServer(t)

	// Build a real GraphStore backed by the already-open db.Store from the fixture.
	gs := docgraph.NewGraphStore(f.dbStore)

	// Wire the GraphStore into the server. We reach into the handler via a
	// fresh httptest.Server that wraps a Server with the store set.
	srv := newGraphServer(t, f, gs)

	req := httptest.NewRequest(http.MethodGet, "/api/graph?project=empty-project", nil)
	w := httptest.NewRecorder()
	srv.handleGraph(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var gr graphResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if gr.Nodes == nil {
		t.Error("nodes must be a non-null array")
	}
	if gr.Edges == nil {
		t.Error("edges must be a non-null array")
	}
	if len(gr.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(gr.Nodes))
	}
	if len(gr.Edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(gr.Edges))
	}
}

// TestHandleGraph_WithRefs verifies that a project with saved references
// returns the correct nodes and edges in Cytoscape shape.
func TestHandleGraph_WithRefs(t *testing.T) {
	f := newCoverageServer(t)
	gs := docgraph.NewGraphStore(f.dbStore)

	ctx := context.Background()

	// Seed document rows before inserting references (FK constraint).
	seedDoc(t, f.dbStore, "myproject/a.md", "myproject")
	seedDoc(t, f.dbStore, "myproject/b.md", "myproject")

	// Seed two docs with outgoing references.
	if err := gs.SaveRefs(ctx, "myproject/a.md", []docgraph.DocRef{
		{
			SourcePath: "myproject/a.md",
			TargetPath: "myproject/b.md",
			LinkType:   docgraph.LinkTypeMD,
			LineNum:    3,
			AnchorText: "B doc",
		},
	}); err != nil {
		t.Fatalf("SaveRefs a.md: %v", err)
	}
	if err := gs.SaveRefs(ctx, "myproject/b.md", []docgraph.DocRef{
		{
			SourcePath: "myproject/b.md",
			TargetPath: "myproject/c.md",
			LinkType:   docgraph.LinkTypeWikilink,
			LineNum:    7,
			AnchorText: "C doc",
		},
	}); err != nil {
		t.Fatalf("SaveRefs b.md: %v", err)
	}

	srv := newGraphServer(t, f, gs)

	req := httptest.NewRequest(http.MethodGet, "/api/graph?project=myproject", nil)
	w := httptest.NewRecorder()
	srv.handleGraph(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var gr graphResponse
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Expect 3 nodes: a.md, b.md (source+target), c.md (stub target).
	if len(gr.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(gr.Nodes))
	}
	// Expect 2 edges.
	if len(gr.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(gr.Edges))
	}

	// Verify edge shape: source, target, linkType, and synthetic ID.
	edgesByID := make(map[string]graphEdgeData)
	for _, e := range gr.Edges {
		edgesByID[e.Data.ID] = e.Data
	}

	e1, ok := edgesByID["myproject/a.md::myproject/b.md"]
	if !ok {
		t.Error("missing edge a.md -> b.md")
	} else {
		if e1.Source != "myproject/a.md" {
			t.Errorf("edge source: got %q", e1.Source)
		}
		if e1.Target != "myproject/b.md" {
			t.Errorf("edge target: got %q", e1.Target)
		}
		if e1.LinkType != "md-link" {
			t.Errorf("edge linkType: got %q", e1.LinkType)
		}
	}

	// Verify node labels are derived from file names (no extension).
	nodesByID := make(map[string]graphNodeData)
	for _, n := range gr.Nodes {
		nodesByID[n.Data.ID] = n.Data
	}
	if nd, ok := nodesByID["myproject/a.md"]; ok {
		if nd.Label != "a" {
			t.Errorf("node label for a.md: expected %q, got %q", "a", nd.Label)
		}
	} else {
		t.Error("node myproject/a.md missing")
	}
}

// TestHandleGraph_CrossProjectIsolation verifies that refs from a different
// project are not included in the response.
func TestHandleGraph_CrossProjectIsolation(t *testing.T) {
	f := newCoverageServer(t)
	gs := docgraph.NewGraphStore(f.dbStore)
	ctx := context.Background()

	// Seed document rows before inserting references (FK constraint).
	seedDoc(t, f.dbStore, "projectA/doc.md", "projectA")
	seedDoc(t, f.dbStore, "projectB/doc.md", "projectB")

	// Seed refs for two different projects.
	if err := gs.SaveRefs(ctx, "projectA/doc.md", []docgraph.DocRef{
		{
			SourcePath: "projectA/doc.md",
			TargetPath: "projectA/other.md",
			LinkType:   docgraph.LinkTypeMD,
		},
	}); err != nil {
		t.Fatalf("SaveRefs projectA: %v", err)
	}
	if err := gs.SaveRefs(ctx, "projectB/doc.md", []docgraph.DocRef{
		{
			SourcePath: "projectB/doc.md",
			TargetPath: "projectB/other.md",
			LinkType:   docgraph.LinkTypeMD,
		},
	}); err != nil {
		t.Fatalf("SaveRefs projectB: %v", err)
	}

	srv := newGraphServer(t, f, gs)

	req := httptest.NewRequest(http.MethodGet, "/api/graph?project=projectA", nil)
	w := httptest.NewRecorder()
	srv.handleGraph(w, req)

	var gr graphResponse
	if err := json.NewDecoder(w.Result().Body).Decode(&gr); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, n := range gr.Nodes {
		if len(n.Data.ID) >= 8 && n.Data.ID[:8] == "projectB" {
			t.Errorf("projectB node leaked into projectA graph: %q", n.Data.ID)
		}
	}
	if len(gr.Edges) != 1 {
		t.Errorf("expected 1 edge for projectA, got %d", len(gr.Edges))
	}
}

// newGraphServer builds a bare *Server (no HTTP listener) with a GraphStore
// injected, suitable for direct handler calls via httptest.ResponseRecorder.
// It reuses the db.Store from the coverage fixture so migrations are already run.
func newGraphServer(t *testing.T, f *coverageFixture, gs *docgraph.GraphStore) *Server {
	t.Helper()
	s := &Server{
		db:         f.dbStore,
		graphStore: gs,
	}
	return s
}

// TestLabelFromID verifies the label derivation helper in isolation.
func TestLabelFromID(t *testing.T) {
	cases := []struct {
		id   string
		want string
	}{
		{"docs/adr/001-init.md", "001-init"},
		{"my-doc.md", "my-doc"},
		{"readme", "readme"},
		{"a/b/c/deep.md", "deep"},
		{"noext", "noext"},
	}
	for _, tc := range cases {
		got := labelFromID(tc.id)
		if got != tc.want {
			t.Errorf("labelFromID(%q) = %q, want %q", tc.id, got, tc.want)
		}
	}
}
