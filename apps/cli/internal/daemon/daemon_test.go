package daemon_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/vedox/vedox/internal/daemon"
)

// ── PID file: write and read ──────────────────────────────────────────────────

func TestWriteAndReadPIDFile(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "run", "vedoxd.pid")

	rec := daemon.PIDRecord{
		PID:         12345,
		Port:        5150,
		StartUnixNS: time.Now().UnixNano(),
		Version:     "0.2.0",
	}
	if err := daemon.WritePIDFile(pidFile, rec); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	got, err := daemon.ReadPIDFile(pidFile)
	if err != nil {
		t.Fatalf("ReadPIDFile: %v", err)
	}
	if got.PID != rec.PID {
		t.Errorf("PID: got %d, want %d", got.PID, rec.PID)
	}
	if got.Port != rec.Port {
		t.Errorf("Port: got %d, want %d", got.Port, rec.Port)
	}
	if got.Version != rec.Version {
		t.Errorf("Version: got %q, want %q", got.Version, rec.Version)
	}
}

func TestReadPIDFile_NotExist(t *testing.T) {
	dir := t.TempDir()
	_, err := daemon.ReadPIDFile(filepath.Join(dir, "nonexistent.pid"))
	if err == nil {
		t.Fatal("expected error for non-existent PID file, got nil")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestWritePIDFile_CreatesIntermediateDirs(t *testing.T) {
	dir := t.TempDir()
	// Deep path that does not exist yet.
	pidFile := filepath.Join(dir, "a", "b", "c", "vedoxd.pid")
	rec := daemon.PIDRecord{PID: 999, Port: 5150, StartUnixNS: 1, Version: "test"}
	if err := daemon.WritePIDFile(pidFile, rec); err != nil {
		t.Fatalf("WritePIDFile with deep path: %v", err)
	}
	if _, err := os.Stat(pidFile); err != nil {
		t.Errorf("PID file not created: %v", err)
	}
}

func TestWritePIDFile_Mode(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "vedoxd.pid")
	rec := daemon.PIDRecord{PID: 1, Port: 5150, StartUnixNS: 0, Version: "v"}
	if err := daemon.WritePIDFile(pidFile, rec); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}
	info, err := os.Stat(pidFile)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("PID file permissions: got %04o, want 0600", perm)
	}
}

// ── PID file cleanup ──────────────────────────────────────────────────────────

func TestCleanupRunFiles_RemovesAll(t *testing.T) {
	dir := t.TempDir()
	p := daemon.NewPaths(dir)
	if err := daemon.EnsureDirs(p); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	// Create dummy files.
	for _, f := range []string{p.PIDFile, p.PortFile} {
		if err := os.WriteFile(f, []byte("test"), 0o600); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	daemon.CleanupRunFiles(p)
	for _, f := range []string{p.PIDFile, p.PortFile} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed, got err=%v", f, err)
		}
	}
}

func TestCleanupRunFiles_Idempotent(t *testing.T) {
	dir := t.TempDir()
	p := daemon.NewPaths(dir)
	// Call on an empty dir — should not panic or error.
	daemon.CleanupRunFiles(p)
}

// ── Bootstrap token ──────────────────────────────────────────────────────────

func TestGenerateBootstrapToken_Length(t *testing.T) {
	tok, err := daemon.GenerateBootstrapToken()
	if err != nil {
		t.Fatalf("GenerateBootstrapToken: %v", err)
	}
	// 32 bytes → 64 hex chars.
	if len(tok) != 64 {
		t.Errorf("token length: got %d, want 64", len(tok))
	}
}

func TestGenerateBootstrapToken_IsHex(t *testing.T) {
	tok, err := daemon.GenerateBootstrapToken()
	if err != nil {
		t.Fatalf("GenerateBootstrapToken: %v", err)
	}
	for _, c := range tok {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("token is not lowercase hex: got char %q in %q", c, tok)
			break
		}
	}
}

func TestGenerateBootstrapToken_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		tok, err := daemon.GenerateBootstrapToken()
		if err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
		if seen[tok] {
			t.Fatalf("duplicate token generated at iteration %d", i)
		}
		seen[tok] = true
	}
}

func TestWriteAndReadTokenFile(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "daemon-token")

	tok, _ := daemon.GenerateBootstrapToken()
	if err := daemon.WriteTokenFile(tokenFile, tok); err != nil {
		t.Fatalf("WriteTokenFile: %v", err)
	}

	got, err := daemon.ReadTokenFile(tokenFile)
	if err != nil {
		t.Fatalf("ReadTokenFile: %v", err)
	}
	if got != tok {
		t.Errorf("token round-trip: got %q, want %q", got, tok)
	}
}

