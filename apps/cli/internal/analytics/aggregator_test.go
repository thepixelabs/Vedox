package analytics

// Tests for Aggregator: SQLite-tail cursor persistence, aggregation math,
// and GlobalDB roll-up correctness.

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// In-process fakes
// ---------------------------------------------------------------------------

// fakeWorkspace is a WorkspaceReader backed by an in-memory SQLite.
type fakeWorkspace struct {
	db *sql.DB
	mu sync.Mutex
}

func newFakeWorkspace(t *testing.T) *fakeWorkspace {
	t.Helper()
	// Use a named in-memory DB so we can open multiple handles to the same file.
	dbName := fmt.Sprintf("file:aggtest_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := sql.Open("sqlite", dbName)
	if err != nil {
		t.Fatalf("open workspace db: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	// Bootstrap the events table directly (no migration runner needed in tests).
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS events (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			kind       TEXT NOT NULL,
			timestamp  TEXT NOT NULL,
			session_id TEXT NOT NULL,
			properties TEXT
		)`); err != nil {
		t.Fatalf("create events table: %v", err)
	}
	return &fakeWorkspace{db: db}
}

func (f *fakeWorkspace) ReadDB() *sql.DB { return f.db }

// insertEvent inserts a raw event row for testing.
func (f *fakeWorkspace) insertEvent(t *testing.T, kind, timestamp, sessionID, properties string) int64 {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	var propsArg any
	if properties != "" {
		propsArg = properties
	}
	res, err := f.db.Exec(
		`INSERT INTO events(kind, timestamp, session_id, properties) VALUES (?, ?, ?, ?)`,
		kind, timestamp, sessionID, propsArg,
	)
	if err != nil {
		t.Fatalf("insertEvent: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// ---------------------------------------------------------------------------
// fakeGlobalDB implements GlobalDBWriter for testing.
// ---------------------------------------------------------------------------

type dailyKey struct{ date, kind string }

type fakeGlobalDB struct {
	mu     sync.Mutex
	daily  map[dailyKey]int
	cache  *AnalyticsCachePayload // last written cache
	caches []AnalyticsCachePayload
}

// AnalyticsCachePayload is a local alias so the fake can store what it receives.
type AnalyticsCachePayload struct {
	HasRun            bool
	TotalDocs         int
	DocsPerProject    string
	ChangeVelocity7d  int
	ChangeVelocity30d int
	UpdatedAt         string
}

func newFakeGlobalDB() *fakeGlobalDB {
	return &fakeGlobalDB{daily: make(map[dailyKey]int)}
}

func (f *fakeGlobalDB) IncrementDailyEvent(_ context.Context, date, kind string, delta int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.daily[dailyKey{date, kind}] += delta
	return nil
}

func (f *fakeGlobalDB) SumDailyEvents(_ context.Context, kind, fromDate, toDate string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var total int
	for k, v := range f.daily {
		if k.kind == kind && k.date >= fromDate && k.date <= toDate {
			total += v
		}
	}
	return total, nil
}

func (f *fakeGlobalDB) UpsertAnalyticsCacheRaw(_ context.Context,
	hasRun bool, totalDocs int, docsPerProject string,
	vel7d, vel30d int, updatedAt string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := AnalyticsCachePayload{
		HasRun:            hasRun,
		TotalDocs:         totalDocs,
		DocsPerProject:    docsPerProject,
		ChangeVelocity7d:  vel7d,
		ChangeVelocity30d: vel30d,
		UpdatedAt:         updatedAt,
	}
	f.cache = &p
	f.caches = append(f.caches, p)
	return nil
}

func (f *fakeGlobalDB) getDailyCount(date, kind string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.daily[dailyKey{date, kind}]
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestAggregator_TailCursorAdvances verifies that each cycle only processes
// rows with id > last_seen_id (SQLite-tail pattern).
func TestAggregator_TailCursorAdvances(t *testing.T) {
	ws := newFakeWorkspace(t)
	gdb := newFakeGlobalDB()
	a := NewAggregator(ws, gdb)

	today := time.Now().UTC().Format("2006-01-02")
	ts := time.Now().UTC().Format(time.RFC3339)

	// Insert 3 events.
	ws.insertEvent(t, EventKindDocumentPublished, ts, "s", "")
	ws.insertEvent(t, EventKindDocumentPublished, ts, "s", "")
	ws.insertEvent(t, EventKindDocumentPublished, ts, "s", "")

	// First cycle: should process all 3.
	a.cycle(context.Background())
	if n := gdb.getDailyCount(today, EventKindDocumentPublished); n != 3 {
		t.Errorf("after first cycle: daily count = %d, want 3", n)
	}

	// Insert 2 more events after the cursor has advanced.
	ws.insertEvent(t, EventKindDocumentPublished, ts, "s", "")
	ws.insertEvent(t, EventKindDocumentPublished, ts, "s", "")

	// Second cycle: should process only the 2 new events.
	a.cycle(context.Background())
	if n := gdb.getDailyCount(today, EventKindDocumentPublished); n != 5 {
		t.Errorf("after second cycle: daily count = %d, want 5", n)
	}
}

// TestAggregator_MultiKindAggregation verifies that different event kinds are
// rolled up into separate daily counters.
func TestAggregator_MultiKindAggregation(t *testing.T) {
	ws := newFakeWorkspace(t)
	gdb := newFakeGlobalDB()
	a := NewAggregator(ws, gdb)

	today := time.Now().UTC().Format("2006-01-02")
	ts := time.Now().UTC().Format(time.RFC3339)

	ws.insertEvent(t, EventKindDocumentPublished, ts, "s", "")
	ws.insertEvent(t, EventKindDocumentPublished, ts, "s", "")
	ws.insertEvent(t, EventKindAgentTriggered, ts, "s", "")
	ws.insertEvent(t, EventKindSearchExecuted, ts, "s", "")
	ws.insertEvent(t, EventKindSearchExecuted, ts, "s", "")
	ws.insertEvent(t, EventKindSearchExecuted, ts, "s", "")

	a.cycle(context.Background())

	if n := gdb.getDailyCount(today, EventKindDocumentPublished); n != 2 {
		t.Errorf("document.published = %d, want 2", n)
	}
	if n := gdb.getDailyCount(today, EventKindAgentTriggered); n != 1 {
		t.Errorf("agent.triggered = %d, want 1", n)
	}
	if n := gdb.getDailyCount(today, EventKindSearchExecuted); n != 3 {
		t.Errorf("search.executed = %d, want 3", n)
	}
}

// TestAggregator_EmptyCycleIsNoOp verifies that a cycle with no new events
// does not write anything to GlobalDB and does not crash.
func TestAggregator_EmptyCycleIsNoOp(t *testing.T) {
	ws := newFakeWorkspace(t)
	gdb := newFakeGlobalDB()
	a := NewAggregator(ws, gdb)

	// Cycle with no events at all.
	a.cycle(context.Background())

	// The cache is still written (refreshCache runs after tailAndRollup).
	// That's correct: an empty cycle still updates the cache with zeros.
	// The important thing is nothing panics and daily counters stay zero.
	today := time.Now().UTC().Format("2006-01-02")
	if n := gdb.getDailyCount(today, EventKindDocumentPublished); n != 0 {
		t.Errorf("daily count after empty cycle = %d, want 0", n)
	}
}

// TestAggregator_HasRunFlipsAfterFirstCycle verifies that HasRun() is false
// before the first cycle and true after.
func TestAggregator_HasRunFlipsAfterFirstCycle(t *testing.T) {
	ws := newFakeWorkspace(t)
	gdb := newFakeGlobalDB()
	a := NewAggregator(ws, gdb)

	if a.HasRun() {
		t.Error("HasRun() = true before first cycle; want false")
	}

	a.cycle(context.Background())

	if !a.HasRun() {
		t.Error("HasRun() = false after first cycle; want true")
	}
}

// TestAggregator_CacheTotalDocs verifies that refreshCache computes TotalDocs
// correctly from accumulated daily events.
func TestAggregator_CacheTotalDocs(t *testing.T) {
	ws := newFakeWorkspace(t)
	gdb := newFakeGlobalDB()
	a := NewAggregator(ws, gdb)

	ts := time.Now().UTC().Format(time.RFC3339)
	for i := 0; i < 7; i++ {
		ws.insertEvent(t, EventKindDocumentPublished, ts, "s", "")
	}

	a.cycle(context.Background())

	if gdb.cache == nil {
		t.Fatal("cache was not written by aggregator")
	}
	if gdb.cache.TotalDocs != 7 {
		t.Errorf("TotalDocs = %d, want 7", gdb.cache.TotalDocs)
	}
	if !gdb.cache.HasRun {
		t.Error("HasRun in cache = false; want true")
	}
}

// TestAggregator_CursorPersistsBetweenCycles verifies that lastID advances
// correctly so events are never double-counted.
func TestAggregator_CursorPersistsBetweenCycles(t *testing.T) {
	ws := newFakeWorkspace(t)
	gdb := newFakeGlobalDB()
	a := NewAggregator(ws, gdb)

	ts := time.Now().UTC().Format(time.RFC3339)
	today := time.Now().UTC().Format("2006-01-02")

	// Cycle 1: 4 events.
	for i := 0; i < 4; i++ {
		ws.insertEvent(t, EventKindRepoRegistered, ts, "s", "")
	}
	a.cycle(context.Background())

	// Cycle 2: 3 more events.
	for i := 0; i < 3; i++ {
		ws.insertEvent(t, EventKindRepoRegistered, ts, "s", "")
	}
	a.cycle(context.Background())

	// Total should be 4 + 3 = 7, not 4+4+3 = 11 (double-counted first batch).
	if n := gdb.getDailyCount(today, EventKindRepoRegistered); n != 7 {
		t.Errorf("daily count after 2 cycles = %d, want 7 (no double-count)", n)
	}
}

// TestAggregator_StartStop verifies that Start/Stop do not deadlock and that
// at least one cycle runs during the alive window.
func TestAggregator_StartStop(t *testing.T) {
	ws := newFakeWorkspace(t)
	gdb := newFakeGlobalDB()
	a := NewAggregator(ws, gdb)

	ts := time.Now().UTC().Format(time.RFC3339)
	ws.insertEvent(t, EventKindOnboardingStarted, ts, "s", "")

	ctx, cancel := context.WithCancel(context.Background())
	a.Start(ctx)

	// Allow the startup cycle to run.
	time.Sleep(100 * time.Millisecond)
	cancel()
	a.Stop()

	if !a.HasRun() {
		t.Error("HasRun() = false after Start/Stop; expected at least one cycle")
	}
}
