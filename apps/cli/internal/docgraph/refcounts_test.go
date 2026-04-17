package docgraph_test

// Tests for the doc_reference_counts denormalised aggregate (FIX-ARCH-07).
//
// These tests reuse the graphFixture defined in docgraph_integration_test.go
// (same _test package). Each test exercises the SaveRefs/DeleteRefs lifecycle
// and asserts the cached counts stay in lockstep with what SELECT COUNT(*)
// would return against doc_references — the contract the fast aggregate must
// honour. We deliberately do NOT mock the SQLite layer; a real db.Store is
// opened so the migration is exercised end-to-end.

import (
	"context"
	"testing"

	"github.com/vedox/vedox/internal/docgraph"
)

// assertCounts fetches the cached counts row, compares to the expected
// (refCount, backlinkCount), and cross-checks against a live COUNT(*) query
// so a drift between aggregate and source-of-truth is caught immediately.
func assertCounts(
	t *testing.T,
	f *graphFixture,
	docID string,
	wantRef, wantBack int,
) {
	t.Helper()
	got, err := f.graph.GetReferenceCounts(f.ctx, docID)
	if err != nil {
		t.Fatalf("GetReferenceCounts(%q): %v", docID, err)
	}
	if got.RefCount != wantRef || got.BacklinkCount != wantBack {
		t.Errorf("counts(%q) = {ref=%d, back=%d}, want {ref=%d, back=%d}",
			docID, got.RefCount, got.BacklinkCount, wantRef, wantBack)
	}

	// Cross-check against doc_references so a stale cache fails loudly.
	var liveRef, liveBack int
	if err := f.store.ReadDB().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM doc_references WHERE source_doc_id = ?`,
		docID,
	).Scan(&liveRef); err != nil {
		t.Fatalf("live ref count(%q): %v", docID, err)
	}
	if err := f.store.ReadDB().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM doc_references WHERE target_path = ? AND link_type != ?`,
		docID, string(docgraph.LinkTypeVedoxScheme),
	).Scan(&liveBack); err != nil {
		t.Fatalf("live backlink count(%q): %v", docID, err)
	}
	if liveRef != got.RefCount {
		t.Errorf("ref_count cache drift on %q: cached=%d live=%d", docID, got.RefCount, liveRef)
	}
	if liveBack != got.BacklinkCount {
		t.Errorf("backlink_count cache drift on %q: cached=%d live=%d", docID, got.BacklinkCount, liveBack)
	}
}

// TestRefCounts_BasicSaveAndUpdate covers the simplest happy path: A links to
// B, then A is rewritten to link to B and C. The cached counts must reflect
// each rewrite without leaking the old state.
func TestRefCounts_BasicSaveAndUpdate(t *testing.T) {
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

	// Round 1: A -> B only.
	f.extractAndSave(t, docA, []byte("# A\n\nSee [B](b.md).\n"))
	assertCounts(t, f, docA, 1, 0)
	assertCounts(t, f, docB, 0, 1)
	assertCounts(t, f, docC, 0, 0)

	// Round 2: A -> B and A -> C. B's backlink count stays at 1, C gains one.
	f.extractAndSave(t, docA, []byte("# A\n\nSee [B](b.md) and [C](c.md).\n"))
	assertCounts(t, f, docA, 2, 0)
	assertCounts(t, f, docB, 0, 1)
	assertCounts(t, f, docC, 0, 1)
}

// TestRefCounts_ReplaceTargetDecrementsOldBacklink rewrites a doc so that an
// old target loses its backlink. This is the classic case where a naive
// "increment-on-insert" implementation would leak — the backlink_count for
// the dropped target must drop to 0 in the same tx that inserted the new edge.
func TestRefCounts_ReplaceTargetDecrementsOldBacklink(t *testing.T) {
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
	assertCounts(t, f, docB, 0, 1)
	assertCounts(t, f, docC, 0, 0)

	// A now points only at C — B must lose its backlink.
	f.extractAndSave(t, docA, []byte("# A\n\nSee [C](c.md).\n"))
	assertCounts(t, f, docA, 1, 0)
	assertCounts(t, f, docB, 0, 0)
	assertCounts(t, f, docC, 0, 1)
}

