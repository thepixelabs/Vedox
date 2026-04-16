package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TempDir creates a temporary directory that is automatically removed when
// the test ends. It is a thin wrapper around t.TempDir() that also evaluates
// symlinks so paths are stable on macOS (/private/var/folders vs /var/folders).
func TempDir(t *testing.T) string {
	t.Helper()
	raw := t.TempDir()
	resolved, err := filepath.EvalSymlinks(raw)
	if err != nil {
		// EvalSymlinks is best-effort; fall back to the original path.
		return raw
	}
	return resolved
}

// AssertFileContains reads the file at path and fails the test if it does not
// contain substring.
func AssertFileContains(t *testing.T, path, substring string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("AssertFileContains: read %s: %v", path, err)
	}
	if !strings.Contains(string(b), substring) {
		t.Errorf("AssertFileContains: %s\n  want substring: %q\n  got content:    %q", path, substring, string(b))
	}
}

// AssertFileNotContains reads the file at path and fails the test if it
// contains substring.
func AssertFileNotContains(t *testing.T, path, substring string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("AssertFileNotContains: read %s: %v", path, err)
	}
	if strings.Contains(string(b), substring) {
		t.Errorf("AssertFileNotContains: %s\n  must not contain: %q\n  but got content:  %q", path, substring, string(b))
	}
}