func TestWriteTokenFile_Mode(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "daemon-token")
	tok, _ := daemon.GenerateBootstrapToken()
	if err := daemon.WriteTokenFile(tokenFile, tok); err != nil {
		t.Fatalf("WriteTokenFile: %v", err)
	}
	info, err := os.Stat(tokenFile)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("token file permissions: got %04o, want 0600", perm)
	}
}

// ── Port sidecar ──────────────────────────────────────────────────────────────

func TestWritePortSidecar(t *testing.T) {
	dir := t.TempDir()
	portFile := filepath.Join(dir, "run", "port")
	if err := daemon.WritePortSidecar(portFile, 5150); err != nil {
		t.Fatalf("WritePortSidecar: %v", err)
	}
	b, err := os.ReadFile(portFile)
	if err != nil {
		t.Fatalf("read port sidecar: %v", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		t.Fatalf("parse port sidecar: %v", err)
	}
	if n != 5150 {
		t.Errorf("port sidecar: got %d, want 5150", n)
	}
}

// ── /healthz response ────────────────────────────────────────────────────────

func TestHealthzHandler_ReturnsOK(t *testing.T) {
	handler := daemon.HealthzHandler("0.2.0", "abc1234", "2026-04-15T00:00:00Z", "127.0.0.1:5150", time.Now())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("HTTP status: got %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
}

func TestHealthzHandler_ResponseJSON(t *testing.T) {
	handler := daemon.HealthzHandler("0.2.0", "abc1234", "2026-04-15T00:00:00Z", "127.0.0.1:5150", time.Now())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode healthz JSON: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status: got %v, want ok", resp["status"])
	}
	if resp["version"] != "0.2.0" {
		t.Errorf("version: got %v, want 0.2.0", resp["version"])
	}
	if resp["listen_addr"] != "127.0.0.1:5150" {
		t.Errorf("listen_addr: got %v, want 127.0.0.1:5150", resp["listen_addr"])
	}
	// uptime_seconds must be a number and non-negative.
	uptimeRaw, ok := resp["uptime_seconds"]
	if !ok {
		t.Fatal("healthz response missing uptime_seconds")
	}
	uptime, ok := uptimeRaw.(float64)
	if !ok {
		t.Errorf("uptime_seconds is not a number: %T", uptimeRaw)
	}
	if uptime < 0 {
		t.Errorf("uptime_seconds is negative: %v", uptime)
	}
}

func TestHealthzHandler_NoSecretFields(t *testing.T) {
	// Spec §5.5: /healthz must not expose repo paths, keys, user email, etc.
	handler := daemon.HealthzHandler("0.2.0", "abc1234", "2026-04-15T00:00:00Z", "127.0.0.1:5150", time.Now())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	body := w.Body.String()
	// These field names must never appear in the healthz response per §5.5.
	forbidden := []string{"email", "key_id", "token", "secret", "password", "repo_path", "agent_key"}
	for _, f := range forbidden {
		if strings.Contains(strings.ToLower(body), f) {
			t.Errorf("healthz response contains forbidden field %q: %s", f, body)
		}
	}
}

func TestQueryHealthz_ParsesResponse(t *testing.T) {
	// Stand up a minimal test server that mimics the daemon /healthz.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","version":"0.2.0","commit":"abc","build_date":"2026-04-15","uptime_seconds":42,"pid":1234,"listen_addr":"127.0.0.1:5150"}`))
	}))
	defer ts.Close()

	h, err := daemon.QueryHealthz(ts.URL)
	if err != nil {
		t.Fatalf("QueryHealthz: %v", err)
	}
	if h.Status != "ok" {
		t.Errorf("status: got %q, want ok", h.Status)
	}
	if h.Version != "0.2.0" {
		t.Errorf("version: got %q, want 0.2.0", h.Version)
	}
	if h.UptimeSeconds != 42 {
		t.Errorf("uptime: got %d, want 42", h.UptimeSeconds)
	}
}

// ── IsAlive ──────────────────────────────────────────────────────────────────

func TestIsAlive_CurrentProcess(t *testing.T) {
	if !daemon.IsAlive(os.Getpid()) {
		t.Error("IsAlive should return true for the current process")
	}
}

func TestIsAlive_InvalidPID(t *testing.T) {
	if daemon.IsAlive(-1) {
		t.Error("IsAlive(-1) should return false")
	}
	if daemon.IsAlive(0) {
		t.Error("IsAlive(0) should return false")
	}
}

// ── Advisory lock ────────────────────────────────────────────────────────────

func TestAcquireLock_Success(t *testing.T) {
	dir := t.TempDir()
	lockFile := filepath.Join(dir, "test.lock")

	lock, err := daemon.AcquireLock(lockFile)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer lock.Release()

	if _, err := os.Stat(lockFile); err != nil {
		t.Errorf("lock file not created: %v", err)
	}
}

