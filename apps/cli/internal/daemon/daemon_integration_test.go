package daemon_test

// Integration tests for the daemon package that exercise cross-component
// behaviour: the full PID file lifecycle, bootstrap token file properties,
// /healthz over a real HTTP server, and port conflict detection.
//
// None of these tests fork a child process — the daemon is run as a goroutine
// with an in-process httptest.Server so the test suite remains hermetic and
// works under -race.

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vedox/vedox/internal/daemon"
	"github.com/vedox/vedox/internal/portcheck"
	"github.com/vedox/vedox/internal/testutil"
)

// ---- helpers ----------------------------------------------------------------

// newTestPaths creates a Paths rooted at a temporary directory and ensures all
// subdirectories exist.
func newTestPaths(t *testing.T) daemon.Paths {
	t.Helper()
	dir := testutil.TempDir(t)
	p := daemon.NewPaths(dir)
	if err := daemon.EnsureDirs(p); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	return p
}

// startTestServer starts a minimal HTTP server with the daemon's /healthz
// handler on a random loopback port. It registers t.Cleanup to shut down the
// server. Returns the base URL (e.g. "http://127.0.0.1:PORT") and the port.
func startTestServer(t *testing.T, p daemon.Paths, version, commit, buildDate string) (baseURL string, port int) {
	t.Helper()

	startTime := time.Now()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", daemon.HealthzHandler(version, commit, buildDate, "", startTime))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port = ln.Addr().(*net.TCPAddr).Port
	baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)

	srv := &http.Server{Handler: mux}
	go srv.Serve(ln) //nolint:errcheck

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(ctx) //nolint:errcheck
		ln.Close()
	})

	// Wait until the server is reachable.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/healthz")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	return baseURL, port
}

// ---- Test: PID file lifecycle -----------------------------------------------

