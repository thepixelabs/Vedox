-- Migration 005: doc reference graph
--
-- Stores outgoing reference edges extracted from Markdown documents.
-- One row per link occurrence; multiple rows for the same source→target
-- pair are allowed when the same link appears on multiple lines.
--
-- Design notes:
--   • source_doc_id references documents(id) with ON DELETE CASCADE so
--     deleting a document auto-removes its outgoing edges.
--   • target_path stores the raw resolved target (workspace-relative slash
--     path for md-link/wikilink/frontmatter; vedox:// URI for vedox-scheme).
--     It is NOT a foreign key to documents — the target may not yet exist
--     (forward reference) or may be a source-code file, not a doc.
--   • Broken-link detection is a LEFT JOIN / NOT EXISTS against documents at
--     query time, not a stored flag, so it stays correct as the doc set changes.
--   • anchor_text stores the in-doc heading anchor or vedox:// line range
--     (e.g. "L10-L25"); for md-link it is the display text.
--
-- schema_version: 5

CREATE TABLE IF NOT EXISTS doc_references (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    source_doc_id TEXT    NOT NULL,
    target_path   TEXT    NOT NULL,
    link_type     TEXT    NOT NULL CHECK (link_type IN ('md-link','wikilink','frontmatter','vedox-scheme')),
    line_num      INTEGER NOT NULL DEFAULT 0,
    anchor_text   TEXT    NOT NULL DEFAULT '',
    created_at    TEXT    NOT NULL,
    FOREIGN KEY (source_doc_id) REFERENCES documents(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_doc_refs_source ON doc_references(source_doc_id);
CREATE INDEX IF NOT EXISTS idx_doc_refs_target ON doc_references(target_path);
CREATE INDEX IF NOT EXISTS idx_doc_refs_kind   ON doc_references(link_type);

