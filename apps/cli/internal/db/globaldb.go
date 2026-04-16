package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed global_migrations/*.sql
var globalMigrationsFS embed.FS

// GlobalDBPath is the canonical path of the global SQLite file.
// Callers should use os.UserHomeDir() to resolve "~".
const GlobalDBPath = ".vedox/global.db"

// GlobalDB is the handle for the cross-workspace global database
// (~/.vedox/global.db). It holds the repo registry, agent install
// state, and daily event roll-ups that span workspaces.
//
// Like the workspace Store, GlobalDB uses a single-writer goroutine for
// all writes and a separate read-only pool for concurrent reads.
// Each DB file has its own writer funnel; no cross-DB transactions are
// ever attempted (FINAL_PLAN.md dual-DB split ruling).
type GlobalDB struct {
	path    string
	writeDB *sql.DB
	readDB  *sql.DB
	writer  *writer
}

// OpenGlobalDB opens (and migrates) the global database at the supplied
// path. The directory is created if it does not exist.
//
// Typical callers pass filepath.Join(os.UserHomeDir(), GlobalDBPath).
func OpenGlobalDB(path string) (*GlobalDB, error) {
	if path == "" {
		return nil, fmt.Errorf("vedox/globaldb: path must not be empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("vedox/globaldb: create dir %s: %w", filepath.Dir(path), err)
	}

	// Writer connection: single goroutine owns it exclusively.
	writeDSN := "file:" + path +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=synchronous(NORMAL)"
	writeDB, err := sql.Open("sqlite", writeDSN)
	if err != nil {
		return nil, fmt.Errorf("vedox/globaldb: open write db: %w", err)
	}
	writeDB.SetMaxOpenConns(1)
	if err := writeDB.Ping(); err != nil {
		_ = writeDB.Close()
		return nil, fmt.Errorf("vedox/globaldb: ping write db: %w", err)
	}

	if err := globalMigrate(writeDB); err != nil {
		_ = writeDB.Close()
		return nil, err
	}

	// Reader pool: read-only, concurrent.
	readDSN := "file:" + path +
		"?_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=query_only(ON)" +
		"&mode=ro"
	readDB, err := sql.Open("sqlite", readDSN)
	if err != nil {
		_ = writeDB.Close()
		return nil, fmt.Errorf("vedox/globaldb: open read db: %w", err)
	}
	readDB.SetMaxOpenConns(4)
	if err := readDB.Ping(); err != nil {
		_ = writeDB.Close()
		_ = readDB.Close()
		return nil, fmt.Errorf("vedox/globaldb: ping read db: %w", err)
	}

	return &GlobalDB{
		path:    path,
		writeDB: writeDB,
		readDB:  readDB,
		writer:  newWriter(writeDB),
	}, nil
}

// Close stops the writer goroutine and releases all DB handles.
func (g *GlobalDB) Close() error {
	if g.writer != nil {
		g.writer.close()
	}
	var firstErr error
	if g.writeDB != nil {
		if err := g.writeDB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if g.readDB != nil {
		if err := g.readDB.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Path returns the absolute path of the global SQLite file.
func (g *GlobalDB) Path() string { return g.path }

// ---------------------------------------------------------------------------
// Schema — global.db tables
//
// Three tables at launch. All DDL is idempotent (IF NOT EXISTS) so
// globalMigrate can run on every startup without risk.
//
// repos            — multi-repo registry (one row per registered repo)
// agent_installs   — per-provider agent install state and health
// events_daily     — daily aggregate roll-ups from per-workspace events tables
//
// Naming follows the subject.verb event convention ratified in FINAL_PLAN.md
// OQ-K resolution: event kind strings are dot-separated, lowercase.
// ---------------------------------------------------------------------------

const globalSchema = `
-- schema_version table mirrors the workspace DB convention so the same
-- currentSchemaVersion helper can be reused against this connection.
CREATE TABLE IF NOT EXISTS schema_version (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);

-- repos holds one row per documentation repository registered with Vedox.
-- A repo may be created via gh CLI or manually registered from an existing
-- path. The type column drives routing decisions in the Doc Agent:
--   'private'  — personal/private documentation (autosync ON by default)
--   'public'   — project-scoped, intended for public audiences
--   'inbox'    — bare-local fallback (no remote); created during onboarding
--                step-2 skip (OQ-E default c, FINAL_PLAN.md line 33)
CREATE TABLE IF NOT EXISTS repos (
    id         TEXT PRIMARY KEY,           -- UUID v4
    name       TEXT NOT NULL,              -- human display name
    type       TEXT NOT NULL               -- 'private' | 'public' | 'inbox'
                   CHECK (type IN ('private', 'public', 'inbox')),
    root_path  TEXT NOT NULL,              -- absolute path on this machine
    remote_url TEXT,                       -- NULL for inbox type
    status     TEXT NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active', 'archived', 'error')),
    created_at TEXT NOT NULL,              -- RFC3339
    updated_at TEXT NOT NULL               -- RFC3339
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_repos_root_path ON repos(root_path);
CREATE INDEX        IF NOT EXISTS idx_repos_type      ON repos(type);
CREATE INDEX        IF NOT EXISTS idx_repos_status    ON repos(status);

-- agent_installs tracks which AI provider has a Vedox Doc Agent installed
-- and its last known health. repo_id is nullable: a NULL means the install
-- is global (applies to all repos); a non-NULL value scopes it to one repo.
CREATE TABLE IF NOT EXISTS agent_installs (
    id           TEXT PRIMARY KEY,         -- UUID v4
    provider     TEXT NOT NULL             -- 'claude-code' | 'codex' | 'copilot' | 'gemini'
                     CHECK (provider IN ('claude-code', 'codex', 'copilot', 'gemini')),
    version      TEXT NOT NULL,            -- semver or commit SHA of the installed pack
    install_date TEXT NOT NULL,            -- RFC3339
    health_status TEXT NOT NULL DEFAULT 'unknown'
                     CHECK (health_status IN ('healthy', 'degraded', 'failed', 'unknown')),
    repo_id      TEXT,                     -- NULL = global; FK to repos
    FOREIGN KEY (repo_id) REFERENCES repos(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_agent_installs_provider    ON agent_installs(provider);
CREATE INDEX IF NOT EXISTS idx_agent_installs_repo_id     ON agent_installs(repo_id) WHERE repo_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_agent_installs_health      ON agent_installs(health_status);

-- events_daily holds daily aggregate roll-ups copied from per-workspace
-- events tables by the analytics aggregator goroutine. This allows
-- cross-repo analytics ("total agent.triggered events in the last 30 days
-- across all repos") without N round-trips to each workspace DB.
--
-- kind uses the same subject.verb taxonomy as the workspace events table
-- (FINAL_PLAN.md OQ-K resolution, WS-K).
-- The (date, kind) pair is unique: the aggregator UPSERTs so re-runs are
-- idempotent.
CREATE TABLE IF NOT EXISTS events_daily (
    date  TEXT NOT NULL,                   -- ISO 8601 date: YYYY-MM-DD
    kind  TEXT NOT NULL,                   -- subject.verb, e.g. 'document.published'
    count INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (date, kind)
);

CREATE INDEX IF NOT EXISTS idx_events_daily_kind ON events_daily(kind, date DESC);

-- analytics_cache holds precomputed summary values written by the background
-- aggregator (WS-K). A single row keyed on 'summary' is UPSERTED every 60s.
-- The GET /api/analytics/summary handler reads from this cache so the HTTP
-- response path never does aggregate SQL directly.
--
-- The aggregator sets has_run=1 after the first successful aggregation cycle;
-- the API handler reflects this as pipeline_ready=true in the JSON response.
--
-- docs_per_project is a JSON object mapping project name to doc count.
-- Stored as TEXT rather than a normalised table to avoid a read-time JOIN on
-- the hot path (the summary endpoint is called on every editor load).
CREATE TABLE IF NOT EXISTS analytics_cache (
    key                  TEXT PRIMARY KEY,   -- always 'summary' for now
    has_run              INTEGER NOT NULL DEFAULT 0,
    total_docs           INTEGER NOT NULL DEFAULT 0,
    docs_per_project     TEXT    NOT NULL DEFAULT '{}', -- JSON
    change_velocity_7d   INTEGER NOT NULL DEFAULT 0,
    change_velocity_30d  INTEGER NOT NULL DEFAULT 0,
    updated_at           TEXT    NOT NULL               -- RFC3339
);
`

// globalMigrate applies the global schema in two phases:
//
//  1. Bootstrap (version 1): the inline globalSchema DDL is applied once via
//     INSERT OR IGNORE on schema_version — this is idempotent on every open.
//
//  2. Versioned (version >= 1001): SQL files embedded from global_migrations/
//     are applied in ascending numeric order using the same framework as the
//     workspace migration runner. This allows incremental global DB changes
//     without editing the inline bootstrap SQL.
func globalMigrate(db *sql.DB) error {
	// Phase 1: bootstrap.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("vedox/globaldb: begin schema tx: %w", err)
	}
	if _, err := tx.Exec(globalSchema); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("vedox/globaldb: apply schema: %w", err)
	}
	// Record version 1 if not already present. INSERT OR IGNORE is safe
	// here because version 1 is the only version in this bootstrap path.
	if _, err := tx.Exec(
		`INSERT OR IGNORE INTO schema_version(version, applied_at) VALUES (1, ?)`,
		time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("vedox/globaldb: record schema_version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("vedox/globaldb: commit schema: %w", err)
	}

	// Phase 2: versioned incremental migrations from global_migrations/*.sql.
	return runGlobalMigrations(db)
}

// loadGlobalMigrations reads the embedded global_migrations directory and
// returns migrations in ascending version order.  File names must follow the
// NNN_description.sql convention (same as workspace migrations).
func loadGlobalMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(globalMigrationsFS, "global_migrations")
	if err != nil {
		return nil, fmt.Errorf("vedox/globaldb: read embedded global_migrations: %w", err)
	}
	var out []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("vedox/globaldb: malformed global migration name %q", e.Name())
		}
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("vedox/globaldb: global migration %q has non-numeric prefix: %w", e.Name(), err)
		}
		b, err := fs.ReadFile(globalMigrationsFS, "global_migrations/"+e.Name())
		if err != nil {
			return nil, fmt.Errorf("vedox/globaldb: read global migration %q: %w", e.Name(), err)
		}
		out = append(out, migration{version: v, name: e.Name(), sql: string(b)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

// runGlobalMigrations applies pending global_migrations SQL files that have
// not yet been recorded in schema_version.  Each migration runs in its own
// transaction (consistent with the workspace migration runner).
func runGlobalMigrations(db *sql.DB) error {
	migs, err := loadGlobalMigrations()
	if err != nil {
		return err
	}
	current, err := currentSchemaVersion(db)
	if err != nil {
		return fmt.Errorf("vedox/globaldb: read schema_version: %w", err)
	}
	for _, m := range migs {
		if m.version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("vedox/globaldb: begin tx for global migration %s: %w", m.name, err)
		}
		if _, err := tx.Exec(m.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("vedox/globaldb: apply global migration %s: %w", m.name, err)
		}
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO schema_version(version, applied_at) VALUES (?, ?)`,
			m.version, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("vedox/globaldb: record schema_version for %s: %w", m.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("vedox/globaldb: commit global migration %s: %w", m.name, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Repo CRUD
// ---------------------------------------------------------------------------

// Repo is a documentation repository registered with Vedox.
type Repo struct {
	ID        string
	Name      string
	Type      string // "private" | "public" | "inbox"
	RootPath  string
	RemoteURL string // empty for inbox type
	Status    string // "active" | "archived" | "error"
	CreatedAt string // RFC3339
	UpdatedAt string // RFC3339
}

// SaveRepo is the canonical name for inserting or updating a repo row,
// satisfying the RepoStore interface used by registry and analytics packages.
// It delegates to UpsertRepo.
func (g *GlobalDB) SaveRepo(ctx context.Context, r Repo) error {
	return g.UpsertRepo(ctx, r)
}

// UpsertRepo inserts or updates a repo row. ID must be a non-empty UUID.
func (g *GlobalDB) UpsertRepo(ctx context.Context, r Repo) error {
	if r.ID == "" {
		return fmt.Errorf("vedox/globaldb: UpsertRepo: empty ID")
	}
	if r.Name == "" {
		return fmt.Errorf("vedox/globaldb: UpsertRepo: empty Name")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if r.CreatedAt == "" {
		r.CreatedAt = now
	}
	r.UpdatedAt = now
	var remoteArg any
	if r.RemoteURL != "" {
		remoteArg = r.RemoteURL
	}
	return g.writer.submit(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO repos(id, name, type, root_path, remote_url, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				name       = excluded.name,
				type       = excluded.type,
				root_path  = excluded.root_path,
				remote_url = excluded.remote_url,
				status     = excluded.status,
				updated_at = excluded.updated_at`,
			r.ID, r.Name, r.Type, r.RootPath, remoteArg, r.Status, r.CreatedAt, r.UpdatedAt,
		)
		return err
	})
}

// GetRepo returns the repo with the given ID, or nil if not found.
func (g *GlobalDB) GetRepo(ctx context.Context, id string) (*Repo, error) {
	row := g.readDB.QueryRowContext(ctx,
		`SELECT id, name, type, root_path, COALESCE(remote_url,''), status, created_at, updated_at
		 FROM repos WHERE id = ?`, id)
	r := &Repo{}
	err := row.Scan(&r.ID, &r.Name, &r.Type, &r.RootPath, &r.RemoteURL, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("vedox/globaldb: GetRepo: %w", err)
	}
	return r, nil
}

// ListRepos returns all repos ordered by name. Status filter is optional
// (pass "" to return all statuses).
func (g *GlobalDB) ListRepos(ctx context.Context, status string) ([]*Repo, error) {
	q := `SELECT id, name, type, root_path, COALESCE(remote_url,''), status, created_at, updated_at
	      FROM repos`
	args := []any{}
	if status != "" {
		q += ` WHERE status = ?`
		args = append(args, status)
	}
	q += ` ORDER BY name ASC`
	rows, err := g.readDB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("vedox/globaldb: ListRepos: %w", err)
	}
	defer rows.Close()
	var out []*Repo
	for rows.Next() {
		r := &Repo{}
		if err := rows.Scan(&r.ID, &r.Name, &r.Type, &r.RootPath, &r.RemoteURL, &r.Status, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("vedox/globaldb: ListRepos scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteRepo removes a repo by ID. Agent installs scoped to this repo have
// their repo_id set to NULL via the ON DELETE SET NULL FK constraint.
func (g *GlobalDB) DeleteRepo(ctx context.Context, id string) error {
	return g.writer.submit(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM repos WHERE id = ?`, id)
		return err
	})
}

// ---------------------------------------------------------------------------
// AgentInstall CRUD
// ---------------------------------------------------------------------------

// AgentInstall records a provider-level Doc Agent install.
type AgentInstall struct {
	ID           string
	Provider     string // "claude-code" | "codex" | "copilot" | "gemini"
	Version      string
	InstallDate  string // RFC3339
	HealthStatus string // "healthy" | "degraded" | "failed" | "unknown"
	RepoID       string // empty = global
}

// UpsertAgentInstall inserts or replaces an agent install row.
func (g *GlobalDB) UpsertAgentInstall(ctx context.Context, a AgentInstall) error {
	if a.ID == "" {
		return fmt.Errorf("vedox/globaldb: UpsertAgentInstall: empty ID")
	}
	if a.InstallDate == "" {
		a.InstallDate = time.Now().UTC().Format(time.RFC3339)
	}
	if a.HealthStatus == "" {
		a.HealthStatus = "unknown"
	}
	var repoArg any
	if a.RepoID != "" {
		repoArg = a.RepoID
	}
	return g.writer.submit(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO agent_installs(id, provider, version, install_date, health_status, repo_id)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				provider      = excluded.provider,
				version       = excluded.version,
				install_date  = excluded.install_date,
				health_status = excluded.health_status,
				repo_id       = excluded.repo_id`,
			a.ID, a.Provider, a.Version, a.InstallDate, a.HealthStatus, repoArg,
		)
		return err
	})
}

// ListAgentInstalls returns all agent installs, optionally filtered by provider.
func (g *GlobalDB) ListAgentInstalls(ctx context.Context, provider string) ([]*AgentInstall, error) {
	q := `SELECT id, provider, version, install_date, health_status, COALESCE(repo_id,'')
	      FROM agent_installs`
	args := []any{}
	if provider != "" {
		q += ` WHERE provider = ?`
		args = append(args, provider)
	}
	q += ` ORDER BY install_date DESC`
	rows, err := g.readDB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("vedox/globaldb: ListAgentInstalls: %w", err)
	}
	defer rows.Close()
	var out []*AgentInstall
	for rows.Next() {
		a := &AgentInstall{}
		if err := rows.Scan(&a.ID, &a.Provider, &a.Version, &a.InstallDate, &a.HealthStatus, &a.RepoID); err != nil {
			return nil, fmt.Errorf("vedox/globaldb: ListAgentInstalls scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Daily event roll-up
// ---------------------------------------------------------------------------

// IncrementDailyEvent adds delta to the (date, kind) aggregate counter.
// If the row does not exist it is created. Idempotent with delta=0.
//
// date must be an ISO 8601 date string (YYYY-MM-DD).
// kind must be a subject.verb event kind (see analytics.EventKind* constants).
func (g *GlobalDB) IncrementDailyEvent(ctx context.Context, date, kind string, delta int) error {
	if date == "" || kind == "" {
		return fmt.Errorf("vedox/globaldb: IncrementDailyEvent: date and kind must not be empty")
	}
	return g.writer.submit(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO events_daily(date, kind, count) VALUES (?, ?, ?)
			ON CONFLICT(date, kind) DO UPDATE SET count = count + excluded.count`,
			date, kind, delta,
		)
		return err
	})
}

// GetDailyEventCount returns the aggregate count for (date, kind), or 0 if
// no row exists.
func (g *GlobalDB) GetDailyEventCount(ctx context.Context, date, kind string) (int, error) {
	var count int
	err := g.readDB.QueryRowContext(ctx,
		`SELECT COALESCE(count, 0) FROM events_daily WHERE date = ? AND kind = ?`,
		date, kind,
	).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("vedox/globaldb: GetDailyEventCount: %w", err)
	}
	return count, nil
}

// SumDailyEvents returns the total event count for the given kind between
// fromDate and toDate inclusive (both in YYYY-MM-DD format). Returns 0 if
// no rows match. This is the primary read path for the analytics summary
// endpoint — it is issued against the read-only pool and is safe to call
// concurrently.
func (g *GlobalDB) SumDailyEvents(ctx context.Context, kind, fromDate, toDate string) (int, error) {
	var total int
	err := g.readDB.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(count), 0) FROM events_daily
		 WHERE kind = ? AND date >= ? AND date <= ?`,
		kind, fromDate, toDate,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("vedox/globaldb: SumDailyEvents: %w", err)
	}
	return total, nil
}

// ---------------------------------------------------------------------------
// Writer / reader access — used by the analytics aggregator (WS-K).
// ---------------------------------------------------------------------------

// SubmitWrite enqueues a write operation on the global DB's single-writer
// goroutine. The aggregator uses this to UPSERT analytics_cache rows without
// bypassing the serialisation invariant. fn runs inside an open *sql.Tx;
// return an error to roll back, return nil to commit.
func (g *GlobalDB) SubmitWrite(ctx context.Context, fn func(tx *sql.Tx) error) error {
	return g.writer.submit(ctx, fn)
}

// ReadDB returns the read-only *sql.DB connection pool for the global
// database. External packages may issue SELECT queries directly; writes
// through this handle will fail (query_only=ON).
func (g *GlobalDB) ReadDB() *sql.DB { return g.readDB }

// ---------------------------------------------------------------------------
// analytics_cache CRUD
// ---------------------------------------------------------------------------

// AnalyticsCache holds the precomputed values written by the aggregator.
type AnalyticsCache struct {
	HasRun            bool
	TotalDocs         int
	DocsPerProject    string // JSON object
	ChangeVelocity7d  int
	ChangeVelocity30d int
	UpdatedAt         string // RFC3339
}

// UpsertAnalyticsCache inserts or replaces the single 'summary' row in
// analytics_cache. Called by the aggregator after every 60s cycle.
func (g *GlobalDB) UpsertAnalyticsCache(ctx context.Context, c AnalyticsCache) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if c.UpdatedAt == "" {
		c.UpdatedAt = now
	}
	if c.DocsPerProject == "" {
		c.DocsPerProject = "{}"
	}
	return g.UpsertAnalyticsCacheRaw(ctx,
		c.HasRun, c.TotalDocs, c.DocsPerProject,
		c.ChangeVelocity7d, c.ChangeVelocity30d, c.UpdatedAt,
	)
}

// UpsertAnalyticsCacheRaw is the primitive-argument form of UpsertAnalyticsCache.
// It satisfies the analytics.GlobalDBWriter interface without requiring the
// analytics package to be imported into internal/db (which would create a cycle).
func (g *GlobalDB) UpsertAnalyticsCacheRaw(ctx context.Context,
	hasRun bool, totalDocs int, docsPerProject string,
	vel7d, vel30d int, updatedAt string,
) error {
	hasRunInt := 0
	if hasRun {
		hasRunInt = 1
	}
	if docsPerProject == "" {
		docsPerProject = "{}"
	}
	if updatedAt == "" {
		updatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return g.writer.submit(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO analytics_cache(key, has_run, total_docs, docs_per_project,
			                           change_velocity_7d, change_velocity_30d, updated_at)
			VALUES ('summary', ?, ?, ?, ?, ?, ?)
			ON CONFLICT(key) DO UPDATE SET
				has_run             = excluded.has_run,
				total_docs          = excluded.total_docs,
				docs_per_project    = excluded.docs_per_project,
				change_velocity_7d  = excluded.change_velocity_7d,
				change_velocity_30d = excluded.change_velocity_30d,
				updated_at          = excluded.updated_at`,
			hasRunInt, totalDocs, docsPerProject,
			vel7d, vel30d, updatedAt,
		)
		return err
	})
}

// GetAnalyticsCache returns the current 'summary' row, or nil if the
// aggregator has not run yet. Reads from the read-only pool.
func (g *GlobalDB) GetAnalyticsCache(ctx context.Context) (*AnalyticsCache, error) {
	row := g.readDB.QueryRowContext(ctx, `
		SELECT has_run, total_docs, docs_per_project,
		       change_velocity_7d, change_velocity_30d, updated_at
		FROM analytics_cache WHERE key = 'summary'`)
	var (
		c      AnalyticsCache
		hasRun int
	)
	err := row.Scan(&hasRun, &c.TotalDocs, &c.DocsPerProject,
		&c.ChangeVelocity7d, &c.ChangeVelocity30d, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("vedox/globaldb: GetAnalyticsCache: %w", err)
	}
	c.HasRun = hasRun == 1
	return &c, nil
}
