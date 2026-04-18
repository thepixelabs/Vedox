package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/vedox/vedox/internal/ai"
)

// handleAIProviders handles GET /api/ai/providers.
//
// Returns the list of known AI CLI providers (with availability flags) plus
// AlterGo account discovery results. Never errors — missing binaries and a
// missing ~/.altergo directory both produce valid JSON with Available=false.
func (s *Server) handleAIProviders(w http.ResponseWriter, r *http.Request) {
	type providersResponse struct {
		Providers []ai.ProviderInfo `json:"providers"`
		Altergo   ai.AltergoInfo    `json:"altergo"`
	}

	writeJSON(w, http.StatusOK, providersResponse{
		Providers: ai.DetectAvailableProviders(),
		Altergo:   ai.DiscoverAltergo(),
	})
}

// generateNamesRequest is the JSON body expected by POST /api/ai/generate-names.
type generateNamesRequest struct {
	Provider   string               `json:"provider"`
	Account    string               `json:"account"`
	Params     ai.GenerationParams  `json:"params"`
	Count      int                  `json:"count"`
	Refinement *ai.RefinementInput  `json:"refinement"`
}

// handleGenerateNames handles POST /api/ai/generate-names.
//
// Validates the request, submits an async job to the AI job store, and
// immediately returns the job ID so the client can poll for results.
//
// Error codes:
//
//	VDX-000 — malformed JSON body
//	VDX-400 — invalid request (unknown provider, count out of range)
func (s *Server) handleGenerateNames(w http.ResponseWriter, r *http.Request) {
	// Reject oversized bodies early. The prompt is bounded, so 64KB is generous.
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)

	var req generateNamesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-000", "invalid JSON body")
		return
	}

	providerID := ai.ProviderID(req.Provider)
	if ai.BinaryForProvider(providerID) == "" {
		writeError(w, http.StatusBadRequest, "VDX-400", "unknown provider: "+req.Provider)
		return
	}

	// Allow count=0 to mean "use the default" (10) rather than rejecting it.
	count := req.Count
	if count < 0 || count > 20 {
		writeError(w, http.StatusBadRequest, "VDX-400", "count must be between 1 and 20")
		return
	}
	if count == 0 {
		count = 10
	}

	job := s.aiJobStore.Submit(ai.GenerationRequest{
		Provider:    providerID,
		AccountName: req.Account,
		Params:      req.Params,
		Count:       count,
		Refinement:  req.Refinement,
	})

	writeJSON(w, http.StatusAccepted, map[string]string{"jobId": job.ID})
}

// handleGenerateNamesStatus handles GET /api/ai/generate-names/{jobId}.
//
// Returns the current state of the generation job. The shape is identical
// whether the job is pending, running, done, or error so the client can use
// a single polling loop.
//
// Error codes:
//
//	VDX-404 — job not found (unknown ID or server restart cleared in-memory state)
func (s *Server) handleGenerateNamesStatus(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")

	// Snapshot (not Get) so writeJSON encodes a value copy taken under the
	// JobStore's lock. Get returns an aliased *GenerationJob that would
	// race with run()'s concurrent field mutations during JSON encoding.
	job, ok := s.aiJobStore.Snapshot(jobID)
	if !ok {
		writeError(w, http.StatusNotFound, "VDX-404",
			"job not found; it may have expired or the server was restarted")
		return
	}

	writeJSON(w, http.StatusOK, job)
}
