// Package voice — server.go
//
// VoiceServer exposes the voice pipeline state over HTTP so the editor UI
// and CLI commands can interact with push-to-talk without needing direct
// access to the Pipeline struct.
//
// Routes (mounted at /api/voice by the daemon):
//
//	POST /api/voice/ptt    — activate or deactivate PTT programmatically
//	GET  /api/voice/status — current pipeline state + last transcript/command
//
// The VoiceServer holds a reference to the running Pipeline and a small
// amount of state shared between the pipeline activity callback and the HTTP
// handlers (last transcript, last command).  All shared state is protected by
// a sync.RWMutex.
package voice

import (
	"encoding/json"
	"net/http"
	"sync"
)

// ---------------------------------------------------------------------------
// VoiceServer
// ---------------------------------------------------------------------------

// VoiceServer is the HTTP facade over the voice Pipeline.  Construct one with
// NewVoiceServer after creating and starting the Pipeline; then call
// Mount to register routes on the daemon's ServeMux.
type VoiceServer struct {
	pipeline *Pipeline

	mu             sync.RWMutex
	state          VoiceState
	lastTranscript string
	lastCommand    string
}

// NewVoiceServer constructs a VoiceServer wrapping the given Pipeline.
// The Pipeline must not be nil.  Call this before Pipeline.Start so that
// the activity callback is registered before the pipeline emits any states.
func NewVoiceServer(p *Pipeline) *VoiceServer {
	vs := &VoiceServer{
		pipeline: p,
		state:    VoiceStateIdle,
	}

	// Register ourselves as the pipeline activity observer.  The callback is
	// called from the pipeline goroutine; it must not block.
	p.OnActivity(vs.onActivity)
	return vs
}

// onActivity records state transitions from the Pipeline goroutine.
func (vs *VoiceServer) onActivity(s VoiceState) {
	vs.mu.Lock()
	vs.state = s
	vs.mu.Unlock()
}

// SetLastTranscript records the most recent transcription result.
// Called by the daemon glue after a PTT cycle completes.
func (vs *VoiceServer) SetLastTranscript(text string) {
	vs.mu.Lock()
	vs.lastTranscript = text
	vs.mu.Unlock()
}

// SetLastCommand records the most recent parsed command string.
func (vs *VoiceServer) SetLastCommand(cmd string) {
	vs.mu.Lock()
	vs.lastCommand = string(cmd)
	vs.mu.Unlock()
}

// HandlePTT is the exported HTTP handler for POST /api/voice/ptt.
// In production it is registered on the chi router by api.Server.Mount so
// that corsMiddleware and loggingMiddleware apply (FIX-SEC-02 / HIGH-03).
func (vs *VoiceServer) HandlePTT(w http.ResponseWriter, r *http.Request) {
	vs.handlePTT(w, r)
}

// HandleStatus is the exported HTTP handler for GET /api/voice/status.
// In production it is registered on the chi router by api.Server.Mount so
// that corsMiddleware and loggingMiddleware apply (FIX-SEC-02 / HIGH-03).
func (vs *VoiceServer) HandleStatus(w http.ResponseWriter, r *http.Request) {
	vs.handleStatus(w, r)
}

// Mount registers the voice HTTP endpoints directly on a plain http.ServeMux.
// This is provided for tests and tooling that operate outside the chi router.
// Production code must NOT call this — use api.Server.SetVoiceServer instead
// so that CORS and logging middleware are applied via the chi router.
//
//	POST /api/voice/ptt
//	GET  /api/voice/status
func (vs *VoiceServer) Mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/voice/ptt", vs.HandlePTT)
	mux.HandleFunc("GET /api/voice/status", vs.HandleStatus)
}

// ---------------------------------------------------------------------------
// POST /api/voice/ptt
// ---------------------------------------------------------------------------

// pttRequest is the JSON body for POST /api/voice/ptt.
type pttRequest struct {
	Active bool `json:"active"`
}

// handlePTT activates or deactivates push-to-talk on the pipeline.
//
// Request:  POST /api/voice/ptt
//
//	Content-Type: application/json
//	{"active": true}   ← activates PTT
//	{"active": false}  ← deactivates PTT
//
// Response 204 No Content on success.
// Response 400 if the body cannot be decoded.
func (vs *VoiceServer) handlePTT(w http.ResponseWriter, r *http.Request) {
	var req pttRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON body"}`, http.StatusBadRequest)
		return
	}

	vs.pipeline.SetPTT(req.Active)
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// GET /api/voice/status
// ---------------------------------------------------------------------------

// statusResponse is the JSON payload for GET /api/voice/status.
type statusResponse struct {
	Enabled        bool   `json:"enabled"`
	State          string `json:"state"`
	LastTranscript string `json:"lastTranscript"`
	LastCommand    string `json:"lastCommand"`
}

// handleStatus returns the current voice pipeline state.
//
// Response 200 with JSON body:
//
//	{
//	  "enabled": true,
//	  "state": "idle" | "listening" | "transcribing" | "dispatching" | "error",
//	  "lastTranscript": "...",
//	  "lastCommand": "..."
//	}
func (vs *VoiceServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	vs.mu.RLock()
	resp := statusResponse{
		Enabled:        true,
		State:          vs.state.String(),
		LastTranscript: vs.lastTranscript,
		LastCommand:    vs.lastCommand,
	}
	vs.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}
