package api

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// browseResponse is the JSON body returned by GET /api/browse.
type browseResponse struct {
	/** Absolute path of the directory being listed. */
	Path string `json:"path"`
	/** Absolute path of the parent directory. Empty if at root. */
	Parent string `json:"parent"`
	/** List of subdirectories found. Files are excluded. */
	Directories []dirEntry `json:"directories"`
}

type dirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// handleBrowse handles GET /api/browse?path=...
//
// It returns a list of subdirectories for the given path. This is used by the
// frontend to implement a folder picker for importing or linking projects.
//
// Authentication: the bootstrap token must be supplied as
//
//	Authorization: Bearer <token>
//
// (enforced by the requireBootstrapToken middleware wired in server.go Mount).
//
// Query parameters:
//
//	path: absolute path to list. If empty, defaults to the user's home directory
//	      on macOS/Linux or the current drive root on Windows.
//
// Errors:
//
//	VDX-401: No valid bootstrap token supplied (handled by middleware, not here).
//	VDX-403: Requested path is outside $HOME.
//	VDX-100: Path could not be resolved to an absolute path.
//	VDX-102: The path is not a directory or could not be read.
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))

	if path == "" {
		// Default to Home directory
		home, err := os.UserHomeDir()
		if err != nil {
			// Fallback to workspace root if home is unavailable
			path = s.workspaceRoot
		} else {
			path = home
		}
	} else {
		path = expandTilde(path)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VDX-100", "path could not be resolved")
		return
	}

	// Home-directory boundary check (CRIT-02 / FIX-SEC-01 / MED-02).
	// Reject any path that escapes $HOME so that an authenticated caller
	// (e.g. a compromised frontend) cannot enumerate /etc, /proc, or other
	// sensitive directories. withinHomeDir uses filepath.Clean on both paths
	// to defeat double-dot traversal before the HasPrefix comparison.
	//
	// Symlink-escape guard (re-audit #2): filepath.Abs does NOT resolve
	// symlinks, so a user-owned symlink inside $HOME that points outside it
	// (e.g. ~/escape -> /etc) would pass the prefix check but os.ReadDir
	// would then follow the symlink and enumerate the target. We resolve
	// the real path here and re-check the boundary before touching the
	// filesystem. EvalSymlinks fails when the path does not exist; in that
	// case we fall back to the abs comparison (which is still safe because
	// a non-existent path cannot contain a symlink).
	if !withinHomeDir(abs) {
		writeError(w, http.StatusForbidden, "VDX-403", "path is outside the allowed home directory boundary")
		return
	}
	if real, serr := filepath.EvalSymlinks(abs); serr == nil {
		if !withinHomeDir(real) {
			writeError(w, http.StatusForbidden, "VDX-403", "path is outside the allowed home directory boundary")
			return
		}
		abs = real
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		status := http.StatusInternalServerError
		if os.IsPermission(err) {
			status = http.StatusForbidden
		} else if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		// MED-02 fix: strip the absolute path from the error message so that
		// the OS-level error (which embeds abs) is not leaked to the client.
		// The operator can correlate via the structured log entry.
		writeError(w, status, "VDX-102", "could not read directory")
		return
	}

	var dirs []dirEntry
	for _, entry := range entries {
		// Only include directories, skip hidden ones (optional, but cleaner for a picker)
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			dirs = append(dirs, dirEntry{
				Name: entry.Name(),
				Path: filepath.Join(abs, entry.Name()),
			})
		}
	}

	// Sort alphabetically
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})

	parent := filepath.Dir(abs)
	// If we are at the root (e.g. / on Unix), filepath.Dir returns the same path.
	if parent == abs {
		parent = ""
	}

	// Special case for Windows: filepath.Dir("C:\") is "C:\".
	if runtime.GOOS == "windows" && len(abs) <= 3 && strings.HasSuffix(abs, ":\\") {
		parent = ""
	}

	writeJSON(w, http.StatusOK, browseResponse{
		Path:        abs,
		Parent:      parent,
		Directories: dirs,
	})
}
