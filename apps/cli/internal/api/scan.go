package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

// detectedProject mirrors the DetectedProject shape the SvelteKit onboarding
// store expects. It is intentionally a small, stable subset of scanner.Project:
// the frontend only needs enough to render a checklist and remember what the
// user picked. Adding fields here is fine, but renaming any of these four
// breaks the onboarding step — keep them aligned with
// apps/editor/src/lib/stores/onboarding.svelte.ts: DetectedProject.
type detectedProject struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	HasGit   bool   `json:"hasGit"`
	DocCount int    `json:"docCount"`
}

// scanSummaryResponse is the JSON body returned by GET /api/scan.
// Wrapping the array in {projects: [...]} matches the onboarding fetch in
// ScanProjects.svelte (which pulls data.projects) and leaves room to grow
// the payload with metadata like scannedAt without breaking the client.
type scanSummaryResponse struct {
	Projects []detectedProject `json:"projects"`
}

// startScanRequest is the optional JSON body for POST /api/scan.
type startScanRequest struct {
	// WorkspaceRoot overrides the server's configured workspace root.
	// If empty or omitted, the server's configured workspace root is used.
	WorkspaceRoot string `json:"workspaceRoot"`
}

// startScanResponse is the JSON body returned by POST /api/scan.
type startScanResponse struct {
	JobID string `json:"jobId"`
}

// handleStartScan handles POST /api/scan.
//
// It starts an async workspace scan and returns the job ID immediately.
// The caller should poll GET /api/scan/:jobId to track progress.
//
// Request body (optional JSON):
//
//	{"workspaceRoot": "/path/to/workspace"}
//
// If workspaceRoot is absent the server's configured workspace root is used.
func (s *Server) handleStartScan(w http.ResponseWriter, r *http.Request) {
	root := s.workspaceRoot

	// Parse optional body. We tolerate a missing body gracefully.
	if r.ContentLength > 0 {
		var req startScanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "VDX-000", "invalid JSON body")
			return
		}
		if req.WorkspaceRoot != "" {
			// Resolve to absolute path and verify it exists.
			abs, err := filepath.Abs(req.WorkspaceRoot)
			if err != nil {
				writeError(w, http.StatusBadRequest, "VDX-100",
					"workspaceRoot could not be resolved to an absolute path")
				return
			}
			root = abs
		}
	}

	job := s.jobStore.StartScan(root)
	writeJSON(w, http.StatusAccepted, startScanResponse{JobID: job.ID})
}

// handleGetScanSummary handles GET /api/scan.
//
// It returns the projects from the most recently completed scan for the
// server's workspace root. When no cached scan exists (first-run, or after a
// daemon restart), it falls back to a synchronous scan so the onboarding
// flow in the editor never has to kick off a POST + poll dance just to
// show the first screen.
//
// The response shape matches the frontend's DetectedProject type:
//
//	{"projects": [{"path": "...", "name": "...", "hasGit": true, "docCount": 12}, ...]}
//
// Note the difference vs. POST /api/scan (which returns {"jobId": "..."})
// and GET /api/scan/{jobId} (which returns the full ScanJob). This summary
// endpoint is deliberately synchronous and lightweight — it exists so the
// editor's onboarding step can render immediately instead of double-dipping.
func (s *Server) handleGetScanSummary(w http.ResponseWriter, r *http.Request) {
	// Fast path: a completed scan job exists for this workspace. Use the
	// snapshot accessor so we never race with an in-flight runScan.
	if job, ok := s.jobStore.LastCompletedSnapshot(s.workspaceRoot); ok {
		out := make([]detectedProject, 0, len(job.Projects))
		for _, p := range job.Projects {
			out = append(out, detectedProject{
				Path:     p.AbsPath,
				Name:     p.Name,
				HasGit:   true, // Scanner only records projects with .git present.
				DocCount: p.DocCount,
			})
		}
		writeJSON(w, http.StatusOK, scanSummaryResponse{Projects: out})
		return
	}

	// Slow path: no cached scan — run one synchronously so the response is
	// never empty on first-run. Scanner.Scan is bounded by maxDepth (5) and
	// skips node_modules/vendor/hidden dirs, so worst-case it's still a
	// handful of seconds on a large home directory.
	scanned, err := s.jobStore.Scanner().Scan(s.workspaceRoot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "VDX-100",
			"workspace scan failed")
		return
	}

	out := make([]detectedProject, 0, len(scanned))
	for _, p := range scanned {
		out = append(out, detectedProject{
			Path:     p.AbsPath,
			Name:     p.Name,
			HasGit:   true,
			DocCount: p.DocCount,
		})
	}
	writeJSON(w, http.StatusOK, scanSummaryResponse{Projects: out})
}

// handleGetScanJob handles GET /api/scan/:jobId.
//
// Returns the current state of the scan job identified by jobId.
// The response shape matches the ScanJob struct.
//
// Returns 404 if the job ID is unknown (including after a server restart).
func (s *Server) handleGetScanJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "VDX-000", "missing jobId parameter")
		return
	}

	// Snapshot (not Get) so writeJSON encodes a value copy taken under the
	// JobStore's lock. Get returns an aliased *ScanJob that would race with
	// runScan's concurrent field mutations during JSON encoding.
	job, ok := s.jobStore.Snapshot(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "VDX-101",
			"scan job not found; it may have been lost on server restart — reissue POST /api/scan")
		return
	}

	writeJSON(w, http.StatusOK, job)
}
