package analytics

// Tests for the retention Pruner: verify that events older than the
// configured retention window are DELETEd, that events inside the window
// are preserved, and that Stop() returns promptly without leaking goroutines.

import (
	"context"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// seedEvent inserts a single row into the events table with an explicit
// timestamp so the test can place it either inside or outside the retention
// window. The table is created here if it doesn't exist yet — the pruner
// creates it too, but seeding first lets tests exercise "fresh install with
// old rows" without depending on ordering.
func (m *memDBWriter) seedEvent(t *testing.T, kind string, ts time.Time) {
	t.Helper()
	if _, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			kind       TEXT NOT NULL,
			timestamp  TEXT NOT NULL,
			session_id TEXT NOT NULL,
			properties TEXT
		)`); err != nil {
		t.Fatalf("create events table: %v", err)
	}
	if _, err := m.db.Exec(
		`INSERT INTO events(kind, timestamp, session_id) VALUES (?, ?, ?)`,
		kind, ts.UTC().Format(time.RFC3339), "seed-session",
	); err != nil {
		t.Fatalf("insert seeded event: %v", err)
	}
}

// countAll returns the total number of rows in the events table, or 0 if
// the table does not exist yet.
func (m *memDBWriter) countAll(t *testing.T) int {
	t.Helper()
	var n int
	row := m.db.QueryRow(`SELECT COUNT(*) FROM events`)
	if err := row.Scan(&n); err != nil {
		// Table may not exist.
		return 0
	}
	return n
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestPruner_DeletesExpiredEvents verifies that events older than the
// retention window are removed on the first prune pass. We pin the clock
// via PrunerConfig so the cutoff is deterministic.
func TestPruner_DeletesExpiredEvents(t *testing.T) {
	w := newMemDBWriter(t)

	// Fix "now" at a known instant.
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

	// Seed a row that is 400 days old (well outside 365d retention) and one
	// that is 30 days old (well inside).
	old := now.Add(-400 * 24 * time.Hour)
	fresh := now.Add(-30 * 24 * time.Hour)
	w.seedEvent(t, EventKindDocumentPublished, old)
	w.seedEvent(t, EventKindDocumentViewed, fresh)

	p := NewPruner(w, PrunerConfig{
		Interval:  time.Hour, // irrelevant for a single-shot test
		Retention: 365 * 24 * time.Hour,
		Clock:     func() time.Time { return now },
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Start runs an immediate prune synchronously before returning.
	p.Start(ctx)
	p.Stop()

	if got := w.countAll(t); got != 1 {
		t.Errorf("events after prune = %d, want 1 (fresh event preserved)", got)
	}
	// Verify the fresh event survived and the old one is gone.
	var survivingKind string
	row := w.db.QueryRow(`SELECT kind FROM events LIMIT 1`)
	if err := row.Scan(&survivingKind); err != nil {
		t.Fatalf("scan surviving kind: %v", err)
	}
	if survivingKind != EventKindDocumentViewed {
		t.Errorf("surviving kind = %q, want %q", survivingKind, EventKindDocumentViewed)
	}
}

// TestPruner_PreservesFreshEvents verifies that when ALL events are within
// the retention window, no rows are deleted.
func TestPruner_PreservesFreshEvents(t *testing.T) {
	w := newMemDBWriter(t)

	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

	// Seed 3 rows all within the last 7 days.
	for i := 1; i <= 3; i++ {
		w.seedEvent(t, EventKindAgentTriggered, now.Add(-time.Duration(i)*24*time.Hour))
	}

	p := NewPruner(w, PrunerConfig{
		Retention: 365 * 24 * time.Hour,
		Clock:     func() time.Time { return now },
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	p.Stop()

	if got := w.countAll(t); got != 3 {
		t.Errorf("events after prune = %d, want 3 (no deletions expected)", got)
	}
}

// TestPruner_BoundaryCondition verifies an event exactly at the cutoff is
// NOT deleted. The DELETE uses `<` (strictly less than), so the boundary
// row is preserved.
func TestPruner_BoundaryCondition(t *testing.T) {
	w := newMemDBWriter(t)
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	retention := 365 * 24 * time.Hour

	// Place a row exactly at the cutoff (now - retention). String comparison
	// against an identical RFC3339 stamp should NOT satisfy `<`.
	boundary := now.Add(-retention)
	w.seedEvent(t, EventKindOnboardingStarted, boundary)

	// And a row one second older than the cutoff — should be deleted.
	older := boundary.Add(-time.Second)
	w.seedEvent(t, EventKindOnboardingCompleted, older)

	p := NewPruner(w, PrunerConfig{
		Retention: retention,
		Clock:     func() time.Time { return now },
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	p.Stop()

	if got := w.countAll(t); got != 1 {
		t.Errorf("events after prune = %d, want 1 (boundary preserved, older deleted)", got)
	}
	var kind string
	if err := w.db.QueryRow(`SELECT kind FROM events`).Scan(&kind); err != nil {
		t.Fatalf("scan boundary survivor: %v", err)
	}
	if kind != EventKindOnboardingStarted {
		t.Errorf("surviving kind = %q, want %q (boundary row)", kind, EventKindOnboardingStarted)
	}
}

// TestPruner_EmptyTableOK verifies that starting the pruner against a DB
// with no events table (and no events) does not panic or error.
func TestPruner_EmptyTableOK(t *testing.T) {
	w := newMemDBWriter(t)

	p := NewPruner(w, PrunerConfig{
		Retention: 365 * 24 * time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	p.Stop()

	// Table was created by pruneOnce (CREATE TABLE IF NOT EXISTS), count == 0.
	if got := w.countAll(t); got != 0 {
		t.Errorf("events after prune = %d, want 0", got)
	}
}

// TestPruner_GracefulShutdown verifies that Stop returns promptly and can be
// called safely more than once. Also verifies that Stop is safe when invoked
// while a tick is about to fire.
func TestPruner_GracefulShutdown(t *testing.T) {
	w := newMemDBWriter(t)

	// Short interval so the ticker is live; short retention so the prune
	// body does real work.
	p := NewPruner(w, PrunerConfig{
		Interval:  50 * time.Millisecond,
		Retention: time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	// Let at least one tick fire.
	time.Sleep(120 * time.Millisecond)

	// Stop must return within a reasonable time — if the goroutine is
	// deadlocked on a channel, this test will hit the deadline.
	stopDone := make(chan struct{})
	go func() {
		p.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
		// Good.
	case <-time.After(2 * time.Second):
		t.Fatal("Pruner.Stop blocked longer than 2s; expected prompt shutdown")
	}

	// Second Stop must be a no-op, not a panic on closed channel.
	p.Stop()
}

// TestPruner_StopViaContext verifies that cancelling the context passed to
// Start terminates the goroutine (even without calling Stop directly).
func TestPruner_StopViaContext(t *testing.T) {
	w := newMemDBWriter(t)

	p := NewPruner(w, PrunerConfig{
		Interval:  50 * time.Millisecond,
		Retention: time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	p.Start(ctx)

	time.Sleep(80 * time.Millisecond)
	cancel() // ask the goroutine to exit

	// Stop should still return promptly (the goroutine has already exited
	// via the ctx.Done branch, so Stop's close(done) is redundant but safe).
	done := make(chan struct{})
	go func() {
		p.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Pruner.Stop after ctx cancel blocked longer than 2s")
	}
}

// TestPruner_DefaultsApplied verifies that a zero PrunerConfig uses the
// documented defaults (24h / 365d). We cannot wait 24h, but we can poke the
// config methods directly to assert they return the compile-time constants.
func TestPruner_DefaultsApplied(t *testing.T) {
	cfg := PrunerConfig{}
	if got := cfg.interval(); got != defaultPrunerInterval {
		t.Errorf("default interval = %v, want %v", got, defaultPrunerInterval)
	}
	if got := cfg.retention(); got != defaultPrunerRetention {
		t.Errorf("default retention = %v, want %v", got, defaultPrunerRetention)
	}
	// Clock default: wall clock — must not be the zero value.
	if cfg.now().IsZero() {
		t.Error("default clock returned zero time")
	}
}

// TestPruner_RunsPeriodically verifies that the pruner DELETEs on subsequent
// ticks, not just on the initial immediate run. We seed a fresh event, let
// the first prune pass leave it alone, then mutate its timestamp to be
// expired and assert the next tick removes it.
func TestPruner_RunsPeriodically(t *testing.T) {
	w := newMemDBWriter(t)

	// Fix the clock so we can reason about cutoffs.
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

	// Seed a fresh event that survives the first prune.
	w.seedEvent(t, EventKindVoiceActivated, now.Add(-30*24*time.Hour))

	p := NewPruner(w, PrunerConfig{
		Interval:  40 * time.Millisecond,
		Retention: 365 * 24 * time.Hour,
		Clock:     func() time.Time { return now },
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	defer p.Stop()

	// Initial prune completed synchronously inside Start; the fresh event
	// should still be there.
	if got := w.countAll(t); got != 1 {
		t.Fatalf("after initial prune count = %d, want 1", got)
	}

	// Now backdate the event to 400 days old by UPDATE — simulates the clock
	// advancing past retention without actually sleeping for days.
	oldTs := now.Add(-400 * 24 * time.Hour).UTC().Format(time.RFC3339)
	if _, err := w.db.Exec(`UPDATE events SET timestamp = ?`, oldTs); err != nil {
		t.Fatalf("backdate event: %v", err)
	}

	// Wait for at least one more tick to fire (~40ms interval → poll up to
	// 2s to account for scheduler jitter under -race).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if w.countAll(t) == 0 {
			return // pruner picked up the stale row on a subsequent tick
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("periodic prune did not delete backdated event after 2s")
}

// TestCollector_StartsPruner is an integration-style test: a Collector
// constructed via NewCollector spins up a Pruner that actually deletes
// expired rows once Start runs. We use NewCollectorWithPruner to set a
// tight retention window so the assertion is fast.
func TestCollector_StartsPruner(t *testing.T) {
	w := newMemDBWriter(t)

	// Pin now and give the pruner a short retention so the seeded row
	// is past the cutoff.
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)

	// Seed a row that's 2 hours old; retention is 1 hour.
	w.seedEvent(t, EventKindSettingsReset, now.Add(-2*time.Hour))

	c := NewCollectorWithPruner(w, "sess-prune-it", PrunerConfig{
		Interval:  time.Hour, // not relied upon; the initial sync prune will clean
		Retention: time.Hour,
		Clock:     func() time.Time { return now },
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)
	c.Stop()

	if got := w.countAll(t); got != 0 {
		t.Errorf("expected pruner to clear stale row on collector start; got %d row(s)", got)
	}
}

// Compile-time guard: *memDBWriter (defined in collector_test.go) is the
// DBWriter the pruner consumes.
var _ DBWriter = (*memDBWriter)(nil)
