package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/docgraph"
	"github.com/vedox/vedox/internal/store"
)

// seedDoc inserts a minimal document row into the db so that references
// attached to it resolve, and so loadProjectDocs picks it up as a known node.
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

// TestHandleGraph_MissingProject verifies that omitting ?project= returns
// 400 VDX-400 — the endpoint is per-project only.
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
// injected returns 503 VDX-503 even when ?project= is supplied.
func TestHandleGraph_NilStore(t *testing.T) {
	f := newCoverageServer(t)

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

// TestHandleGraph_UnknownProject verifies that a known-to-be-absent project
// returns 200 with an empty graph (not 404 / 400). This matches how other
// project-scoped endpoints behave.
func TestHandleGraph_UnknownProject(t *testing.T) {
	f := newCoverageServer(t)
	gs := docgraph.NewGraphStore(f.dbStore)

	srv := newGraphServer(t, f, gs, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/graph?project=does-not-exist", nil)
	w := httptest.NewRecorder()
	srv.handleGraph(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var gr docgraph.Graph
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if gr.Nodes == nil || gr.Edges == nil {
		t.Error("nodes/edges must be non-null arrays for an empty graph")
	}
	if len(gr.Nodes) != 0 || len(gr.Edges) != 0 {
		t.Errorf("expected empty graph, got %d nodes %d edges", len(gr.Nodes), len(gr.Edges))
	}
}

// TestHandleGraph_WithRefs verifies the enriched flat schema:
// - nodes carry project/slug/title/type/status/degree_in/out/modified
// - edges carry kind (frontend-canonical enum) and broken flag
// - envelope carries truncated/total_nodes/total_edges
func TestHandleGraph_WithRefs(t *testing.T) {
	f := newCoverageServer(t)
	gs := docgraph.NewGraphStore(f.dbStore)
	ctx := context.Background()

	// Seed two docs in the same project; c.md is intentionally NOT seeded so
	// the b.md → c.md edge resolves to a broken synthesized node.
	seedDoc(t, f.dbStore, "myproject/a.md", "myproject")
	seedDoc(t, f.dbStore, "myproject/b.md", "myproject")

	if err := gs.SaveRefs(ctx, "myproject/a.md", []docgraph.DocRef{{
		SourcePath: "myproject/a.md",
		TargetPath: "b.md", // relative path from myproject/ → resolves to myproject/b.md
		LinkType:   docgraph.LinkTypeMD,
		LineNum:    3,
	}}); err != nil {
		t.Fatalf("SaveRefs a.md: %v", err)
	}
	if err := gs.SaveRefs(ctx, "myproject/b.md", []docgraph.DocRef{{
		SourcePath: "myproject/b.md",
		TargetPath: "c.md", // relative path → would be myproject/c.md; not seeded → broken
		LinkType:   docgraph.LinkTypeWikilink,
		LineNum:    7,
	}}); err != nil {
		t.Fatalf("SaveRefs b.md: %v", err)
	}

	srv := newGraphServer(t, f, gs, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/graph?project=myproject", nil)
	w := httptest.NewRecorder()
	srv.handleGraph(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var gr docgraph.Graph
	if err := json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Expect 3 nodes: a.md, b.md (both seeded), plus synthesised c.md (broken target).
	if len(gr.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d: %+v", len(gr.Nodes), gr.Nodes)
	}
	if len(gr.Edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(gr.Edges))
	}
	if gr.TotalNodes != 3 || gr.TotalEdges != 2 {
		t.Errorf("totals: nodes=%d edges=%d want 3/2", gr.TotalNodes, gr.TotalEdges)
	}
	if gr.Truncated {
		t.Error("unexpected truncation on a 3-node graph")
	}

	// Node lookups by id.
	nodes := map[string]docgraph.GraphNode{}
	for _, n := range gr.Nodes {
		nodes[n.ID] = n
	}

	a, ok := nodes["myproject/a.md"]
	if !ok {
		t.Fatal("missing node myproject/a.md")
	}
	// Slug is derived from id by stripping the project prefix, not read from
	// the DB slug column. id="myproject/a.md", project="myproject" → "a.md".
	if a.Project != "myproject" || a.Slug != "a.md" || a.Type != "how-to" ||
		a.Status != "published" {
		t.Errorf("a.md fields: %+v", a)
	}
	if a.DegreeOut != 1 {
		t.Errorf("a.md degree_out = %d, want 1", a.DegreeOut)
	}

	b := nodes["myproject/b.md"]
	if b.DegreeIn != 1 {
		t.Errorf("b.md degree_in = %d, want 1 (from a.md)", b.DegreeIn)
	}

	// Broken synthesized node for c.md.
	c, ok := nodes["myproject/c.md"]
	if !ok {
		t.Fatalf("expected synthesized missing node myproject/c.md; got %v", nodes)
	}
	if c.Type != "missing" || c.Status != "broken" {
		t.Errorf("missing node type=%q status=%q, want missing/broken", c.Type, c.Status)
	}

	// Edge shape: frontend-canonical kind enum + broken flag.
	edges := map[string]docgraph.GraphEdge{}
	for _, e := range gr.Edges {
		edges[e.Source+"->"+e.Target] = e
	}
	ab := edges["myproject/a.md->myproject/b.md"]
	if ab.Kind != "mdlink" {
		t.Errorf("a->b kind = %q, want mdlink", ab.Kind)
	}
	if ab.Broken {
		t.Error("a->b should not be broken — b.md is seeded")
	}
	bc := edges["myproject/b.md->myproject/c.md"]
	if bc.Kind != "wikilink" {
		t.Errorf("b->c kind = %q, want wikilink", bc.Kind)
	}
	if !bc.Broken {
		t.Error("b->c should be broken — c.md is not seeded")
	}
}

// TestHandleGraph_CrossProjectIsolation verifies that refs from other
// projects are not included in the response.
func TestHandleGraph_CrossProjectIsolation(t *testing.T) {
	f := newCoverageServer(t)
	gs := docgraph.NewGraphStore(f.dbStore)
	ctx := context.Background()

	seedDoc(t, f.dbStore, "projectA/doc.md", "projectA")
	seedDoc(t, f.dbStore, "projectA/other.md", "projectA")
	seedDoc(t, f.dbStore, "projectB/doc.md", "projectB")
	seedDoc(t, f.dbStore, "projectB/other.md", "projectB")

	if err := gs.SaveRefs(ctx, "projectA/doc.md", []docgraph.DocRef{{
		SourcePath: "projectA/doc.md",
		TargetPath: "other.md",
		LinkType:   docgraph.LinkTypeMD,
	}}); err != nil {
		t.Fatalf("SaveRefs projectA: %v", err)
	}
	if err := gs.SaveRefs(ctx, "projectB/doc.md", []docgraph.DocRef{{
		SourcePath: "projectB/doc.md",
		TargetPath: "other.md",
		LinkType:   docgraph.LinkTypeMD,
	}}); err != nil {
		t.Fatalf("SaveRefs projectB: %v", err)
	}

	srv := newGraphServer(t, f, gs, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/graph?project=projectA", nil)
	w := httptest.NewRecorder()
	srv.handleGraph(w, req)

	var gr docgraph.Graph
	if err := json.NewDecoder(w.Result().Body).Decode(&gr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, n := range gr.Nodes {
		if n.Project == "projectB" {
			t.Errorf("projectB node leaked into projectA graph: %+v", n)
		}
	}
	if len(gr.Edges) != 1 {
		t.Errorf("expected 1 edge for projectA, got %d", len(gr.Edges))
	}
}

// TestHandleGraph_VedoxSchemeExcluded verifies that vedox-scheme edges
// (source-code cross-links) are intentionally filtered out of the graph
// in v1. They pollute the doc-type chip list and the user's primary
// workflow is doc-to-doc navigation.
func TestHandleGraph_VedoxSchemeExcluded(t *testing.T) {
	f := newCoverageServer(t)
	gs := docgraph.NewGraphStore(f.dbStore)
	ctx := context.Background()

	seedDoc(t, f.dbStore, "p/runbook.md", "p")

	if err := gs.SaveRefs(ctx, "p/runbook.md", []docgraph.DocRef{{
		SourcePath: "p/runbook.md",
		TargetPath: "vedox://file/apps/cli/main.go",
		LinkType:   docgraph.LinkTypeVedoxScheme,
		LineNum:    10,
	}}); err != nil {
		t.Fatalf("SaveRefs: %v", err)
	}

	srv := newGraphServer(t, f, gs, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/graph?project=p", nil)
	w := httptest.NewRecorder()
	srv.handleGraph(w, req)

	var gr docgraph.Graph
	if err := json.NewDecoder(w.Result().Body).Decode(&gr); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(gr.Edges) != 0 {
		t.Errorf("expected 0 edges (vedox-scheme excluded), got %d", len(gr.Edges))
	}
}

// newGraphServer builds a bare *Server with a GraphStore injected, suitable
// for direct handler calls via httptest.ResponseRecorder.
func newGraphServer(t *testing.T, f *coverageFixture, gs *docgraph.GraphStore, reg *store.ProjectRegistry) *Server {
	t.Helper()
	return &Server{
		db:         f.dbStore,
		graphStore: gs,
		registry:   reg,
	}
}
