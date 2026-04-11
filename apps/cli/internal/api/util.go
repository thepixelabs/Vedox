package api

import (
	"os"
	"path/filepath"
	"strings"
)

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
