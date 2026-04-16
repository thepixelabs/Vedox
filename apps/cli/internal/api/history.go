package api

import (
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/vedox/vedox/internal/history"
)

// historyResponse is the JSON shape returned by GET /api/projects/{project}/docs/*/history.
type historyResponse struct {
	DocPath string                 `json:"docPath"`
	Entries []history.HistoryEntry `json:"entries"`
}

// handleDocHistory returns the git-backed history timeline for a single doc.
//
// Route: GET /api/projects/{project}/docs/{docPath}/history
//
// Query params:
//
//	limit  — max entries to return (default 50, max 500)
//
// The handler validates the doc path against the workspace boundary, then
// shells out to git via history.FileHistory. Results are returned as a JSON
// timeline ordered most-recent first.
func (s *Server) handleDocHistory(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	docPath := chi.URLParam(r, "*")

	// The wildcard ends in "/history"; strip that suffix to get the real doc path.
	if strings.HasSuffix(docPath, "/history") {
		docPath = docPath[:len(docPath)-len("/history")]
	} else {
		writeError(w, http.StatusBadRequest, "VDX-000", "missing document path before /history")
		return
	}

	if docPath == "" {
		writeError(w, http.StatusBadRequest, "VDX-000", "missing document path")
		return
	}

	// Validate and resolve the doc path inside the workspace boundary.
	relPath, err := s.validateDocPath(project, docPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VDX-005", "invalid document path")
		return
	}

	// Parse ?limit query param. Default 50, cap at 500.
	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if n, err := strconv.Atoi(lStr); err == nil && n > 0 {
			if n > 500 {
				n = 500
			}
			limit = n
		}
	}

	// The file path for git is workspace-relative so git -C workspaceRoot works.
	filePath := relPath

	// Determine the repo root. We try the project directory first (useful for
	// symlinked external repos), then fall back to the workspace root.
	repoRoot := filepath.Join(s.workspaceRoot, project)

	entries, err := history.FileHistoryContext(r.Context(), repoRoot, filePath, limit)
	if err != nil {
		// Try workspace root as fallback repo root.
		entries, err = history.FileHistoryContext(r.Context(), s.workspaceRoot, filePath, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "VDX-000",
				"failed to read git history: "+sanitiseError(err))
			return
		}
	}

	// Ensure non-nil slice so JSON renders [] not null.
	if entries == nil {
		entries = []history.HistoryEntry{}
	}

	writeJSON(w, http.StatusOK, historyResponse{
		DocPath: relPath,
		Entries: entries,
	})
}

// sanitiseError returns the error message without any user-controlled content
// that could be reflected back. For git errors this is safe — the message is
// from the git binary, not from request input.
func sanitiseError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// Never reflect more than 200 chars back to the caller.
	if len(msg) > 200 {
		return msg[:200] + "..."
	}
	return msg
}
