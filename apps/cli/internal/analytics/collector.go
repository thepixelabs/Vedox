package analytics

// Collector is the write-side of the analytics pipeline.
//
// It accepts Event values via Emit, validates them, and enqueues them on a
// buffered channel (cap 256). A background flush goroutine drains the channel
// every 5 seconds and batch-inserts the events into the per-workspace SQLite
// events table, routing all writes through the Store's single-writer funnel
// (architectural rule R12).
//
// The events table is created by migration 006_events.sql and lives in the
// workspace .vedox/index.db alongside documents and tasks.
//
// Usage:
//
//	c := analytics.NewCollector(store, sessionID)
//	c.Start(ctx)
//	defer c.Stop()
//	c.Emit(analytics.Event{Kind: analytics.EventKindDocumentPublished, ...})

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"
)

const (
	// collectorBufferSize is the capacity of the internal event channel.
	// At 256 slots and a flush every 5s, the buffer accommodates ~51 events/s
	// sustained without blocking callers — comfortably above any anticipated
	// Vedox write rate (FINAL_PLAN.md: "analytics must never slow the daemon").
	collectorBufferSize = 256

	// flushInterval is how often the background goroutine drains the channel
	// and commits a batch INSERT into the events table.
	flushInterval = 5 * time.Second
)

// DBWriter is the subset of db.Store that Collector needs. Using an interface
// keeps the analytics package free of an import cycle with internal/db.
type DBWriter interface {
	SubmitWrite(ctx context.Context, fn func(tx *sql.Tx) error) error
}

// Collector holds the buffered channel and background goroutine that drain
// events from callers into the workspace SQLite events table.
//
// Collector also owns a retention Pruner that runs on a daily cadence and
// deletes events older than 365 days (FINAL_PLAN.md changelog item 5). The
// pruner lifecycle is bound to the Collector's — Start starts both, Stop
// stops both — so callers do not wire retention separately.
type Collector struct {
	db        DBWriter
	sessionID string
	ch        chan Event
	done      chan struct{}
	stopped   chan struct{}
	pruner    *Pruner
}

// NewCollector creates a Collector backed by the given DBWriter. sessionID is
// the per-daemon-start opaque identifier that is stored on every event row.
// Call Start to launch the background goroutine before calling Emit.
//
// The Collector owns a retention Pruner at default settings (24h cadence,
// 365d window) constructed here and started alongside the flush goroutine
// in Start. Tests that need a non-default pruner can use
// NewCollectorWithPruner.
func NewCollector(db DBWriter, sessionID string) *Collector {
	return NewCollectorWithPruner(db, sessionID, PrunerConfig{})
}

// NewCollectorWithPruner is NewCollector with an explicit PrunerConfig. Use
// this in tests to shrink the prune interval / retention window for fast
// assertions; pass a zero PrunerConfig for production defaults.
func NewCollectorWithPruner(db DBWriter, sessionID string, pcfg PrunerConfig) *Collector {
	return &Collector{
		db:        db,
		sessionID: sessionID,
		ch:        make(chan Event, collectorBufferSize),
		done:      make(chan struct{}),
		stopped:   make(chan struct{}),
		pruner:    NewPruner(db, pcfg),
	}
}

// Start launches the background flush goroutine and the retention pruner.
// Both run until ctx is cancelled or Stop is called. Start must be called
// exactly once.
func (c *Collector) Start(ctx context.Context) {
	if c.pruner != nil {
		c.pruner.Start(ctx)
	}
	go c.run(ctx)
}

// Stop signals the flush goroutine to drain any remaining buffered events
// and exit, then stops the retention pruner. It blocks until both
// goroutines have finished. Safe to call more than once (subsequent calls
// are no-ops after the first).
func (c *Collector) Stop() {
	select {
	case <-c.done:
		// already stopped
	default:
		close(c.done)
	}
	<-c.stopped
	if c.pruner != nil {
		c.pruner.Stop()
	}
}

// Emit validates the event and enqueues it for asynchronous insertion.
// If the buffer is full the event is dropped and a warning is logged —
// analytics data loss is always preferable to blocking the caller.
// Emit is safe to call concurrently from multiple goroutines.
//
// The Collector's own sessionID is substituted when the caller leaves
// Event.SessionID empty. The substitution happens BEFORE Validate() so
// callers that rely on the Collector-owned session (the common case for
// HTTP handlers that do not track their own sessions) don't fail
// validation with "SessionID must not be empty".
func (c *Collector) Emit(e Event) error {
	// Inherit the collector's sessionID when the caller leaves it empty.
	// This is done before Validate so the required-field check still
	// catches the truly pathological case where both the caller and the
	// collector have an empty sessionID (a construction bug worth
	// surfacing).
	if e.SessionID == "" {
		e.SessionID = c.sessionID
	}
	if err := e.Validate(); err != nil {
		return err
	}
	select {
	case c.ch <- e:
	default:
		slog.Warn("analytics: event channel full, dropping event",
			"kind", e.Kind,
			"buffer_cap", collectorBufferSize,
		)
	}
	return nil
}

// run is the background goroutine body.
func (c *Collector) run(ctx context.Context) {
	defer close(c.stopped)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.flush(context.Background())
			return
		case <-c.done:
			c.flush(context.Background())
			return
		case <-ticker.C:
			c.flush(ctx)
		}
	}
}

// flush drains all events currently in the channel and writes them as a
// single batch INSERT. If the channel is empty, it returns immediately.
// Errors are logged but never returned — analytics failures must not
// propagate to callers.
func (c *Collector) flush(ctx context.Context) {
	// Drain up to collectorBufferSize events in one pass (non-blocking).
	var batch []Event
	for {
		select {
		case e := <-c.ch:
			batch = append(batch, e)
		default:
			goto drained
		}
	}
drained:
	if len(batch) == 0 {
		return
	}

	err := c.db.SubmitWrite(ctx, func(tx *sql.Tx) error {
		// Ensure the events table exists (idempotent DDL; migration 006 creates
		// it on first Open but this guard makes the Collector usable in tests
		// that open a Store without running the full migration stack).
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

		stmt, err := tx.Prepare(`
			INSERT INTO events(kind, timestamp, session_id, properties)
			VALUES (?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, e := range batch {
			var propsJSON *string
			if len(e.Properties) > 0 {
				b, marshalErr := json.Marshal(e.Properties)
				if marshalErr == nil {
					s := string(b)
					propsJSON = &s
				}
			}
			if _, err := stmt.Exec(
				e.Kind,
				e.Timestamp.UTC().Format(time.RFC3339),
				e.SessionID,
				propsJSON,
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		slog.Warn("analytics: failed to flush event batch",
			"count", len(batch),
			"error", err,
		)
	}
}
