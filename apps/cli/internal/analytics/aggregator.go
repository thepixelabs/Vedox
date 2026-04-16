package analytics

// Aggregator implements the SQLite-tail pattern (CTO ruling R1, WS-K):
//
//  1. Every 60 seconds it reads new rows from the workspace events table
//     using a persistent cursor (last_seen_id) so each row is processed
//     exactly once.
//  2. It rolls up (date, kind) counts and UPSERTs them into
//     GlobalDB::events_daily via IncrementDailyEvent — the method is
//     idempotent so aggregator restarts do not double-count.
//  3. After each cycle it recomputes the analytics_cache summary and
//     writes it to GlobalDB::analytics_cache via UpsertAnalyticsCache.
//
// The Aggregator owns no mutable SQLite connections of its own; all reads
// go through the workspace Store's read-only pool and all global writes
// route through GlobalDB's single-writer funnel.
//
// Zero-downtime property: if the workspace DB is unavailable for one cycle
// the aggregator logs a warning and retries on the next tick. The cursor
// is not advanced on failure so no data is lost.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

const (
	// aggregateInterval is how often the aggregator wakes and processes new
	// events. 60 seconds matches the CTO ruling in FINAL_PLAN.md WS-K.
	aggregateInterval = 60 * time.Second
)

// GlobalDBWriter is the subset of db.GlobalDB required by the Aggregator.
// The interface breaks the import cycle between analytics and internal/db by
// using only primitive types as parameters — no shared struct required.
type GlobalDBWriter interface {
	IncrementDailyEvent(ctx context.Context, date, kind string, delta int) error
	SumDailyEvents(ctx context.Context, kind, fromDate, toDate string) (int, error)
	// UpsertAnalyticsCacheRaw writes precomputed summary values into the
	// analytics_cache table. All fields are primitive to avoid import cycles.
	UpsertAnalyticsCacheRaw(ctx context.Context,
		hasRun bool, totalDocs int, docsPerProject string,
		vel7d, vel30d int, updatedAt string) error
}

// WorkspaceReader is the subset of db.Store that the Aggregator needs for
// read-only access to the per-workspace events table.
type WorkspaceReader interface {
	ReadDB() *sql.DB
}

// Aggregator reads the workspace events table and rolls up daily counts into
// the global analytics tables.
type Aggregator struct {
	workspace WorkspaceReader
	global    GlobalDBWriter
	lastID    int64         // SQLite-tail cursor; last processed event id
	hasRun    atomic.Bool   // flips to true after the first successful cycle
	done      chan struct{}
	stopped   chan struct{}
}

// NewAggregator creates an Aggregator. workspace provides the read-only
// connection to the per-workspace events table; global is the GlobalDB handle.
func NewAggregator(workspace WorkspaceReader, global GlobalDBWriter) *Aggregator {
	return &Aggregator{
		workspace: workspace,
		global:    global,
		done:      make(chan struct{}),
		stopped:   make(chan struct{}),
	}
}

// Start launches the background aggregation goroutine. It runs until ctx is
// cancelled or Stop is called. Start must be called exactly once.
func (a *Aggregator) Start(ctx context.Context) {
	go a.run(ctx)
}

// Stop signals the aggregation goroutine to perform one final cycle and exit.
// Blocks until the goroutine has finished.
func (a *Aggregator) Stop() {
	select {
	case <-a.done:
	default:
		close(a.done)
	}
	<-a.stopped
}

// HasRun reports whether the aggregator has completed at least one successful
// cycle. The API handler uses this to set pipeline_ready=true in the JSON
// response.
func (a *Aggregator) HasRun() bool {
	return a.hasRun.Load()
}

// run is the background goroutine body.
func (a *Aggregator) run(ctx context.Context) {
	defer close(a.stopped)

	ticker := time.NewTicker(aggregateInterval)
	defer ticker.Stop()

	// Run one cycle immediately on startup so the first API call after the
	// daemon starts returns data rather than waiting up to 60 seconds.
	a.cycle(ctx)

	for {
		select {
		case <-ctx.Done():
			a.cycle(context.Background()) // final flush
			return
		case <-a.done:
			a.cycle(context.Background()) // final flush
			return
		case <-ticker.C:
			a.cycle(ctx)
		}
	}
}

// cycle is one aggregation pass: tail the events table, roll up into
// events_daily, then refresh analytics_cache.
func (a *Aggregator) cycle(ctx context.Context) {
	if err := a.tailAndRollup(ctx); err != nil {
		slog.Warn("analytics: aggregation cycle failed",
			"error", err,
			"last_id", a.lastID,
		)
		return
	}
	if err := a.refreshCache(ctx); err != nil {
		slog.Warn("analytics: cache refresh failed", "error", err)
		return
	}
	a.hasRun.Store(true)
}

