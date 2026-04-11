package api

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/vedox/vedox/internal/links"
	"github.com/vedox/vedox/internal/store"
)

// linkRequest is the JSON body for POST /api/link.
type linkRequest struct {
	// ExternalRoot is the absolute filesystem path to the external project
	// directory. It must exist and must not be inside the Vedox workspace.
	ExternalRoot string `json:"externalRoot"`

	// ProjectName is the logical name under which the project will be visible
	// inside Vedox. Must be non-empty and must not contain "/" or "..".
	ProjectName string `json:"projectName"`
}

// linkResponse is the JSON body returned by POST /api/link on success.
type linkResponse struct {
	ProjectName string `json:"projectName"`
	DocCount    int    `json:"docCount"`
	Framework   string `json:"framework"`
}

// handleLinkProject handles POST /api/link.
//
// It validates the request, constructs a SymlinkAdapter, registers it in the
// ProjectRegistry, persists the link to .vedox/links.json, and returns a
// summary of the linked project.
//
// Validation rules:
//   - externalRoot must be a non-empty absolute path.
//   - externalRoot must exist and resolve to a real directory.
//   - externalRoot must not be inside s.workspaceRoot.
//   - projectName must not be empty, and must not contain "/" or "..".
func (s *Server) handleLinkProject(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64 KB is more than enough for a link request

	var req linkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-000", "invalid JSON body")
		return
	}

	// -- Input validation -------------------------------------------------------

	req.ExternalRoot = strings.TrimSpace(req.ExternalRoot)
	req.ProjectName = strings.TrimSpace(req.ProjectName)

	req.ExternalRoot = expandTilde(req.ExternalRoot)

	if req.ExternalRoot == "" {
		writeError(w, http.StatusBadRequest, "VDX-000", "externalRoot must not be empty")
		return
	}
	if !filepath.IsAbs(req.ExternalRoot) {
		writeError(w, http.StatusBadRequest, "VDX-000",
			"externalRoot must be an absolute path (e.g. /Users/alice/projects/my-api)")
		return
	}
	if req.ProjectName == "" {
		writeError(w, http.StatusBadRequest, "VDX-000", "projectName must not be empty")
		return
	}
	if strings.Contains(req.ProjectName, "..") || strings.Contains(req.ProjectName, "/") {
		writeError(w, http.StatusBadRequest, "VDX-000",
			"projectName must not contain '..' or '/'")
		return
	}

	// Resolve symlinks on externalRoot itself so the containment check and
	// SymlinkAdapter constructor operate on the real path.
	realExternal, err := filepath.EvalSymlinks(req.ExternalRoot)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VDX-000",
			"externalRoot does not exist or cannot be resolved: "+req.ExternalRoot)
		return
	}
	realExternal = filepath.Clean(realExternal)

	info, err := os.Stat(realExternal)
	if err != nil || !info.IsDir() {
		writeError(w, http.StatusBadRequest, "VDX-000",
			"externalRoot must be a directory that exists on the filesystem")
		return
	}

	// Containment check: the resolved external root must not be inside the
	// Vedox workspace. Linking a sub-directory of the workspace would create
	// two overlapping stores with conflicting write semantics.
	wsWithSep := s.workspaceRoot + string(os.PathSeparator)
	if realExternal == s.workspaceRoot || strings.HasPrefix(realExternal, wsWithSep) {
		writeError(w, http.StatusBadRequest, "VDX-000",
			"externalRoot must not be inside the Vedox workspace. "+
				"Use Import & Migrate for docs already inside the workspace.")
		return
	}

	// -- Create and register the adapter ----------------------------------------

	adapter, err := store.NewSymlinkAdapter(req.ExternalRoot, req.ProjectName, s.workspaceRoot)
	if err != nil {
		slog.Error("api.handleLinkProject: NewSymlinkAdapter failed",
			"externalRoot", req.ExternalRoot,
			"projectName", req.ProjectName,
			"error", err.Error(),
		)
		writeError(w, http.StatusInternalServerError, "VDX-000",
			"could not create symlink adapter for the external project")
		return
	}

	s.registry.Register(req.ProjectName, adapter)

	// -- Persist the link -------------------------------------------------------

	if err := links.Add(s.workspaceRoot, links.LinkedProject{
		ProjectName:  req.ProjectName,
		ExternalRoot: req.ExternalRoot,
	}); err != nil {
		// Non-fatal: the adapter is registered in memory. The link will be lost
		// on restart. Log the error prominently and continue.
		slog.Error("api.handleLinkProject: failed to persist link",
			"projectName", req.ProjectName,
			"error", err.Error(),
		)
	}

	// -- Count docs and detect framework ----------------------------------------

	docCount := countMarkdownFiles(realExternal)
	framework := detectFramework(realExternal)

	slog.Info("api.handleLinkProject: project linked",
		slog.String("projectName", req.ProjectName),
		slog.String("externalRoot", realExternal),
		slog.Int("docCount", docCount),
		slog.String("framework", framework),
	)

	writeJSON(w, http.StatusOK, linkResponse{
		ProjectName: req.ProjectName,
		DocCount:    docCount,
		Framework:   framework,
	})
}

// -- Helpers ------------------------------------------------------------------

// countMarkdownFiles walks root and counts all .md files, skipping
// node_modules, .git, and other common non-doc directories. It never reads
// file contents — only directory entries.
func countMarkdownFiles(root string) int {
	count := 0
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == ".git" || name == ".vedox" ||
				name == "vendor" || name == "dist" || name == "build" {
				return fs.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(p), ".md") {
			count++
		}
		return nil
	})
	return count
}

// detectFramework applies cheap file-presence heuristics to identify the doc
// framework used by the external project. The detection order matters: more
// specific signals are checked before generic ones.
//
// This mirrors the scanner's framework detection heuristics (VDX-P2-A) so
// linked projects report the same framework strings as scanned projects.
func detectFramework(root string) string {
	signals := []struct {
		file      string
		framework string
	}{
		{"astro.config.mjs", "Astro"},
		{"astro.config.ts", "Astro"},
		{"mkdocs.yml", "MkDocs"},
		{"mkdocs.yaml", "MkDocs"},
		{"_config.yml", "Jekyll"},
		{"docusaurus.config.js", "Docusaurus"},
		{"docusaurus.config.ts", "Docusaurus"},
		{"vitepress.config.ts", "VitePress"},
		{"vitepress.config.mts", "VitePress"},
		{".vitepress/config.ts", "VitePress"},
		{"book.toml", "mdBook"},
	}

	for _, s := range signals {
		if _, err := os.Stat(filepath.Join(root, s.file)); err == nil {
			return s.framework
		}
	}

	// Fall back to "README" if there is any README.md at the root.
	if _, err := os.Stat(filepath.Join(root, "README.md")); err == nil {
		return "README"
	}

	return "unknown"
}
