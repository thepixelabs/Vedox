package providers

// internal_test.go — white-box tests for helpers that are not exported.
// Kept in the providers package (not providers_test) so we can exercise
// daemonPort, sha256Hex, etc. without inflating the public API.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── daemonPort ────────────────────────────────────────────────────────────────

// TestDaemonPort_ParsesWithAndWithoutPort covers the URL shapes the installers
// encounter: the default localhost URL, a non-default port, no port (should
// fall back), and malformed input.
func TestDaemonPort(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1:5150":  "5150",
		"http://127.0.0.1:65534": "65534",
		"https://vedox.local:443":      "443",
		"http://127.0.0.1":              "5150", // no port → default
		"":                              "5150", // empty → default
		"not a url":                     "5150", // unparseable → default
		"http://[::1]:5150":             "5150", // IPv6 literal works
	}
	for in, want := range cases {
		got := daemonPort(in)
		if got != want {
			t.Errorf("daemonPort(%q) = %q, want %q", in, got, want)
		}
	}
}

// ── atomicFileWrite regression tests ─────────────────────────────────────────

// TestAtomicFileWrite_AppliesFileMode verifies that the file mode passed to
// atomicFileWrite lands on the final file. The fix replaced path-based
// os.Chmod with fd-based tmp.Chmod; this test catches accidental regressions
// that would leave the tmp-default mode on the target.
func TestAtomicFileWrite_AppliesFileMode(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "sub", "secret.json")
	data := []byte(`{"hello":"world"}`)

	if err := atomicFileWrite(dir, target, data, 0o755, 0o600); err != nil {
		t.Fatalf("atomicFileWrite: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("file mode: got %v, want 0o600", got)
	}
}

// TestAtomicFileWrite_DoesNotLeaveTmpOnSuccess confirms the tmp file is
// renamed away (not left in the directory) after a successful write.
func TestAtomicFileWrite_DoesNotLeaveTmpOnSuccess(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "file.txt")
	if err := atomicFileWrite(dir, target, []byte("payload"), 0o755, 0o644); err != nil {
		t.Fatalf("atomicFileWrite: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".vedox-provider-") {
			t.Errorf("tmp file left behind: %s", e.Name())
		}
	}
}

// TestAtomicFileWrite_RejectsBoundaryEscape verifies the sandbox check.
func TestAtomicFileWrite_RejectsBoundaryEscape(t *testing.T) {
	dir := t.TempDir()
	// target is two levels above the boundary — should be rejected.
	err := atomicFileWrite(dir, filepath.Join(dir, "..", "..", "evil"),
		[]byte("x"), 0o755, 0o644)
	if err == nil {
		t.Fatal("expected error for out-of-boundary target, got nil")
	}
	if !strings.Contains(err.Error(), "escapes boundary") {
		t.Errorf("expected 'escapes boundary' in error, got %v", err)
	}
}
