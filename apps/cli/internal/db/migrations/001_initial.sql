-- 001_initial.sql — Vedox metadata store initial schema
CREATE TABLE IF NOT EXISTS schema_version (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS documents (
    id              TEXT PRIMARY KEY,
    project         TEXT NOT NULL,
    title           TEXT,
    type            TEXT,
    status          TEXT,
    date            TEXT,
    tags            TEXT,
    author          TEXT,
    content_hash    TEXT NOT NULL,
    mod_time        TEXT NOT NULL,
    size            INTEGER NOT NULL,
    raw_frontmatter TEXT
);

CREATE INDEX IF NOT EXISTS idx_documents_project ON documents(project);
CREATE INDEX IF NOT EXISTS idx_documents_type    ON documents(type);
CREATE INDEX IF NOT EXISTS idx_documents_status  ON documents(status);

-- documents_fts is an INTERNAL-content FTS5 table (not external content). The
-- earlier design used content='documents' / content_rowid='rowid', but that
-- requires every FTS5-declared column to also exist on the parent `documents`
-- table — and the body lives only in FTS5, never in `documents`. With the
-- mismatched schema, FTS5 issued
--   SELECT rowid, title, content, tags FROM documents WHERE rowid = ?
-- whenever it needed the original text (e.g. inside snippet()), which failed
-- with "no such column: content" → surfaced to callers as "SQL logic error".
--
-- Switching to internal content means FTS5 owns the title/content/tags it is
-- given via INSERT. The triggers below keep FTS5 in sync with `documents`
-- using ordinary DELETE/INSERT statements (the FTS5 'delete' / 'delete-all'
-- commands are only valid on external/contentless tables). The FTS5 rowid is
-- always set to the `documents` rowid by the trigger so callers can JOIN the
-- two tables on rowid.
CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
    title, content, tags,
    tokenize='porter unicode61'
);

CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN
    INSERT INTO documents_fts(rowid, title, content, tags)
    VALUES (new.rowid, new.title, '', new.tags);
END;

CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN
    DELETE FROM documents_fts WHERE rowid = old.rowid;
END;

CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN
    DELETE FROM documents_fts WHERE rowid = old.rowid;
    INSERT INTO documents_fts(rowid, title, content, tags)
    VALUES (new.rowid, new.title, '', new.tags);
END;
