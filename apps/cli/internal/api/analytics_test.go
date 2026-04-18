package api

// Tests for the analytics summary endpoint:
//
//	GET /api/analytics/summary — handleAnalyticsSummary
//
// The pipeline is not yet wired so all counts default to zero. Tests verify:
//   - correct JSON shape with pipeline_ready=false when no data exists
//   - counts increment correctly once events_daily rows are present
//   - 503 is returned when no GlobalDB is injected

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// analyticsFixture is a test server with GlobalDB injected.
type analyticsFixture struct {
	server *httptest.Server
	gdb    *db.GlobalDB
}

func newAnalyticsFixture(t *testing.T) *analyticsFixture {
	t.Helper()

	gdbPath := filepath.Join(t.TempDir(), "global.db")
	gdb, err := db.OpenGlobalDB(gdbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB: %v", err)
	}
	t.Cleanup(func() { _ = gdb.Close() })

	wsRoot := t.TempDir()
	adapter, err := store.NewLocalAdapter(wsRoot, nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}
	wsDB, err := db.Open(db.Options{WorkspaceRoot: wsRoot})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = wsDB.Close() })

	srv := NewServer(adapter, wsDB, wsRoot, scanner.NewJobStore(), ai.NewJobStore(3), store.NewProjectRegistry(), agentauth.PassthroughAuth())
	srv.SetGlobalDB(gdb)

	mux := http.NewServeMux()
	srv.Mount(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return &analyticsFixture{server: ts, gdb: gdb}
}

// ── GET /api/analytics/summary ────────────────────────────────────────────────

// TestAnalyticsSummary_Empty verifies the zero-state response shape.
// pipeline_ready must be false and all counts must be 0.
func TestAnalyticsSummary_Empty(t *testing.T) {
	f := newAnalyticsFixture(t)

	resp, err := f.server.Client().Get(f.server.URL + "/api/analytics/summary")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got analyticsSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.PipelineReady {
		t.Error("pipeline_ready must be false before the pipeline is wired")
	}
	if got.TotalDocs != 0 {
		t.Errorf("total_docs = %d, want 0 for empty DB", got.TotalDocs)
	}
	if got.DocsLast7Days != 0 {
		t.Errorf("docs_last_7_days = %d, want 0", got.DocsLast7Days)
	}
	if got.DocsLast30Days != 0 {
		t.Errorf("docs_last_30_days = %d, want 0", got.DocsLast30Days)
	}
	if got.AgentTriggeredLast7Days != 0 {
		t.Errorf("agent_triggered_last_7_days = %d, want 0", got.AgentTriggeredLast7Days)
	}
	if got.AgentTriggeredLast30Days != 0 {
		t.Errorf("agent_triggered_last_30_days = %d, want 0", got.AgentTriggeredLast30Days)
	}
}

// TestAnalyticsSummary_WithData seeds events_daily and verifies counts.
// We insert events in the past 3 days and one event in the distant past.
func TestAnalyticsSummary_WithData(t *testing.T) {
	f := newAnalyticsFixture(t)
	ctx := context.Background()

	// Seed events: 2 published docs 3 days ago, 1 published doc 60 days ago.
	if err := f.gdb.IncrementDailyEvent(ctx, "2026-04-12", "document.published", 2); err != nil {
		t.Fatalf("seed 7d: %v", err)
	}
	if err := f.gdb.IncrementDailyEvent(ctx, "2026-02-14", "document.published", 1); err != nil {
		t.Fatalf("seed 60d: %v", err)
	}
	// Seed 5 agent triggers 2 days ago.
	if err := f.gdb.IncrementDailyEvent(ctx, "2026-04-13", "agent.triggered", 5); err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	resp, err := f.server.Client().Get(f.server.URL + "/api/analytics/summary")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got analyticsSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Total docs should include all time (2 + 1 = 3).
	if got.TotalDocs != 3 {
		t.Errorf("total_docs = %d, want 3", got.TotalDocs)
	}
	// 7-day window: only the 2 from 2026-04-12 (within 7 days of 2026-04-15).
	if got.DocsLast7Days != 2 {
		t.Errorf("docs_last_7_days = %d, want 2", got.DocsLast7Days)
	}
	// 30-day window: same, the 60-day-old event is outside the window.
	if got.DocsLast30Days != 2 {
		t.Errorf("docs_last_30_days = %d, want 2", got.DocsLast30Days)
	}
	// Agent triggers: 5 in 7 days and 30 days.
	if got.AgentTriggeredLast7Days != 5 {
		t.Errorf("agent_triggered_last_7_days = %d, want 5", got.AgentTriggeredLast7Days)
	}
	if got.AgentTriggeredLast30Days != 5 {
		t.Errorf("agent_triggered_last_30_days = %d, want 5", got.AgentTriggeredLast30Days)
	}
}

// TestAnalyticsSummary_NilGlobalDB verifies 503 when GlobalDB is absent.
func TestAnalyticsSummary_NilGlobalDB(t *testing.T) {
	wsRoot := t.TempDir()
	adapter, _ := store.NewLocalAdapter(wsRoot, nil)
	wsDB, _ := db.Open(db.Options{WorkspaceRoot: wsRoot})
	t.Cleanup(func() { _ = wsDB.Close() })

	srv := NewServer(adapter, wsDB, wsRoot, scanner.NewJobStore(), ai.NewJobStore(3), store.NewProjectRegistry(), agentauth.PassthroughAuth())
	// No SetGlobalDB call.

	mux := http.NewServeMux()
	srv.Mount(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	resp, err := ts.Client().Get(ts.URL + "/api/analytics/summary")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

// TestAnalyticsSummary_ContentType verifies response is application/json.
func TestAnalyticsSummary_ContentType(t *testing.T) {
	f := newAnalyticsFixture(t)

	resp, err := f.server.Client().Get(f.server.URL + "/api/analytics/summary")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
