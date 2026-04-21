package docgraph_test

import (
	"context"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/docgraph"
)

// seedDocIn inserts a documents row with the given project/slug so
// GetGraphForProject picks it up as a known node.
func seedDocIn(t *testing.T, s *db.Store, id, project, slug, title, typ, status string) {
	t.Helper()
	err := s.UpsertDoc(context.Background(), &db.Doc{
		ID:          id,
		Project:     project,
		Slug:        slug,
		Title:       title,
		Type:        typ,
		Status:      status,
		ContentHash: "h",
		ModTime:     "2026-01-01T00:00:00Z",
		Size:        1,
	})
	if err != nil {
		t.Fatalf("UpsertDoc(%q): %v", id, err)
	}
}

// TestGetGraphForProject_EmptyProject: unknown or empty project returns a
// zero-value Graph (non-nil slices, zeros across the board).
func TestGetGraphForProject_EmptyProject(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	g, err := f.graph.GetGraphForProject(f.ctx, "does-not-exist")
	if err != nil {
		t.Fatalf("GetGraphForProject: %v", err)
	}
	if len(g.Nodes) != 0 || len(g.Edges) != 0 {
		t.Errorf("expected empty graph, got %d nodes %d edges", len(g.Nodes), len(g.Edges))
	}
	if g.Truncated || g.TotalNodes != 0 || g.TotalEdges != 0 {
		t.Errorf("expected zero envelope values, got truncated=%v total=%d/%d",
			g.Truncated, g.TotalNodes, g.TotalEdges)
	}
}

// TestGetGraphForProject_RejectsEmptyProject guards against accidental
// cross-project leakage — passing "" must error, not fall through to
// "match everything".
func TestGetGraphForProject_RejectsEmptyProject(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	if _, err := f.graph.GetGraphForProject(f.ctx, ""); err == nil {
		t.Fatal("expected error for empty project, got nil")
	}
}

// TestGetGraphForProject_SingleDocNoRefs: one doc, no refs → one node, no
// edges, degrees zero.
func TestGetGraphForProject_SingleDocNoRefs(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	seedDocIn(t, f.store, "p/a.md", "p", "a", "Alpha", "how-to", "published")

	g, err := f.graph.GetGraphForProject(f.ctx, "p")
	if err != nil {
		t.Fatalf("GetGraphForProject: %v", err)
	}
	if len(g.Nodes) != 1 || len(g.Edges) != 0 {
		t.Fatalf("expected 1 node 0 edges, got %d/%d", len(g.Nodes), len(g.Edges))
	}
	n := g.Nodes[0]
	// Slug is now derived from id by stripping the project prefix — no DB
	// column required. For id="p/a.md" and project="p" the slug is "a.md".
	if n.ID != "p/a.md" || n.Project != "p" || n.Slug != "a.md" || n.Title != "Alpha" ||
		n.Type != "how-to" || n.Status != "published" ||
		n.DegreeIn != 0 || n.DegreeOut != 0 {
		t.Errorf("unexpected node: %+v", n)
	}
}

// TestGetGraphForProject_SlugDerivedFromID verifies that the Slug field on
// each GraphNode is derived from the doc's id by stripping the project prefix,
// without reading the slug column from the database. This is the fix for
// VDX-500: workspaces where migration 003 silently failed (leaving the slug
// column absent) now return a correct slug rather than a query-time error.
func TestGetGraphForProject_SlugDerivedFromID(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	// Seed docs at various nesting depths to exercise the prefix-strip logic.
	// The slug values passed to seedDocIn are arbitrary (the schema requires
	// them to be unique per project) — loadProjectDocs ignores the stored slug
	// column entirely and derives Slug from the id. What we assert is the
	// derived value that comes back via GetGraphForProject.
	seedDocIn(t, f.store, "proj/top.md", "proj", "slug-top", "Top", "how-to", "published")
	seedDocIn(t, f.store, "proj/docs/nested.md", "proj", "slug-nested", "Nested", "adr", "published")
	seedDocIn(t, f.store, "proj/docs/adr/deep.md", "proj", "slug-deep", "Deep", "adr", "published")

	g, err := f.graph.GetGraphForProject(f.ctx, "proj")
	if err != nil {
		t.Fatalf("GetGraphForProject: %v", err)
	}
	if len(g.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(g.Nodes))
	}

	slugByID := make(map[string]string, len(g.Nodes))
	for _, n := range g.Nodes {
		slugByID[n.ID] = n.Slug
	}

	cases := []struct {
		id   string
		want string
	}{
		{"proj/top.md", "top.md"},
		{"proj/docs/nested.md", "docs/nested.md"},
		{"proj/docs/adr/deep.md", "docs/adr/deep.md"},
	}
	for _, c := range cases {
		if got := slugByID[c.id]; got != c.want {
			t.Errorf("slug for %q: got %q, want %q", c.id, got, c.want)
		}
	}
}

