package api

import (
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/vedox/vedox/internal/links"
)

// gitStatusResponse is the JSON returned by GET /api/projects/{project}/git/status.
//
// Fields:
//
//	Branch — current branch name, "HEAD" for detached, or "unknown"
//	Dirty  — true if the working tree has uncommitted changes
//	Ahead  — commits ahead of the upstream
//	Behind — commits behind the upstream
type gitStatusResponse struct {
	Branch string `json:"branch"`
	Dirty  bool   `json:"dirty"`
	Ahead  int    `json:"ahead"`
	Behind int    `json:"behind"`
}

// handleGitStatus handles GET /api/projects/{project}/git/status.
//
// Reads git state via the `git` CLI rooted at the project's directory. If the
// project is not inside a Git repository, returns 200 with branch="(no git)"
// and Dirty=false. If `git` is unavailable on PATH, returns 200 with the same
// sentinel so the frontend status bar can degrade gracefully.
//
// This endpoint is intentionally best-effort: transient git errors must not
// break the editor chrome.
func (s *Server) handleGitStatus(w http.ResponseWriter, r *http.Request) {
	projectName := chi.URLParam(r, "project")
	if projectName == "" {
		writeError(w, http.StatusBadRequest, "VDX-101", "missing project name")
		return
	}

	// Resolve the project root: linked projects may live outside the workspace
	// root, so we check .vedox/links.json first.
	projectRoot := filepath.Join(s.workspaceRoot, projectName)
	if linked, err := links.Load(s.workspaceRoot); err == nil {
		for _, lp := range linked {
			if lp.ProjectName == projectName {
				projectRoot = lp.ExternalRoot
				break
			}
		}
	}

	resp := gitStatusResponse{
		Branch: "(no git)",
		Dirty:  false,
		Ahead:  0,
		Behind: 0,
	}

	// Branch
	if branch, ok := runGit(projectRoot, "rev-parse", "--abbrev-ref", "HEAD"); ok {
		resp.Branch = strings.TrimSpace(branch)
	} else {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Dirty (porcelain check)
	if out, ok := runGit(projectRoot, "status", "--porcelain"); ok {
		resp.Dirty = strings.TrimSpace(out) != ""
	}

	// Ahead / behind relative to upstream, if any.
	if out, ok := runGit(projectRoot, "rev-list", "--left-right", "--count", "@{u}...HEAD"); ok {
		fields := strings.Fields(strings.TrimSpace(out))
		if len(fields) == 2 {
			if behind, err := strconv.Atoi(fields[0]); err == nil {
				resp.Behind = behind
			}
			if ahead, err := strconv.Atoi(fields[1]); err == nil {
				resp.Ahead = ahead
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// runGit executes `git <args...>` with the given working directory.
// Returns (stdout, true) on exit code 0, or ("", false) on any error.
// Never propagates errors — this is a best-effort probe.
func runGit(dir string, args ...string) (string, bool) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return string(out), true
}
