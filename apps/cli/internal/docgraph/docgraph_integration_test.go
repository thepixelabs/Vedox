package docgraph_test

// Integration tests for the doc graph extractor (WS-G).
//
// These tests exercise Extract + GraphStore end-to-end: real SQLite via db.Open,
// real file content, real link resolution. Nothing is mocked.
//
// Test inventory (9 tests):
//   TestDocGraph_OutgoingAndBacklinks        — A→B md-link, B→C wikilink; verify GetOutgoing + GetBacklinks
//   TestDocGraph_FrontmatterRelated          — C has related:[A]; verify frontmatter edge
//   TestDocGraph_ThreeNodeGraph              — combined A→B→C with C→A; full round-trip
//   TestDocGraph_BrokenLinkDetection         — A links to nonexistent missing.md; GetBrokenLinks finds it
//   TestDocGraph_IncrementalUpdate           — remove a link from A, re-save, old edge gone
//   TestDocGraph_VedoxSchemeExtraction       — vedox://file/main.tf#L10-L25 extracted with anchor
//   TestDocGraph_SaveRefs_EmptyDocID         — SaveRefs("", ...) returns error
//   TestDocGraph_GetOutgoing_EmptyDocID      — GetOutgoing("") returns error
//   TestDocGraph_GetBacklinks_EmptyTarget    — GetBacklinks("") returns error

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/docgraph"
)

// sqliteInitOnce serializes the first db.Open() call across parallel tests to
// avoid a data race in modernc.org/sqlite's Xsqlite3_initialize() which runs
// before SQLite's internal mutex is established.
var sqliteInitOnce sync.Once

// ── fixture ───────────────────────────────────────────────────────────────────

// graphFixture holds an open db.Store and a GraphStore backed by it.
// It is constructed per-test so each test gets its own isolated SQLite file.
type graphFixture struct {
	store  *db.Store
	graph  *docgraph.GraphStore
	ctx    context.Context
}

func newGraphFixture(t *testing.T) *graphFixture {
	t.Helper()

	raw := t.TempDir()
	resolved, err := filepath.EvalSymlinks(raw)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	// Serialize the first db.Open to let SQLite's global init complete
	// before any parallel test opens a second connection.
	sqliteInitOnce.Do(func() {
		probe, probeErr := db.Open(db.Options{WorkspaceRoot: resolved})
		if probeErr != nil {
			t.Fatalf("sqlite init probe: %v", probeErr)
		}
		_ = probe.Close()
	})

	s, err := db.Open(db.Options{WorkspaceRoot: resolved})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	g := docgraph.NewGraphStore(s)
	return &graphFixture{store: s, graph: g, ctx: context.Background()}
}

// seedDoc inserts a minimal documents row so foreign-key constraints and
// broken-link queries work correctly.
func (f *graphFixture) seedDoc(t *testing.T, docID string) {
	t.Helper()
	err := f.store.UpsertDoc(f.ctx, &db.Doc{
		ID:          docID,
		Project:     "test",
		Title:       docID,
		ContentHash: "abc123",
		ModTime:     "2026-01-01T00:00:00Z",
		Size:        1,
	})
	if err != nil {
		t.Fatalf("UpsertDoc(%q): %v", docID, err)
	}
}

// extractAndSave is a shorthand for Extract → SaveRefs.
func (f *graphFixture) extractAndSave(t *testing.T, sourcePath string, content []byte) {
	t.Helper()
	refs := docgraph.Extract(sourcePath, content)
	if err := f.graph.SaveRefs(f.ctx, sourcePath, refs); err != nil {
		t.Fatalf("SaveRefs(%q): %v", sourcePath, err)
	}
}

// refKey uniquely identifies a DocRef for set membership checks.
type refKey struct {
	source   string
	target   string
	linkType docgraph.LinkType
}

func refSet(refs []docgraph.DocRef) map[refKey]docgraph.DocRef {
	m := make(map[refKey]docgraph.DocRef, len(refs))
	for _, r := range refs {
		m[refKey{r.SourcePath, r.TargetPath, r.LinkType}] = r
	}
	return m
}

// ── Tests ─────────────────────────────────────────────────────────────────────

