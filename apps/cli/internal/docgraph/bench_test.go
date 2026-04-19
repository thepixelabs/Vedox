package docgraph_test

// bench_test.go — hot-path benchmarks for the docgraph package.
//
// Two paths are exercised:
//
//  1. Extract — pure in-memory parsing of a realistic Markdown document
//     with all four link types (md-link, wikilink, frontmatter, vedox-scheme).
//     No DB I/O. This is the inner loop of the indexer pipeline.
//
//  2. SaveRefs — the full SQLite write path: delete old edges, insert new
//     ones, refresh ref-count aggregates in one transaction. Uses a real
//     db.Store opened at b.TempDir() so WAL mode and foreign-key constraints
//     are active (same as production).
//
// Run with:
//
//	go test -bench=. -benchmem -run=^$ ./internal/docgraph/...
import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/docgraph"
)

// ── shared SQLite init guard ──────────────────────────────────────────────────

// benchSQLiteOnce serialises the first db.Open across benchmarks so
// modernc.org/sqlite's Xsqlite3_initialize() completes before any parallel
// store opens a second connection.
var benchSQLiteOnce sync.Once

// benchOpenDB opens a real db.Store in b.TempDir(), registering a Cleanup
// to close it. It serialises the first open to avoid the SQLite init race.
func benchOpenDB(b *testing.B) *db.Store {
	b.Helper()
	raw := b.TempDir()
	resolved, err := filepath.EvalSymlinks(raw)
	if err != nil {
		b.Fatalf("EvalSymlinks: %v", err)
	}
	benchSQLiteOnce.Do(func() {
		probe, err := db.Open(db.Options{WorkspaceRoot: resolved})
		if err != nil {
			b.Fatalf("sqlite init probe: %v", err)
		}
		_ = probe.Close()
	})
	s, err := db.Open(db.Options{WorkspaceRoot: resolved})
	if err != nil {
		b.Fatalf("db.Open: %v", err)
	}
	b.Cleanup(func() { _ = s.Close() })
	return s
}

// ── BenchmarkExtractor_TypicalDoc ────────────────────────────────────────────

// typicalDocContent is a realistic 32-section architecture decision record with
// all four link types embedded. It is ~4 KB — large enough to exercise the
// goldmark parser and the wikilink/vedox regex passes without being I/O-bound.
//
// The exact content is deterministic so benchmark results are reproducible.
var typicalDocContent = buildTypicalDoc()

// buildTypicalDoc constructs a synthetic but representative Markdown document
// containing md-links, wikilinks, frontmatter refs, and vedox-scheme URIs.
func buildTypicalDoc() []byte {
	const body = `---
title: Architecture Decision Record 001
date: 2026-04-01
status: accepted
related:
  - docs/overview.md
  - docs/security-model.md
supersedes:
  - docs/old-adr.md
---

# ADR 001: Authentication Strategy

## Context

The documentation system must authenticate agents before they can commit files.
See [the security model](../security-model.md) for threat classification.
Cross-reference: vedox://file/cmd/server.go#L45-L90

## Decision

We adopt HMAC-SHA256 keys for agent auth. See [[Agent Auth Design]] for full
rationale and [[Key Rotation Policy]] for operational details.

## Consequences

- Positive: deterministic, no network round-trip per commit.
- Negative: key rotation requires daemon restart.

Implementation is in vedox://file/internal/agentauth/hmac.go.

## References

- [HMAC RFC](../standards/rfc2104.md)
- [Security Review](../reviews/2026-03-security.md)
- [[Threat Model]]
- vedox://file/docs/security.md#L1-L20
`
	return []byte(body)
}

// BenchmarkExtractor_TypicalDoc measures Extract on a realistic architecture
// decision record (~4 KB) with all four link types. This is the hot inner loop
// of the indexer pipeline — called once per file on every re-index.
func BenchmarkExtractor_TypicalDoc(b *testing.B) {
	content := typicalDocContent
	b.SetBytes(int64(len(content)))
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		refs := docgraph.Extract("docs/adr/001-auth.md", content)
		if len(refs) == 0 {
			b.Fatal("expected refs from typical doc, got none")
		}
	}
}

