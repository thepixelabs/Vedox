// Package daemon manages the Vedox daemon lifecycle: PID file management,
// advisory locking, bootstrap token generation, signal handling, and
// daemonization via self-re-exec.
//
// Design principles:
//   - PID file at ~/.vedox/run/vedoxd.pid (mode 0600)
//   - Advisory lock at ~/.vedox/run/vedoxd.pid.lock (held for daemon lifetime)
//   - Bootstrap token at ~/.vedox/daemon-token (mode 0600)
//   - Log file at ~/.vedox/logs/vedoxd.log (rotated by lumberjack)
//   - Port sidecar at ~/.vedox/run/port (mode 0600)
//
// All paths are derived from VedoxHome (default: ~/.vedox).
package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Paths holds the canonical file paths derived from a given home directory.
type Paths struct {
	Home      string // ~/.vedox
	RunDir    string // ~/.vedox/run
	LogDir    string // ~/.vedox/logs
	CrashDir  string // ~/.vedox/crashes
	PIDFile   string // ~/.vedox/run/vedoxd.pid
	LockFile  string // ~/.vedox/run/vedoxd.pid.lock
	TokenFile string // ~/.vedox/daemon-token
	LogFile   string // ~/.vedox/logs/vedoxd.log
	PortFile  string // ~/.vedox/run/port
}

// NewPaths returns the canonical Paths for the given vedoxHome directory.
func NewPaths(vedoxHome string) Paths {
	run := filepath.Join(vedoxHome, "run")
	logs := filepath.Join(vedoxHome, "logs")
	return Paths{
		Home:      vedoxHome,
		RunDir:    run,
		LogDir:    logs,
		CrashDir:  filepath.Join(vedoxHome, "crashes"),
		PIDFile:   filepath.Join(run, "vedoxd.pid"),
		LockFile:  filepath.Join(run, "vedoxd.pid.lock"),
		TokenFile: filepath.Join(vedoxHome, "daemon-token"),
		LogFile:   filepath.Join(logs, "vedoxd.log"),
		PortFile:  filepath.Join(run, "port"),
	}
}

// DefaultVedoxHome returns ~/.vedox, creating it if necessary.
func DefaultVedoxHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	vedoxHome := filepath.Join(home, ".vedox")
	if err := os.MkdirAll(vedoxHome, 0o700); err != nil {
		return "", fmt.Errorf("cannot create %s: %w", vedoxHome, err)
	}
	return vedoxHome, nil
}

// EnsureDirs creates all runtime directories required by the daemon.
func EnsureDirs(p Paths) error {
	for _, dir := range []string{p.RunDir, p.LogDir, p.CrashDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("cannot create %s: %w", dir, err)
		}
	}
	return nil
}

// LumberjackWriter returns a lumberjack.Logger configured per the spec:
// 50 MB per file, 7 rotated backups, 30-day max age, gzip compression.
func LumberjackWriter(logFile string) *lumberjack.Logger {
	return &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    50, // MB
		MaxBackups: 7,
		MaxAge:     30,   // days
		Compress:   true,
		LocalTime:  true,
	}
}

// GenerateBootstrapToken generates a cryptographically random 32-byte hex
// token. This is the Jupyter-style token that the editor uses for its first
// authenticated connection.
func GenerateBootstrapToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("token generation failed: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// WriteTokenFile writes token to tokenFile atomically (temp file then rename)
// at mode 0o600. Any existing token file is replaced.
func WriteTokenFile(tokenFile, token string) error {
	dir := filepath.Dir(tokenFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cannot create token dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".daemon-token-*")
	if err != nil {
		return fmt.Errorf("cannot create temp token file: %w", err)
	}
	tmp.Close()
	if err := os.WriteFile(tmp.Name(), []byte(token+"\n"), 0o600); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("cannot write token: %w", err)
	}
	if err := os.Rename(tmp.Name(), tokenFile); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("cannot install token file: %w", err)
	}
	return nil
}

// ReadTokenFile reads the token from tokenFile. Returns an error if the file
// does not exist or cannot be read.
func ReadTokenFile(tokenFile string) (string, error) {
	b, err := os.ReadFile(tokenFile)
	if err != nil {
		return "", fmt.Errorf("cannot read token file %s: %w", tokenFile, err)
	}
	return strings.TrimSpace(string(b)), nil
}

// PIDRecord is the content written to the PID file.
// Format: "<pid> <port> <start_unix_ns> <version>\n"
type PIDRecord struct {
	PID         int
	Port        int
	StartUnixNS int64
	Version     string
}

