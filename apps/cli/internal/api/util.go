package api

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// utf8ValidString wraps utf8.ValidString. Kept as a thin helper so call sites
// read clearly at the enforcement point.
func utf8ValidString(s string) bool { return utf8.ValidString(s) }

// validateJSONDepth walks a json.RawMessage and rejects it when nested
// array/object depth exceeds max. This guards against "JSON bomb" payloads —
// deeply nested structures that blow out the stack during a later decode.
//
// The implementation uses json.Decoder.Token to stream the document without
// building the full tree. Each '{' or '[' token increments depth; each '}' or
// ']' decrements. If depth ever exceeds max, the function returns an error.
func validateJSONDepth(raw json.RawMessage, max int) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	depth := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("invalid json: %w", err)
		}
		switch d := tok.(type) {
		case json.Delim:
			if d == '{' || d == '[' {
				depth++
				if depth > max {
					return fmt.Errorf("nesting depth exceeds %d", max)
				}
			} else {
				depth--
			}
		}
	}
}

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

// resolveExistingAncestor walks absPath upward until it finds a component that
// exists on disk, then runs filepath.EvalSymlinks on that ancestor and returns
// the real path. The remaining (non-existent) tail is re-joined unchanged so
// callers can reason about whether the post-resolution path escapes a boundary
// even when the leaf does not exist yet (e.g. a to-be-created repo directory).
//
// Returns "" when no ancestor can be resolved (extremely unusual — only if /
// itself fails EvalSymlinks, which is effectively never).
//
// This is the companion to filepath.EvalSymlinks for write-side handlers where
// the target path does not yet exist but an intermediate ancestor might be a
// user-owned symlink that escapes the boundary.
func resolveExistingAncestor(absPath string) string {
	absPath = filepath.Clean(absPath)
	cur := absPath
	var tail []string
	for {
		if _, err := filepath.EvalSymlinks(cur); err == nil {
			real, err := filepath.EvalSymlinks(cur)
			if err != nil {
				return ""
			}
			if len(tail) == 0 {
				return real
			}
			// Re-attach the unresolved tail components.
			for i := len(tail) - 1; i >= 0; i-- {
				real = filepath.Join(real, tail[i])
			}
			return real
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return ""
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
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