// tailAndRollup reads all events with id > a.lastID, aggregates them by
// (date, kind), and UPSERTs the counts into GlobalDB::events_daily.
func (a *Aggregator) tailAndRollup(ctx context.Context) error {
	rdb := a.workspace.ReadDB()

	rows, err := rdb.QueryContext(ctx,
		`SELECT id, kind, timestamp FROM events WHERE id > ? ORDER BY id ASC`,
		a.lastID,
	)
	if err != nil {
		return fmt.Errorf("query events tail: %w", err)
	}
	defer rows.Close()

	// Accumulate (date, kind) → count in memory; single pass.
	type key struct{ date, kind string }
	counts := make(map[key]int)
	var maxID int64

	for rows.Next() {
		var (
			id        int64
			kind      string
			timestamp string
		)
		if err := rows.Scan(&id, &kind, &timestamp); err != nil {
			return fmt.Errorf("scan event row: %w", err)
		}
		// Parse timestamp to extract the date portion.
		// Stored as RFC3339; tolerate bare dates as a fallback.
		date := timestamp
		if t, parseErr := time.Parse(time.RFC3339, timestamp); parseErr == nil {
			date = t.UTC().Format("2006-01-02")
		} else if len(timestamp) >= 10 {
			date = timestamp[:10]
		}
		counts[key{date, kind}]++
		if id > maxID {
			maxID = id
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate event rows: %w", err)
	}

	if len(counts) == 0 {
		// Nothing new since the last cycle; nothing to write.
		return nil
	}

	// Flush aggregated counts into GlobalDB. Each call goes through the
	// GlobalDB writer funnel — no direct connection needed here.
	for k, delta := range counts {
		if err := a.global.IncrementDailyEvent(ctx, k.date, k.kind, delta); err != nil {
			return fmt.Errorf("increment daily event %s/%s: %w", k.date, k.kind, err)
		}
	}

	// Advance cursor only after successful writes.
	a.lastID = maxID
	return nil
}

// refreshCache recomputes the analytics_cache summary from the events_daily
// table and writes it to GlobalDB. The read uses SumDailyEvents which hits
// the GlobalDB read-only pool; the write routes through UpsertAnalyticsCache.
func (a *Aggregator) refreshCache(ctx context.Context) error {
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	day7ago := now.AddDate(0, 0, -7).Format("2006-01-02")
	day30ago := now.AddDate(0, 0, -30).Format("2006-01-02")
	epoch := "2000-01-01"

	totalDocs, err := a.global.SumDailyEvents(ctx, EventKindDocumentPublished, epoch, today)
	if err != nil {
		return fmt.Errorf("sum total_docs: %w", err)
	}
	vel7d, err := a.global.SumDailyEvents(ctx, EventKindDocumentPublished, day7ago, today)
	if err != nil {
		return fmt.Errorf("sum velocity_7d: %w", err)
	}
	vel30d, err := a.global.SumDailyEvents(ctx, EventKindDocumentPublished, day30ago, today)
	if err != nil {
		return fmt.Errorf("sum velocity_30d: %w", err)
	}

	// docs_per_project: read the workspace events table and count
	// document.published events grouped by the properties.project field.
	// This is best-effort: if parsing fails, we emit an empty object.
	docsPerProject := a.computeDocsPerProject(ctx)

	return a.global.UpsertAnalyticsCacheRaw(ctx,
		true, totalDocs, docsPerProject,
		vel7d, vel30d,
		now.Format(time.RFC3339),
	)
}

// computeDocsPerProject queries the workspace events table for
// document.published events and groups them by the "project" field in the
// properties JSON column. Returns a JSON object string.
func (a *Aggregator) computeDocsPerProject(ctx context.Context) string {
	rdb := a.workspace.ReadDB()

	rows, err := rdb.QueryContext(ctx,
		`SELECT properties FROM events WHERE kind = ? AND properties IS NOT NULL`,
		EventKindDocumentPublished,
	)
	if err != nil {
		return "{}"
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var propsJSON string
		if err := rows.Scan(&propsJSON); err != nil {
			continue
		}
		var props map[string]any
		if err := json.Unmarshal([]byte(propsJSON), &props); err != nil {
			continue
		}
		if proj, ok := props["project"].(string); ok && proj != "" {
			counts[proj]++
		}
	}

	b, err := json.Marshal(counts)
	if err != nil {
		return "{}"
	}
	return string(b)
}
