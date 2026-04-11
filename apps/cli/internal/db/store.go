package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// IndexDBRelPath is the workspace-relative path of the SQLite file.
// Callers should never hard-code this elsewhere.
const IndexDBRelPath = ".vedox/index.db"

// Store is the public handle to the metadata / FTS index. It owns a
// write connection fronted by the single-writer goroutine and a
// separate read-only connection for concurrent reads (WAL makes this
// safe). Close must be called at shutdown.
type Store struct {
	workspaceRoot string
	dbPath        string

	writeDB *sql.DB // used exclusively by the writer goroutine
	readDB  *sql.DB // reader pool; WAL lets readers run without blocking the writer
	writer  *writer
}

// Options controls Store construction. A nil Logger is valid (no-op).
type Options struct {
	WorkspaceRoot string
	Logger        func(string)
}

// Open creates .vedox/ if needed, opens the SQLite file in WAL mode,
// runs pending migrations, and returns a ready Store.
//
// Two *sql.DB handles are used because modernc.org/sqlite (like every
// other SQLite binding) serialises on a single handle. Splitting read
// and write handles lets the reader pool scale with cores while the
// writer remains a single goroutine.
func Open(opts Options) (*Store, error) {
	if opts.WorkspaceRoot == "" {
		return nil, fmt.Errorf("vedox: Open requires WorkspaceRoot")
	}
	dbDir := filepath.Join(opts.WorkspaceRoot, ".vedox")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, fmt.Errorf("vedox: create %s: %w", dbDir, err)
	}
	dbPath := filepath.Join(opts.WorkspaceRoot, IndexDBRelPath)

	// Writer connection: serialised through the writer goroutine so
	// a single open handle is sufficient. `_busy_timeout` gives the
	// writer a grace window if a long reader is active.
	writeDSN := "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)"
	writeDB, err := sql.Open("sqlite", writeDSN)
	if err != nil {
		return nil, fmt.Errorf("vedox: open write db: %w", err)
	}
	writeDB.SetMaxOpenConns(1)
	if err := writeDB.Ping(); err != nil {
		_ = writeDB.Close()
		return nil, fmt.Errorf("vedox: ping write db: %w", err)
	}

	if err := runMigrations(writeDB, opts.Logger); err != nil {
		_ = writeDB.Close()
		return nil, err
	}

	// Reader pool: read-only mode is enforced via query_only pragma
	// so stray writes fail loudly instead of racing the writer.
	readDSN := "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=query_only(ON)&mode=ro"
	readDB, err := sql.Open("sqlite", readDSN)
	if err != nil {
		_ = writeDB.Close()
		return nil, fmt.Errorf("vedox: open read db: %w", err)
	}
	readDB.SetMaxOpenConns(4)
	if err := readDB.Ping(); err != nil {
		_ = writeDB.Close()
		_ = readDB.Close()
		return nil, fmt.Errorf("vedox: ping read db: %w", err)
	}

	s := &Store{
		workspaceRoot: opts.WorkspaceRoot,
		dbPath:        dbPath,
		writeDB:       writeDB,
		readDB:        readDB,
		writer:        newWriter(writeDB),
	}
	return s, nil
}

