package analytics

// Pruner implements the 365-day retention policy for the workspace events
// table (FINAL_PLAN.md changelog item 5: "Events retention default: 365 days.
// One-line default in pruner; no schema change.").
//
// It runs a background goroutine that wakes on a fixed daily cadence and
// executes:
//
//	DELETE FROM events WHERE timestamp < (now - retention)
//
// The write routes through the Store's single-writer funnel (architectural
// rule R12) via the DBWriter interface — the pruner holds no mutable SQLite
// connections of its own.
//
// Design decisions:
//   - Daily cadence (24h) is a plan-level default; exposed via PrunerConfig
//     for tests to shrink to sub-second intervals.
//   - Retention 365d is the default; exposed via PrunerConfig so operators
//     can tighten it without a recompile (future work; v2.0 ships default).
//   - The first prune runs immediately on Start() so a daemon restart does
//     not mean waiting up to 24h for the retention window to enforce.
//   - Errors are logged but never returned: analytics failures must never
//     propagate to callers.
//   - Stop() drains: no final prune on shutdown (the next start will catch
//     anything that aged past retention while the daemon was down).
//
// The pruner is started by the Collector when the Collector itself starts,
// so that every daemon instance with analytics enabled has retention
// enforced without extra wiring.

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

const (
	// defaultPrunerInterval is how often the pruner wakes to enforce the
	// retention window. Daily matches FINAL_PLAN.md WS-K ("runs daily").
	defaultPrunerInterval = 24 * time.Hour

	// defaultPrunerRetention is the age beyond which events are deleted.
	// 365 days matches FINAL_PLAN.md changelog item 5 (product-engineer
	// dissent accepted: 90d was too short for year-over-year analytics).
	defaultPrunerRetention = 365 * 24 * time.Hour
)

// PrunerConfig parameterises a Pruner. Zero values fall back to the defaults
// above. Tests use this to shrink Interval to milliseconds.
type PrunerConfig struct {
	// Interval is how often DELETE runs. Default: 24h.
	Interval time.Duration

	// Retention is the age beyond which events are deleted. Default: 365d.
	Retention time.Duration

	// Clock is the time source. Default: time.Now. Tests inject a fake clock
	// to pin "now" and make retention math deterministic.
	Clock func() time.Time
}

func (c PrunerConfig) interval() time.Duration {
	if c.Interval <= 0 {
		return defaultPrunerInterval
	}
	return c.Interval
}

func (c PrunerConfig) retention() time.Duration {
	if c.Retention <= 0 {
		return defaultPrunerRetention
	}
	return c.Retention
}

func (c PrunerConfig) now() time.Time {
	if c.Clock == nil {
		return time.Now()
	}
	return c.Clock()
}

// Pruner runs a periodic DELETE over the events table to enforce retention.
type Pruner struct {
	db      DBWriter
	cfg     PrunerConfig
	done    chan struct{}
	stopped chan struct{}
}

// NewPruner constructs a Pruner. Pass a zero PrunerConfig for production
// defaults (24h cadence, 365d retention, wall clock). Start must be called
// exactly once to launch the background goroutine.
func NewPruner(db DBWriter, cfg PrunerConfig) *Pruner {
	return &Pruner{
		db:      db,
		cfg:     cfg,
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

// Start launches the background prune goroutine. It runs until ctx is
// cancelled or Stop is called. An immediate prune runs before the first
// tick so retention is enforced on daemon boot. Start blocks until the
// initial prune has completed so tests and callers can assume the table
// is in its post-prune state once Start returns.
func (p *Pruner) Start(ctx context.Context) {
	// Run the initial prune synchronously so callers can assume "Start
	// returned" implies "first pass complete". This matches the pattern
	// used by Aggregator.run (immediate cycle before entering the ticker
	// loop) but hoisted out of the goroutine for determinism in tests.
	p.pruneOnce(ctx)
	go p.run(ctx)
}

// Stop signals the prune goroutine to exit cleanly. Blocks until the
// goroutine has finished. Safe to call more than once.
func (p *Pruner) Stop() {
	select {
	case <-p.done:
		// already stopped
	default:
		close(p.done)
	}
	<-p.stopped
}

// run is the background goroutine body.
func (p *Pruner) run(ctx context.Context) {
	defer close(p.stopped)

	ticker := time.NewTicker(p.cfg.interval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.done:
			return
		case <-ticker.C:
			p.pruneOnce(ctx)
		}
	}
}

// pruneOnce executes a single DELETE of events older than retention. It
// logs on success (with the deleted row count) and on failure. Errors are
// never returned — analytics failures must not propagate to callers.
func (p *Pruner) pruneOnce(ctx context.Context) {
	cutoff := p.cfg.now().Add(-p.cfg.retention()).UTC().Format(time.RFC3339)

	var deleted int64
	err := p.db.SubmitWrite(ctx, func(tx *sql.Tx) error {
		// Guard: if the table does not yet exist (Collector has not flushed
		// its first batch), the DELETE would error. Create the same table
		// shape the Collector uses; idempotent.
		if _, err := tx.Exec(`
			CREATE TABLE IF NOT EXISTS events (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				kind       TEXT NOT NULL,
				timestamp  TEXT NOT NULL,
				session_id TEXT NOT NULL,
				properties TEXT
			)`); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`DELETE FROM events WHERE timestamp < ?`, cutoff,
		)
		if err != nil {
			return err
		}
		n, raErr := res.RowsAffected()
		if raErr == nil {
			deleted = n
		}
		return nil
	})
	if err != nil {
		slog.Warn("analytics: prune failed",
			"cutoff", cutoff,
			"retention", p.cfg.retention().String(),
			"error", err,
		)
		return
	}
	if deleted > 0 {
		slog.Info("analytics: pruned expired events",
			"deleted", deleted,
			"cutoff", cutoff,
			"retention", p.cfg.retention().String(),
		)
	}
}