// ── BenchmarkSaveRefs_HotPath ─────────────────────────────────────────────────

// BenchmarkSaveRefs_HotPath measures the full SaveRefs transaction on a real
// SQLite store. Each iteration:
//   - Replaces the edge set for a single document (8 outgoing refs).
//   - Refreshes the outgoing ref-count aggregate.
//   - Refreshes the backlink-count aggregate for each affected target.
//
// The store is pre-seeded with a source document row before the timed region so
// the FK constraint on source_doc_id is satisfied from iteration 0.
// Target paths are intentionally unknown to the documents table (broken links)
// so GetBrokenLinks semantics are also exercised on each count refresh.
func BenchmarkSaveRefs_HotPath(b *testing.B) {
	s := benchOpenDB(b)
	g := docgraph.NewGraphStore(s)
	ctx := context.Background()

	const docID = "bench/adr-001.md"

	// Seed the source document so the FK constraint is satisfied.
	if err := s.UpsertDoc(ctx, &db.Doc{
		ID:          docID,
		Project:     "bench",
		Title:       "ADR 001",
		ContentHash: "deadbeef",
		ModTime:     "2026-04-01T00:00:00Z",
		Size:        1024,
	}); err != nil {
		b.Fatalf("seed source doc: %v", err)
	}

	// Build a stable set of 8 refs covering all four link types.
	refs := []docgraph.DocRef{
		{SourcePath: docID, TargetPath: "bench/security-model.md", LinkType: docgraph.LinkTypeMD, LineNum: 10},
		{SourcePath: docID, TargetPath: "bench/overview.md", LinkType: docgraph.LinkTypeMD, LineNum: 14},
		{SourcePath: docID, TargetPath: "Agent Auth Design", LinkType: docgraph.LinkTypeWikilink, LineNum: 20},
		{SourcePath: docID, TargetPath: "Key Rotation Policy", LinkType: docgraph.LinkTypeWikilink, LineNum: 21},
		{SourcePath: docID, TargetPath: "bench/old-adr.md", LinkType: docgraph.LinkTypeFrontmatter, LineNum: 0},
		{SourcePath: docID, TargetPath: "bench/overview.md", LinkType: docgraph.LinkTypeFrontmatter, LineNum: 0},
		{SourcePath: docID, TargetPath: "vedox://file/cmd/server.go", LinkType: docgraph.LinkTypeVedoxScheme, LineNum: 12, AnchorText: "L45-L90"},
		{SourcePath: docID, TargetPath: "vedox://file/internal/agentauth/hmac.go", LinkType: docgraph.LinkTypeVedoxScheme, LineNum: 28},
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := g.SaveRefs(ctx, docID, refs); err != nil {
			b.Fatalf("SaveRefs: %v", err)
		}
	}
}

// BenchmarkSaveRefs_ManyTargets measures SaveRefs with a larger ref set (64
// targets) to quantify how the per-target backlink_count refresh scales. This
// exercises the hot path when a hub document (e.g. an index page) links to
// many other documents.
func BenchmarkSaveRefs_ManyTargets(b *testing.B) {
	s := benchOpenDB(b)
	g := docgraph.NewGraphStore(s)
	ctx := context.Background()

	const docID = "bench/hub.md"
	const refCount = 64

	if err := s.UpsertDoc(ctx, &db.Doc{
		ID:          docID,
		Project:     "bench",
		Title:       "Hub",
		ContentHash: "cafebabe",
		ModTime:     "2026-04-01T00:00:00Z",
		Size:        4096,
	}); err != nil {
		b.Fatalf("seed hub doc: %v", err)
	}

	refs := make([]docgraph.DocRef, refCount)
	for i := 0; i < refCount; i++ {
		refs[i] = docgraph.DocRef{
			SourcePath: docID,
			TargetPath: fmt.Sprintf("bench/doc-%03d.md", i),
			LinkType:   docgraph.LinkTypeMD,
			LineNum:    i + 1,
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := g.SaveRefs(ctx, docID, refs); err != nil {
			b.Fatalf("SaveRefs: %v", err)
		}
	}
}