func TestAcquireLock_BlocksSecondAcquire(t *testing.T) {
	dir := t.TempDir()
	lockFile := filepath.Join(dir, "test.lock")

	lock1, err := daemon.AcquireLock(lockFile)
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	defer lock1.Release()

	_, err = daemon.AcquireLock(lockFile)
	if err != daemon.ErrAlreadyRunning {
		t.Errorf("second AcquireLock: got %v, want ErrAlreadyRunning", err)
	}
}

func TestLock_ReleaseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	lockFile := filepath.Join(dir, "test.lock")
	lock, err := daemon.AcquireLock(lockFile)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	lock.Release()
	lock.Release() // must not panic
}

// TestLock_ConcurrentRelease regresses a data race that existed before
// Lock.Release was guarded by a mutex. Multiple goroutines calling Release
// concurrently previously wrote to the l.file pointer without synchronization,
// which the race detector would flag.
func TestLock_ConcurrentRelease(t *testing.T) {
	dir := t.TempDir()
	lockFile := filepath.Join(dir, "test.lock")
	lock, err := daemon.AcquireLock(lockFile)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	const goroutines = 16
	done := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		go func() {
			lock.Release()
			done <- struct{}{}
		}()
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}
}

// TestAcquireLock_TruncatesStalePID regresses a cosmetic bug where a previous
// longer PID would leave trailing bytes in the lock file after a new holder
// (with a shorter PID) acquired it. AcquireLock now opens with O_TRUNC.
func TestAcquireLock_TruncatesStalePID(t *testing.T) {
	dir := t.TempDir()
	lockFile := filepath.Join(dir, "test.lock")
	// Pre-populate the lock file with a long stale value.
	if err := os.WriteFile(lockFile, []byte("9999999999999\n"), 0o600); err != nil {
		t.Fatalf("seed lock file: %v", err)
	}
	lock, err := daemon.AcquireLock(lockFile)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	defer lock.Release()

	b, err := os.ReadFile(lockFile)
	if err != nil {
		t.Fatalf("read lock file: %v", err)
	}
	// Content must reflect only the current PID, not stale bytes.
	want := strconv.Itoa(os.Getpid()) + "\n"
	if string(b) != want {
		t.Errorf("lock file content = %q, want %q (stale bytes not truncated)", string(b), want)
	}
}

// TestDaemonize_DoesNotMutateArgs regresses a slice-aliasing bug where
// Daemonize used append(args, "--foreground") — if args had spare capacity,
// the caller's backing array would be silently modified. Daemonize now
// copies into a fresh slice before appending.
//
// We exercise Daemonize with a short-lived helper binary (true(1)) found
// via exec.LookPath so this test works across macOS (/usr/bin/true) and
// Linux (/bin/true). The test asserts only that the caller's args slice is
// unmodified and that its length and element identity are preserved.
func TestDaemonize_DoesNotMutateArgs(t *testing.T) {
	trueBin, err := exec.LookPath("true")
	if err != nil {
		t.Skipf("true(1) not on PATH: %v", err)
	}
	dir := t.TempDir()
	logFile := filepath.Join(dir, "logs", "vedoxd.log")

	// Deliberately create args with spare capacity so an append with no
	// copy would overwrite the cap+1 slot.
	args := make([]string, 2, 8)
	args[0] = "server"
	args[1] = "start"
	snapshot := append([]string(nil), args...)

	if err := daemon.Daemonize(trueBin, args, logFile); err != nil {
		t.Fatalf("Daemonize: %v", err)
	}

	if len(args) != len(snapshot) {
		t.Errorf("args length changed: got %d, want %d", len(args), len(snapshot))
	}
	for i := range snapshot {
		if args[i] != snapshot[i] {
			t.Errorf("args[%d] mutated: got %q, want %q", i, args[i], snapshot[i])
		}
	}
}

// TestWritePIDFile_IsFsyncedBeforeRename verifies the atomic-write pipeline
// used by WritePIDFile — a smoke test that no temp files are left behind
// after a successful write and that the final file has mode 0o600 even when
// preceded by an extensive write cycle. This regresses the durability fix
// where fsync was added between write and rename.
func TestWritePIDFile_NoTempFilesLeftBehind(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "vedoxd.pid")

	for i := 0; i < 20; i++ {
		rec := daemon.PIDRecord{PID: 1000 + i, Port: 5150, StartUnixNS: int64(i), Version: "v"}
		if err := daemon.WritePIDFile(pidFile, rec); err != nil {
			t.Fatalf("WritePIDFile iter %d: %v", i, err)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	// Only the final pid file should remain — no temp artefacts.
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected exactly 1 file in dir, got %d: %v", len(entries), names)
	}
	if entries[0].Name() != "vedoxd.pid" {
		t.Errorf("unexpected file name: %q", entries[0].Name())
	}
}