// TestDocGraph_OutgoingAndBacklinks creates A→B (md-link) and B→C (wikilink),
// saves both, and verifies GetOutgoing and GetBacklinks are consistent.
func TestDocGraph_OutgoingAndBacklinks(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	const (
		docA = "docs/a.md"
		docB = "docs/b.md"
		docC = "docs/c.md"
	)
	f.seedDoc(t, docA)
	f.seedDoc(t, docB)
	f.seedDoc(t, docC)

	contentA := []byte("# A\n\nSee [B](b.md) for details.\n")
	contentB := []byte("# B\n\nRelated: [[c]].\n")

	f.extractAndSave(t, docA, contentA)
	f.extractAndSave(t, docB, contentB)

	// --- GetOutgoing(A) must contain the A→B md-link ---
	outA, err := f.graph.GetOutgoing(f.ctx, docA)
	if err != nil {
		t.Fatalf("GetOutgoing(%q): %v", docA, err)
	}
	setA := refSet(outA)
	if _, ok := setA[refKey{docA, "docs/b.md", docgraph.LinkTypeMD}]; !ok {
		t.Errorf("GetOutgoing(%q): missing md-link to %q; got %+v", docA, "docs/b.md", outA)
	}

	// --- GetOutgoing(B) must contain the B→C wikilink ---
	outB, err := f.graph.GetOutgoing(f.ctx, docB)
	if err != nil {
		t.Fatalf("GetOutgoing(%q): %v", docB, err)
	}
	foundWikilink := false
	for _, r := range outB {
		if r.LinkType == docgraph.LinkTypeWikilink {
			foundWikilink = true
			break
		}
	}
	if !foundWikilink {
		t.Errorf("GetOutgoing(%q): expected a wikilink ref, got %+v", docB, outB)
	}

	// --- GetBacklinks(B) must include the A→B edge ---
	backB, err := f.graph.GetBacklinks(f.ctx, "docs/b.md")
	if err != nil {
		t.Fatalf("GetBacklinks(%q): %v", "docs/b.md", err)
	}
	foundBacklink := false
	for _, r := range backB {
		if r.SourcePath == docA {
			foundBacklink = true
			break
		}
	}
	if !foundBacklink {
		t.Errorf("GetBacklinks(%q): expected source %q, got %+v", "docs/b.md", docA, backB)
	}
}

// TestDocGraph_FrontmatterRelated verifies that a frontmatter `related:` field
// produces a LinkTypeFrontmatter edge.
func TestDocGraph_FrontmatterRelated(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	const (
		docA = "docs/a.md"
		docC = "docs/c.md"
	)
	f.seedDoc(t, docA)
	f.seedDoc(t, docC)

	// C declares A as related via frontmatter.
	contentC := []byte("---\nrelated:\n  - docs/a.md\n---\n\n# C\n\nBody.\n")
	f.extractAndSave(t, docC, contentC)

	outC, err := f.graph.GetOutgoing(f.ctx, docC)
	if err != nil {
		t.Fatalf("GetOutgoing(%q): %v", docC, err)
	}

	found := false
	for _, r := range outC {
		if r.LinkType == docgraph.LinkTypeFrontmatter && r.TargetPath == "docs/a.md" {
			found = true
			if r.AnchorText != "related" {
				t.Errorf("frontmatter ref: AnchorText = %q, want %q", r.AnchorText, "related")
			}
			break
		}
	}
	if !found {
		t.Errorf("GetOutgoing(%q): expected frontmatter edge to %q, got %+v", docC, "docs/a.md", outC)
	}
}

