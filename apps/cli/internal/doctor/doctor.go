// Package doctor implements the vedox doctor diagnostic check suite.
//
// Design principles:
//   - Every check is independent: a panic or hard error in one check must not
//     prevent the remaining checks from running (each check is wrapped in a
//     recover shim in RunAll).
//   - Checks are read-only by default. No mutations are made unless the caller
//     explicitly invokes a fix function returned in Check.Fix.
//   - The suite runs without a daemon being alive. Checks that require the
//     daemon degrade gracefully to FAIL with an actionable fix hint.
//   - No outbound network calls. All checks are local: shell exec, filesystem,
//     localhost TCP, OS keychain.
//
// Status values:
//
//	StatusPass — check passed; no action needed
//	StatusWarn — check passed with caveats; no action required but worth noting
//	StatusFail — check failed; Fix contains a suggested remediation command
package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/vedox/vedox/internal/daemon"
	"github.com/vedox/vedox/internal/gitcheck"
	"github.com/vedox/vedox/internal/portcheck"
	"github.com/vedox/vedox/internal/registry"
	"github.com/vedox/vedox/internal/secrets"
)

// Status is the outcome of a single diagnostic check.
type Status string

const (
	// StatusPass means the check passed with no issues.
	StatusPass Status = "PASS"
	// StatusWarn means the check found a non-critical issue worth noting.
	StatusWarn Status = "WARN"
	// StatusFail means the check found a hard failure requiring action.
	StatusFail Status = "FAIL"
)

// Check is the result of a single diagnostic check.
type Check struct {
	// Name is a short human-readable label for the check (e.g. "git installed").
	Name string `json:"name"`

	// Status is PASS, WARN, or FAIL.
	Status Status `json:"status"`

	// Message is a one-line human-readable result description.
	Message string `json:"message"`

	// Fix is a suggested command or action the user should take on WARN/FAIL.
	// Empty when no mechanical fix is known.
	Fix string `json:"fix,omitempty"`
}

// Config holds the inputs RunAll needs. All fields have safe defaults via
// DefaultConfig so callers in tests can override only the fields they care
// about.
type Config struct {
	// VedoxHome is the ~/.vedox directory. If empty, DefaultVedoxHome() is used.
	VedoxHome string

	// DefaultPort is the daemon port to probe when the daemon is not running.
	// Defaults to portcheck.DefaultPort (5150).
	DefaultPort int

	// CLIVersion is the version string of the running CLI binary. Used to
	// compare against the daemon's reported version.
	CLIVersion string

	// ReposJSONPath overrides the path to repos.json. Empty means derive from
	// VedoxHome.
	ReposJSONPath string

	// IndexDBPath overrides the path to index.db. Empty means derive from
	// VedoxHome.
	IndexDBPath string

	// SecretStore is an optional pre-constructed SecretStore used by the
	// keychain check. When nil, the check calls secrets.AutoDetect() and
	// inspects the concrete type. Tests inject secrets.NewInMemoryStore()
	// here to avoid touching the real OS keychain.
	SecretStore secrets.SecretStore
}

// DefaultConfig returns a Config with all fields populated from the environment.
// Returns an error only if the user home directory cannot be determined.
func DefaultConfig(cliVersion string) (Config, error) {
	home, err := daemon.DefaultVedoxHome()
	if err != nil {
		return Config{}, err
	}
	return Config{
		VedoxHome:   home,
		DefaultPort: portcheck.DefaultPort,
		CLIVersion:  cliVersion,
	}, nil
}

func (c *Config) reposJSONPath() string {
	if c.ReposJSONPath != "" {
		return c.ReposJSONPath
	}
	return filepath.Join(c.VedoxHome, "repos.json")
}

func (c *Config) indexDBPath() string {
	if c.IndexDBPath != "" {
		return c.IndexDBPath
	}
	return filepath.Join(c.VedoxHome, "index.db")
}

// RunAll runs every diagnostic check and returns the results in a stable order.
// Each check is guarded with a recover so a panic in one check does not prevent
// the remaining checks from running.
func RunAll(cfg Config) []Check {
	checks := []checkFn{
		checkGitInstalled,
		checkGitIdentity,
		checkGHInstalled,
		checkGHAuthenticated,
		checkDaemonRunning(cfg),
		checkPortAvailable(cfg),
		checkRegistryValid(cfg),
		checkDiskSpace(cfg),
		func() Check { return checkKeychainAccessible(cfg) },
		checkLogDirWritable(cfg),
		checkWALSize(cfg),
		checkInotifyLimit,
	}

	results := make([]Check, 0, len(checks))
	for _, fn := range checks {
		results = append(results, safeRun(fn))
	}
	return results
}

