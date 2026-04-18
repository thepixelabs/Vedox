package api_test

// Re-audit security tests for GET /api/browse (wave 1 — verifying wave-0 fixes).
//
// These tests assert the advanced attack vectors that wave-0 tests did NOT
// cover: symlink escapes, URL-encoded traversal, and post-clean traversal.

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBrowse_URLEncodedTraversal asserts that a URL-encoded "../.." sequence
// is resolved by net/url BEFORE reaching the handler (Go's http.ServeMux /
// chi decode the path), so the effective `path=` query value is compared by
// withinHomeDir after filepath.Clean. Sending %2e%2e%2f%2e%2e%2f etc. must
// not be able to escape $HOME.
func TestBrowse_URLEncodedTraversal(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}

	f := newBrowseFixture(t)

	// Build a path that would, after URL decode + filepath.Clean, resolve to
	// /etc — which is outside $HOME on every supported OS. The request URL
	// carries the raw %2e%2e sequences; the server must reject with 403.
	escape := filepath.Join(home, "%2e%2e", "%2e%2e", "etc")
	resp := browseGet(t, f, escape, "Bearer "+testBrowseToken)

	// With URL-encoded traversal the decoded path STILL contains ".." which
	// filepath.Clean would resolve out; after Clean the path is "/etc" which
	// is not under $HOME. The expected behaviour is 403.
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
		// Accept 404 if the Clean result points to a path that is inside home
		// but does not exist (home/%2e%2e literal). Anything else is a bypass.
		body := readBody(t, resp)
		if !strings.Contains(body, "VDX-403") {
			t.Fatalf("URL-encoded traversal: got %d body=%s — expected 403 or home-contained 404",
				resp.StatusCode, body)
		}
	}
	body := readBody(t, resp)
	// Whatever the status, the response must NOT list contents of /etc.
	// Common /etc entries on macOS and Linux — reject any appearance.
	for _, leak := range []string{"hosts", "passwd", "resolv.conf", "nginx", "systemd"} {
		if strings.Contains(body, `"name":"`+leak+`"`) {
			t.Fatalf("URL-encoded traversal leaked /etc entry %q: body=%s", leak, body)
		}
	}
}

// TestBrowse_DoubleDotInsideHome asserts that a path like
// /Users/victim/../etc — which filepath.Clean collapses to /etc — is rejected
// AFTER abs resolution because withinHomeDir runs filepath.Clean itself and
// compares the clean form against the home dir.
func TestBrowse_DoubleDotInsideHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}

	f := newBrowseFixture(t)

	// A path that literally traverses out of home after Clean.
	escape := filepath.Join(home, "..", "..", "etc")
	resp := browseGet(t, f, escape, "Bearer "+testBrowseToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("double-dot traversal: got %d, want 403 (body=%s)",
			resp.StatusCode, readBody(t, resp))
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "VDX-403") {
		t.Errorf("expected VDX-403 in body, got %s", body)
	}
}

// TestBrowse_SymlinkEscapeInsideHome is the critical test missed by wave 0.
// We create a symlink INSIDE $HOME that points OUTSIDE $HOME, then request
// browse on that symlink. filepath.Abs does not resolve symlinks, so without
// an EvalSymlinks-based guard the daemon would happily follow the symlink
// and enumerate the target directory (potentially /etc or /tmp). The re-audit
// fix resolves the real path after the prefix check and re-verifies the
// boundary.
func TestBrowse_SymlinkEscapeInsideHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}

	// Create the trap directory + symlink inside $HOME, pointing to /tmp.
	// Using a unique name avoids collisions with prior test runs.
	trapDir, err := os.MkdirTemp(home, ".vedox-reaudit-trap-*")
	if err != nil {
		t.Fatalf("MkdirTemp in $HOME: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(trapDir) })

	symlinkPath := filepath.Join(trapDir, "escape")
	if err := os.Symlink("/tmp", symlinkPath); err != nil {
		t.Skipf("symlink creation failed (read-only FS or permissions): %v", err)
	}

	f := newBrowseFixture(t)
	resp := browseGet(t, f, symlinkPath, "Bearer "+testBrowseToken)

	// The re-audit guard must reject with 403. Without the guard the status
	// would be 200 and the body would contain /tmp entries.
	if resp.StatusCode != http.StatusForbidden {
		body := readBody(t, resp)
		t.Fatalf("symlink escape: got %d (want 403 VDX-403) body=%s",
			resp.StatusCode, body)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "VDX-403") {
		t.Errorf("expected VDX-403, got %s", body)
	}
	// Absolute path must not leak in the response (MED-02 carry-forward).
	if strings.Contains(body, "/tmp") {
		t.Errorf("response leaks /tmp: %s", body)
	}
}

// TestBrowse_SymlinkTraversalTargetEscapes covers the variant where the
// symlink is valid but its target IS outside $HOME. This is the concrete
// exploit described in the re-audit brief: ln -s /etc ~/escape, then
// GET /api/browse?path=~/escape.
func TestBrowse_SymlinkTraversalTargetEscapes(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}

	trapDir, err := os.MkdirTemp(home, ".vedox-reaudit-trap-*")
	if err != nil {
		t.Fatalf("MkdirTemp in $HOME: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(trapDir) })

	symlinkPath := filepath.Join(trapDir, "etc-link")
	if err := os.Symlink("/etc", symlinkPath); err != nil {
		t.Skipf("symlink creation failed: %v", err)
	}

	f := newBrowseFixture(t)
	resp := browseGet(t, f, symlinkPath, "Bearer "+testBrowseToken)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("symlink->/etc escape: got %d, want 403", resp.StatusCode)
	}
	// Defensive: body must not contain common /etc listings.
	body := readBody(t, resp)
	for _, leak := range []string{"passwd", "hosts", "ssh", "systemd"} {
		if strings.Contains(body, `"name":"`+leak+`"`) {
			t.Fatalf("symlink->/etc escape leaked %q: %s", leak, body)
		}
	}
}
