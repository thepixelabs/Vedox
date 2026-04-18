package daemon_test

// Tests for TailLog, DefaultVedoxHome, SendSignal, and WaitForExit. These
// were previously 0% covered yet underpin the `vedox server logs --follow`
// and graceful-stop paths that users interact with directly.

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/vedox/vedox/internal/daemon"
)

// TestTailLog_LastN_NoFollow verifies the simple "dump the last N lines and
// return" path — the default invocation of `vedox server logs`.
func TestTailLog_LastN_NoFollow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	content := "line-1\nline-2\nline-3\nline-4\nline-5\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf bytes.Buffer
	err := daemon.TailLog(context.Background(), path, 3, false, &buf)
	if err != nil {
		t.Fatalf("TailLog: %v", err)
	}
	got := buf.String()
	// Last 3 lines: line-3, line-4, line-5
	if !strings.Contains(got, "line-3") || !strings.Contains(got, "line-5") {
		t.Errorf("tail -n 3 output missing expected lines: %q", got)
	}
	if strings.Contains(got, "line-1") {
		t.Errorf("tail -n 3 emitted line-1 (beyond the window): %q", got)
	}
}

// TestTailLog_MissingFile returns a distinctive error, not a panic, so the
// CLI can print a helpful message ("daemon may not have started yet").
func TestTailLog_MissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nope.log")

	var buf bytes.Buffer
	err := daemon.TailLog(context.Background(), path, 10, false, &buf)
	if err == nil {
		t.Fatalf("TailLog(missing) = nil, want error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q; want substring 'not found'", err.Error())
	}
}

// TestTailLog_Follow_EmitsAppended verifies the follow branch: after the
// initial dump, new lines appended to the file are streamed to w.
// This is the branch that powers `vedox server logs --follow`.
func TestTailLog_Follow_EmitsAppended(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "follow.log")
	if err := os.WriteFile(path, []byte("seed\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	// A safe writer we can read from after the goroutine exits.
	var buf safeBuffer

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- daemon.TailLog(ctx, path, 10, true, &buf)
	}()

	// Give TailLog time to emit the initial content and settle in its tick
	// loop before appending. The poll interval is 250 ms.
	time.Sleep(400 * time.Millisecond)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open append: %v", err)
	}
	if _, err := f.WriteString("new-line-1\n"); err != nil {
		t.Fatalf("append: %v", err)
	}
	f.Close()

	// Wait past one more tick, then stop.
	time.Sleep(400 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("TailLog follow: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("TailLog did not return after ctx cancel")
	}

	got := buf.String()
	if !strings.Contains(got, "seed") {
		t.Errorf("initial content 'seed' missing: %q", got)
	}
	if !strings.Contains(got, "new-line-1") {
		t.Errorf("appended content 'new-line-1' missing: %q", got)
	}
}

// TestTailLog_Follow_RotationDetection exercises the `current < lastSize`
// branch that detects log rotation (file was truncated or replaced).
func TestTailLog_Follow_RotationDetection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rotate.log")
	if err := os.WriteFile(path, []byte("pre-rotation-a\npre-rotation-b\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	var buf safeBuffer

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- daemon.TailLog(ctx, path, 10, true, &buf)
	}()
	time.Sleep(400 * time.Millisecond)

	// Simulate a rotation: truncate the file and write new shorter content.
	if err := os.WriteFile(path, []byte("post-rotation\n"), 0o600); err != nil {
		t.Fatalf("rotate (truncate + write): %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("TailLog: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("did not return")
	}
	got := buf.String()
	if !strings.Contains(got, "post-rotation") {
		t.Errorf("post-rotation content missing; rotation detection failed: %q", got)
	}
}

// TestDefaultVedoxHome creates and returns ~/.vedox. We redirect HOME to a
// temp dir so the real home is not touched.
func TestDefaultVedoxHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got, err := daemon.DefaultVedoxHome()
	if err != nil {
		t.Fatalf("DefaultVedoxHome: %v", err)
	}
	expect := filepath.Join(home, ".vedox")
	if got != expect {
		t.Errorf("path = %q, want %q", got, expect)
	}
	// Directory must actually exist with 0700.
	info, err := os.Stat(got)
	if err != nil {
		t.Fatalf("stat created dir: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("result is not a directory")
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("mode = %v, want 0700", info.Mode().Perm())
	}
}

// TestSendSignal_And_WaitForExit exercises both functions against a real
// sleep process. `kill -0` in SendSignal validates the PID; WaitForExit
// polls until the process terminates.
func TestSendSignal_And_WaitForExit(t *testing.T) {
	// Start a sleep we can signal without affecting the test harness.
	cmd := exec.Command("sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	pid := cmd.Process.Pid
	t.Cleanup(func() {
		// Best-effort cleanup in case we failed before signalling.
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	// signal 0 = no-op probe; must succeed for a live PID.
	if err := daemon.SendSignal(pid, syscall.Signal(0)); err != nil {
		t.Fatalf("SendSignal(0) live pid: %v", err)
	}

	// Actual terminate.
	if err := daemon.SendSignal(pid, syscall.SIGTERM); err != nil {
		t.Fatalf("SendSignal(SIGTERM): %v", err)
	}

	// Reap to avoid zombie accounting interfering with WaitForExit polling.
	_, _ = cmd.Process.Wait()

	if alive := daemon.WaitForExit(pid, 3*time.Second); !alive {
		t.Errorf("WaitForExit = false, want true (process should be gone)")
	}
}

// safeBuffer guards a bytes.Buffer with a Mutex so concurrent writer/reader
// access from the TailLog goroutine and the test's String() read does not
// trigger the race detector.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
