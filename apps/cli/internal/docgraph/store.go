// Package docgraph — store.go provides the SQLite persistence layer for the
// doc reference graph. All writes go through the single-writer goroutine
// inherited from db.Store; reads use the shared read-only connection pool.
//
// Schema is created by migration 005_doc_graph.sql. This file does not run
// migrations itself — callers must open a db.Store (which runs all pending
// migrations) before constructing a GraphStore.
package docgraph

import (
	"context"
	"database/sql"
	"fmt"
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
// The operation is a single transaction: delete old edges, insert new ones,
// then attempt to heal any previously-broken inbound edges that now resolve.
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
// document is deleted from the index.
func (g *GraphStore) DeleteRefs(ctx context.Context, docID string) error {
	if docID == "" {
		return fmt.Errorf("docgraph: DeleteRefs: empty docID")
	}
	return g.submitFn(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM doc_references WHERE source_doc_id = ?`, docID)
		return err
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
	// so this query is index-assisted even on large graphs. The '%' wildcard is
	// appended in Go to keep the query string a string literal.
	rows, err := g.readDB.QueryContext(ctx,
		`SELECT source_doc_id, target_path, link_type, line_num, anchor_text
		   FROM doc_references
		  WHERE source_doc_id LIKE ? ESCAPE '\'
		  ORDER BY source_doc_id, line_num, id`,
		prefix+"%",
	)
	if err != nil {
		return nil, fmt.Errorf("docgraph: GetAllRefsForPrefix %q: %w", prefix, err)
	}
	defer rows.Close()
	return scanRefs(rows)
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