// checkFn is a function that runs a single diagnostic check.
type checkFn func() Check

// safeRun calls fn and recovers from any panic, converting it to a FAIL check.
func safeRun(fn checkFn) (c Check) {
	defer func() {
		if r := recover(); r != nil {
			c = Check{
				Name:    "unknown",
				Status:  StatusFail,
				Message: fmt.Sprintf("check panicked: %v", r),
			}
		}
	}()
	return fn()
}

// ---- Individual checks -------------------------------------------------------

// checkGitInstalled shells out to `git --version` and reports the version or a
// hard failure if git is not installed.
func checkGitInstalled() Check {
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return Check{
			Name:    "git installed",
			Status:  StatusFail,
			Message: "git is not installed or not on PATH",
			Fix:     "install git from https://git-scm.com",
		}
	}
	ver := strings.TrimSpace(string(out))
	return Check{
		Name:    "git installed",
		Status:  StatusPass,
		Message: ver,
	}
}

// checkGitIdentity reads git config user.name and user.email and warns if
// either is unset.
func checkGitIdentity() Check {
	ident, err := gitcheck.Check()
	if err != nil {
		return Check{
			Name:    "git identity",
			Status:  StatusFail,
			Message: "git user.name or user.email is not set",
			Fix:     `run: git config --global user.name "Your Name" && git config --global user.email "you@example.com"`,
		}
	}
	return Check{
		Name:    "git identity",
		Status:  StatusPass,
		Message: fmt.Sprintf("%s (%s)", ident.Name, ident.Email),
	}
}

// checkGHInstalled probes for the GitHub CLI. The gh CLI is optional (WARN, not
// FAIL) but must be >= 2.20.0 for full feature support.
func checkGHInstalled() Check {
	out, err := exec.Command("gh", "--version").Output() // #nosec G204 — fixed command
	if err != nil {
		return Check{
			Name:    "gh CLI installed",
			Status:  StatusWarn,
			Message: "gh CLI not found — repo creation and PR features will be unavailable",
			Fix:     "install: brew install gh   or   sudo apt install gh",
		}
	}
	// gh version output: "gh version 2.47.0 (2024-04-03)"
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	ver := parseGHVersion(line)
	if ver != "" && !ghVersionAtLeast(ver, 2, 20, 0) {
		return Check{
			Name:    "gh CLI installed",
			Status:  StatusWarn,
			Message: fmt.Sprintf("gh %s installed but >= 2.20.0 is required for full support", ver),
			Fix:     "upgrade: brew upgrade gh",
		}
	}
	return Check{
		Name:    "gh CLI installed",
		Status:  StatusPass,
		Message: line,
	}
}

// checkGHAuthenticated calls `gh auth status` to determine if the user is
// logged in. Only runs when gh is installed.
func checkGHAuthenticated() Check {
	// Check if gh is present first.
	if _, err := exec.LookPath("gh"); err != nil {
		// gh not installed — covered by checkGHInstalled; skip auth check.
		return Check{
			Name:    "gh authenticated",
			Status:  StatusWarn,
			Message: "gh CLI not installed — skipping auth check",
		}
	}
	out, err := exec.Command("gh", "auth", "status").CombinedOutput() // #nosec G204
	if err != nil {
		return Check{
			Name:    "gh authenticated",
			Status:  StatusWarn,
			Message: "gh CLI is not authenticated with GitHub",
			Fix:     "run: gh auth login --web",
		}
	}
	// Extract "Logged in to github.com account <user>" from output.
	summary := strings.TrimSpace(string(out))
	if idx := strings.Index(summary, "\n"); idx != -1 {
		summary = strings.TrimSpace(summary[:idx])
	}
	return Check{
		Name:    "gh authenticated",
		Status:  StatusPass,
		Message: summary,
	}
}

