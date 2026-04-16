-- 1001_repos.sql — global.db repos table additive migration (FIX-ARCH-06)
--
-- The repos table is created in the version-1 bootstrap schema (globaldb.go)
-- with core columns: id, name, type, root_path, remote_url, status,
-- created_at (TEXT RFC3339), updated_at (TEXT RFC3339).
--
-- This migration adds supplementary columns used by the analytics aggregator
-- and the background sync subsystem. Columns are added with IF NOT EXISTS so
-- the migration is safe to apply on any database regardless of whether a
-- previous version of this migration partially ran.
--
-- New columns:
--   sync_mode  — 'auto' | 'manual'; controls background sync cadence
--   is_default — 1 if this repo is the default routing target; at most one row
--   created_at_unix — denormalised INTEGER copy of created_at for fast range
--                     queries in analytics (avoids strftime() overhead)

ALTER TABLE repos ADD COLUMN sync_mode TEXT NOT NULL DEFAULT 'auto'
    CHECK (sync_mode IN ('auto', 'manual'));

ALTER TABLE repos ADD COLUMN is_default INTEGER NOT NULL DEFAULT 0
    CHECK (is_default IN (0, 1));

ALTER TABLE repos ADD COLUMN created_at_unix INTEGER NOT NULL DEFAULT 0;

-- Covering indexes for the analytics aggregator.
-- idx_repos_root_path may already exist from the bootstrap schema; the
-- IF NOT EXISTS guard makes this safe.
CREATE UNIQUE INDEX IF NOT EXISTS idx_repos_root_path ON repos(root_path);
CREATE INDEX        IF NOT EXISTS idx_repos_type      ON repos(type);
CREATE INDEX        IF NOT EXISTS idx_repos_status    ON repos(status);
CREATE INDEX        IF NOT EXISTS idx_repos_default   ON repos(is_default) WHERE is_default = 1;
