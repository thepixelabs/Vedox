-- Migration 007: denormalised reference counts for the doc graph.
--
-- This table is a write-time aggregate of doc_references. It exists to
-- answer two hot queries cheaply:
--
--   1. "How many outgoing/incoming links does <doc> have?" — single PK lookup
--      instead of two COUNT(*) scans of doc_references.
--   2. "What are the most-referenced docs?" — single index range scan instead
--      of GROUP BY target_path over the entire edges table.
--
-- Maintenance contract (enforced by docgraph.GraphStore.SaveRefs/DeleteRefs):
--   * Every call that mutates doc_references MUST also update the affected
--     rows here, in the SAME transaction. There is no trigger -- we keep the
--     update logic in Go so it can recompute correctly when the same target
--     appears multiple times in a single SaveRefs payload.
--   * doc_id is the workspace-relative slash path (identical to documents.id
--     for source docs and to target_path for inbound counts). vedox:// URIs
--     are intentionally excluded -- they target source-code files, not docs,
--     and would skew the "most-referenced doc" rankings.
--   * A row is created on first touch (either as source of an outgoing edge
--     or as target of an incoming edge) and persists thereafter. Rows are
--     never deleted automatically; ref_count/backlink_count drop to 0 when
--     the last edge is removed.
--
-- Indexes:
--   * PRIMARY KEY on doc_id covers GetReferenceCounts.
--   * idx_doc_ref_counts_backlinks_desc covers TopReferencedDocs without a
--     filesort. The DESC matters: SQLite's B-tree can walk the index in
--     reverse, so ORDER BY backlink_count DESC LIMIT N is O(N) not O(rows).
--
-- schema_version: 7

CREATE TABLE IF NOT EXISTS doc_reference_counts (
    doc_id          TEXT    PRIMARY KEY,
    ref_count       INTEGER NOT NULL DEFAULT 0,
    backlink_count  INTEGER NOT NULL DEFAULT 0,
    updated_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_doc_ref_counts_backlinks_desc
    ON doc_reference_counts(backlink_count DESC);

-- Backfill from existing doc_references rows. Idempotent at migration time:
-- the migration runner only applies versions newer than the current
-- schema_version, so this INSERT runs exactly once on upgrade.
--
-- The inner UNION ALL covers both sides of every edge:
--   * sources: every distinct source_doc_id with its outgoing count
--   * targets: every distinct target_path (excluding vedox-scheme) with its
--     inbound count
--
-- After UNION ALL the same doc_id may appear twice (once as source, once as
-- target); the outer GROUP BY collapses both rows into the final pair.
INSERT INTO doc_reference_counts (doc_id, ref_count, backlink_count, updated_at)
SELECT
    doc_id,
    SUM(rc) AS ref_count,
    SUM(bc) AS backlink_count,
    CAST(strftime('%s','now') AS INTEGER) AS updated_at
FROM (
    SELECT source_doc_id AS doc_id, COUNT(*) AS rc, 0 AS bc
      FROM doc_references
     GROUP BY source_doc_id
    UNION ALL
    SELECT target_path AS doc_id, 0 AS rc, COUNT(*) AS bc
      FROM doc_references
     WHERE link_type != 'vedox-scheme'
     GROUP BY target_path
) AS edge_counts
GROUP BY doc_id
ON CONFLICT(doc_id) DO NOTHING;
