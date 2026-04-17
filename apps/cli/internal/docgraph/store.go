// Package docgraph — store.go provides the SQLite persistence layer for the
// doc reference graph. All writes go through the single-writer goroutine
// inherited from db.Store; reads use the shared read-only connection pool.
//
// Schema is created by migration 005_doc_graph.sql (edges) and
// 007_doc_reference_counts.sql (denormalised aggregate). This file does not
// run migrations itself — callers must open a db.Store (which runs all
// pending migrations) before constructing a GraphStore.
package docgraph

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// GraphStore persists and queries the doc reference graph.
// It holds a write handle (routed through the single-writer funnel via
// submitFn) and a read handle for concurrent reads.
//
// Construct with NewGraphStore; the caller is responsible for the lifecycle
// of the underlying *sql.DB handles.
type GraphStore struct {
	// submitFn is the single-writer funnel from db.Store. It accepts a
	// function that executes inside a transaction and returns its error.
	// This matches the signature of db.Store's internal writer.submit.
	submitFn func(ctx context.Context, fn func(tx *sql.Tx) error) error

	// readDB is a read-only *sql.DB handle. WAL mode allows concurrent reads
	// alongside the single writer.
	readDB *sql.DB
}

// Submitter is the interface that db.Store exposes for graph writes.
// We accept an interface rather than the concrete db.Store so that tests
// can provide a lightweight fake without opening a real SQLite file.
type Submitter interface {
	SubmitWrite(ctx context.Context, fn func(tx *sql.Tx) error) error
	ReadDB() *sql.DB
}

// NewGraphStore constructs a GraphStore from the provided Submitter.
func NewGraphStore(s Submitter) *GraphStore {
	return &GraphStore{
		submitFn: s.SubmitWrite,
		readDB:   s.ReadDB(),
	}
}