// TestGetGraphForProject_ResolvesPath: relative md-link target resolves
// against the source doc's directory to a seeded doc id.
func TestGetGraphForProject_ResolvesPath(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	seedDocIn(t, f.store, "p/docs/a.md", "p", "a", "A", "adr", "published")
	seedDocIn(t, f.store, "p/docs/b.md", "p", "b", "B", "adr", "published")

	if err := f.graph.SaveRefs(f.ctx, "p/docs/a.md", []docgraph.DocRef{{
		SourcePath: "p/docs/a.md",
		TargetPath: "b.md", // resolves to p/docs/b.md
		LinkType:   docgraph.LinkTypeMD,
	}}); err != nil {
		t.Fatalf("SaveRefs: %v", err)
	}

	g, err := f.graph.GetGraphForProject(f.ctx, "p")
	if err != nil {
		t.Fatalf("GetGraphForProject: %v", err)
	}
	if len(g.Nodes) != 2 || len(g.Edges) != 1 {
		t.Fatalf("nodes=%d edges=%d want 2/1", len(g.Nodes), len(g.Edges))
	}
	e := g.Edges[0]
	if e.Source != "p/docs/a.md" || e.Target != "p/docs/b.md" {
		t.Errorf("edge endpoints: %+v", e)
	}
	if e.Kind != "mdlink" || e.Broken {
		t.Errorf("edge kind=%q broken=%v, want mdlink/false", e.Kind, e.Broken)
	}
}

// TestGetGraphForProject_ResolvesSlug: wikilink target resolves by file-basename
// lookup within the project. The bySlug map is keyed on path.Base(id without
// extension), so a wikilink "hmac-auth" resolves when a doc's id ends in
// "/hmac-auth.md". This test uses ids whose basenames intentionally match the
// wikilink targets so resolution succeeds without a DB slug column.
func TestGetGraphForProject_ResolvesSlug(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	// id basename "hmac-auth" matches wikilink target "hmac-auth".
	seedDocIn(t, f.store, "p/hmac-auth.md", "p", "hmac-auth", "HMAC Auth", "adr", "published")
	seedDocIn(t, f.store, "p/overview.md", "p", "overview", "Overview", "how-to", "published")

	if err := f.graph.SaveRefs(f.ctx, "p/overview.md", []docgraph.DocRef{{
		SourcePath: "p/overview.md",
		TargetPath: "hmac-auth",
		LinkType:   docgraph.LinkTypeWikilink,
	}}); err != nil {
		t.Fatalf("SaveRefs: %v", err)
	}

	g, err := f.graph.GetGraphForProject(f.ctx, "p")
	if err != nil {
		t.Fatalf("GetGraphForProject: %v", err)
	}
	if len(g.Edges) != 1 {
		t.Fatalf("edges=%d want 1", len(g.Edges))
	}
	e := g.Edges[0]
	if e.Target != "p/hmac-auth.md" || e.Kind != "wikilink" || e.Broken {
		t.Errorf("edge: %+v", e)
	}
}

// TestGetGraphForProject_BrokenTarget: unresolvable target emits broken
// edge + synthesised "missing" node so the UI can render the dangling end.
func TestGetGraphForProject_BrokenTarget(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	seedDocIn(t, f.store, "p/a.md", "p", "a", "A", "adr", "published")
	if err := f.graph.SaveRefs(f.ctx, "p/a.md", []docgraph.DocRef{{
		SourcePath: "p/a.md",
		TargetPath: "ghost.md",
		LinkType:   docgraph.LinkTypeMD,
	}}); err != nil {
		t.Fatalf("SaveRefs: %v", err)
	}

	g, err := f.graph.GetGraphForProject(f.ctx, "p")
	if err != nil {
		t.Fatalf("GetGraphForProject: %v", err)
	}

	var missing *docgraph.GraphNode
	for i := range g.Nodes {
		if g.Nodes[i].Type == "missing" {
			missing = &g.Nodes[i]
		}
	}
	if missing == nil {
		t.Fatalf("expected a missing synthesised node, got %+v", g.Nodes)
	}
	if missing.Status != "broken" {
		t.Errorf("missing node status = %q, want broken", missing.Status)
	}
	if len(g.Edges) != 1 || !g.Edges[0].Broken {
		t.Errorf("expected 1 broken edge, got %+v", g.Edges)
	}
}

