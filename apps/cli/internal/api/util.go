package api

import (
	"os"
	"path/filepath"
	"strings"
)

// withinHomeDir returns true when absPath is equal to, or a descendant of, the
// current user's home directory. It returns false when the home directory
// cannot be determined (fail-closed).
//
// Security: this guard prevents arbitrary filesystem writes through the
// /api/repos/create and /api/repos/register endpoints (HIGH-01, HIGH-04).
func withinHomeDir(absPath string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	// Ensure both paths end without a trailing slash so HasPrefix is exact.
	home = filepath.Clean(home)
	absPath = filepath.Clean(absPath)
	return absPath == home || strings.HasPrefix(absPath, home+string(filepath.Separator))
}

// expandTilde replaces a leading "~/" or "~" with the current user's home directory.
// If the path does not start with a tilde, it is returned unchanged.
// If the home directory cannot be determined, the original path is returned.
func expandTilde(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}

	if path == "~" {
		return home
	}

	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
		return filepath.Join(home, path[2:])
	}

	return path
}
