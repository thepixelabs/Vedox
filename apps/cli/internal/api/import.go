package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/vedox/vedox/internal/importer"
	"github.com/vedox/vedox/internal/store"
)

// importRequest is the JSON body expected by POST /api/import.
type importRequest struct {
	// SrcProjectRoot is the absolute path to the source project on the local
	// filesystem. Must be absolute and must not reside inside the Vedox
	// workspace root.
	SrcProjectRoot string `json:"srcProjectRoot"`

	// ProjectName is the sub-directory name that will be created inside the
	// Vedox workspace to hold the imported files. Must be a single path
	// segment (no forward or back slashes). If omitted, the base name of
	// SrcProjectRoot is used.
	ProjectName string `json:"projectName"`
}

// importResponse is the JSON body returned by a successful POST /api/import.
// It mirrors importer.ImportResult so the frontend can display a rich summary.
type importResponse struct {
	Imported []string `json:"imported"`
	Skipped  []string `json:"skipped"`
	Warnings []string `json:"warnings"`
}

// handleImport handles POST /api/import.
//
// It validates the request, delegates to importer.Import, and returns a
// summary of the import operation.
//
// Validation:
//   - srcProjectRoot must be provided, absolute, and must exist on disk.
//   - srcProjectRoot must not be inside (or equal to) the workspace root.
//   - All path components are sanitised: filepath.Clean + filepath.IsAbs.
//   - projectName must be a single path segment (no slashes) — defaults to
//     the base name of srcProjectRoot if omitted.
//
// Error codes:
//
//	VDX-000 — malformed JSON body
//	VDX-005 — path traversal: srcProjectRoot is inside the workspace root
//	VDX-200 — srcProjectRoot is missing, not absolute, or does not exist
//	VDX-201 — projectName is invalid (contains path separators)
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	// Reject oversized bodies early. 1MB is generous for a path + name pair.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req importRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-000", "invalid JSON body")
		return
	}

	// Validate srcProjectRoot: must be present and absolute.
	rawSrc := strings.TrimSpace(req.SrcProjectRoot)
	src := filepath.Clean(expandTilde(rawSrc))

	if src == "" || src == "." {
		writeError(w, http.StatusBadRequest, "VDX-200",
			"srcProjectRoot is required")
		return
	}
	if !filepath.IsAbs(src) {
		writeError(w, http.StatusBadRequest, "VDX-200",
			"srcProjectRoot must be an absolute path (start with /)")
		return
	}

	// Verify the path exists on disk.
	if _, err := os.Stat(src); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-200",
			"srcProjectRoot does not exist or is not accessible")
		return
	}

	// Path traversal guard: srcProjectRoot must not be inside workspaceRoot.
	// We use real paths (symlink-resolved) for the comparison so symlink tricks
	// don't bypass the check.
	srcReal := resolveReal(src)
	workspaceReal := resolveReal(s.workspaceRoot)
	workspaceWithSep := workspaceReal + string(os.PathSeparator)
	if srcReal == workspaceReal || strings.HasPrefix(srcReal, workspaceWithSep) {
		writeError(w, http.StatusBadRequest, "VDX-005",
			"srcProjectRoot must not be inside the Vedox workspace root")
		return
	}

	// Derive projectName from the request or fall back to the base dir name.
	projectName := strings.TrimSpace(req.ProjectName)
	if projectName == "" {
		projectName = filepath.Base(src)
	}

	// projectName must be a single path segment — reject anything with slashes.
	if strings.ContainsAny(projectName, `/\`) || projectName == "." || projectName == ".." {
		writeError(w, http.StatusBadRequest, "VDX-201",
			"projectName must be a single directory name with no path separators")
		return
	}

	result, err := importer.Import(src, projectName, s.workspaceRoot, s.store, s.db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "VDX-202",
			"import failed: "+err.Error())
		return
	}

	// Register a LocalAdapter for the imported project so it appears in
	// GET /api/projects immediately (the scanner won't find it — no .git dir).
	projectDir := filepath.Join(s.workspaceRoot, projectName)
	adapter, adapterErr := store.NewLocalAdapter(projectDir, nil)
	if adapterErr != nil {
		slog.Warn("api.handleImport: could not register project adapter (project imported but may not appear in list until rescan)",
			"projectName", projectName, "error", adapterErr.Error())
	} else {
		s.registry.Register(projectName, adapter)
	}

	// Invalidate the cached scan so GET /api/projects reflects the new project
	// on the very next request (without requiring a manual rescan).
	s.jobStore.InvalidateCache(s.workspaceRoot)

	writeJSON(w, http.StatusOK, importResponse{
		Imported: result.Imported,
		Skipped:  result.Skipped,
		Warnings: result.Warnings,
	})
}

// resolveReal attempts filepath.EvalSymlinks and falls back to filepath.Clean
// if symlink resolution fails (e.g. path doesn't exist yet).
func resolveReal(p string) string {
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return real
}
