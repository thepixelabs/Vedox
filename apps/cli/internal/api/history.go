package api

import (
	"net/http"
	"os"
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

	// Determine which filesystem layout the caller is running with, then pick
	// the (repoRoot, filePath) pair that targets the right git repo and the
	// right tree path inside it:
	//
	//   Layout A — "project is its own git repo" (e.g. symlinked external repos):
	//       workspaceRoot/project/.git exists. The file is committed at the
	//       repo-relative tree path equal to docPath, NOT the project-prefixed
	//       workspace-relative relPath. Use repoRoot=workspaceRoot/project and
	//       filePath=docPath.
	//
	//   Layout B — "workspace is the git repo, project is a subdirectory":
	//       workspaceRoot/.git exists (or is discoverable). The file is
	//       committed at tree path project/docPath == relPath. Use
	//       repoRoot=workspaceRoot and filePath=relPath.
	//
	// A previous revision always passed relPath to both attempts and relied on
	// a fallback cascade, which silently returned empty history in Layout B
	// because the Layout-A attempt walked up to the workspace repo and then
	// failed to match the doubly-prefixed pathspec. Probing for the inner
	// .git directory first makes the choice deterministic and removes the
	// cascade entirely.
	var (
		repoRoot string
		filePath string
	)
	projectDir := filepath.Join(s.workspaceRoot, project)
	if isGitRepo(projectDir) {
		repoRoot = projectDir
		filePath = docPath
	} else {
		repoRoot = s.workspaceRoot
		filePath = relPath
	}

	entries, err := history.FileHistoryContext(r.Context(), repoRoot, filePath, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "VDX-000",
			"failed to read git history: "+sanitiseError(err))
		return
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

// isGitRepo reports whether dir is the working tree of a git repository.
// It checks for a .git entry (directory, file, or symlink) inside dir — which
// covers normal clones, git-worktree checkouts (where .git is a file), and
// symlinked-repo setups. It does not shell out to git.
func isGitRepo(dir string) bool {
	info, err := os.Lstat(filepath.Join(dir, ".git"))
	if err != nil {
		return false
	}
	return info != nil
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
