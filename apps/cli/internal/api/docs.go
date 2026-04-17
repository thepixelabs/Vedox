package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	vdxerr "github.com/vedox/vedox/internal/errors"
	"github.com/vedox/vedox/internal/gitcheck"
	"github.com/vedox/vedox/internal/store"
)

// docResponse is the JSON shape returned by GET and POST doc endpoints.
// ModTime is formatted as RFC3339 for unambiguous parsing on the frontend.
type docResponse struct {
	Path     string                 `json:"path"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
	ModTime  string                 `json:"modTime"`
	Size     int64                  `json:"size"`
}

// writeRequest is the JSON body for POST /api/projects/:project/docs/*path.
type writeRequest struct {
	Content string `json:"content"`
}

// publishRequest is the JSON body for POST /api/projects/:project/docs/*path/publish.
type publishRequest struct {
	Message string `json:"message"`
}

// draftsSubDir is the workspace-relative directory where auto-save drafts live.
// e.g. .vedox/drafts/docs/architecture/adr-001.md.draft.md
const draftsSubDir = ".vedox/drafts"

// draftSuffix is appended to the original filename to form the draft filename.
const draftSuffix = ".draft.md"

// handleListDocs returns all Markdown (.md) files under the project directory,
// recursively. Directories named ".vedox" and "node_modules" are pruned from
// the walk so generated and internal files are never returned.
//
// Returned paths are workspace-relative (e.g. "myproject/docs/adr-001.md").
func (s *Server) handleListDocs(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	if project == "" {
		writeError(w, http.StatusBadRequest, string(vdxerr.ErrPathTraversal), "missing project")
		return
	}

	// Route to the correct DocStore. External (symlinked) projects are served by
	// a SymlinkAdapter registered in the ProjectRegistry; local projects use the
	// default LocalAdapter via the filesystem walk below.
	docStore := s.storeForProject(project)

	// For external projects (SymlinkAdapter), delegate listing to the store's own
	// List method — the files don't exist under workspaceRoot so a local walk
	// would return nothing. SymlinkAdapter.List is recursive by design.
	if _, isExternal := s.registry.Get(project); isExternal {
		docs, listErr := docStore.List(".")
		if listErr != nil {
			handleStoreError(w, listErr)
			return
		}
		out := make([]docResponse, 0, len(docs))
		for _, d := range docs {
			out = append(out, docToResponse(d))
		}
		writeJSON(w, http.StatusOK, out)
		return
	}

	// Validate the project name: must not contain ".." or "/" so that the
	// filepath.Join below cannot escape the workspace root.
	projectDir, err := s.safeProjectPath(project)
	if err != nil {
		writeError(w, http.StatusBadRequest, string(vdxerr.ErrPathTraversal),
			"invalid project path")
		return
	}

	// Walk the project directory recursively, collecting all .md files.
	// We read each file through the DocStore so that secret-file blocking and
	// frontmatter parsing are applied consistently.
	var out []docResponse

	walkErr := filepath.WalkDir(projectDir, func(absPath string, d fs.DirEntry, err error) error {
		if err != nil {
			// Log and skip unreadable entries rather than aborting the entire walk.
			slog.Warn("api.handleListDocs: walk error, skipping",
				"path", absPath,
				"error", err.Error(),
			)
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			// Prune directories that must never be surfaced.
			if name == ".vedox" || name == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}

		// Only surface Markdown files.
		if !strings.EqualFold(filepath.Ext(absPath), ".md") {
			return nil
		}

		// Convert to workspace-relative path for the response and for DocStore.Read.
		relPath, relErr := filepath.Rel(s.workspaceRoot, absPath)
		if relErr != nil {
			slog.Warn("api.handleListDocs: cannot compute relative path, skipping",
				"abs", absPath,
				"error", relErr.Error(),
			)
			return nil
		}

		doc, readErr := docStore.Read(relPath)
		if readErr != nil {
			// Secret-blocked files produce VDX-006; silently skip them so a
			// directory listing never reveals their presence.
			slog.Debug("api.handleListDocs: store.Read skipped",
				"path", relPath,
				"error", readErr.Error(),
			)
			return nil
		}

		out = append(out, docToResponse(doc))
		return nil
	})

	if walkErr != nil {
		slog.Error("api.handleListDocs: walk failed", "project", project, "error", walkErr.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}

	if out == nil {
		out = []docResponse{}
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGetDoc returns the content of a single document.
//
// Draft precedence: if a draft file exists under .vedox/drafts/ and its
// modification time is newer than the committed file, the draft is returned
// instead. The response path always reflects the original (committed) path so
// the frontend can track identity correctly.
func (s *Server) handleGetDoc(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	// chi encodes the wildcard as "*" in URLParam.
	docPath := chi.URLParam(r, "*")

	relPath, err := s.validateDocPath(project, docPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, string(vdxerr.ErrPathTraversal),
			"invalid document path")
		return
	}

	docStore := s.storeForProject(project)
	committed, committedErr := docStore.Read(relPath)

	// Check for a draft that is newer than the committed file.
	// Drafts always live in the local workspace store regardless of project type.
	draftRelPath := draftRelativePath(relPath)
	draft, draftErr := s.store.Read(draftRelPath)

	// Decision tree:
	//   - Have draft + no committed file  → serve draft
	//   - Have draft + committed file, draft is newer → serve draft
	//   - Have committed file, no newer draft → serve committed
	//   - Neither → 404
	switch {
	case draftErr == nil && (committedErr != nil || draft.ModTime.After(committed.ModTime)):
		// Serve draft but report the canonical path, not the draft path.
		draft.Path = relPath
		writeJSON(w, http.StatusOK, docToResponse(draft))

	case committedErr == nil:
		writeJSON(w, http.StatusOK, docToResponse(committed))

	default:
		if isNotFound(committedErr) {
			writeError(w, http.StatusNotFound, "VDX-000", "document not found")
		} else {
			handleStoreError(w, committedErr)
		}
	}
}

// handleWriteDoc auto-saves content to the draft location for the given path.
// It does not touch the committed file — that requires an explicit Publish.
//
// Draft path: .vedox/drafts/<relPath>.draft.md
//
// The response returns the saved draft's metadata so the frontend can update
// its "last saved" indicator.
func (s *Server) handleWriteDoc(w http.ResponseWriter, r *http.Request) {
	// Enforce a 1 MB body limit before reading anything. MaxBytesReader wraps
	// r.Body so the JSON decoder will receive an error once the limit is hit,
	// rather than buffering an arbitrarily large payload into memory.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	project := chi.URLParam(r, "project")
	docPath := chi.URLParam(r, "*")

	relPath, err := s.validateDocPath(project, docPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, string(vdxerr.ErrPathTraversal),
			"invalid document path")
		return
	}

	var req writeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// http.MaxBytesReader sets a *http.MaxBytesError when the limit is exceeded.
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge,
				string(vdxerr.ErrPayloadTooLarge), "Request body exceeds 1MB limit.")
			return
		}
		writeError(w, http.StatusBadRequest, "VDX-000", "invalid JSON body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "VDX-000", "content must not be empty")
		return
	}

	draftPath := draftRelativePath(relPath)
	if err := s.store.Write(draftPath, req.Content); err != nil {
		handleStoreError(w, err)
		return
	}

	// Read back the draft so we can return accurate modTime and size.
	saved, err := s.store.Read(draftPath)
	if err != nil {
		// Write succeeded but read-back failed; return a synthetic response.
		writeJSON(w, http.StatusOK, docResponse{
			Path:     relPath,
			Content:  req.Content,
			Metadata: map[string]interface{}{},
			ModTime:  time.Now().UTC().Format(time.RFC3339),
			Size:     int64(len(req.Content)),
		})
		return
	}
	saved.Path = relPath
	writeJSON(w, http.StatusOK, docToResponse(saved))
}

// handleDeleteDoc deletes both the committed file and any draft for the given path.
func (s *Server) handleDeleteDoc(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	docPath := chi.URLParam(r, "*")

	relPath, err := s.validateDocPath(project, docPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, string(vdxerr.ErrPathTraversal),
			"invalid document path")
		return
	}

	// Delete the committed file via the correct store (symlinked projects return VDX-011).
	if err := s.storeForProject(project).Delete(relPath); err != nil {
		if !isNotFound(err) {
			handleStoreError(w, err)
			return
		}
		// File not found is acceptable — maybe only a draft existed.
	}

	// Best-effort: also delete any lingering draft. Failure here is not fatal.
	draftPath := draftRelativePath(relPath)
	if delErr := s.store.Delete(draftPath); delErr != nil {
		slog.Debug("api.handleDeleteDoc: draft delete skipped",
			"path", draftPath,
			"reason", delErr.Error(),
		)
	}

	w.WriteHeader(http.StatusNoContent)
}

// handlePublish promotes a document to Git by:
//  1. Moving the draft (if any) over the committed file atomically via DocStore.Write.
//  2. Shelling out to git add + git commit with the author sourced from git config.
//
// If git config user.name or user.email is not set the request fails with VDX-003.
// If there is no draft the committed file is used as-is (re-commit, useful for
// frontmatter-only changes made via the REST API).
func (s *Server) handlePublish(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	// The POST /* dispatcher forwards here when the wildcard ends in "/publish"
	// (or equals "publish" for a root-level doc). Strip that suffix to recover
	// the actual document path, e.g. "adr/001.md/publish" → "adr/001.md".
	docPath := chi.URLParam(r, "*")
	if strings.HasSuffix(docPath, "/publish") {
		docPath = docPath[:len(docPath)-len("/publish")]
	} else {
		docPath = "" // was exactly "publish" — root-level publish (unusual)
	}

	relPath, err := s.validateDocPath(project, docPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, string(vdxerr.ErrPathTraversal),
			"invalid document path")
		return
	}

	var req publishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-000", "invalid JSON body")
		return
	}
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "VDX-000", "message must not be empty")
		return
	}

	// Resolve git identity. Fail fast with VDX-003 if unset — we must not
	// produce anonymous commits (EPIC-001 §7 item 3).
	authorName, authorEmail, err := gitIdentity()
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, string(vdxerr.ErrGitIdentityUnset),
			err.Error())
		return
	}

	// If a draft exists, promote it to the committed path first.
	// Drafts live in the local workspace store; promotion goes through the
	// project's own store (VDX-011 for symlinked projects — publish is invalid).
	docStore := s.storeForProject(project)
	draftRelPath := draftRelativePath(relPath)
	draft, draftErr := s.store.Read(draftRelPath)
	if draftErr == nil {
		// Promote draft → committed file.
		if err := docStore.Write(relPath, draft.Content); err != nil {
			handleStoreError(w, err)
			return
		}
		// Clean up the draft after a successful promote.
		if err := s.store.Delete(draftRelPath); err != nil {
			slog.Warn("api.handlePublish: draft cleanup failed",
				"draft_path", draftRelPath,
				"error", err.Error(),
			)
			// Non-fatal: the committed file is already correct.
		}
	}

	// Absolute path for git commands.
	absPath := filepath.Join(s.workspaceRoot, relPath)

	// git add
	addCmd := exec.CommandContext(r.Context(), "git", "add", absPath) // #nosec G204 — absPath is workspace-validated
	addCmd.Dir = s.workspaceRoot
	if out, err := addCmd.CombinedOutput(); err != nil {
		slog.Error("api.handlePublish: git add failed",
			"path", relPath,
			"output", string(out),
		)
		writeError(w, http.StatusInternalServerError, "VDX-000",
			"git add failed; check server logs")
		return
	}

	// git commit
	author := fmt.Sprintf("%s <%s>", authorName, authorEmail)
	commitCmd := exec.CommandContext(r.Context(), // #nosec G204 — all args are validated
		"git", "commit",
		"-m", req.Message,
		"--author", author,
	)
	commitCmd.Dir = s.workspaceRoot
	if out, err := commitCmd.CombinedOutput(); err != nil {
		slog.Error("api.handlePublish: git commit failed",
			"path", relPath,
			"output", string(out),
		)
		writeError(w, http.StatusInternalServerError, "VDX-000",
			"git commit failed; check server logs")
		return
	}

	// Emit document.published. Fire-and-forget: if the collector is nil
	// (dev-server) or its buffer is full, the publish still succeeds.
	s.emitEvent("document.published", map[string]any{
		"project": project,
		"path":    relPath,
	})

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "published",
		"path":   relPath,
		"author": author,
	})
}

// -- Path validation ----------------------------------------------------------

// validateDocPath validates and returns the workspace-relative path for a
// document given the raw project name and the chi wildcard value.
//
// Security: uses filepath.Clean + workspace root prefix assertion, consistent
// with LocalAdapter.safePath. This is defence-in-depth — the DocStore also
// validates, but we want to return a clean HTTP 400 before touching the store.
func (s *Server) validateDocPath(project, docPath string) (string, error) {
	if project == "" || docPath == "" {
		return "", fmt.Errorf("empty path component")
	}

	// Reject any literal ".." in the project name component.
	if strings.Contains(project, "..") {
		return "", fmt.Errorf("invalid project name")
	}

	// Build the candidate relative path and clean it.
	rel := filepath.Clean(filepath.Join(project, docPath))

	// After cleaning, the relative path must still start with the project name.
	// This catches traversal attempts like project="foo", docPath="../../etc/passwd".
	if !strings.HasPrefix(rel, project+string(filepath.Separator)) && rel != project {
		return "", fmt.Errorf("path traversal detected")
	}

	// Assert that the resulting absolute path is inside the workspace root.
	abs := filepath.Join(s.workspaceRoot, rel)
	abs = filepath.Clean(abs)
	rootWithSep := s.workspaceRoot + string(os.PathSeparator)
	if abs != s.workspaceRoot && !strings.HasPrefix(abs, rootWithSep) {
		slog.Warn("api: path traversal attempt blocked",
			"code", "VDX-005",
			"project", project,
			"doc_path", docPath,
		)
		return "", fmt.Errorf("path escapes workspace root")
	}

	return rel, nil
}

// safeProjectPath validates the project name alone and returns its absolute path.
func (s *Server) safeProjectPath(project string) (string, error) {
	if strings.Contains(project, "..") || strings.Contains(project, "/") {
		return "", fmt.Errorf("invalid project name")
	}
	abs := filepath.Clean(filepath.Join(s.workspaceRoot, project))
	rootWithSep := s.workspaceRoot + string(os.PathSeparator)
	if !strings.HasPrefix(abs, rootWithSep) {
		return "", fmt.Errorf("project path escapes workspace root")
	}
	return abs, nil
}

// -- Draft helpers ------------------------------------------------------------

// draftRelativePath returns the workspace-relative path for the draft file
// corresponding to relPath.
// e.g. "docs/adr-001.md" → ".vedox/drafts/docs/adr-001.md.draft.md"
func draftRelativePath(relPath string) string {
	return filepath.Join(draftsSubDir, relPath+draftSuffix)
}

// -- Git identity -------------------------------------------------------------

// gitIdentity reads user.name and user.email from git config and returns them.
// Returns VDX-003 via error message if either is unset.
// It delegates to gitcheck.GetConfigValue so there is a single implementation
// of the `git config` shell-out across the codebase.
func gitIdentity() (name, email string, err error) {
	name, err = gitcheck.GetConfigValue("user.name")
	if err != nil || strings.TrimSpace(name) == "" {
		return "", "", fmt.Errorf("[VDX-003] git user.name not set. "+
			"Fix: git config --global user.name \"Your Name\"")
	}
	email, err = gitcheck.GetConfigValue("user.email")
	if err != nil || strings.TrimSpace(email) == "" {
		return "", "", fmt.Errorf("[VDX-003] git user.email not set. "+
			"Fix: git config --global user.email \"you@example.com\"")
	}
	return strings.TrimSpace(name), strings.TrimSpace(email), nil
}

// -- Response helpers ---------------------------------------------------------

// docToResponse converts a store.Doc to the JSON response shape.
func docToResponse(d *store.Doc) docResponse {
	meta := d.Metadata
	if meta == nil {
		meta = map[string]interface{}{}
	}
	return docResponse{
		Path:     d.Path,
		Content:  d.Content,
		Metadata: meta,
		ModTime:  d.ModTime.UTC().Format(time.RFC3339),
		Size:     d.Size,
	}
}

// handleStoreError maps DocStore errors to HTTP responses.
//
//	VDX-005 (path traversal)  → 400 Bad Request
//	VDX-006 (secret file)     → 403 Forbidden
//	VDX-011 (read-only doc)   → 405 Method Not Allowed
//	everything else            → 500 Internal Server Error
func handleStoreError(w http.ResponseWriter, err error) {
	var vdxErr *vdxerr.VedoxError
	if errors.As(err, &vdxErr) {
		switch vdxErr.Code {
		case vdxerr.ErrPathTraversal:
			writeError(w, http.StatusBadRequest, string(vdxErr.Code), vdxErr.Message)
			return
		case vdxerr.ErrSecretFileBlocked:
			writeError(w, http.StatusForbidden, string(vdxErr.Code), vdxErr.Message)
			return
		case vdxerr.ErrReadOnly:
			// 405 Method Not Allowed is the semantically correct code here:
			// the resource exists but the HTTP method (write/delete) is not
			// permitted on it. The Allow header signals which methods are valid.
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, string(vdxErr.Code), vdxErr.Message)
			return
		}
	}
	slog.Error("api: store error", "error", err.Error())
	writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
}

// isNotFound reports whether err is an os.ErrNotExist-wrapped error. DocStore
// operations wrap os.Remove / os.ReadFile errors which in turn wrap os.ErrNotExist.
func isNotFound(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