// SaveRefs atomically replaces all outgoing references for docID with refs.
// The operation is a single transaction: snapshot old targets, delete old
// edges, insert new ones, then refresh the doc_reference_counts row for the
// source AND for every target whose inbound count may have changed (the union
// of old and new targets). Doing the count maintenance inside the same tx
// guarantees the aggregate never drifts from the underlying edge set even if
// multiple SaveRefs calls interleave on the writer goroutine.
//
// docID must be the workspace-relative slash path of the source document
// (identical to db.Doc.ID). refs is the full set of outgoing edges for the
// current file content.
func (g *GraphStore) SaveRefs(ctx context.Context, docID string, refs []DocRef) error {
	if docID == "" {
		return fmt.Errorf("docgraph: SaveRefs: empty docID")
	}
	now := time.Now().UTC().Format(time.RFC3339)

	return g.submitFn(ctx, func(tx *sql.Tx) error {
		// Snapshot the targets that the source previously pointed at — we
		// need them so we can recompute the backlink_count for each one
		// after the rewrite. vedox-scheme rows are excluded to mirror the
		// counts table's exclusion of source-code targets.
		oldTargets, err := selectAffectedTargets(tx, docID)
		if err != nil {
			return fmt.Errorf("read old targets for %q: %w", docID, err)
		}

		// Delete all existing outgoing edges for this source.
		if _, err := tx.Exec(
			`DELETE FROM doc_references WHERE source_doc_id = ?`, docID,
		); err != nil {
			return fmt.Errorf("delete old refs for %q: %w", docID, err)
		}

		// Insert new edges.
		stmt, err := tx.Prepare(
			`INSERT INTO doc_references
				(source_doc_id, target_path, link_type, line_num, anchor_text, created_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
		)
		if err != nil {
			return fmt.Errorf("prepare insert ref: %w", err)
		}
		defer stmt.Close()

		for _, r := range refs {
			if _, err := stmt.Exec(
				docID,
				r.TargetPath,
				string(r.LinkType),
				r.LineNum,
				r.AnchorText,
				now,
			); err != nil {
				return fmt.Errorf("insert ref %q -> %q: %w", docID, r.TargetPath, err)
			}
		}

		// Build the union of old + new targets that need backlink_count
		// refreshed. Using a set deduplicates the case where the same
		// target appears in both sides of the rewrite.
		affected := make(map[string]struct{}, len(oldTargets)+len(refs))
		for _, t := range oldTargets {
			affected[t] = struct{}{}
		}
		for _, r := range refs {
			if r.LinkType == LinkTypeVedoxScheme {
				continue
			}
			if r.TargetPath == "" {
				continue
			}
			affected[r.TargetPath] = struct{}{}
		}

		nowUnix := time.Now().UTC().Unix()
		if err := refreshOutgoingCount(tx, docID, nowUnix); err != nil {
			return fmt.Errorf("refresh ref_count for %q: %w", docID, err)
		}
		for target := range affected {
			if err := refreshInboundCount(tx, target, nowUnix); err != nil {
				return fmt.Errorf("refresh backlink_count for %q: %w", target, err)
			}
		}
		return nil
	})
}

// GetOutgoing returns all outgoing references from docID.
func (g *GraphStore) GetOutgoing(ctx context.Context, docID string) ([]DocRef, error) {
	if docID == "" {
		return nil, fmt.Errorf("docgraph: GetOutgoing: empty docID")
	}
	rows, err := g.readDB.QueryContext(ctx,
		`SELECT source_doc_id, target_path, link_type, line_num, anchor_text
		   FROM doc_references
		  WHERE source_doc_id = ?
		  ORDER BY line_num, id`,
		docID,
	)
	if err != nil {
		return nil, fmt.Errorf("docgraph: GetOutgoing %q: %w", docID, err)
	}
	defer rows.Close()
	return scanRefs(rows)
}

// GetBacklinks returns all references whose target_path matches targetPath.
// targetPath should be a workspace-relative slash path (e.g. "docs/adr/001.md").
func (g *GraphStore) GetBacklinks(ctx context.Context, targetPath string) ([]DocRef, error) {
	if targetPath == "" {
		return nil, fmt.Errorf("docgraph: GetBacklinks: empty targetPath")
	}
	rows, err := g.readDB.QueryContext(ctx,
		`SELECT source_doc_id, target_path, link_type, line_num, anchor_text
		   FROM doc_references
		  WHERE target_path = ?
		  ORDER BY source_doc_id, line_num`,
		targetPath,
	)
	if err != nil {
		return nil, fmt.Errorf("docgraph: GetBacklinks %q: %w", targetPath, err)
	}
	defer rows.Close()
	return scanRefs(rows)
}

// GetBrokenLinks returns all references whose target_path does not match any
// row in the documents table. These are links to files that do not exist in
// the index (either never existed, were deleted, or were mis-typed).
func (g *GraphStore) GetBrokenLinks(ctx context.Context) ([]DocRef, error) {
	rows, err := g.readDB.QueryContext(ctx,
		`SELECT r.source_doc_id, r.target_path, r.link_type, r.line_num, r.anchor_text
		   FROM doc_references r
		  WHERE NOT EXISTS (
		        SELECT 1 FROM documents d WHERE d.id = r.target_path
		  )
		    AND r.link_type != ?
		  ORDER BY r.source_doc_id, r.line_num`,
		string(LinkTypeVedoxScheme), // vedox:// targets are source-code paths, not docs
	)
	if err != nil {
		return nil, fmt.Errorf("docgraph: GetBrokenLinks: %w", err)
	}
	defer rows.Close()
	return scanRefs(rows)
}

// DeleteRefs removes all outgoing references for docID. Call this when a
// document is deleted from the index. The doc_reference_counts row for the
// source is zeroed (not deleted — it may still be a target of other docs)
// and every former target's backlink_count is recomputed in the same tx.
func (g *GraphStore) DeleteRefs(ctx context.Context, docID string) error {
	if docID == "" {
		return fmt.Errorf("docgraph: DeleteRefs: empty docID")
	}
	return g.submitFn(ctx, func(tx *sql.Tx) error {
		oldTargets, err := selectAffectedTargets(tx, docID)
		if err != nil {
			return fmt.Errorf("read old targets for %q: %w", docID, err)
		}
		if _, err := tx.Exec(`DELETE FROM doc_references WHERE source_doc_id = ?`, docID); err != nil {
			return err
		}
		nowUnix := time.Now().UTC().Unix()
		if err := refreshOutgoingCount(tx, docID, nowUnix); err != nil {
			return fmt.Errorf("refresh ref_count for %q: %w", docID, err)
		}
		for _, target := range oldTargets {
			if err := refreshInboundCount(tx, target, nowUnix); err != nil {
				return fmt.Errorf("refresh backlink_count for %q: %w", target, err)
			}
		}
		return nil
	})
}

// GetAllRefsForPrefix returns all outgoing references whose source_doc_id begins
// with the given prefix. This is used by the graph API endpoint to retrieve every
// edge that belongs to a specific project in one query, avoiding N+1 round-trips
// over individual document IDs.
//
// prefix must be a workspace-relative slash path prefix, e.g. "my-project/".
// The trailing slash is significant — callers must include it so that a project
// named "foo" does not accidentally match "foobar/".
func (g *GraphStore) GetAllRefsForPrefix(ctx context.Context, prefix string) ([]DocRef, error) {
	if prefix == "" {
		return nil, fmt.Errorf("docgraph: GetAllRefsForPrefix: empty prefix")
	}
	// SQLite LIKE with a trailing wildcard uses the idx_doc_refs_source index,
	// so this query is index-assisted even on large graphs. The caller's
	// prefix is escaped below so a project id containing SQLite LIKE
	// metacharacters ('%' and '_') does not accidentally match siblings.
	rows, err := g.readDB.QueryContext(ctx,
		`SELECT source_doc_id, target_path, link_type, line_num, anchor_text
		   FROM doc_references
		  WHERE source_doc_id LIKE ? ESCAPE '\'
		  ORDER BY source_doc_id, line_num, id`,
		escapeLikePrefix(prefix)+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("docgraph: GetAllRefsForPrefix %q: %w", prefix, err)
	}
	defer rows.Close()
	return scanRefs(rows)
}

// ReferenceCounts is the aggregate edge-count view of a single document.
type ReferenceCounts struct {
	// DocID is the workspace-relative slash path.
	DocID string
	// RefCount is the number of outgoing edges from this doc.
	RefCount int
	// BacklinkCount is the number of inbound edges (vedox-scheme excluded).
	BacklinkCount int
	// UpdatedAt is the unix timestamp of the last count refresh.
	UpdatedAt int64
}

// GetReferenceCounts returns the cached ref_count and backlink_count for
// docID. If no counts row exists for docID (the doc has never been a source
// or a non-vedox target) the zero ReferenceCounts is returned with no error.
func (g *GraphStore) GetReferenceCounts(ctx context.Context, docID string) (ReferenceCounts, error) {
	if docID == "" {
		return ReferenceCounts{}, fmt.Errorf("docgraph: GetReferenceCounts: empty docID")
	}
	var rc ReferenceCounts
	err := g.readDB.QueryRowContext(ctx,
		`SELECT doc_id, ref_count, backlink_count, updated_at
		   FROM doc_reference_counts
		  WHERE doc_id = ?`,
		docID,
	).Scan(&rc.DocID, &rc.RefCount, &rc.BacklinkCount, &rc.UpdatedAt)
	if err == sql.ErrNoRows {
		return ReferenceCounts{DocID: docID}, nil
	}
	if err != nil {
		return ReferenceCounts{}, fmt.Errorf("docgraph: GetReferenceCounts %q: %w", docID, err)
	}
	return rc, nil
}

// TopReferencedDocs returns the top `limit` docs by backlink_count, descending,
// with ties broken by doc_id for determinism. Docs with zero backlinks are
// excluded — there is no use case where we want to surface unreferenced docs
// in a "most referenced" list, and excluding them keeps the result set tight
// on workspaces with many leaf docs.
//
// limit must be > 0; a non-positive limit returns an error rather than the
// confusing default behaviour of SQLite's LIMIT (which silently treats <=0
// as "no limit").
func (g *GraphStore) TopReferencedDocs(ctx context.Context, limit int) ([]ReferenceCounts, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("docgraph: TopReferencedDocs: limit must be > 0, got %d", limit)
	}
	rows, err := g.readDB.QueryContext(ctx,
		`SELECT doc_id, ref_count, backlink_count, updated_at
		   FROM doc_reference_counts
		  WHERE backlink_count > 0
		  ORDER BY backlink_count DESC, doc_id ASC
		  LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("docgraph: TopReferencedDocs: %w", err)
	}
	defer rows.Close()
	var out []ReferenceCounts
	for rows.Next() {
		var rc ReferenceCounts
		if err := rows.Scan(&rc.DocID, &rc.RefCount, &rc.BacklinkCount, &rc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("docgraph: scan TopReferencedDocs row: %w", err)
		}
		out = append(out, rc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("docgraph: iterate TopReferencedDocs: %w", err)
	}
	return out, nil
}

// selectAffectedTargets returns the distinct non-vedox-scheme targets that
// docID currently points to. It is called BEFORE rewriting doc_references in
// SaveRefs/DeleteRefs so we know which targets need their backlink_count
// refreshed afterwards. vedox-scheme targets are excluded because they refer
// to source-code files, not docs, and intentionally do not get a counts row.
func selectAffectedTargets(tx *sql.Tx, docID string) ([]string, error) {
	rows, err := tx.Query(
		`SELECT DISTINCT target_path
		   FROM doc_references
		  WHERE source_doc_id = ?
		    AND link_type != ?`,
		docID, string(LinkTypeVedoxScheme),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// refreshOutgoingCount sets doc_reference_counts.ref_count for docID to the
// current COUNT(*) of outgoing edges from doc_references and bumps updated_at.
// The row is created if it does not yet exist (UPSERT semantics).
func refreshOutgoingCount(tx *sql.Tx, docID string, now int64) error {
	var n int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM doc_references WHERE source_doc_id = ?`,
		docID,
	).Scan(&n); err != nil {
		return err
	}
	_, err := tx.Exec(
		`INSERT INTO doc_reference_counts (doc_id, ref_count, backlink_count, updated_at)
		 VALUES (?, ?, 0, ?)
		 ON CONFLICT(doc_id) DO UPDATE
		   SET ref_count = excluded.ref_count,
		       updated_at = excluded.updated_at`,
		docID, n, now,
	)
	return err
}

// refreshInboundCount sets doc_reference_counts.backlink_count for target to
// the current COUNT(*) of non-vedox-scheme inbound edges and bumps updated_at.
// The row is created if it does not yet exist.
func refreshInboundCount(tx *sql.Tx, target string, now int64) error {
	if target == "" {
		return nil
	}
	var n int
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM doc_references
		  WHERE target_path = ?
		    AND link_type != ?`,
		target, string(LinkTypeVedoxScheme),
	).Scan(&n); err != nil {
		return err
	}
	_, err := tx.Exec(
		`INSERT INTO doc_reference_counts (doc_id, ref_count, backlink_count, updated_at)
		 VALUES (?, 0, ?, ?)
		 ON CONFLICT(doc_id) DO UPDATE
		   SET backlink_count = excluded.backlink_count,
		       updated_at = excluded.updated_at`,
		target, n, now,
	)
	return err
}

// escapeLikePrefix escapes the LIKE metacharacters '%' and '_' (plus the
// escape char '\' itself) so an arbitrary caller-supplied prefix matches
// literally. Matches the `ESCAPE '\'` clause in GetAllRefsForPrefix.
func escapeLikePrefix(s string) string {
	// Escape the escape character first so we do not double-escape below.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// scanRefs scans a *sql.Rows result set into a []DocRef slice.
func scanRefs(rows *sql.Rows) ([]DocRef, error) {
	var refs []DocRef
	for rows.Next() {
		var r DocRef
		var lt string
		if err := rows.Scan(
			&r.SourcePath,
			&r.TargetPath,
			&lt,
			&r.LineNum,
			&r.AnchorText,
		); err != nil {
			return nil, fmt.Errorf("scan ref row: %w", err)
		}
		r.LinkType = LinkType(lt)
		refs = append(refs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ref rows: %w", err)
	}
	return refs, nil
}