// TestRefCounts_DeleteRefsClearsBoth deletes a source doc's outgoing edges
// entirely. Its ref_count must zero out, and every former target's
// backlink_count must drop accordingly.
func TestRefCounts_DeleteRefsClearsBoth(t *testing.T) {
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

	f.extractAndSave(t, docA, []byte("# A\n\nSee [B](b.md) and [C](c.md).\n"))
	assertCounts(t, f, docA, 2, 0)
	assertCounts(t, f, docB, 0, 1)
	assertCounts(t, f, docC, 0, 1)

	if err := f.graph.DeleteRefs(f.ctx, docA); err != nil {
		t.Fatalf("DeleteRefs(%q): %v", docA, err)
	}
	assertCounts(t, f, docA, 0, 0)
	assertCounts(t, f, docB, 0, 0)
	assertCounts(t, f, docC, 0, 0)
}

// TestRefCounts_MultipleCycles hammers SaveRefs/DeleteRefs in alternation to
// verify the aggregate never drifts no matter how many rewrite cycles occur.
// This is the regression test for the bug FIX-ARCH-07 was filed to prevent —
// long-lived workspaces accumulating stale counts.
func TestRefCounts_MultipleCycles(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	const (
		docA = "docs/a.md"
		docB = "docs/b.md"
		docC = "docs/c.md"
		docD = "docs/d.md"
	)
	for _, d := range []string{docA, docB, docC, docD} {
		f.seedDoc(t, d)
	}

	cycles := []struct {
		name    string
		content []byte
		// want is a map[docID]{refCount, backlinkCount} expected after this cycle.
		want map[string][2]int
	}{
		{
			name:    "A->B,C",
			content: []byte("# A\n\nSee [B](b.md) and [C](c.md).\n"),
			want: map[string][2]int{
				docA: {2, 0}, docB: {0, 1}, docC: {0, 1}, docD: {0, 0},
			},
		},
		{
			name:    "A->C,D",
			content: []byte("# A\n\nSee [C](c.md) and [D](d.md).\n"),
			want: map[string][2]int{
				docA: {2, 0}, docB: {0, 0}, docC: {0, 1}, docD: {0, 1},
			},
		},
		{
			name:    "A->B only",
			content: []byte("# A\n\nSee [B](b.md).\n"),
			want: map[string][2]int{
				docA: {1, 0}, docB: {0, 1}, docC: {0, 0}, docD: {0, 0},
			},
		},
		{
			name:    "A no links",
			content: []byte("# A\n\nNothing here.\n"),
			want: map[string][2]int{
				docA: {0, 0}, docB: {0, 0}, docC: {0, 0}, docD: {0, 0},
			},
		},
		{
			name:    "A->B,C,D",
			content: []byte("# A\n\nSee [B](b.md), [C](c.md), [D](d.md).\n"),
			want: map[string][2]int{
				docA: {3, 0}, docB: {0, 1}, docC: {0, 1}, docD: {0, 1},
			},
		},
	}

	for _, c := range cycles {
		c := c
		t.Run(c.name, func(t *testing.T) {
			f.extractAndSave(t, docA, c.content)
			for d, w := range c.want {
				assertCounts(t, f, d, w[0], w[1])
			}
		})
	}

	// Final cleanup: DeleteRefs zeroes everything.
	if err := f.graph.DeleteRefs(f.ctx, docA); err != nil {
		t.Fatalf("DeleteRefs(%q): %v", docA, err)
	}
	for _, d := range []string{docA, docB, docC, docD} {
		assertCounts(t, f, d, 0, 0)
	}
}

