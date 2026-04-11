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
// Query parameters:
//   path: absolute path to list. If empty, defaults to the user's home directory
//         on macOS/Linux or the current drive root on Windows.
//
// Errors:
//   VDX-102: The path is not a directory or could not be read.
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

	entries, err := os.ReadDir(abs)
	if err != nil {
		status := http.StatusInternalServerError
		if os.IsPermission(err) {
			status = http.StatusForbidden
		} else if os.IsNotExist(err) {
			status = http.StatusNotFound
		}
		writeError(w, status, "VDX-102", "could not read directory: "+err.Error())
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