// TestIntegration_PIDLifecycle verifies the full write → read → cleanup
// cycle for a PID file in the context of realistic paths, including
// intermediate directory creation and mode enforcement.
func TestIntegration_PIDLifecycle(t *testing.T) {
	p := newTestPaths(t)

	rec := daemon.PIDRecord{
		PID:         os.Getpid(),
		Port:        5150,
		StartUnixNS: time.Now().UnixNano(),
		Version:     "0.2.0-test",
	}

	// Write the PID file.
	if err := daemon.WritePIDFile(p.PIDFile, rec); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	// File must exist at the expected path.
	if _, err := os.Stat(p.PIDFile); err != nil {
		t.Fatalf("PID file not created at %s: %v", p.PIDFile, err)
	}

	// Mode must be 0600.
	info, err := os.Stat(p.PIDFile)
	if err != nil {
		t.Fatalf("stat PID file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("PID file permissions: got %04o, want 0600", perm)
	}

	// Read back and verify every field.
	got, err := daemon.ReadPIDFile(p.PIDFile)
	if err != nil {
		t.Fatalf("ReadPIDFile: %v", err)
	}
	if got.PID != rec.PID {
		t.Errorf("PID: got %d, want %d", got.PID, rec.PID)
	}
	if got.Port != rec.Port {
		t.Errorf("Port: got %d, want %d", got.Port, rec.Port)
	}
	if got.StartUnixNS != rec.StartUnixNS {
		t.Errorf("StartUnixNS: got %d, want %d", got.StartUnixNS, rec.StartUnixNS)
	}
	if got.Version != rec.Version {
		t.Errorf("Version: got %q, want %q", got.Version, rec.Version)
	}

	// IsAlive must be true for the current process.
	if !daemon.IsAlive(got.PID) {
		t.Error("IsAlive: should be true for the current process PID")
	}

	// CleanupRunFiles removes the PID file.
	daemon.CleanupRunFiles(p)

	if _, err := os.Stat(p.PIDFile); !os.IsNotExist(err) {
		t.Errorf("expected PID file to be removed by CleanupRunFiles, stat: %v", err)
	}
}

// ---- Test: bootstrap token file properties ----------------------------------

// TestIntegration_BootstrapTokenFile verifies the complete token lifecycle:
// generation → write → read-back, with mode and format checks.
func TestIntegration_BootstrapTokenFile(t *testing.T) {
	p := newTestPaths(t)

	// Generate a token.
	tok, err := daemon.GenerateBootstrapToken()
	if err != nil {
		t.Fatalf("GenerateBootstrapToken: %v", err)
	}

	// Must be exactly 64 hex characters (32 random bytes encoded as hex).
	if len(tok) != 64 {
		t.Errorf("token length: got %d, want 64", len(tok))
	}
	for _, c := range tok {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("token is not lowercase hex: character %q in %q", c, tok)
			break
		}
	}

	// Write to the canonical token file path.
	if err := daemon.WriteTokenFile(p.TokenFile, tok); err != nil {
		t.Fatalf("WriteTokenFile: %v", err)
	}

	// File must exist and have mode 0600.
	info, err := os.Stat(p.TokenFile)
	if err != nil {
		t.Fatalf("stat token file %s: %v", p.TokenFile, err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("token file permissions: got %04o, want 0600", perm)
	}

	// Read back must return the exact same token (trimming the trailing newline
	// that WriteTokenFile appends).
	got, err := daemon.ReadTokenFile(p.TokenFile)
	if err != nil {
		t.Fatalf("ReadTokenFile: %v", err)
	}
	if got != tok {
		t.Errorf("token round-trip mismatch: got %q, want %q", got, tok)
	}

	// The raw file content must end with a newline per spec.
	raw, err := os.ReadFile(p.TokenFile)
	if err != nil {
		t.Fatalf("ReadFile token: %v", err)
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Error("token file must end with newline")
	}
}

// ---- Test: /healthz responds over a real listener ---------------------------

// TestIntegration_HealthzOverRealListener starts an HTTP server on a random
// loopback port, hits /healthz via QueryHealthz, and asserts the response
// fields match what was passed to HealthzHandler.
func TestIntegration_HealthzOverRealListener(t *testing.T) {
	p := newTestPaths(t)
	baseURL, port := startTestServer(t, p, "0.2.0", "abc1234", "2026-04-15T00:00:00Z")

	h, err := daemon.QueryHealthz(baseURL)
	if err != nil {
		t.Fatalf("QueryHealthz: %v", err)
	}

	if h.Status != "ok" {
		t.Errorf("status: got %q, want ok", h.Status)
	}
	if h.Version != "0.2.0" {
		t.Errorf("version: got %q, want 0.2.0", h.Version)
	}
	if h.Commit != "abc1234" {
		t.Errorf("commit: got %q, want abc1234", h.Commit)
	}
	if h.UptimeSeconds < 0 {
		t.Errorf("uptime_seconds must not be negative, got %d", h.UptimeSeconds)
	}
	if h.PID <= 0 {
		t.Errorf("pid must be positive, got %d", h.PID)
	}
	_ = port // used for server startup; port is embedded in baseURL
}

// ---- Test: /healthz JSON structure does not leak sensitive fields -----------

// TestIntegration_HealthzNoSecretFields hits a real listener and asserts the
// raw JSON body contains none of the forbidden field names from spec §5.5.
func TestIntegration_HealthzNoSecretFields(t *testing.T) {
	p := newTestPaths(t)
	baseURL, _ := startTestServer(t, p, "test", "sha", "date")

	resp, err := http.Get(baseURL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	forbidden := []string{"email", "key_id", "token", "secret", "password", "repo_path", "agent_key"}
	for _, f := range forbidden {
		if _, ok := raw[f]; ok {
			t.Errorf("healthz JSON contains forbidden field %q", f)
		}
	}
}

// ---- Test: port conflict detection ------------------------------------------

// TestIntegration_PortConflict binds a port explicitly, then verifies that
// portcheck.CheckPort returns a VDX-001 error for the same port. This
// simulates the daemon startup failing cleanly when its port is already taken.
func TestIntegration_PortConflict(t *testing.T) {
	// Bind a port on loopback.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	// portcheck.CheckPort must see the port as in-use.
	err = portcheck.CheckPort(port)
	if err == nil {
		t.Fatalf("expected port conflict error for port %d, got nil", port)
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "VDX-001") {
		t.Errorf("expected VDX-001 error code in message, got: %v", err)
	}
	if !strings.Contains(errMsg, fmt.Sprintf("%d", port)) {
		t.Errorf("expected port number %d in error message, got: %v", port, err)
	}
}

// ---- Test: advisory lock prevents double-start ------------------------------

// TestIntegration_AdvisoryLock_DoubleStart verifies that attempting to acquire
// the daemon advisory lock twice returns ErrAlreadyRunning on the second
// attempt, simulating two daemon instances starting against the same PID dir.
func TestIntegration_AdvisoryLock_DoubleStart(t *testing.T) {
	p := newTestPaths(t)

	lock1, err := daemon.AcquireLock(p.LockFile)
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	defer lock1.Release()

	_, err = daemon.AcquireLock(p.LockFile)
	if err != daemon.ErrAlreadyRunning {
		t.Errorf("second AcquireLock: got %v, want ErrAlreadyRunning", err)
	}
}

// ---- Test: lock released, new instance can acquire -------------------------

// TestIntegration_LockReleaseAllowsRe-Acquire verifies that after Release(),
// a new AcquireLock call succeeds (simulating a clean daemon restart).
func TestIntegration_LockReleaseAllowsReAcquire(t *testing.T) {
	p := newTestPaths(t)

	lock1, err := daemon.AcquireLock(p.LockFile)
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	lock1.Release()

	lock2, err := daemon.AcquireLock(p.LockFile)
	if err != nil {
		t.Fatalf("second AcquireLock after Release: %v", err)
	}
	defer lock2.Release()
}

// ---- Test: UptimeCounter increments under context cancellation --------------

// TestIntegration_UptimeCounter_Increments starts the uptime counter, waits
// for at least one tick, then cancels the context and verifies the counter
// stopped incrementing.
func TestIntegration_UptimeCounter_Increments(t *testing.T) {
	var c daemon.UptimeCounter
	ctx, cancel := context.WithCancel(context.Background())

	daemon.StartUptimeCounter(ctx, &c)

	// Wait for at least 1 second so the ticker fires at least once.
	time.Sleep(1100 * time.Millisecond)
	cancel()

	val := c.Get()
	if val < 1 {
		t.Errorf("expected uptime counter >= 1 after 1s, got %d", val)
	}

	// After cancel, the counter must not increment further.
	before := c.Get()
	time.Sleep(600 * time.Millisecond)
	after := c.Get()
	if after > before {
		t.Errorf("uptime counter should not increment after context cancel: before=%d after=%d", before, after)
	}
}

// ---- Test: EnsureDirs creates all required directories ----------------------

// TestIntegration_EnsureDirs verifies that EnsureDirs creates RunDir, LogDir,
// and CrashDir with mode 0700 under the given home directory.
func TestIntegration_EnsureDirs(t *testing.T) {
	dir := testutil.TempDir(t)
	p := daemon.NewPaths(filepath.Join(dir, "vedox-home"))

	if err := daemon.EnsureDirs(p); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}

	for _, d := range []string{p.RunDir, p.LogDir, p.CrashDir} {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("directory not created: %s: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected directory, got file: %s", d)
		}
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Errorf("directory %s permissions: got %04o, want 0700", d, perm)
		}
	}
}

