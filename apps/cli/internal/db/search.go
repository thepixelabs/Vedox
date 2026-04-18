package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

// Search runs a BM25-ranked FTS5 query. If project is non-empty it is
// used as an equality filter against documents.project. Results are
// ordered by FTS5 `rank` (lower score = better match).
//
// Query sanitisation: FTS5 has its own mini query syntax (AND/OR/NOT,
// quoting, column filters). Users will paste arbitrary strings, so we
// wrap the input in a single MATCH expression that quotes each token
// and joins them with implicit AND. Callers who want the raw syntax
// can pre-quote.
// SearchFilters are optional structured filters applied alongside the FTS5
// MATCH clause. They are intentionally NOT routed through FTS5 column syntax
// (e.g. `type:adr`) because the unicode61 tokenizer in this SQLite build
// treats `:` as punctuation, and sanitizeFTSQuery strips it by design.
// Filters must be separate SQL WHERE clauses.
type SearchFilters struct {
	Type    string
	Status  string
	Project string
	Tag     string
}

func (s *Store) Search(ctx context.Context, query string, filters SearchFilters) ([]*SearchResult, error) {
	q := sanitizeFTSQuery(query)
	if q == "" {
		return nil, nil
	}
	var (
		rows *sql.Rows
		err  error
	)
	sqlStr := `
		SELECT d.id, d.project, d.title, d.type, d.status,
		       snippet(documents_fts, 1, '<mark>', '</mark>', ' … ', 16) AS snip,
		       bm25(documents_fts) AS score
		FROM documents_fts
		JOIN documents d ON d.rowid = documents_fts.rowid
		WHERE documents_fts MATCH ?`
	args := []any{q}
	if filters.Project != "" {
		sqlStr += ` AND d.project = ?`
		args = append(args, filters.Project)
	}
	if filters.Type != "" {
		sqlStr += ` AND d.type = ?`
		args = append(args, filters.Type)
	}
	if filters.Status != "" {
		sqlStr += ` AND d.status = ?`
		args = append(args, filters.Status)
	}
	if filters.Tag != "" {
		// Tags are stored as a JSON-marshalled array of strings; json_each
		// expands them so we can test for exact membership without LIKE.
		sqlStr += ` AND EXISTS (SELECT 1 FROM json_each(d.tags) WHERE value = ?)`
		args = append(args, filters.Tag)
	}
	sqlStr += ` ORDER BY score ASC LIMIT 100`

	rows, err = s.readDB.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("vedox: fts query: %w", err)
	}
	defer rows.Close()

	var out []*SearchResult
	for rows.Next() {
		r := &SearchResult{}
		if err := rows.Scan(&r.ID, &r.Project, &r.Title, &r.Type, &r.Status, &r.Snippet, &r.Score); err != nil {
			return nil, fmt.Errorf("vedox: scan fts row: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// GetDoc returns the stored metadata row for a given path, or nil if
// not present. Body is not persisted and will be empty.
func (s *Store) GetDoc(ctx context.Context, id string) (*Doc, error) {
	row := s.readDB.QueryRowContext(ctx,
		`SELECT id, project, slug, title, type, status, date, tags, author,
		        content_hash, mod_time, size, word_count, raw_frontmatter
		 FROM documents WHERE id = ?`, id)
	d := &Doc{}
	var tags sql.NullString
	var slug sql.NullString
	err := row.Scan(&d.ID, &d.Project, &slug, &d.Title, &d.Type, &d.Status, &d.Date,
		&tags, &d.Author, &d.ContentHash, &d.ModTime, &d.Size, &d.WordCount, &d.RawFrontmatter)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if slug.Valid {
		d.Slug = slug.String
	}
	if tags.Valid && tags.String != "" {
		// Stored as a JSON array; we return the raw form in Tags by
		// splitting, but callers that need strict parsing should use
		// encoding/json on d.RawFrontmatter.
		_ = tags // tags are surfaced via the FTS layer; not parsed here
	}
	return d, nil
}

// CountDocs returns the number of rows currently in the documents
// table. Useful for reindex assertions and tests.
func (s *Store) CountDocs(ctx context.Context) (int, error) {
	var n int
	err := s.readDB.QueryRowContext(ctx, `SELECT count(*) FROM documents`).Scan(&n)
	return n, err
}

// sanitizeFTSQuery turns a user string into an FTS5 MATCH expression
// that treats each whitespace-separated token as a required term.
//
// We deliberately drop FTS5 operators (NOT, AND, OR, NEAR, column filters)
// and any non-alphanumeric punctuation so users never accidentally trigger
// FTS5 query syntax errors. The FTS5 unicode61 tokenizer treats most
// punctuation (including hyphens) as token separators, so a query like
// "debounce-sentinel-99" must be split into three tokens to match what
// the indexer stored — wrapping it in double quotes as a phrase produces
// a SQLite logic error.
//
// Algorithm:
//  1. Replace every non-alphanumeric (Unicode-aware) rune with a space.
//  2. Split on whitespace into clean tokens.
//  3. Wrap each token in double quotes (FTS5 phrase syntax) — at this point
//     the token contains no special characters so quoting is just a safety
//     belt against any tokens that happen to collide with FTS5 keywords.
//  4. Join with spaces (implicit AND).
func sanitizeFTSQuery(in string) string {
	in = strings.TrimSpace(in)
	if in == "" {
		return ""
	}
	// Replace non-alphanumeric runes with spaces. Keeps unicode letters and
	// digits which the unicode61 tokenizer also indexes.
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return ' '
	}, in)
	fields := strings.Fields(cleaned)
	if len(fields) == 0 {
		return ""
	}
	quoted := make([]string, 0, len(fields))
	for _, f := range fields {
		quoted = append(quoted, `"`+f+`"`)
	}
	return strings.Join(quoted, " ")
}