// TestDocGraph_ThreeNodeGraph builds a complete A→B→C ring with C also linking
// back to A via frontmatter, saves all three, and verifies the full edge set.
func TestDocGraph_ThreeNodeGraph(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	const (
		docA = "docs/a.md"
		docB = "docs/b.md"
		docC = "docs/c.md"
	)
	f.seedDoc(t, docA)
	f.seedDoc(t, docB)
	f.seedDoc(t, docC)

	f.extractAndSave(t, docA, []byte("# A\n\nSee [B](b.md).\n"))
	f.extractAndSave(t, docB, []byte("# B\n\nSee [[c]].\n"))
	f.extractAndSave(t, docC, []byte("---\nrelated:\n  - docs/a.md\n---\n\n# C\n"))

	// A has one outgoing edge.
	outA, err := f.graph.GetOutgoing(f.ctx, docA)
	if err != nil {
		t.Fatalf("GetOutgoing(A): %v", err)
	}
	if len(outA) != 1 {
		t.Errorf("GetOutgoing(A): expected 1 edge, got %d: %+v", len(outA), outA)
	}

	// B has one outgoing wikilink.
	outB, err := f.graph.GetOutgoing(f.ctx, docB)
	if err != nil {
		t.Fatalf("GetOutgoing(B): %v", err)
	}
	if len(outB) != 1 {
		t.Errorf("GetOutgoing(B): expected 1 edge, got %d: %+v", len(outB), outB)
	}
	if outB[0].LinkType != docgraph.LinkTypeWikilink {
		t.Errorf("GetOutgoing(B): expected wikilink, got %q", outB[0].LinkType)
	}

	// C has one frontmatter edge back to A.
	outC, err := f.graph.GetOutgoing(f.ctx, docC)
	if err != nil {
		t.Fatalf("GetOutgoing(C): %v", err)
	}
	if len(outC) != 1 {
		t.Errorf("GetOutgoing(C): expected 1 edge, got %d: %+v", len(outC), outC)
	}
	if outC[0].LinkType != docgraph.LinkTypeFrontmatter {
		t.Errorf("GetOutgoing(C): expected frontmatter, got %q", outC[0].LinkType)
	}

	// A receives one backlink from C.
	backA, err := f.graph.GetBacklinks(f.ctx, "docs/a.md")
	if err != nil {
		t.Fatalf("GetBacklinks(A): %v", err)
	}
	foundC := false
	for _, r := range backA {
		if r.SourcePath == docC {
			foundC = true
			break
		}
	}
	if !foundC {
		t.Errorf("GetBacklinks(A): expected source C, got %+v", backA)
	}
}

// TestDocGraph_BrokenLinkDetection verifies that a link to a nonexistent doc
// appears in GetBrokenLinks and that links to existing docs do not.
func TestDocGraph_BrokenLinkDetection(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	const docA = "docs/a.md"
	f.seedDoc(t, docA)
	// Note: "docs/missing.md" is intentionally NOT seeded.

	// A links to missing.md (broken) and a.md itself (not broken — docA exists).
	contentA := []byte("# A\n\nSee [missing](missing.md) and [self](a.md).\n")
	f.extractAndSave(t, docA, contentA)

	broken, err := f.graph.GetBrokenLinks(f.ctx)
	if err != nil {
		t.Fatalf("GetBrokenLinks: %v", err)
	}

	// "docs/missing.md" must appear in broken links.
	foundMissing := false
	for _, r := range broken {
		if r.TargetPath == "docs/missing.md" {
			foundMissing = true
			break
		}
	}
	if !foundMissing {
		t.Errorf("GetBrokenLinks: expected %q, got %+v", "docs/missing.md", broken)
	}

	// "docs/a.md" must NOT appear as broken (the doc exists).
	for _, r := range broken {
		if r.TargetPath == "docs/a.md" {
			t.Errorf("GetBrokenLinks: %q is a valid doc but appeared as broken", "docs/a.md")
		}
	}
}

