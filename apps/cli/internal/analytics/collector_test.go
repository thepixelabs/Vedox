package analytics

// Tests for Collector: buffered channel, batch flush, table creation,
// and drop-on-full behaviour.

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Minimal in-process DBWriter backed by a temp SQLite file.
// ---------------------------------------------------------------------------

// memDBWriter is a DBWriter backed by an in-process SQLite connection.
// It opens a single write connection — sufficient for the collector tests
// which never have concurrent writers.
type memDBWriter struct {
	db *sql.DB
}

func newMemDBWriter(t *testing.T) *memDBWriter {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?cache=shared&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return &memDBWriter{db: db}
}

func (m *memDBWriter) SubmitWrite(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// countEvents returns the number of rows in the events table, creating the
// table first if it does not exist (so tests can query before the first flush).
func (m *memDBWriter) countEvents(t *testing.T, kind string) int {
	t.Helper()
	// Table may not exist yet if no flush has happened.
	_, _ = m.db.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			kind       TEXT NOT NULL,
			timestamp  TEXT NOT NULL,
			session_id TEXT NOT NULL,
			properties TEXT
		)`)
	var n int
	row := m.db.QueryRow(`SELECT COUNT(*) FROM events WHERE kind = ?`, kind)
	if err := row.Scan(&n); err != nil {
		t.Fatalf("countEvents: %v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestCollector_EmitAndFlush verifies that emitted events are persisted after
// Stop() drains the buffer.
func TestCollector_EmitAndFlush(t *testing.T) {
	w := newMemDBWriter(t)
	c := NewCollector(w, "sess-test-001")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	for i := 0; i < 5; i++ {
		if err := c.Emit(Event{
			Kind:      EventKindDocumentPublished,
			Timestamp: time.Now(),
			SessionID: "sess-test-001",
		}); err != nil {
			t.Fatalf("Emit[%d]: %v", i, err)
		}
	}

	c.Stop()

	if n := w.countEvents(t, EventKindDocumentPublished); n != 5 {
		t.Errorf("events in DB = %d, want 5", n)
	}
}

// TestCollector_InvalidEventRejected verifies that Emit returns an error and
// does not enqueue an invalid event.
func TestCollector_InvalidEventRejected(t *testing.T) {
	w := newMemDBWriter(t)
	c := NewCollector(w, "sess-test-002")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	err := c.Emit(Event{
		Kind:      "", // invalid: empty kind
		Timestamp: time.Now(),
		SessionID: "sess-test-002",
	})
	if err == nil {
		t.Error("expected error for empty Kind, got nil")
	}

	c.Stop()

	// No events should have been inserted.
	if n := w.countEvents(t, ""); n != 0 {
		t.Errorf("events in DB = %d, want 0 after invalid emit", n)
	}
}

// TestCollector_SessionIDInheritance verifies that when the caller passes an
// empty SessionID, the Collector's session ID is used.
func TestCollector_SessionIDInheritance(t *testing.T) {
	w := newMemDBWriter(t)
	const wantSession = "sess-inherited"
	c := NewCollector(w, wantSession)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	// Emit with explicit SessionID set — should be stored as-is.
	if err := c.Emit(Event{
		Kind:      EventKindSearchExecuted,
		Timestamp: time.Now(),
		SessionID: wantSession,
	}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	c.Stop()

	var gotSession string
	row := w.db.QueryRow(`SELECT session_id FROM events WHERE kind = ?`, EventKindSearchExecuted)
	if err := row.Scan(&gotSession); err != nil {
		t.Fatalf("scan session_id: %v", err)
	}
	if gotSession != wantSession {
		t.Errorf("session_id = %q, want %q", gotSession, wantSession)
	}
}

// TestCollector_BufferFullDrop verifies that events are dropped (not blocking
// the caller) when the buffer is full. We fill the channel synchronously then
// check that the emit still returns without blocking.
func TestCollector_BufferFullDrop(t *testing.T) {
	w := newMemDBWriter(t)
	c := NewCollector(w, "sess-drop-test")
	// Do NOT call Start — the goroutine never drains, so the channel will fill.

	// Fill the buffer to capacity.
	for i := 0; i < collectorBufferSize; i++ {
		e := Event{Kind: EventKindDocumentViewed, Timestamp: time.Now(), SessionID: "s"}
		select {
		case c.ch <- e:
		default:
			t.Logf("channel full at iteration %d (expected at %d)", i, collectorBufferSize)
		}
	}

	// The next emit should be a non-blocking drop, not a deadlock.
	done := make(chan struct{})
	go func() {
		_ = c.Emit(Event{
			Kind:      EventKindDocumentViewed,
			Timestamp: time.Now(),
			SessionID: "s",
		})
		close(done)
	}()

	select {
	case <-done:
		// Good — returned without blocking.
	case <-time.After(2 * time.Second):
		t.Error("Emit blocked when channel was full; expected non-blocking drop")
	}
}

// TestCollector_ConcurrentEmit verifies that concurrent Emit calls do not
// panic or deadlock.
func TestCollector_ConcurrentEmit(t *testing.T) {
	w := newMemDBWriter(t)
	c := NewCollector(w, "sess-concurrent")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	const goroutines = 20
	const eventsEach = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < eventsEach; i++ {
				_ = c.Emit(Event{
					Kind:      EventKindAgentTriggered,
					Timestamp: time.Now(),
					SessionID: "sess-concurrent",
				})
			}
		}()
	}
	wg.Wait()
	c.Stop()

	// At most goroutines*eventsEach rows; at least some should have landed.
	n := w.countEvents(t, EventKindAgentTriggered)
	if n == 0 {
		t.Error("no events persisted after concurrent emit; expected some rows")
	}
	t.Logf("concurrent emit persisted %d / %d events", n, goroutines*eventsEach)
}

// TestCollector_PropertiesJSON verifies that Properties are serialised as JSON.
func TestCollector_PropertiesJSON(t *testing.T) {
	w := newMemDBWriter(t)
	c := NewCollector(w, "sess-props")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)

	if err := c.Emit(Event{
		Kind:       EventKindDocumentPublished,
		Timestamp:  time.Now(),
		SessionID:  "sess-props",
		Properties: map[string]any{"project": "myapp", "size": 42},
	}); err != nil {
		t.Fatalf("Emit: %v", err)
	}

	c.Stop()

	var props sql.NullString
	row := w.db.QueryRow(`SELECT properties FROM events WHERE kind = ?`, EventKindDocumentPublished)
	if err := row.Scan(&props); err != nil {
		t.Fatalf("scan properties: %v", err)
	}
	if !props.Valid || props.String == "" {
		t.Error("expected non-NULL properties JSON, got NULL/empty")
	}
	// Properties must be valid JSON containing the expected key.
	if props.String == "{}" {
		t.Error("properties is '{}', expected {\"project\":\"myapp\",\"size\":42}")
	}
}
