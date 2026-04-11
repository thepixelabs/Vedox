-- 002_tasks.sql — Per-project task backlog
CREATE TABLE tasks (
    id         TEXT PRIMARY KEY,
    project    TEXT NOT NULL,
    title      TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'todo',  -- todo | in-progress | done
    position   REAL NOT NULL DEFAULT 0,       -- fractional indexing for drag-to-reorder
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX tasks_project_idx ON tasks(project, position);