// checkDaemonRunning reads the PID file, verifies the process is alive, and
// optionally hits /healthz to confirm version parity.
func checkDaemonRunning(cfg Config) checkFn {
	return func() Check {
		paths := daemon.NewPaths(cfg.VedoxHome)
		rec, err := daemon.ReadPIDFile(paths.PIDFile)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return Check{
					Name:    "daemon running",
					Status:  StatusFail,
					Message: "daemon is not running (no PID file found)",
					Fix:     "run: vedox server start",
				}
			}
			return Check{
				Name:    "daemon running",
				Status:  StatusFail,
				Message: fmt.Sprintf("could not read PID file: %v", err),
				Fix:     "run: vedox server start",
			}
		}

		if !daemon.IsAlive(rec.PID) {
			return Check{
				Name:    "daemon running",
				Status:  StatusFail,
				Message: fmt.Sprintf("PID %d found in PID file but process is not alive (stale PID file)", rec.PID),
				Fix:     "run: vedox server start",
			}
		}

		// Process is alive — try /healthz for version check.
		baseURL := fmt.Sprintf("http://127.0.0.1:%d", rec.Port)
		hz, err := daemon.QueryHealthz(baseURL)
		if err != nil {
			return Check{
				Name:    "daemon running",
				Status:  StatusWarn,
				Message: fmt.Sprintf("PID %d is alive but /healthz unreachable: %v", rec.PID, err),
				Fix:     "run: vedox server restart",
			}
		}

		uptime := time.Duration(hz.UptimeSeconds) * time.Second
		msg := fmt.Sprintf("running (pid %d, uptime %s, port %d, version %s)", rec.PID, formatUptime(uptime), rec.Port, hz.Version)

		// Version parity check.
		if cfg.CLIVersion != "" && hz.Version != "" && hz.Version != cfg.CLIVersion {
			return Check{
				Name:    "daemon running",
				Status:  StatusWarn,
				Message: msg + fmt.Sprintf(" — version mismatch: CLI is %s, daemon is %s", cfg.CLIVersion, hz.Version),
				Fix:     "run: vedox server restart",
			}
		}

		return Check{
			Name:    "daemon running",
			Status:  StatusPass,
			Message: msg,
		}
	}
}

// checkPortAvailable checks whether the default daemon port is free. Only
// reports when the daemon is NOT running — if the daemon is up it holds the
// port and a "port in use" result would be a false alarm.
func checkPortAvailable(cfg Config) checkFn {
	return func() Check {
		paths := daemon.NewPaths(cfg.VedoxHome)
		_, pidErr := daemon.ReadPIDFile(paths.PIDFile)
		if pidErr == nil {
			// Daemon appears to be running; skip this check.
			return Check{
				Name:    "port available",
				Status:  StatusPass,
				Message: fmt.Sprintf("port %d held by running daemon — skipping availability check", cfg.DefaultPort),
			}
		}

		port := cfg.DefaultPort
		if port == 0 {
			port = portcheck.DefaultPort
		}

		if err := portcheck.CheckPort(port); err != nil {
			return Check{
				Name:    "port available",
				Status:  StatusFail,
				Message: fmt.Sprintf("port %d is already in use by another process", port),
				Fix:     fmt.Sprintf("identify the process with: lsof -i :%d   then set an alternate port in ~/.vedox/config.toml", port),
			}
		}

		return Check{
			Name:    "port available",
			Status:  StatusPass,
			Message: fmt.Sprintf("port %d is available", port),
		}
	}
}

// checkRegistryValid opens repos.json and checks that every registered repo's
// RootPath exists on disk. Orphan repos are surfaced as WARNs.
func checkRegistryValid(cfg Config) checkFn {
	return func() Check {
		reposPath := cfg.reposJSONPath()

		if _, err := os.Stat(reposPath); errors.Is(err, os.ErrNotExist) {
			// No repos.json yet — user hasn't registered any repos. Not a failure.
			return Check{
				Name:    "registry valid",
				Status:  StatusPass,
				Message: "no repos.json found — no repos registered yet",
			}
		}

		reg, err := registry.NewFileRegistry(reposPath, nil)
		if err != nil {
			return Check{
				Name:    "registry valid",
				Status:  StatusFail,
				Message: fmt.Sprintf("could not open repos.json: %v", err),
				Fix:     "check that ~/.vedox/repos.json is valid JSON",
			}
		}

		repos, err := reg.List()
		if err != nil {
			return Check{
				Name:    "registry valid",
				Status:  StatusFail,
				Message: fmt.Sprintf("could not list repos: %v", err),
			}
		}

		if len(repos) == 0 {
			return Check{
				Name:    "registry valid",
				Status:  StatusPass,
				Message: "registry is empty — no repos registered",
			}
		}

		var orphans []string
		for _, r := range repos {
			if _, err := os.Stat(r.RootPath); errors.Is(err, os.ErrNotExist) {
				orphans = append(orphans, r.Name)
			}
		}

		if len(orphans) > 0 {
			return Check{
				Name:    "registry valid",
				Status:  StatusWarn,
				Message: fmt.Sprintf("%d of %d repos are orphaned (path missing): %s", len(orphans), len(repos), strings.Join(orphans, ", ")),
				Fix:     "run: git clone <repo-url> <expected-path>   or remove the orphaned entry with: vedox repos remove <name>",
			}
		}

		return Check{
			Name:    "registry valid",
			Status:  StatusPass,
			Message: fmt.Sprintf("%d repo(s) reachable on disk", len(repos)),
		}
	}
}

