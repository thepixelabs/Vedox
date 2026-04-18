-- Migration 006: per-workspace events table for the analytics pipeline (WS-K).
--
-- The Collector writes raw events here; the Aggregator tail-scans them
-- (WHERE id > last_seen_id) and rolls them up into GlobalDB::events_daily.
--
-- kind follows the subject.verb taxonomy from analytics.EventKind* constants.
-- properties is a JSON blob (may be NULL for events with no extra attributes).
-- session_id is the per-daemon-start UUID threaded through every event emitter.

CREATE TABLE IF NOT EXISTS events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    kind        TEXT NOT NULL,
    timestamp   TEXT NOT NULL,   -- RFC3339
    session_id  TEXT NOT NULL,
    properties  TEXT             -- JSON or NULL
);

CREATE INDEX IF NOT EXISTS idx_events_kind      ON events(kind);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);