// ---- Test: port sidecar + PID file form a consistent record -----------------

// TestIntegration_PortSidecarAndPIDConsistency writes both the port sidecar
// and PID file, then reads them back and verifies the port values agree.
func TestIntegration_PortSidecarAndPIDConsistency(t *testing.T) {
	p := newTestPaths(t)
	const wantPort = 5175

	rec := daemon.PIDRecord{
		PID:         os.Getpid(),
		Port:        wantPort,
		StartUnixNS: time.Now().UnixNano(),
		Version:     "integration-test",
	}
	if err := daemon.WritePIDFile(p.PIDFile, rec); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}
	if err := daemon.WritePortSidecar(p.PortFile, wantPort); err != nil {
		t.Fatalf("WritePortSidecar: %v", err)
	}

	pidRec, err := daemon.ReadPIDFile(p.PIDFile)
	if err != nil {
		t.Fatalf("ReadPIDFile: %v", err)
	}
	if pidRec.Port != wantPort {
		t.Errorf("PID file port: got %d, want %d", pidRec.Port, wantPort)
	}

	portData, err := os.ReadFile(p.PortFile)
	if err != nil {
		t.Fatalf("ReadFile port sidecar: %v", err)
	}
	portStr := strings.TrimSpace(string(portData))
	if portStr != fmt.Sprintf("%d", wantPort) {
		t.Errorf("port sidecar: got %q, want %d", portStr, wantPort)
	}
}