// checkDiskSpace checks that the partition containing ~/.vedox/ has at least
// 500 MB free (the spec threshold).
func checkDiskSpace(cfg Config) checkFn {
	return func() Check {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(cfg.VedoxHome, &stat); err != nil {
			return Check{
				Name:    "disk space",
				Status:  StatusWarn,
				Message: fmt.Sprintf("could not stat filesystem at %s: %v", cfg.VedoxHome, err),
			}
		}
		// Available blocks × block size = free bytes accessible to non-root.
		freeBytes := stat.Bavail * uint64(stat.Bsize) //nolint:unconvert
		freeMB := freeBytes / (1024 * 1024)
		freeGB := float64(freeBytes) / (1024 * 1024 * 1024)

		const thresholdMB = 500
		if freeMB < thresholdMB {
			return Check{
				Name:    "disk space",
				Status:  StatusFail,
				Message: fmt.Sprintf("only %.1f GB available on the ~/.vedox partition — 500 MB minimum required", freeGB),
				Fix:     "free disk space on the partition",
			}
		}
		return Check{
			Name:    "disk space",
			Status:  StatusPass,
			Message: fmt.Sprintf("%.1f GB available", freeGB),
		}
	}
}

// checkKeychainAccessible reports whether the OS keychain is available AND
// usable as a backend for HMAC secrets.
//
// Three outcomes:
//  1. cfg.SecretStore is provided (tests) — assume caller knows what they're
//     doing; report PASS without probing the real keychain.
//  2. AutoDetect returns a non-keychain backend (AgeStore / EnvStore /
//     InMemoryStore) — the keychain is unreachable on this platform or the
//     operator has opted into a file/env-based tier. WARN, do NOT probe write.
//  3. AutoDetect returns a KeyringStore — perform a short write/read/delete
//     probe with a unique PID + nanosecond-timestamped key. If Put is denied
//     (sandboxed / MDM context) report WARN and do NOT leak any key.
func checkKeychainAccessible(cfg Config) Check {
	// Test-injected stub: skip real keychain probing entirely.
	if cfg.SecretStore != nil {
		if _, isKeyring := cfg.SecretStore.(*secrets.KeyringStore); isKeyring {
			return doKeychainProbe(cfg.SecretStore)
		}
		return Check{
			Name:    "keychain accessible",
			Status:  StatusPass,
			Message: "using injected secret store (test stub) — keychain probe skipped",
		}
	}

	store, err := secrets.AutoDetect()
	if err != nil {
		return Check{
			Name:    "keychain accessible",
			Status:  StatusWarn,
			Message: fmt.Sprintf("no secret storage backend available: %v", err),
			Fix:     "set VEDOX_AGE_PASSPHRASE_FILE, VEDOX_HMAC_KEY_FILE, or enable the OS keychain",
		}
	}

	ks, isKeyring := store.(*secrets.KeyringStore)
	if !isKeyring {
		return Check{
			Name:    "keychain accessible",
			Status:  StatusWarn,
			Message: "OS keychain unreachable — falling back to age-encrypted file or env-var secrets",
			Fix:     "on macOS: unlock the login keychain; on Linux: start gnome-keyring or kwallet; on headless: this is expected",
		}
	}

	return doKeychainProbe(ks)
}

