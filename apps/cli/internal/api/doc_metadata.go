package api

import (
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// DocMetadataResponse is the JSON shape returned by GET .../metadata.
type DocMetadataResponse struct {
	LastModified string             `json:"lastModified"`
	Contributors []ContributorEntry `json:"contributors"`
	Branch       string             `json:"branch"`
	CommitHash   string             `json:"commitHash"`
}

// ContributorEntry holds a single contributor name + email pair.
type ContributorEntry struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// handleDocMetadata returns git-derived metadata for a single document file:
// last modified time, contributor list, current branch, and latest commit hash.
//
// Route: GET /api/projects/{project}/docs/{docPath}/metadata
func (s *Server) handleDocMetadata(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	docPath := chi.URLParam(r, "*")

	// The wildcard ends in "/metadata"; strip that suffix to get the real doc path.
	if strings.HasSuffix(docPath, "/metadata") {
		docPath = docPath[:len(docPath)-len("/metadata")]
	} else {
		writeError(w, http.StatusBadRequest, "VDX-000", "missing document path")
		return
	}

	relPath, err := s.validateDocPath(project, docPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VDX-000", "invalid document path")
		return
	}

	absPath := filepath.Join(s.workspaceRoot, relPath)

	// Last commit touching this file: hash + author date.
	logOut, err := exec.CommandContext(r.Context(),
		"git", "-C", s.workspaceRoot, "log", "-1",
		"--format=%H%n%ai",
		"--", absPath,
	).Output()

	var commitHash, lastModified string
	if err == nil {
		lines := strings.SplitN(strings.TrimSpace(string(logOut)), "\n", 2)
		if len(lines) >= 1 {
			commitHash = strings.TrimSpace(lines[0])
		}
		if len(lines) >= 2 {
			t, terr := time.Parse("2006-01-02 15:04:05 -0700", strings.TrimSpace(lines[1]))
			if terr == nil {
				lastModified = t.UTC().Format(time.RFC3339)
			}
		}
	}

	// Contributors (unique authors who touched this file).
	authOut, err := exec.CommandContext(r.Context(),
		"git", "-C", s.workspaceRoot, "log",
		"--format=%an%n%ae",
		"--", absPath,
	).Output()

	var contributors []ContributorEntry
	seen := map[string]bool{}
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(authOut)), "\n")
		for i := 0; i+1 < len(lines); i += 2 {
			email := strings.TrimSpace(lines[i+1])
			if !seen[email] {
				seen[email] = true
				contributors = append(contributors, ContributorEntry{
					Name:  strings.TrimSpace(lines[i]),
					Email: email,
				})
			}
		}
	}

	// Ensure non-nil slice for clean JSON ([] not null).
	if contributors == nil {
		contributors = []ContributorEntry{}
	}

	// Current branch.
	branchOut, _ := exec.CommandContext(r.Context(),
		"git", "-C", s.workspaceRoot, "rev-parse", "--abbrev-ref", "HEAD",
	).Output()
	branch := strings.TrimSpace(string(branchOut))

	resp := DocMetadataResponse{
		LastModified: lastModified,
		Contributors: contributors,
		Branch:       branch,
		CommitHash:   commitHash,
	}

	writeJSON(w, http.StatusOK, resp)
}