// TestGetGraphForProject_VedoxSchemeExcluded: vedox-scheme refs are
// intentionally filtered out in v1.
func TestGetGraphForProject_VedoxSchemeExcluded(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	seedDocIn(t, f.store, "p/a.md", "p", "a", "A", "runbook", "published")
	if err := f.graph.SaveRefs(f.ctx, "p/a.md", []docgraph.DocRef{{
		SourcePath: "p/a.md",
		TargetPath: "vedox://file/apps/cli/main.go",
		LinkType:   docgraph.LinkTypeVedoxScheme,
	}}); err != nil {
		t.Fatalf("SaveRefs: %v", err)
	}

	g, err := f.graph.GetGraphForProject(f.ctx, "p")
	if err != nil {
		t.Fatalf("GetGraphForProject: %v", err)
	}
	if len(g.Edges) != 0 {
		t.Errorf("expected vedox-scheme edges excluded, got %d", len(g.Edges))
	}
	// A single-doc node is still fine.
	if len(g.Nodes) != 1 {
		t.Errorf("expected 1 node (source only), got %d", len(g.Nodes))
	}
}

// TestGetGraphForProject_CrossProjectIsolation: refs whose source lives in
// another project must never surface here.
func TestGetGraphForProject_CrossProjectIsolation(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	seedDocIn(t, f.store, "a/x.md", "a", "x", "X", "adr", "published")
	seedDocIn(t, f.store, "b/y.md", "b", "y", "Y", "adr", "published")

	if err := f.graph.SaveRefs(f.ctx, "a/x.md", []docgraph.DocRef{{
		SourcePath: "a/x.md",
		TargetPath: "a/other.md",
		LinkType:   docgraph.LinkTypeMD,
	}}); err != nil {
		t.Fatalf("SaveRefs a: %v", err)
	}
	if err := f.graph.SaveRefs(f.ctx, "b/y.md", []docgraph.DocRef{{
		SourcePath: "b/y.md",
		TargetPath: "b/other.md",
		LinkType:   docgraph.LinkTypeMD,
	}}); err != nil {
		t.Fatalf("SaveRefs b: %v", err)
	}

	g, err := f.graph.GetGraphForProject(f.ctx, "a")
	if err != nil {
		t.Fatalf("GetGraphForProject: %v", err)
	}
	for _, n := range g.Nodes {
		if n.Project != "" && n.Project != "a" {
			t.Errorf("project %q leaked: %+v", n.Project, n)
		}
		if strings.HasPrefix(n.ID, "b/") {
			t.Errorf("node from project b leaked: %q", n.ID)
		}
	}
}

// TestGetGraphForProject_DegreesFromEdges: degree_in/out are computed from
// the resolved edge set (not from doc_reference_counts), so they always
// match the rendered edges.
func TestGetGraphForProject_DegreesFromEdges(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	seedDocIn(t, f.store, "p/a.md", "p", "a", "A", "adr", "published")
	seedDocIn(t, f.store, "p/b.md", "p", "b", "B", "adr", "published")
	seedDocIn(t, f.store, "p/c.md", "p", "c", "C", "adr", "published")

	// a → b, a → c, b → c
	if err := f.graph.SaveRefs(f.ctx, "p/a.md", []docgraph.DocRef{
		{SourcePath: "p/a.md", TargetPath: "b.md", LinkType: docgraph.LinkTypeMD},
		{SourcePath: "p/a.md", TargetPath: "c.md", LinkType: docgraph.LinkTypeMD},
	}); err != nil {
		t.Fatalf("SaveRefs a: %v", err)
	}
	if err := f.graph.SaveRefs(f.ctx, "p/b.md", []docgraph.DocRef{
		{SourcePath: "p/b.md", TargetPath: "c.md", LinkType: docgraph.LinkTypeMD},
	}); err != nil {
		t.Fatalf("SaveRefs b: %v", err)
	}

	g, err := f.graph.GetGraphForProject(f.ctx, "p")
	if err != nil {
		t.Fatalf("GetGraphForProject: %v", err)
	}

	byID := map[string]docgraph.GraphNode{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	if byID["p/a.md"].DegreeOut != 2 || byID["p/a.md"].DegreeIn != 0 {
		t.Errorf("a: out=%d in=%d want 2/0", byID["p/a.md"].DegreeOut, byID["p/a.md"].DegreeIn)
	}
	if byID["p/b.md"].DegreeOut != 1 || byID["p/b.md"].DegreeIn != 1 {
		t.Errorf("b: out=%d in=%d want 1/1", byID["p/b.md"].DegreeOut, byID["p/b.md"].DegreeIn)
	}
	if byID["p/c.md"].DegreeOut != 0 || byID["p/c.md"].DegreeIn != 2 {
		t.Errorf("c: out=%d in=%d want 0/2", byID["p/c.md"].DegreeOut, byID["p/c.md"].DegreeIn)
	}
}
