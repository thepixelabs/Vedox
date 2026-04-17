package api

// handlers for the global repo registry endpoints:
//
//	GET  /api/repos          — list all registered documentation repos
//	POST /api/repos          — create / register a new repo (mirrors `vedox repos add`)
//	POST /api/repos/create   — scaffold a new local repo (mkdir + git init + register)
//	POST /api/repos/register — register an existing local git repo by path
//
// All handlers read/write through the GlobalDB that is injected into Server
// at daemon startup. If globalDB is nil (dev server without a global DB), the
// endpoints return 503 so the rest of the API remains functional.

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/vedox/vedox/internal/db"
)

// repoResponse is the JSON shape returned by GET /api/repos and POST /api/repos.
type repoResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	RootPath  string `json:"root_path"`
	RemoteURL string `json:"remote_url,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func repoToResponse(r *db.Repo) repoResponse {
	return repoResponse{
		ID:        r.ID,
		Name:      r.Name,
		Type:      r.Type,
		RootPath:  r.RootPath,
		RemoteURL: r.RemoteURL,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

// handleListRepos implements GET /api/repos.
// Returns a JSON array (never null) of all repos ordered by name.
// Query parameter ?status filters by status ("active", "archived", "error").
func (s *Server) handleListRepos(w http.ResponseWriter, r *http.Request) {
	if s.globalDB == nil {
		writeError(w, http.StatusServiceUnavailable, "VDX-503",
			"global database not available; start the daemon to enable repo management")
		return
	}

	status := r.URL.Query().Get("status")
	repos, err := s.globalDB.ListRepos(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "VDX-500", "failed to list repos")
		return
	}

	out := make([]repoResponse, 0, len(repos))
	for _, repo := range repos {
		out = append(out, repoToResponse(repo))
	}
	writeJSON(w, http.StatusOK, out)
}

// createRepoRequest is the expected POST /api/repos body.
type createRepoRequest struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	RootPath  string `json:"root_path"`
	RemoteURL string `json:"remote_url"`
}

// handleCreateRepo implements POST /api/repos.
// Creates a new repo entry in the GlobalDB. Mirrors `vedox repos add` but via
// HTTP so the editor's onboarding flow can call it directly.
//
// Validation:
//   - name, type, root_path are required
//   - type must be "private", "public", or "inbox"
//   - remote_url is optional (must be empty for inbox type)
func (s *Server) handleCreateRepo(w http.ResponseWriter, r *http.Request) {
	if s.globalDB == nil {
		writeError(w, http.StatusServiceUnavailable, "VDX-503",
			"global database not available; start the daemon to enable repo management")
		return
	}

	// 64 KB is ample for a repo registration payload; reject anything larger
	// early so we don't buffer multi-MB JSON into memory before decoding.
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)

	var req createRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-400", "invalid JSON body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Type = strings.TrimSpace(req.Type)
	req.RootPath = strings.TrimSpace(req.RootPath)
	req.RemoteURL = strings.TrimSpace(req.RemoteURL)

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "VDX-400", "name is required")
		return
	}
	if req.RootPath == "" {
		writeError(w, http.StatusBadRequest, "VDX-400", "root_path is required")
		return
	}
	switch req.Type {
	case "private", "public", "inbox":
		// valid
	case "":
		writeError(w, http.StatusBadRequest, "VDX-400", "type is required (private|public|inbox)")
		return
	default:
		writeError(w, http.StatusBadRequest, "VDX-400", "type must be one of: private, public, inbox")
		return
	}

	repo := db.Repo{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Type:      req.Type,
		RootPath:  req.RootPath,
		RemoteURL: req.RemoteURL,
		Status:    "active",
	}

	if err := s.globalDB.UpsertRepo(r.Context(), repo); err != nil {
		writeError(w, http.StatusInternalServerError, "VDX-500", "failed to create repo")
		return
	}

	created, err := s.globalDB.GetRepo(r.Context(), repo.ID)
	if err != nil || created == nil {
		writeError(w, http.StatusInternalServerError, "VDX-500", "repo created but could not be retrieved")
		return
	}

	writeJSON(w, http.StatusCreated, repoToResponse(created))
}

// ---------------------------------------------------------------------------
// POST /api/repos/create
// ---------------------------------------------------------------------------

// createRepoWithInitRequest is the expected POST /api/repos/create body.
// The handler creates the directory on disk, runs git init, then registers
// the result in the GlobalDB.
type createRepoWithInitRequest struct {
	// Name is the human-readable display name for the repo.
	Name string `json:"name"`
	// Path is the absolute path where the repo should be created.
	// The directory is created if it does not already exist.
	Path string `json:"path"`
	// Type is the repo type: "private", "public", or "inbox".
	// Defaults to "private" if omitted.
	Type string `json:"type"`
	// Private is a legacy boolean kept for frontend compat; when true it
	// forces Type="private". Ignored when Type is already set.
	Private bool `json:"private"`
}

// handleCreateRepoWithInit implements POST /api/repos/create.
//
// It scaffolds a new local documentation repo:
//  1. Validates inputs.
//  2. Creates the target directory (os.MkdirAll).
//  3. Runs `git init` in the directory.
//  4. Registers the repo in GlobalDB (status=active).
//  5. Returns the created repo JSON (same shape as POST /api/repos).
//
// If the directory already exists and is already a git repo the git init is
// still safe to run — git init is idempotent.
func (s *Server) handleCreateRepoWithInit(w http.ResponseWriter, r *http.Request) {
	if s.globalDB == nil {
		writeError(w, http.StatusServiceUnavailable, "VDX-503",
			"global database not available; start the daemon to enable repo management")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)

	var req createRepoWithInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-400", "invalid JSON body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Path = strings.TrimSpace(req.Path)
	req.Type = strings.TrimSpace(req.Type)

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "VDX-400", "name is required")
		return
	}

	// Resolve the path: expand "~" and make absolute.
	repoPath := expandTilde(req.Path)
	if repoPath == "" {
		writeError(w, http.StatusBadRequest, "VDX-400", "path is required")
		return
	}
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VDX-400", "path could not be resolved to an absolute path")
		return
	}

	// Security: HIGH-01 — only allow repo creation inside the user's home
	// directory. This prevents CORS-spoofed requests from writing arbitrary
	// directories anywhere on the filesystem.
	if !withinHomeDir(absPath) {
		writeError(w, http.StatusBadRequest, "VDX-400",
			"path must be within your home directory")
		return
	}
	// Re-audit: the parent directory may exist and contain a symlink that
	// escapes $HOME. filepath.Abs does not resolve symlinks, so we EvalSymlinks
	// the closest existing ancestor and re-check the boundary. This prevents
	// `~/escape -> /tmp` being used as a scaffolding target.
	if real := resolveExistingAncestor(absPath); real != "" && !withinHomeDir(real) {
		writeError(w, http.StatusBadRequest, "VDX-400",
			"path resolves outside your home directory via a symlink")
		return
	}

	// Derive type. Legacy "private" boolean takes lower priority than explicit type.
	repoType := req.Type
	if repoType == "" {
		if req.Private {
			repoType = "private"
		} else {
			repoType = "private" // safe default
		}
	}
	switch repoType {
	case "private", "public", "inbox":
		// valid
	default:
		writeError(w, http.StatusBadRequest, "VDX-400", "type must be one of: private, public, inbox")
		return
	}

	// Create the directory if it does not exist.
	if err := os.MkdirAll(absPath, 0o755); err != nil {
		slog.Error("repos/create: mkdir failed", "path", absPath, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			fmt.Sprintf("could not create directory: %s", absPath))
		return
	}

	// Run git init. This is idempotent — safe to run on an existing git repo.
	gitCmd := exec.CommandContext(r.Context(), "git", "init", absPath)
	if out, err := gitCmd.CombinedOutput(); err != nil {
		slog.Error("repos/create: git init failed",
			"path", absPath, "output", string(out), "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			"git init failed; ensure git is installed and the path is writable")
		return
	}

	// Register in GlobalDB.
	repo := db.Repo{
		ID:       uuid.New().String(),
		Name:     req.Name,
		Type:     repoType,
		RootPath: absPath,
		Status:   "active",
	}
	if err := s.globalDB.UpsertRepo(r.Context(), repo); err != nil {
		slog.Error("repos/create: upsert repo", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500", "failed to register repo")
		return
	}

	created, err := s.globalDB.GetRepo(r.Context(), repo.ID)
	if err != nil || created == nil {
		writeError(w, http.StatusInternalServerError, "VDX-500", "repo created but could not be retrieved")
		return
	}

	writeJSON(w, http.StatusCreated, repoToResponse(created))
}

// ---------------------------------------------------------------------------
// POST /api/repos/register
// ---------------------------------------------------------------------------

// registerRepoRequest is the expected POST /api/repos/register body.
type registerRepoRequest struct {
	// Path is the absolute path of an existing git repository to register.
	Path string `json:"path"`
	// Name is the human-readable display name. If omitted, the directory
	// basename is used.
	Name string `json:"name"`
	// Type is the repo type: "private", "public", or "inbox".
	// Defaults to "private" if omitted.
	Type string `json:"type"`
}

// handleRegisterRepo implements POST /api/repos/register.
//
// It registers an existing local git repository:
//  1. Validates path exists on disk.
//  2. Confirms path is a git repository (presence of .git directory or file).
//  3. Registers in GlobalDB (status=active).
//  4. Returns the created repo JSON.
//
// The path must already be a git repo — this handler does not run git init.
// Use POST /api/repos/create to scaffold a new repo from scratch.
func (s *Server) handleRegisterRepo(w http.ResponseWriter, r *http.Request) {
	if s.globalDB == nil {
		writeError(w, http.StatusServiceUnavailable, "VDX-503",
			"global database not available; start the daemon to enable repo management")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)

	var req registerRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-400", "invalid JSON body")
		return
	}

	req.Path = strings.TrimSpace(req.Path)
	req.Name = strings.TrimSpace(req.Name)
	req.Type = strings.TrimSpace(req.Type)

	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "VDX-400", "path is required")
		return
	}

	// Resolve path.
	absPath, err := filepath.Abs(expandTilde(req.Path))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VDX-400", "path could not be resolved to an absolute path")
		return
	}

	// Security: HIGH-04 — only allow registering repos inside the user's home
	// directory. This prevents out-of-bounds path registration via spoofed
	// requests.
	if !withinHomeDir(absPath) {
		writeError(w, http.StatusBadRequest, "VDX-400",
			"path must be within your home directory")
		return
	}
	// Symlink-escape guard: resolve the real path before trusting the
	// withinHomeDir result. EvalSymlinks fails for non-existent paths, but the
	// stat below will reject those anyway. If the real path is outside $HOME,
	// reject without leaking the target.
	if real, serr := filepath.EvalSymlinks(absPath); serr == nil {
		if !withinHomeDir(real) {
			writeError(w, http.StatusBadRequest, "VDX-400",
				"path resolves outside your home directory via a symlink")
			return
		}
		absPath = real
	}

	// Path must exist.
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		writeError(w, http.StatusBadRequest, "VDX-400",
			fmt.Sprintf("path does not exist: %s", absPath))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "VDX-500", "could not stat path")
		return
	}
	if !info.IsDir() {
		writeError(w, http.StatusBadRequest, "VDX-400", "path must be a directory")
		return
	}

	// Must be a git repo. We accept both a .git directory (normal repo) and a
	// .git file (git worktree / submodule). We do NOT require the .git entry to
	// itself be a directory because git worktrees use a plain file.
	gitEntry := filepath.Join(absPath, ".git")
	if _, err := os.Stat(gitEntry); os.IsNotExist(err) {
		writeError(w, http.StatusBadRequest, "VDX-400",
			fmt.Sprintf("path is not a git repository (no .git found): %s", absPath))
		return
	}

	// Derive name from basename if not supplied.
	name := req.Name
	if name == "" {
		name = filepath.Base(absPath)
	}

	// Derive type.
	repoType := req.Type
	if repoType == "" {
		repoType = "private"
	}
	switch repoType {
	case "private", "public", "inbox":
		// valid
	default:
		writeError(w, http.StatusBadRequest, "VDX-400", "type must be one of: private, public, inbox")
		return
	}

	repo := db.Repo{
		ID:       uuid.New().String(),
		Name:     name,
		Type:     repoType,
		RootPath: absPath,
		Status:   "active",
	}
	if err := s.globalDB.UpsertRepo(r.Context(), repo); err != nil {
		slog.Error("repos/register: upsert repo", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500", "failed to register repo")
		return
	}

	created, err := s.globalDB.GetRepo(r.Context(), repo.ID)
	if err != nil || created == nil {
		writeError(w, http.StatusInternalServerError, "VDX-500", "repo registered but could not be retrieved")
		return
	}

	writeJSON(w, http.StatusCreated, repoToResponse(created))
}