// TestRefCounts_VedoxSchemeExcludedFromBacklinks verifies that vedox:// edges
// do NOT bump backlink_count. They are source-code references, not doc
// references, and including them would skew the "most referenced doc"
// rankings.
func TestRefCounts_VedoxSchemeExcludedFromBacklinks(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	const docA = "docs/a.md"
	f.seedDoc(t, docA)

	f.extractAndSave(t, docA, []byte("# A\n\nSee vedox://file/main.tf#L10-L25 for the config.\n"))

	// A's outgoing count includes the vedox edge (it lives in doc_references).
	got, err := f.graph.GetReferenceCounts(f.ctx, docA)
	if err != nil {
		t.Fatalf("GetReferenceCounts(A): %v", err)
	}
	if got.RefCount != 1 {
		t.Errorf("A.ref_count = %d, want 1", got.RefCount)
	}

	// The vedox target must NOT have a counts row (or it must be zero).
	vedoxCounts, err := f.graph.GetReferenceCounts(f.ctx, "vedox://file/main.tf")
	if err != nil {
		t.Fatalf("GetReferenceCounts(vedox): %v", err)
	}
	if vedoxCounts.BacklinkCount != 0 {
		t.Errorf("vedox target backlink_count = %d, want 0 (vedox edges must not be counted)",
			vedoxCounts.BacklinkCount)
	}
}

// TestRefCounts_TopReferencedDocs verifies the leaderboard query returns docs
// in descending backlink_count order with ties broken by doc_id, that the
// limit is honoured, and that zero-backlink docs are excluded.
func TestRefCounts_TopReferencedDocs(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)

	const (
		hub  = "docs/hub.md"
		mid  = "docs/mid.md"
		leaf = "docs/leaf.md"
		a    = "docs/a.md"
		b    = "docs/b.md"
		c    = "docs/c.md"
	)
	for _, d := range []string{hub, mid, leaf, a, b, c} {
		f.seedDoc(t, d)
	}

	// hub gets 3 inbound edges, mid gets 2, leaf gets 1, a/b/c get 0.
	f.extractAndSave(t, a, []byte("[hub](hub.md) and [mid](mid.md) and [leaf](leaf.md)\n"))
	f.extractAndSave(t, b, []byte("[hub](hub.md) and [mid](mid.md)\n"))
	f.extractAndSave(t, c, []byte("[hub](hub.md)\n"))

	top, err := f.graph.TopReferencedDocs(f.ctx, 10)
	if err != nil {
		t.Fatalf("TopReferencedDocs: %v", err)
	}
	wantOrder := []string{hub, mid, leaf}
	if len(top) != len(wantOrder) {
		t.Fatalf("TopReferencedDocs returned %d rows, want %d: %+v", len(top), len(wantOrder), top)
	}
	for i, want := range wantOrder {
		if top[i].DocID != want {
			t.Errorf("TopReferencedDocs[%d] = %q, want %q", i, top[i].DocID, want)
		}
	}
	if top[0].BacklinkCount != 3 || top[1].BacklinkCount != 2 || top[2].BacklinkCount != 1 {
		t.Errorf("TopReferencedDocs counts wrong: %+v", top)
	}

	// Limit must clip the result set.
	top2, err := f.graph.TopReferencedDocs(f.ctx, 2)
	if err != nil {
		t.Fatalf("TopReferencedDocs(2): %v", err)
	}
	if len(top2) != 2 {
		t.Errorf("TopReferencedDocs(2) returned %d rows, want 2", len(top2))
	}

	// Bad limit is rejected.
	if _, err := f.graph.TopReferencedDocs(f.ctx, 0); err == nil {
		t.Error("TopReferencedDocs(0) should error on non-positive limit")
	}
	if _, err := f.graph.TopReferencedDocs(f.ctx, -1); err == nil {
		t.Error("TopReferencedDocs(-1) should error on non-positive limit")
	}
}

// TestRefCounts_GetReferenceCounts_Validation verifies the input guard.
func TestRefCounts_GetReferenceCounts_Validation(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)
	if _, err := f.graph.GetReferenceCounts(f.ctx, ""); err == nil {
		t.Error("GetReferenceCounts(\"\") should return an error")
	}
}

// TestRefCounts_MissingDocReturnsZero verifies that asking for counts of a doc
// that has never been touched returns the zero value with no error — the
// "no rows" case must not be an error condition.
func TestRefCounts_MissingDocReturnsZero(t *testing.T) {
	t.Parallel()
	f := newGraphFixture(t)
	got, err := f.graph.GetReferenceCounts(f.ctx, "docs/never-existed.md")
	if err != nil {
		t.Fatalf("GetReferenceCounts on missing doc: %v", err)
	}
	if got.RefCount != 0 || got.BacklinkCount != 0 {
		t.Errorf("missing doc counts = %+v, want zero", got)
	}
}
