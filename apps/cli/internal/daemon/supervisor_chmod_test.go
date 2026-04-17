package daemon

// supervisor_chmod_test.go — FIX-SEC-06 regression coverage for
// writeFileAtomic. The fix replaced path-based os.Chmod(tmpName, perm) (TOCTOU
// — a local attacker that can write in dir could substitute the path between
// Close and Chmod) with tmp.Chmod(perm) via fchmod on the open fd, applied
// BEFORE the file is closed.

import (
	"os"
	"testing"
)

// TestWriteFileAtomic_AppliesFileMode_FCHmod verifies that the mode passed to
// writeFileAtomic lands on the final file. The fix applies chmod via fchmod
// on the open descriptor before Close, eliminating the TOCTOU window.
func TestWriteFileAtomic_AppliesFileMode_FCHmod(t *testing.T) {
	dir := t.TempDir()
	target := dir + "/mode.txt"

	// 0o600: owner read/write only — what a launchd plist or systemd unit
	// directory would use for sensitive config.
	if err := writeFileAtomic(target, []byte("x"), 0o600); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("file mode after writeFileAtomic: got %o, want 0600", got)
	}
}

// TestWriteFileAtomic_NoTmpLeftOnSuccess confirms the temp file is cleaned up
// on the happy path. A leak here could indicate the chmod step failed
// silently after Close.
func TestWriteFileAtomic_NoTmpLeftOnSuccess(t *testing.T) {
	dir := t.TempDir()
	target := dir + "/final.txt"

	if err := writeFileAtomic(target, []byte("payload"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "final.txt" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("unexpected directory contents after writeFileAtomic: %v", names)
	}
}