// doKeychainProbe performs a short, cleanup-safe probe against the supplied
// keyring-backed store. The probe key includes the PID and nanosecond
// timestamp so any leak is identifiable and not a forever-stable
// "vedox:doctor-probe" entry.
func doKeychainProbe(store secrets.SecretStore) Check {
	probeKey := fmt.Sprintf("vedox:doctor-probe-%d-%d", os.Getpid(), time.Now().UnixNano())
	probeVal := []byte(strconv.FormatInt(time.Now().UnixNano(), 16))

	if err := store.Put(probeKey, probeVal); err != nil {
		// Put failed → nothing was written, no cleanup needed.
		return Check{
			Name:    "keychain accessible",
			Status:  StatusWarn,
			Message: fmt.Sprintf("keychain permission denied in this context: %v — HMAC secrets may fall back to age-encrypted file or env var", err),
			Fix:     "grant keychain access to the vedox binary (macOS: Keychain Access); headless Linux: use VEDOX_AGE_PASSPHRASE_FILE",
		}
	}

	// From here on, always attempt Delete before returning so we never leak.
	got, readErr := store.Get(probeKey)
	if delErr := store.Delete(probeKey); delErr != nil && !secrets.IsNotFound(delErr) {
		return Check{
			Name:    "keychain accessible",
			Status:  StatusWarn,
			Message: fmt.Sprintf("keychain probe key could not be deleted: %v — orphan entry %q may remain", delErr, probeKey),
			Fix:     "manually remove the probe entry from Keychain Access / Secret Service",
		}
	}

	if readErr != nil {
		return Check{
			Name:    "keychain accessible",
			Status:  StatusWarn,
			Message: fmt.Sprintf("OS keychain read-back failed after successful write: %v", readErr),
		}
	}
	if string(got) != string(probeVal) {
		return Check{
			Name:    "keychain accessible",
			Status:  StatusWarn,
			Message: "keychain round-trip value mismatch — keychain may be corrupted",
		}
	}

	return Check{
		Name:    "keychain accessible",
		Status:  StatusPass,
		Message: "OS keychain read/write/delete succeeded",
	}
}

// checkLogDirWritable verifies that the daemon log directory exists and is
// writable by the current user.
func checkLogDirWritable(cfg Config) checkFn {
	return func() Check {
		paths := daemon.NewPaths(cfg.VedoxHome)
		logDir := paths.LogDir

		if err := os.MkdirAll(logDir, 0o700); err != nil {
			return Check{
				Name:    "log directory writable",
				Status:  StatusFail,
				Message: fmt.Sprintf("cannot create log directory %s: %v", logDir, err),
				Fix:     fmt.Sprintf("check permissions on %s", filepath.Dir(logDir)),
			}
		}

		// Attempt to create and immediately remove a probe file.
		probe := filepath.Join(logDir, ".vedox-doctor-probe")
		f, err := os.Create(probe)
		if err != nil {
			return Check{
				Name:    "log directory writable",
				Status:  StatusFail,
				Message: fmt.Sprintf("log directory %s is not writable: %v", logDir, err),
				Fix:     fmt.Sprintf("run: chmod 700 %s", logDir),
			}
		}
		f.Close()
		os.Remove(probe)

		return Check{
			Name:    "log directory writable",
			Status:  StatusPass,
			Message: fmt.Sprintf("%s is writable", logDir),
		}
	}
}

// checkWALSize checks whether the SQLite WAL file exists and is under 10 MB.
// A large WAL file indicates the checkpoint goroutine is not keeping up or the
// daemon crashed mid-write. This is a WARN (not FAIL) because reads still work.
func checkWALSize(cfg Config) checkFn {
	return func() Check {
		walPath := cfg.indexDBPath() + "-wal"
		info, err := os.Stat(walPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return Check{
					Name:    "SQLite WAL",
					Status:  StatusPass,
					Message: "no WAL file present (checkpoint is current)",
				}
			}
			return Check{
				Name:    "SQLite WAL",
				Status:  StatusWarn,
				Message: fmt.Sprintf("could not stat WAL file: %v", err),
			}
		}

		const maxWALBytes = 10 * 1024 * 1024 // 10 MB
		sizeMB := float64(info.Size()) / (1024 * 1024)
		if info.Size() > maxWALBytes {
			return Check{
				Name:    "SQLite WAL",
				Status:  StatusWarn,
				Message: fmt.Sprintf("WAL file is %.1f MB — checkpoint may be stalled", sizeMB),
				Fix:     "run: vedox server restart   (daemon checkpoints WAL on clean shutdown)",
			}
		}

		return Check{
			Name:    "SQLite WAL",
			Status:  StatusPass,
			Message: fmt.Sprintf("WAL file present and within limits (%.1f MB)", sizeMB),
		}
	}
}