// WritePIDFile writes the PID record atomically (temp→rename) at mode 0o600.
func WritePIDFile(pidFile string, rec PIDRecord) error {
	dir := filepath.Dir(pidFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cannot create PID dir: %w", err)
	}
	line := fmt.Sprintf("%d %d %d %s\n", rec.PID, rec.Port, rec.StartUnixNS, rec.Version)
	tmp, err := os.CreateTemp(dir, ".vedoxd.pid-*")
	if err != nil {
		return fmt.Errorf("cannot create temp PID file: %w", err)
	}
	tmp.Close()
	if err := os.WriteFile(tmp.Name(), []byte(line), 0o600); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("cannot write PID: %w", err)
	}
	if err := os.Rename(tmp.Name(), pidFile); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("cannot install PID file: %w", err)
	}
	return nil
}

// ReadPIDFile reads the PID record from pidFile.
// Returns os.ErrNotExist if the file does not exist.
func ReadPIDFile(pidFile string) (PIDRecord, error) {
	b, err := os.ReadFile(pidFile)
	if err != nil {
		return PIDRecord{}, err
	}
	fields := strings.Fields(strings.TrimSpace(string(b)))
	if len(fields) < 3 {
		return PIDRecord{}, fmt.Errorf("malformed PID file: %q", string(b))
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return PIDRecord{}, fmt.Errorf("malformed PID field: %w", err)
	}
	port, err := strconv.Atoi(fields[1])
	if err != nil {
		return PIDRecord{}, fmt.Errorf("malformed port field: %w", err)
	}
	startNS, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return PIDRecord{}, fmt.Errorf("malformed start_ns field: %w", err)
	}
	ver := ""
	if len(fields) >= 4 {
		ver = fields[3]
	}
	return PIDRecord{PID: pid, Port: port, StartUnixNS: startNS, Version: ver}, nil
}

// IsAlive returns true if a process with the given PID exists and is running.
// It does NOT verify that the process is the vedox daemon.
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 tests for process existence without actually sending a signal.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// WritePortSidecar writes the port number to the port sidecar file at mode
// 0o600 using an atomic temp→rename pattern.
func WritePortSidecar(portFile string, port int) error {
	dir := filepath.Dir(portFile)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cannot create run dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".port-*")
	if err != nil {
		return fmt.Errorf("cannot create temp port file: %w", err)
	}
	tmp.Close()
	if err := os.WriteFile(tmp.Name(), []byte(strconv.Itoa(port)+"\n"), 0o600); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("cannot write port: %w", err)
	}
	if err := os.Rename(tmp.Name(), portFile); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("cannot install port sidecar: %w", err)
	}
	return nil
}

// CleanupRunFiles removes the PID file, lock file, and port sidecar in the
// correct order per spec §11.4. Errors are logged but do not prevent cleanup
// of subsequent files.
func CleanupRunFiles(p Paths) {
	for _, f := range []string{p.PIDFile, p.PortFile, p.LockFile} {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("cleanup: could not remove file", "path", f, "error", err)
		}
	}
}

// LockFile manages an exclusive advisory lock file for the daemon.
// The lock is held for the entire daemon lifetime.
type Lock struct {
	path string
	file *os.File
}

// AcquireLock opens lockPath for writing and acquires an exclusive,
// non-blocking flock(2). Returns ErrAlreadyRunning if another process holds
// the lock. The caller must call Release() on clean shutdown.
func AcquireLock(lockPath string) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		return nil, fmt.Errorf("cannot create lock dir: %w", err)
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("cannot open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("flock failed: %w", err)
	}
	// Write the current PID into the lock file for human inspection.
	fmt.Fprintf(f, "%d\n", os.Getpid())
	return &Lock{path: lockPath, file: f}, nil
}

// ErrAlreadyRunning is returned when the advisory lock is held by another process.
var ErrAlreadyRunning = errors.New("another vedox daemon is already running")

// Release releases the advisory lock and closes the file. Idempotent.
func (l *Lock) Release() {
	if l.file != nil {
		syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN) //nolint:errcheck
		l.file.Close()
		l.file = nil
	}
}

// HealthzHandler builds the /healthz HTTP handler with an atomic uptime counter.
// startTime is captured once at daemon startup. The handler reads from
// in-memory counters only — no DB, no I/O — per the spec's p99 < 5 ms budget.
func HealthzHandler(version, commit, buildDate, listenAddr string, startTime time.Time) http.HandlerFunc {
	// reposLoaded is an atomic counter that callers may increment as repos are loaded.
	return func(w http.ResponseWriter, r *http.Request) {
		uptime := int64(time.Since(startTime).Seconds())
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		// Construct the response inline rather than via json.Marshal to avoid
		// allocation on a hot path (the spec's p99 < 5 ms requirement).
		fmt.Fprintf(w,
			`{"status":"ok","version":%q,"commit":%q,"build_date":%q,"uptime_seconds":%d,"pid":%d,"listen_addr":%q}`,
			version, commit, buildDate, uptime, os.Getpid(), listenAddr,
		)
	}
}

