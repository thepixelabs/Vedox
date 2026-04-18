package api

// GET /api/preview — resolves a vedox:// URL and returns source code preview
// data for Shiki rendering in the editor.
//
// Query parameters:
//
//	url  (required) — the vedox:// URL to resolve, e.g.:
//	     vedox://file/apps/cli/main.go#L10-L25
//
// The endpoint returns a JSON object whose Content field holds the raw source
// text (UTF-8).  The editor renders this with Shiki using the Language field
// as the grammar identifier.
//
// Error codes:
//
//	VDX-400 — missing or empty url parameter
//	VDX-422 — the vedox:// URL is structurally invalid or fails security checks
//	VDX-403 — the resolved file matches the secret-file blocklist
//	VDX-404 — the resolved file does not exist on disk
//	VDX-415 — the resolved file is a binary file (null byte in first 512 bytes)
//	VDX-500 — unexpected read failure

import (
	"errors"
	"net/http"
	"strings"

	"github.com/vedox/vedox/internal/codepreview"
)

// previewResponse is the JSON body returned by GET /api/preview.
type previewResponse struct {
	// FilePath is the project-relative path extracted from the vedox:// URL.
	FilePath string `json:"file_path"`

	// Language is the Shiki-compatible language identifier inferred from the
	// file extension.  Empty when the extension is not recognised (Shiki renders
	// as plain text).
	Language string `json:"language"`

	// Content is the source code text — either the full file or the requested
	// line range.
	Content string `json:"content"`

	// StartLine is the 1-indexed first line of Content within the original file.
	StartLine int `json:"start_line"`

	// EndLine is the 1-indexed last line of Content within the original file.
	EndLine int `json:"end_line"`

	// TotalLines is the total number of lines in the file (within the 500KB cap).
	TotalLines int `json:"total_lines"`

	// Truncated is true when the file exceeded 500KB and Content is a prefix
	// of the full file.  The editor should show a "file truncated" notice.
	Truncated bool `json:"truncated"`
}

// handlePreview implements GET /api/preview?url=vedox://file/<path>[#anchor].
//
// It resolves the vedox:// URL against the workspace root and returns the file
// content as JSON.  The workspace root is used as the project root; multi-repo
// routing (resolving against a per-project root from the registry) is tracked
// in the v2 roadmap and will be wired once the registry exposes root paths.
//
// The handler is intentionally read-only and does not require agent auth.
// CORS headers are already applied by the corsMiddleware in the chi stack.
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" {
		writeError(w, http.StatusBadRequest, "VDX-400",
			"url query parameter is required (e.g. ?url=vedox://file/apps/cli/main.go#L10-L25)")
		return
	}

	preview, err := codepreview.Resolve(s.workspaceRoot, rawURL)
	if err != nil {
		writePreviewError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, previewResponse{
		FilePath:   preview.FilePath,
		Language:   preview.Language,
		Content:    preview.Content,
		StartLine:  preview.StartLine,
		EndLine:    preview.EndLine,
		TotalLines: preview.TotalLines,
		Truncated:  preview.Truncated,
	})
}

// writePreviewError maps codepreview sentinel errors to appropriate HTTP status
// codes and VDX error codes.
func writePreviewError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, codepreview.ErrSecretFile):
		writeError(w, http.StatusForbidden, "VDX-403",
			"the referenced file matches the secret-file blocklist and cannot be previewed")

	case errors.Is(err, codepreview.ErrFileNotFound):
		writeError(w, http.StatusNotFound, "VDX-404",
			"the referenced file does not exist")

	case errors.Is(err, codepreview.ErrBinaryFile):
		writeError(w, http.StatusUnsupportedMediaType, "VDX-415",
			"the referenced file is a binary file and cannot be previewed as text")

	case errors.Is(err, codepreview.ErrInvalidScheme),
		errors.Is(err, codepreview.ErrInvalidHost),
		errors.Is(err, codepreview.ErrEmptyPath),
		errors.Is(err, codepreview.ErrAbsolutePath),
		errors.Is(err, codepreview.ErrTraversal),
		errors.Is(err, codepreview.ErrSymlinkEscape),
		errors.Is(err, codepreview.ErrInvalidAnchor),
		errors.Is(err, codepreview.ErrAnchorRangeTooBig),
		errors.Is(err, codepreview.ErrAnchorOutOfRange):
		writeError(w, http.StatusUnprocessableEntity, "VDX-422", err.Error())

	default:
		writeError(w, http.StatusInternalServerError, "VDX-500",
			"failed to resolve preview: "+err.Error())
	}
}
