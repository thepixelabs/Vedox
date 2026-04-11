package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

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

	job := s.jobStore.Get(jobID)
	if job == nil {
		writeError(w, http.StatusNotFound, "VDX-101",
			"scan job not found; it may have been lost on server restart — reissue POST /api/scan")
		return
	}

	writeJSON(w, http.StatusOK, job)
}