// HealthzResponse is the decoded form of a /healthz response body.
// Callers that need to parse the response (e.g. server status) use this.
type HealthzResponse struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	Commit        string `json:"commit"`
	BuildDate     string `json:"build_date"`
	UptimeSeconds int64  `json:"uptime_seconds"`
	PID           int    `json:"pid"`
	ListenAddr    string `json:"listen_addr"`
}

// QueryHealthz hits /healthz on the given base URL (e.g. "http://127.0.0.1:5150")
// and returns the parsed response. Timeout is 3 seconds.
func QueryHealthz(baseURL string) (*HealthzResponse, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(baseURL + "/healthz")
	if err != nil {
		return nil, fmt.Errorf("daemon not reachable: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading healthz response: %w", err)
	}
	var h HealthzResponse
	if err := json.Unmarshal(b, &h); err != nil {
		return nil, fmt.Errorf("parsing healthz response: %w", err)
	}
	return &h, nil
}

// Daemonize re-execs the current binary with the given args plus "--foreground"
// appended, detaching from the current terminal. stdout and stderr of the child
// are redirected to logFile. This implements the --no-supervisor behaviour
// described in spec §1.2.
//
// The function returns in the PARENT process immediately after launching the
// child. The child will run as a background process with no controlling terminal.
// A PID file will be written by the child process once it initialises.
func Daemonize(binary string, args []string, logFile string) error {
	// Ensure the log directory exists.
	if err := os.MkdirAll(filepath.Dir(logFile), 0o700); err != nil {
		return fmt.Errorf("cannot create log dir: %w", err)
	}

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("cannot open daemon log %s: %w", logFile, err)
	}

	childArgs := append(args, "--foreground")
	cmd := exec.Command(binary, childArgs...)
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.Stdin = nil
	// SysProcAttr sets Setpgid=true so the child gets its own process group
	// and does not receive signals sent to the parent's process group.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		f.Close()
		return fmt.Errorf("cannot start daemon process: %w", err)
	}
	// Do NOT call cmd.Wait() — we want the child to outlive the parent.
	// Close our copy of the log file; the child has its own fd.
	f.Close()
	fmt.Printf("vedox daemon started (pid %d) — logging to %s\n", cmd.Process.Pid, logFile)
	return nil
}

// UptimeCounter is an atomic seconds-since-start counter for use in /healthz.
// It is updated by a background goroutine started with StartUptimeCounter.
type UptimeCounter struct {
	seconds atomic.Int64
}

// StartUptimeCounter launches a goroutine that ticks once per second until ctx
// is cancelled, updating c.seconds. The goroutine exits cleanly on ctx.Done().
func StartUptimeCounter(ctx context.Context, c *UptimeCounter) {
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.seconds.Add(1)
			}
		}
	}()
}

// Get returns the current uptime in seconds.
func (c *UptimeCounter) Get() int64 {
	return c.seconds.Load()
}

// TailLog tails the last n lines of logFile, writing them to w.
// If follow is true, it blocks until ctx is cancelled, writing new lines as
// they appear (tail -F semantics — survives log rotation).
func TailLog(ctx context.Context, logFile string, n int, follow bool, w io.Writer) error {
	// Read the entire file and print the last n lines.
	b, err := os.ReadFile(logFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("log file not found: %s (daemon may not have started yet)", logFile)
		}
		return fmt.Errorf("reading log file: %w", err)
	}

	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	start := 0
	if n > 0 && len(lines) > n {
		start = len(lines) - n
	}
	for _, line := range lines[start:] {
		fmt.Fprintln(w, line)
	}

	if !follow {
		return nil
	}

	// Follow mode: poll for new content every 250 ms.
	// Production use of tail -F semantics; we track the last known size and
	// re-open the file on shrink (rotation detection).
	lastSize := int64(len(b))
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			info, err := os.Stat(logFile)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					// Log was rotated away; wait for it to reappear.
					lastSize = 0
					continue
				}
				return fmt.Errorf("stat log file: %w", err)
			}
			current := info.Size()
			if current < lastSize {
				// File was truncated/rotated — reset and stream from beginning.
				lastSize = 0
			}
			if current > lastSize {
				f2, err := os.Open(logFile)
				if err != nil {
					continue
				}
				f2.Seek(lastSize, io.SeekStart) //nolint:errcheck
				io.Copy(w, f2)                  //nolint:errcheck
				f2.Close()
				lastSize = current
			}
		}
	}
}

// SendSignal sends sig to the process with the given PID. Returns an error if
// the process does not exist or the signal cannot be delivered.
func SendSignal(pid int, sig syscall.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("cannot find process %d: %w", pid, err)
	}
	if err := proc.Signal(sig); err != nil {
		return fmt.Errorf("signal %v to pid %d failed: %w", sig, pid, err)
	}
	return nil
}

// WaitForExit polls IsAlive every 500 ms until the process with pid exits or
// until timeout elapses. Returns true if the process exited within timeout.
func WaitForExit(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !IsAlive(pid) {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return !IsAlive(pid)
}
