package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/vedox/vedox/internal/links"
	"github.com/vedox/vedox/internal/store"
)

// projectInfo is the JSON shape returned by GET /api/projects.
type projectInfo struct {
	Name              string `json:"name"`
	Path              string `json:"path"`
	RelPath           string `json:"relPath"`
	DocCount          int    `json:"docCount"`
	DetectedFramework string `json:"detectedFramework"`
	LastScanned       string `json:"lastScanned"`
}

// handleListProjects handles GET /api/projects.
//
// It returns the project list from the most recently completed scan job for
// the configured workspace root. If no completed scan exists in the current
// process (e.g. first request after a server restart), it falls back to a
// synchronous scan so the response is never empty.
//
// Linked (read-only) projects persisted in .vedox/links.json are merged into
// the response. Scan results take precedence: if a linked project name matches
// a scanned project it is not duplicated.
//
// The returned projects are sorted by Name ascending (guaranteed by Scanner.Scan).
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	var projects []projectInfo

	// Fast path: a completed scan job exists.
	if job := s.jobStore.LastCompleted(s.workspaceRoot); job != nil {
		projects = make([]projectInfo, 0, len(job.Projects))
		for _, p := range job.Projects {
			projects = append(projects, projectInfo{
				Name:              p.Name,
				Path:              p.AbsPath,
				RelPath:           p.RelPath,
				DocCount:          p.DocCount,
				DetectedFramework: p.DetectedFramework,
				LastScanned:       p.LastScanned.Format("2006-01-02T15:04:05Z"),
			})
		}
	} else {
		// Slow path: no cached scan — run one synchronously.
		scanned, err := s.jobStore.Scanner().Scan(s.workspaceRoot)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "VDX-100",
				"workspace scan failed")
			return
		}
		projects = make([]projectInfo, 0, len(scanned))
		for _, p := range scanned {
			projects = append(projects, projectInfo{
				Name:              p.Name,
				Path:              p.AbsPath,
				RelPath:           p.RelPath,
				DocCount:          p.DocCount,
				DetectedFramework: p.DetectedFramework,
				LastScanned:       p.LastScanned.Format("2006-01-02T15:04:05Z"),
			})
		}
	}

	// Merge projects registered in the ProjectRegistry that the scanner missed.
	// This covers imported projects (no .git dir) and linked projects (external).
	seen := make(map[string]struct{}, len(projects))
	for _, p := range projects {
		seen[p.Name] = struct{}{}
	}

	// Build a lookup of linked project paths from .vedox/links.json so we can
	// use the correct external root for linked projects.
	linkedPaths := make(map[string]string)
	if linked, err := links.Load(s.workspaceRoot); err == nil {
		for _, lp := range linked {
			linkedPaths[lp.ProjectName] = lp.ExternalRoot
		}
	}

	for _, name := range s.registry.List() {
		if _, ok := seen[name]; ok {
			continue
		}
		// Determine the project root: linked projects use the external path,
		// imported (local) projects live under the workspace root.
		root := filepath.Join(s.workspaceRoot, name)
		if extRoot, isLinked := linkedPaths[name]; isLinked {
			root = extRoot
		}
		projects = append(projects, projectInfo{
			Name:              name,
			Path:              root,
			DocCount:          countMarkdownFiles(root),
			DetectedFramework: detectFramework(root),
		})
	}

	writeJSON(w, http.StatusOK, projects)
}

// createProjectRequest is the JSON body expected by POST /api/projects.
type createProjectRequest struct {
	Name        string `json:"name"`
	Tagline     string `json:"tagline"`
	Description string `json:"description"`
}

// createProjectResponse is the JSON body returned by a successful POST /api/projects.
type createProjectResponse struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	DocCount int    `json:"docCount"`
}

// handleCreateProject handles POST /api/projects.
//
// Creates a new project directory inside the workspace root, optionally writes
// a README.md if a tagline or description is provided, registers the project in
// the registry so it appears in GET /api/projects immediately, and invalidates
// the scan cache.
//
// Error codes:
//
//	VDX-000 — malformed JSON body
//	VDX-300 — invalid project name (empty, contains slashes, or is "." / "..")
//	VDX-301 — project already exists on disk
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-000", "invalid JSON body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		writeError(w, http.StatusBadRequest, "VDX-300",
			"name must be a non-empty single directory segment with no path separators")
		return
	}

	projectDir := filepath.Join(s.workspaceRoot, name)

	if _, err := os.Stat(projectDir); err == nil {
		writeError(w, http.StatusConflict, "VDX-301",
			"a project with that name already exists")
		return
	}

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "VDX-302",
			"could not create project directory: "+err.Error())
		return
	}

	// Write a README.md if the caller provided a tagline or description.
	tagline := strings.TrimSpace(req.Tagline)
	description := strings.TrimSpace(req.Description)
	if tagline != "" || description != "" {
		var sb strings.Builder
		sb.WriteString("# " + name + "\n")
		if tagline != "" {
			sb.WriteString("\n> " + tagline + "\n")
		}
		if description != "" {
			sb.WriteString("\n" + description + "\n")
		}
		readmePath := filepath.Join(projectDir, "README.md")
		if err := os.WriteFile(readmePath, []byte(sb.String()), 0644); err != nil {
			// Non-fatal: the directory was created successfully; log and continue.
			slog.Warn("api.handleCreateProject: could not write README.md",
				"projectName", name, "error", err.Error())
		}
	}

	// Register a LocalAdapter so the project appears in GET /api/projects
	// without waiting for a manual rescan (mirrors handleImport behaviour).
	adapter, adapterErr := store.NewLocalAdapter(projectDir, nil)
	if adapterErr != nil {
		slog.Warn("api.handleCreateProject: could not register project adapter",
			"projectName", name, "error", adapterErr.Error())
	} else {
		s.registry.Register(name, adapter)
	}

	// Invalidate the cached scan so the next GET /api/projects reflects the
	// new project without requiring a client-triggered rescan.
	s.jobStore.InvalidateCache(s.workspaceRoot)

	writeJSON(w, http.StatusCreated, createProjectResponse{
		Name:     name,
		Path:     projectDir,
		DocCount: 0,
	})
}
