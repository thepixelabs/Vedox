package api

// GET /api/analytics/summary
//
// Returns a point-in-time analytics summary. When the WS-K aggregator has
// completed at least one cycle, the response is sourced from the
// analytics_cache row (precomputed, O(1) read). On a fresh installation, or
// before the first aggregation cycle completes, the handler falls back to
// reading events_daily directly — the same fast range queries as before.
//
// The handler is read-only and safe to call concurrently.

import (
	"net/http"
	"time"
)

// analyticsSummaryResponse is the JSON shape for GET /api/analytics/summary.
// All numeric fields default to zero when no event data is present.
type analyticsSummaryResponse struct {
	// TotalDocs is the sum of all "document.published" events across all time.
	TotalDocs int `json:"total_docs"`
	// DocsLast7Days is the sum of "document.published" events in the last 7 days.
	DocsLast7Days int `json:"docs_last_7_days"`
	// DocsLast30Days is the sum of "document.published" events in the last 30 days.
	DocsLast30Days int `json:"docs_last_30_days"`
	// AgentTriggeredLast7Days is the sum of "agent.triggered" events in the last 7 days.
	AgentTriggeredLast7Days int `json:"agent_triggered_last_7_days"`
	// AgentTriggeredLast30Days is the sum of "agent.triggered" events in the last 30 days.
	AgentTriggeredLast30Days int `json:"agent_triggered_last_30_days"`
	// ChangeVelocity7d is the number of document.published events in the last 7 days
	// as recorded by the analytics_cache (identical to DocsLast7Days when cached).
	ChangeVelocity7d int `json:"change_velocity_7d"`
	// ChangeVelocity30d is the number of document.published events in the last 30 days.
	ChangeVelocity30d int `json:"change_velocity_30d"`
	// DocsPerProject is a JSON string mapping project name to doc count.
	// Empty object "{}" when the aggregator has not run yet.
	DocsPerProject string `json:"docs_per_project"`
	// PipelineReady is false while the events_daily pipeline has not yet run.
	// When false, all counters may be zero or stale. The editor uses this flag
	// to render a "data pipeline not active" notice rather than a zero-state
	// that might look like a bug.
	PipelineReady bool `json:"pipeline_ready"`
}

// handleAnalyticsSummary implements GET /api/analytics/summary.
func (s *Server) handleAnalyticsSummary(w http.ResponseWriter, r *http.Request) {
	if s.globalDB == nil {
		writeError(w, http.StatusServiceUnavailable, "VDX-503",
			"global database not available; start the daemon to enable analytics")
		return
	}

	ctx := r.Context()
	now := time.Now().UTC()

	// Helper: sum events_daily for a given kind over a date range [from, to].
	// Both dates are ISO 8601 (YYYY-MM-DD). Returns 0 on any error — analytics
	// must never propagate DB errors back to the editor as hard failures.
	sumRange := func(kind, from, to string) int {
		total, err := s.globalDB.SumDailyEvents(ctx, kind, from, to)
		if err != nil {
			return 0
		}
		return total
	}

	today := now.Format("2006-01-02")
	day7ago := now.AddDate(0, 0, -7).Format("2006-01-02")
	day30ago := now.AddDate(0, 0, -30).Format("2006-01-02")
	epoch := "2000-01-01" // effectively "all time"

	// Attempt to read the precomputed cache row written by the aggregator.
	// If the aggregator has run, the cache row provides pre-aggregated counts
	// for docs_per_project and velocity — supplemented by live SumDailyEvents
	// calls for the remaining fields which are fast bounded range scans.
	var (
		pipelineReady  bool
		docsPerProject = "{}"
		vel7d          int
		vel30d         int
	)

	if cache, err := s.globalDB.GetAnalyticsCache(ctx); err == nil && cache != nil && cache.HasRun {
		pipelineReady = true
		docsPerProject = cache.DocsPerProject
		if docsPerProject == "" {
			docsPerProject = "{}"
		}
		vel7d = cache.ChangeVelocity7d
		vel30d = cache.ChangeVelocity30d
	} else {
		// Cache not yet populated — compute velocity live from events_daily.
		vel7d = sumRange("document.published", day7ago, today)
		vel30d = sumRange("document.published", day30ago, today)
	}

	resp := analyticsSummaryResponse{
		TotalDocs:                sumRange("document.published", epoch, today),
		DocsLast7Days:            sumRange("document.published", day7ago, today),
		DocsLast30Days:           sumRange("document.published", day30ago, today),
		AgentTriggeredLast7Days:  sumRange("agent.triggered", day7ago, today),
		AgentTriggeredLast30Days: sumRange("agent.triggered", day30ago, today),
		ChangeVelocity7d:         vel7d,
		ChangeVelocity30d:        vel30d,
		DocsPerProject:           docsPerProject,
		PipelineReady:            pipelineReady,
	}

	writeJSON(w, http.StatusOK, resp)
}
