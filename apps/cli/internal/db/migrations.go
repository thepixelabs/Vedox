package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migration is a single versioned SQL file.
type migration struct {
	version int
	name    string
	sql     string
}

// loadMigrations reads every embedded .sql file and returns them in
// ascending version order. File names must follow NNN_description.sql
// where NNN is a zero-padded integer.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("[VDX-008] read embedded migrations: %w", err)
	}
	var out []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("[VDX-008] malformed migration name %q", e.Name())
		}
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("[VDX-008] migration %q has non-numeric prefix: %w", e.Name(), err)
		}
		b, err := fs.ReadFile(migrationsFS, "migrations/"+e.Name())
		if err != nil {
			return nil, fmt.Errorf("[VDX-008] read migration %q: %w", e.Name(), err)
		}
		out = append(out, migration{version: v, name: e.Name(), sql: string(b)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

// currentSchemaVersion returns the highest applied version, or 0 if
// the schema_version table does not yet exist.
func currentSchemaVersion(db *sql.DB) (int, error) {
	var exists int
	err := db.QueryRow(
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='schema_version'`,
	).Scan(&exists)
	if err != nil {
		return 0, err
	}
	if exists == 0 {
		return 0, nil
	}
	var v sql.NullInt64
	if err := db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&v); err != nil {
		return 0, err
	}
	return int(v.Int64), nil
}

// runMigrations applies any pending migrations atomically. If a
// migration fails the transaction is rolled back and the original
// data is preserved. Callers must abort startup on error and print
// the returned [VDX-008] message to the user.
//
// Each migration runs in its own transaction so a partial sequence
// can still leave the DB in a consistent, known-version state.
func runMigrations(db *sql.DB, log func(string)) error {
	migs, err := loadMigrations()
	if err != nil {
		return err
	}
	current, err := currentSchemaVersion(db)
	if err != nil {
		return fmt.Errorf("[VDX-008] read schema_version: %w", err)
	}
	pending := 0
	for _, m := range migs {
		if m.version <= current {
			continue
		}
		pending++
	}
	if pending == 0 {
		return nil
	}
	if log != nil {
		log(fmt.Sprintf("vedox: applying %d schema migration(s) (current=v%d)", pending, current))
	}
	for _, m := range migs {
		if m.version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("[VDX-008] begin tx for %s: %w", m.name, err)
		}
		if _, err := tx.Exec(m.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("[VDX-008] apply migration %s failed, original data preserved: %w", m.name, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO schema_version(version, applied_at) VALUES (?, ?)`,
			m.version, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("[VDX-008] record schema_version for %s: %w", m.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("[VDX-008] commit migration %s: %w", m.name, err)
		}
	}
	return nil
}