// TestDocGraph_IncrementalUpdate verifies that re-saving with fewer refs
// removes the old edges atomically: the deleted link must not appear in
// GetOutgoing or GetBrokenLinks after the update.
func TestDocGraph_IncrementalUpdate(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	const (
		docA = "docs/a.md"
		docB = "docs/b.md"
	)
	f.seedDoc(t, docA)
	f.seedDoc(t, docB)

	// First save: A links to both B and missing.md.
	contentV1 := []byte("# A\n\nSee [B](b.md) and [gone](missing.md).\n")
	f.extractAndSave(t, docA, contentV1)

	outV1, err := f.graph.GetOutgoing(f.ctx, docA)
	if err != nil {
		t.Fatalf("GetOutgoing v1: %v", err)
	}
	if len(outV1) != 2 {
		t.Fatalf("expected 2 outgoing refs before update, got %d: %+v", len(outV1), outV1)
	}

	// Second save: A no longer links to B — only a bare paragraph remains.
	contentV2 := []byte("# A\n\nNo links here.\n")
	f.extractAndSave(t, docA, contentV2)

	outV2, err := f.graph.GetOutgoing(f.ctx, docA)
	if err != nil {
		t.Fatalf("GetOutgoing v2: %v", err)
	}
	if len(outV2) != 0 {
		t.Errorf("expected 0 outgoing refs after removing all links, got %d: %+v", len(outV2), outV2)
	}

	// The old broken link must no longer appear in GetBrokenLinks.
	broken, err := f.graph.GetBrokenLinks(f.ctx)
	if err != nil {
		t.Fatalf("GetBrokenLinks after update: %v", err)
	}
	for _, r := range broken {
		if r.SourcePath == docA {
			t.Errorf("stale broken link from %q still present after incremental update: %+v", docA, r)
		}
	}
}

// TestDocGraph_VedoxSchemeExtraction verifies that a vedox://file/... URI
// with a line-range anchor is extracted with the correct TargetPath and
// AnchorText and saved as a LinkTypeVedoxScheme edge.
func TestDocGraph_VedoxSchemeExtraction(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	const docA = "docs/a.md"
	f.seedDoc(t, docA)

	// Embed a vedox:// URI as bare prose — the extractor's bare-URI pass
	// must capture it even though it is not inside a markdown link node.
	contentA := []byte("# A\n\nSee vedox://file/main.tf#L10-L25 for the Terraform config.\n")
	f.extractAndSave(t, docA, contentA)

	outA, err := f.graph.GetOutgoing(f.ctx, docA)
	if err != nil {
		t.Fatalf("GetOutgoing(%q): %v", docA, err)
	}

	found := false
	for _, r := range outA {
		if r.LinkType == docgraph.LinkTypeVedoxScheme {
			found = true
			if r.TargetPath != "vedox://file/main.tf" {
				t.Errorf("vedox-scheme TargetPath = %q, want %q", r.TargetPath, "vedox://file/main.tf")
			}
			if r.AnchorText != "L10-L25" {
				t.Errorf("vedox-scheme AnchorText = %q, want %q", r.AnchorText, "L10-L25")
			}
			break
		}
	}
	if !found {
		t.Errorf("GetOutgoing(%q): expected LinkTypeVedoxScheme, got %+v", docA, outA)
	}

	// vedox-scheme targets are NOT doc paths so they must not appear in GetBrokenLinks.
	broken, err := f.graph.GetBrokenLinks(f.ctx)
	if err != nil {
		t.Fatalf("GetBrokenLinks: %v", err)
	}
	for _, r := range broken {
		if r.LinkType == docgraph.LinkTypeVedoxScheme {
			t.Errorf("GetBrokenLinks: vedox-scheme ref should be excluded, got %+v", r)
		}
	}
}

// TestDocGraph_SaveRefs_EmptyDocID verifies that SaveRefs with an empty docID
// returns an error immediately rather than inserting a bad row.
func TestDocGraph_SaveRefs_EmptyDocID(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	err := f.graph.SaveRefs(f.ctx, "", []docgraph.DocRef{})
	if err == nil {
		t.Error("SaveRefs(\"\", ...) should return an error for empty docID")
	}
}

// TestDocGraph_GetOutgoing_EmptyDocID verifies that GetOutgoing with an empty
// docID returns an error.
func TestDocGraph_GetOutgoing_EmptyDocID(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	_, err := f.graph.GetOutgoing(f.ctx, "")
	if err == nil {
		t.Error("GetOutgoing(\"\") should return an error for empty docID")
	}
}

// TestDocGraph_GetBacklinks_EmptyTarget verifies that GetBacklinks with an
// empty target path returns an error.
func TestDocGraph_GetBacklinks_EmptyTarget(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	_, err := f.graph.GetBacklinks(f.ctx, "")
	if err == nil {
		t.Error("GetBacklinks(\"\") should return an error for empty targetPath")
	}
}
