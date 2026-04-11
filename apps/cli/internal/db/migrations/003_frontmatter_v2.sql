-- 003_frontmatter_v2.sql — Add slug column and drop redundant FTS AI/AU triggers
ALTER TABLE documents ADD COLUMN slug TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_project_slug
    ON documents(project, slug) WHERE slug IS NOT NULL;

-- The AI/AU triggers from 001_initial.sql insert empty FTS content rows that
-- UpsertDoc immediately deletes and replaces. On the bulk-import path this
-- caused a bug; UpsertDoc already owns FTS writes explicitly, so the triggers
-- are redundant. The AD (delete) trigger is still needed and is left intact.
DROP TRIGGER IF EXISTS documents_ai;
DROP TRIGGER IF EXISTS documents_au;
