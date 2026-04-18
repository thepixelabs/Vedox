package db

// Doc is a single indexed document. It mirrors the row schema in the
// `documents` table plus the derived full-text body used by FTS5.
//
// The Markdown tree on disk is the source of truth. SQLite is a
// rebuildable cache: deleting the .vedox/index.db file must never
// cause data loss because `Reindex` can regenerate every row from the
// workspace tree.
type Doc struct {
	ID             string   // relative path from workspace root
	Project        string
	Slug           string // URL-safe identifier, unique per project
	Title          string
	Type           string // adr | api-reference | runbook | readme | how-to
	Status         string // draft | review | published | deprecated
	Date           string // ISO 8601
	Tags           []string
	Author         string
	ContentHash    string // SHA256 hex of raw file contents
	ModTime        string // ISO 8601
	Size           int64
	WordCount      int    // len(strings.Fields(Body)); 0 until first UpsertDoc (migration 004)
	RawFrontmatter string // JSON-encoded frontmatter blob
	Body           string // plain text body used for FTS; not stored in `documents`
}

// SearchResult is a single hit returned by `Search`.
type SearchResult struct {
	ID      string
	Project string
	Title   string
	Type    string
	Status  string
	Snippet string
	Score   float64 // BM25; lower is better in SQLite FTS5
}

// DocStore is the contract the reindex routine walks over. The file
// layer (LocalAdapter) implements this; the db package depends only
// on the interface so tests can fake it without touching disk.
type DocStore interface {
	WalkDocs(workspaceRoot string, fn func(*Doc) error) error
}
