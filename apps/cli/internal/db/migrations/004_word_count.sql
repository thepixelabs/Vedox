-- 004_word_count.sql — Add word_count to documents.
--
-- Rationale (from FINAL_PLAN.md R9): analytics dashboard needs word count
-- trends per-doc over time. Computing word count inline during UpsertDoc
-- via strings.Fields(doc.Body) is O(len(body)) and runs on the write path,
-- which is acceptable — the writer goroutine already does comparable work
-- for FTS population.
--
-- Backfill: existing rows are set to 0. The actual count is populated on
-- the next reindex or document open (whichever triggers UpsertDoc first).
-- This is safe because word_count is a derived, rebuildable metric; 0 is
-- a correct representation of "not yet measured" until the doc is touched.
--
-- DEFAULT 0 is intentional: new rows from older writer code paths that
-- do not yet provide a word count remain valid rather than NULL.

ALTER TABLE documents ADD COLUMN word_count INTEGER NOT NULL DEFAULT 0;