// checkInotifyLimit is Linux-only: reads /proc/sys/fs/inotify/max_user_watches
// and warns if it is below the recommended threshold. On non-Linux platforms
// this check always passes.
func checkInotifyLimit() Check {
	if runtime.GOOS != "linux" {
		return Check{
			Name:    "inotify limit",
			Status:  StatusPass,
			Message: fmt.Sprintf("inotify check not applicable on %s", runtime.GOOS),
		}
	}

	const procPath = "/proc/sys/fs/inotify/max_user_watches"
	const recommended = 65536

	b, err := os.ReadFile(procPath)
	if err != nil {
		return Check{
			Name:    "inotify limit",
			Status:  StatusWarn,
			Message: fmt.Sprintf("could not read %s: %v", procPath, err),
		}
	}

	val, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return Check{
			Name:    "inotify limit",
			Status:  StatusWarn,
			Message: fmt.Sprintf("could not parse inotify limit %q: %v", strings.TrimSpace(string(b)), err),
		}
	}

	if val < recommended {
		return Check{
			Name:    "inotify limit",
			Status:  StatusWarn,
			Message: fmt.Sprintf("inotify max_user_watches is %d — recommended: %d", val, recommended),
			Fix:     fmt.Sprintf("run: echo fs.inotify.max_user_watches=%d | sudo tee -a /etc/sysctl.conf && sudo sysctl -p", 524288),
		}
	}

	return Check{
		Name:    "inotify limit",
		Status:  StatusPass,
		Message: fmt.Sprintf("inotify max_user_watches is %d (>= %d)", val, recommended),
	}
}

// ---- Output helpers ----------------------------------------------------------

// AnyFailed returns true if any check in results has Status == StatusFail.
func AnyFailed(results []Check) bool {
	for _, c := range results {
		if c.Status == StatusFail {
			return true
		}
	}
	return false
}

// FormatText formats results as human-readable text matching the spec output
// format. Each line is: "  <indicator>   <name>: <message>".
func FormatText(results []Check) string {
	var sb strings.Builder
	for _, c := range results {
		indicator := indicatorFor(c.Status)
		sb.WriteString(fmt.Sprintf("  %-4s  %s: %s\n", indicator, c.Name, c.Message))
		if c.Fix != "" && c.Status != StatusPass {
			sb.WriteString(fmt.Sprintf("        fix: %s\n", c.Fix))
		}
	}
	return sb.String()
}

// FormatJSON formats results as a JSON array.
func FormatJSON(results []Check) (string, error) {
	b, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Summary returns a one-line summary string (e.g. "9 passed, 1 warning, 0 failed").
func Summary(results []Check) string {
	var pass, warn, fail int
	for _, c := range results {
		switch c.Status {
		case StatusPass:
			pass++
		case StatusWarn:
			warn++
		case StatusFail:
			fail++
		}
	}
	return fmt.Sprintf("%d passed, %d warning(s), %d failed", pass, warn, fail)
}

func indicatorFor(s Status) string {
	switch s {
	case StatusPass:
		return "ok"
	case StatusWarn:
		return "WARN"
	case StatusFail:
		return "FAIL"
	default:
		return "????"
	}
}

// ---- Internal utilities ------------------------------------------------------

// parseGHVersion extracts the semver string from a line like
// "gh version 2.47.0 (2024-04-03)".
func parseGHVersion(line string) string {
	// Expected format: "gh version X.Y.Z (...)"
	parts := strings.Fields(line)
	for i, p := range parts {
		if p == "version" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// ghVersionAtLeast returns true if the semver string ver is >= major.minor.patch.
// Returns true on parse failure so we don't block on an unexpected format.
func ghVersionAtLeast(ver string, major, minor, patch int) bool {
	// Strip any leading "v".
	ver = strings.TrimPrefix(ver, "v")
	parts := strings.SplitN(ver, ".", 3)
	if len(parts) < 3 {
		return true // unknown format — don't block
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	pat, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return true
	}
	if maj != major {
		return maj > major
	}
	if min != minor {
		return min > minor
	}
	return pat >= patch
}

// formatUptime returns a human-readable uptime string (e.g. "3h 22m").
func formatUptime(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm %ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

// isPortInUse is a pure-Go TCP probe that does not use the portcheck package
// so tests can call it without side effects from VedoxError wrapping.
func isPortInUse(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return true
	}
	ln.Close()
	return false
}