// Close stops the writer goroutine and releases DB handles.
func (s *Store) Close() error {
	if s.writer != nil {
		s.writer.close()
	}
	var firstErr error
	if s.writeDB != nil {
		if err := s.writeDB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if s.readDB != nil {
		if err := s.readDB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Path returns the absolute path of the SQLite file.
func (s *Store) Path() string { return s.dbPath }

// ------------------------------------------------------------------
// Write operations — all route through the single writer goroutine.
// ------------------------------------------------------------------

// UpsertDoc inserts or updates a document row and rewrites the matching
// FTS row so the body is searchable.
//
// `documents_fts` is an internal-content FTS5 table; the AI/AU triggers on
// `documents` keep it loosely in sync (title + tags), but because the body
// is not a column on `documents` we still have to write it explicitly. We
// do this by deleting the FTS row the trigger just produced and inserting a
// fresh one whose `content` column is the full body. INSERT OR REPLACE
// would also work but a delete + insert is unambiguous regardless of which
// trigger (AI or AU) ran first, and avoids relying on FTS5's UPDATE
// semantics which surprised us already.
func (s *Store) UpsertDoc(ctx context.Context, doc *Doc) error {
	if doc == nil {
		return fmt.Errorf("vedox: UpsertDoc: nil doc")
	}
	if doc.ID == "" {
		return fmt.Errorf("vedox: UpsertDoc: empty doc.ID")
	}
	tagsJSON, err := json.Marshal(doc.Tags)
	if err != nil {
		return fmt.Errorf("vedox: marshal tags: %w", err)
	}
	return s.writer.submit(ctx, func(tx *sql.Tx) error {
		var slugArg any
		if doc.Slug != "" {
			slugArg = doc.Slug
		} else {
			slugArg = nil
		}
		_, err := tx.Exec(
			`INSERT INTO documents(
				id, project, slug, title, type, status, date, tags, author,
				content_hash, mod_time, size, raw_frontmatter
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				project         = excluded.project,
				slug            = excluded.slug,
				title           = excluded.title,
				type            = excluded.type,
				status          = excluded.status,
				date            = excluded.date,
				tags            = excluded.tags,
				author          = excluded.author,
				content_hash    = excluded.content_hash,
				mod_time        = excluded.mod_time,
				size            = excluded.size,
				raw_frontmatter = excluded.raw_frontmatter`,
			doc.ID, doc.Project, slugArg, doc.Title, doc.Type, doc.Status, doc.Date,
			string(tagsJSON), doc.Author, doc.ContentHash, doc.ModTime,
			doc.Size, doc.RawFrontmatter,
		)
		if err != nil {
			return fmt.Errorf("upsert documents: %w", err)
		}
		// Look up the rowid SQLite assigned to (or already had for) this id.
		// We pin the FTS row to the same rowid so callers can JOIN documents
		// to documents_fts on rowid.
		var rowid int64
		if err := tx.QueryRow(`SELECT rowid FROM documents WHERE id = ?`, doc.ID).Scan(&rowid); err != nil {
			return fmt.Errorf("lookup rowid: %w", err)
		}
		// The AI/AU trigger has already populated documents_fts with empty
		// content. Replace that row with one that includes the full body so
		// snippet() and MATCH against the body work.
		if _, err := tx.Exec(
			`DELETE FROM documents_fts WHERE rowid = ?`, rowid,
		); err != nil {
			return fmt.Errorf("fts delete prior: %w", err)
		}
		if _, err := tx.Exec(
			`INSERT INTO documents_fts(rowid, title, content, tags) VALUES (?, ?, ?, ?)`,
			rowid, doc.Title, doc.Body, string(tagsJSON),
		); err != nil {
			return fmt.Errorf("fts insert: %w", err)
		}
		return nil
	})
}

// DeleteDoc removes a document by its workspace-relative path.
func (s *Store) DeleteDoc(ctx context.Context, path string) error {
	if path == "" {
		return fmt.Errorf("vedox: DeleteDoc: empty path")
	}
	return s.writer.submit(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM documents WHERE id = ?`, path)
		return err
	})
}

// truncate removes every row from documents and the FTS index.
// Internal to the package; Reindex is the exported entry point.
//
// `DELETE FROM documents` already fires the AD trigger which clears the
// matching FTS rows; the explicit FTS delete that follows is a
// belt-and-suspenders sweep for any orphaned rows from earlier bugs.
func (s *Store) truncate(ctx context.Context) error {
	return s.writer.submit(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM documents`); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM documents_fts`); err != nil {
			return err
		}
		return nil
	})
}
