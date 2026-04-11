package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// errorResponse is the canonical JSON error body returned by all API endpoints.
// The error code matches the VDX taxonomy defined in internal/errors.
type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON serialises v as JSON and writes it to w with the given status code.
// Content-Type is always application/json. Serialisation failures are logged
// and a 500 is sent — this should never happen for our internal types.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// At this point headers are already sent; we can only log.
		slog.Error("api: failed to serialise JSON response", "error", err.Error())
	}
}

// writeError writes a canonical VDX error JSON body. code should be a VDX
// error code string (e.g. "VDX-005"); message is the user-facing explanation.
// Never include internal stack traces or file contents in message.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{
		Code:    code,
		Message: message,
	})
}
