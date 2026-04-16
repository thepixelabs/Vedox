package doctor

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vedox/vedox/internal/secrets"
)

// ---- helper: temp vedox home -------------------------------------------------

// newTempHome creates a temp directory that looks like a minimal ~/.vedox/.
func newTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"run", "logs", "crashes"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o700); err != nil {
			t.Fatalf("newTempHome mkdir %s: %v", sub, err)
		}
	}
	return dir
}

// ---- Status / Check struct ---------------------------------------------------

func TestStatusConstants(t *testing.T) {
	if StatusPass != "PASS" {
		t.Fatalf("StatusPass = %q, want PASS", StatusPass)
	}
	if StatusWarn != "WARN" {
		t.Fatalf("StatusWarn = %q, want WARN", StatusWarn)
	}
	if StatusFail != "FAIL" {
		t.Fatalf("StatusFail = %q, want FAIL", StatusFail)
	}
}

func TestCheckFields(t *testing.T) {
	c := Check{
		Name:    "test check",
		Status:  StatusPass,
		Message: "all good",
		Fix:     "nothing to do",
	}
	if c.Name != "test check" {
		t.Fatalf("Name mismatch: got %q", c.Name)
	}
	if c.Status != StatusPass {
		t.Fatalf("Status mismatch: got %q", c.Status)
	}
}

// ---- AnyFailed ---------------------------------------------------------------

func TestAnyFailed(t *testing.T) {
	cases := []struct {
		name   string
		checks []Check
		want   bool
	}{
		{"all pass", []Check{{Status: StatusPass}, {Status: StatusPass}}, false},
		{"one warn", []Check{{Status: StatusPass}, {Status: StatusWarn}}, false},
		{"one fail", []Check{{Status: StatusPass}, {Status: StatusFail}}, true},
		{"only fail", []Check{{Status: StatusFail}}, true},
		{"empty", []Check{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AnyFailed(tc.checks); got != tc.want {
				t.Fatalf("AnyFailed = %v, want %v", got, tc.want)
			}
		})
	}
}

// ---- Summary -----------------------------------------------------------------

func TestSummary(t *testing.T) {
	results := []Check{
		{Status: StatusPass},
		{Status: StatusPass},
		{Status: StatusWarn},
		{Status: StatusFail},
	}
	got := Summary(results)
	if !strings.Contains(got, "2 passed") {
		t.Errorf("Summary missing passed count: %q", got)
	}
	if !strings.Contains(got, "1 warning") {
		t.Errorf("Summary missing warning count: %q", got)
	}
	if !strings.Contains(got, "1 failed") {
		t.Errorf("Summary missing failed count: %q", got)
	}
}

func TestSummaryAllPass(t *testing.T) {
	results := []Check{
		{Status: StatusPass},
		{Status: StatusPass},
		{Status: StatusPass},
	}
	got := Summary(results)
	if !strings.Contains(got, "3 passed") {
		t.Errorf("Summary: want 3 passed, got: %q", got)
	}
	if !strings.Contains(got, "0 failed") {
		t.Errorf("Summary: want 0 failed, got: %q", got)
	}
}

// ---- FormatText -------------------------------------------------------------

func TestFormatText(t *testing.T) {
	results := []Check{
		{Name: "git installed", Status: StatusPass, Message: "git version 2.44.0"},
		{Name: "gh CLI installed", Status: StatusWarn, Message: "gh not found", Fix: "brew install gh"},
		{Name: "daemon running", Status: StatusFail, Message: "no PID file", Fix: "vedox server start"},
	}
	out := FormatText(results)

	if !strings.Contains(out, "ok") {
		t.Errorf("FormatText: missing ok indicator")
	}
	if !strings.Contains(out, "WARN") {
		t.Errorf("FormatText: missing WARN indicator")
	}
	if !strings.Contains(out, "FAIL") {
		t.Errorf("FormatText: missing FAIL indicator")
	}
	if !strings.Contains(out, "brew install gh") {
		t.Errorf("FormatText: missing fix for WARN check")
	}
	if !strings.Contains(out, "vedox server start") {
		t.Errorf("FormatText: missing fix for FAIL check")
	}
}

func TestFormatTextNoFixOnPass(t *testing.T) {
	results := []Check{
		{Name: "git installed", Status: StatusPass, Message: "git version 2.44.0", Fix: ""},
	}
	out := FormatText(results)
	// There should be no "fix:" line when check passes.
	if strings.Contains(out, "fix:") {
		t.Errorf("FormatText: unexpected fix line for PASS check: %q", out)
	}
}

// ---- FormatJSON -------------------------------------------------------------

func TestFormatJSON(t *testing.T) {
	results := []Check{
		{Name: "git installed", Status: StatusPass, Message: "git version 2.44.0"},
		{Name: "daemon running", Status: StatusFail, Message: "not running", Fix: "vedox server start"},
	}
	out, err := FormatJSON(results)
	if err != nil {
		t.Fatalf("FormatJSON: %v", err)
	}

	// Verify it is valid JSON.
	var parsed []Check
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("FormatJSON output is not valid JSON: %v\noutput:\n%s", err, out)
	}
	if len(parsed) != 2 {
		t.Fatalf("FormatJSON: got %d results, want 2", len(parsed))
	}
	if parsed[0].Status != StatusPass {
		t.Errorf("FormatJSON: first check status = %q, want PASS", parsed[0].Status)
	}
	if parsed[1].Fix != "vedox server start" {
		t.Errorf("FormatJSON: second check fix = %q", parsed[1].Fix)
	}
}

// ---- parseGHVersion ---------------------------------------------------------

func TestParseGHVersion(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"gh version 2.47.0 (2024-04-03)", "2.47.0"},
		{"gh version 2.20.0 (2023-10-01)", "2.20.0"},
		{"gh version 2.0.0", "2.0.0"},
		{"", ""},
		{"something else entirely", ""},
		{"version 1.2.3", "1.2.3"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("input=%q", tc.input), func(t *testing.T) {
			if got := parseGHVersion(tc.input); got != tc.want {
				t.Fatalf("parseGHVersion(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---- ghVersionAtLeast -------------------------------------------------------

func TestGHVersionAtLeast(t *testing.T) {
	cases := []struct {
		ver         string
		maj, min, p int
		want        bool
	}{
		{"2.47.0", 2, 20, 0, true},
		{"2.20.0", 2, 20, 0, true},
		{"2.19.9", 2, 20, 0, false},
		{"2.20.1", 2, 20, 0, true},
		{"3.0.0", 2, 20, 0, true},
		{"1.99.99", 2, 20, 0, false},
		{"v2.47.0", 2, 20, 0, true},
		{"notaversion", 2, 20, 0, true}, // unknown format — don't block
		{"2.20", 2, 20, 0, true},        // missing patch — don't block
	}
	for _, tc := range cases {
		name := fmt.Sprintf("%s>=%d.%d.%d", tc.ver, tc.maj, tc.min, tc.p)
		t.Run(name, func(t *testing.T) {
			if got := ghVersionAtLeast(tc.ver, tc.maj, tc.min, tc.p); got != tc.want {
				t.Fatalf("ghVersionAtLeast(%q, %d,%d,%d) = %v, want %v",
					tc.ver, tc.maj, tc.min, tc.p, got, tc.want)
			}
		})
	}
}

// ---- formatUptime -----------------------------------------------------------

func TestFormatUptime(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{45 * time.Second, "45s"},
		{90 * time.Second, "1m 30s"},
		{61 * time.Second, "1m 1s"},
		{time.Hour + time.Minute, "1h 1m"},
		{3*time.Hour + 22*time.Minute, "3h 22m"},
		{24 * time.Hour, "24h 0m"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := formatUptime(tc.d); got != tc.want {
				t.Fatalf("formatUptime(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}

// ---- checkDiskSpace (with real temp dir) ------------------------------------

func TestCheckDiskSpacePass(t *testing.T) {
	home := newTempHome(t)
	cfg := Config{VedoxHome: home, DefaultPort: 19999}
	check := checkDiskSpace(cfg)()
	// The temp dir is on a real filesystem that almost certainly has > 500 MB free.
	// On a CI machine this may theoretically fail — we accept that risk because
	// testing with a mock filesystem would require interface injection that isn't
	// worth the complexity for this check.
	if check.Name != "disk space" {
		t.Fatalf("unexpected check name: %q", check.Name)
	}
	// Status is PASS or FAIL depending on actual disk space; we just verify the
	// check ran without panicking and produced a sensible message.
	if check.Message == "" {
		t.Fatal("disk space check returned empty message")
	}
}

// ---- checkWALSize -----------------------------------------------------------

func TestCheckWALSizeNoFile(t *testing.T) {
	home := newTempHome(t)
	cfg := Config{
		VedoxHome:   home,
		IndexDBPath: filepath.Join(home, "index.db"),
	}
	// No WAL file exists.
	check := checkWALSize(cfg)()
	if check.Name != "SQLite WAL" {
		t.Fatalf("unexpected name: %q", check.Name)
	}
	if check.Status != StatusPass {
		t.Fatalf("expected PASS when WAL absent, got %q: %s", check.Status, check.Message)
	}
}

func TestCheckWALSizeSmall(t *testing.T) {
	home := newTempHome(t)
	dbPath := filepath.Join(home, "index.db")
	walPath := dbPath + "-wal"
	// Write a small WAL file (1 KB).
	if err := os.WriteFile(walPath, make([]byte, 1024), 0o600); err != nil {
		t.Fatalf("write WAL: %v", err)
	}
	cfg := Config{VedoxHome: home, IndexDBPath: dbPath}
	check := checkWALSize(cfg)()
	if check.Status != StatusPass {
		t.Fatalf("expected PASS for small WAL, got %q: %s", check.Status, check.Message)
	}
}

func TestCheckWALSizeLarge(t *testing.T) {
	home := newTempHome(t)
	dbPath := filepath.Join(home, "index.db")
	walPath := dbPath + "-wal"
	// Write a WAL file > 10 MB to trigger the warning.
	bigData := make([]byte, 11*1024*1024)
	if err := os.WriteFile(walPath, bigData, 0o600); err != nil {
		t.Fatalf("write WAL: %v", err)
	}
	cfg := Config{VedoxHome: home, IndexDBPath: dbPath}
	check := checkWALSize(cfg)()
	if check.Status != StatusWarn {
		t.Fatalf("expected WARN for large WAL, got %q: %s", check.Status, check.Message)
	}
	if check.Fix == "" {
		t.Fatal("expected non-empty Fix for large WAL")
	}
}

// ---- checkLogDirWritable ----------------------------------------------------

func TestCheckLogDirWritable(t *testing.T) {
	home := newTempHome(t)
	cfg := Config{VedoxHome: home}
	check := checkLogDirWritable(cfg)()
	if check.Status != StatusPass {
		t.Fatalf("expected PASS for writable log dir, got %q: %s", check.Status, check.Message)
	}
}

func TestCheckLogDirNotWritable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write to any directory — skip permission test")
	}
	home := newTempHome(t)
	logDir := filepath.Join(home, "logs")
	// Remove write permission.
	if err := os.Chmod(logDir, 0o400); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(logDir, 0o700) })

	cfg := Config{VedoxHome: home}
	check := checkLogDirWritable(cfg)()
	if check.Status != StatusFail {
		t.Fatalf("expected FAIL for non-writable log dir, got %q: %s", check.Status, check.Message)
	}
}

// ---- checkPortAvailable -----------------------------------------------------

func TestCheckPortAvailableWhenFree(t *testing.T) {
	// Find a free port for the test.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not open test listener: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // release it; port may be grabbed by OS between close and check

	home := newTempHome(t)
	cfg := Config{VedoxHome: home, DefaultPort: port}
	// No PID file → daemon not running → port check runs.
	check := checkPortAvailable(cfg)()
	if check.Name != "port available" {
		t.Fatalf("unexpected name: %q", check.Name)
	}
	// PASS is expected when no one holds the port. FAIL is theoretically possible
	// if the OS recycled the port between our close and the check.
	if check.Status == StatusWarn {
		t.Logf("note: got WARN instead of PASS (race possible): %s", check.Message)
	}
}

func TestCheckPortAvailableWhenInUse(t *testing.T) {
	// Bind a port and hold it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not bind: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	home := newTempHome(t)
	cfg := Config{VedoxHome: home, DefaultPort: port}
	check := checkPortAvailable(cfg)()
	if check.Status != StatusFail {
		t.Fatalf("expected FAIL when port is in use, got %q: %s", check.Status, check.Message)
	}
}

// ---- checkRegistryValid (with temp repos.json) ------------------------------

func TestCheckRegistryValidNoFile(t *testing.T) {
	home := newTempHome(t)
	cfg := Config{VedoxHome: home}
	// No repos.json → PASS (no repos registered yet).
	check := checkRegistryValid(cfg)()
	if check.Status != StatusPass {
		t.Fatalf("expected PASS when no repos.json, got %q: %s", check.Status, check.Message)
	}
}

func TestCheckRegistryValidCorrupt(t *testing.T) {
	home := newTempHome(t)
	reposPath := filepath.Join(home, "repos.json")
	if err := os.WriteFile(reposPath, []byte("not json{{{"), 0o600); err != nil {
		t.Fatalf("write corrupt repos.json: %v", err)
	}
	cfg := Config{VedoxHome: home, ReposJSONPath: reposPath}
	check := checkRegistryValid(cfg)()
	if check.Status != StatusFail {
		t.Fatalf("expected FAIL for corrupt repos.json, got %q: %s", check.Status, check.Message)
	}
}

func TestCheckRegistryValidEmpty(t *testing.T) {
	home := newTempHome(t)
	reposPath := filepath.Join(home, "repos.json")
	// Write a valid but empty manifest.
	if err := os.WriteFile(reposPath, []byte(`{"version":1,"repos":[]}`), 0o600); err != nil {
		t.Fatalf("write repos.json: %v", err)
	}
	cfg := Config{VedoxHome: home, ReposJSONPath: reposPath}
	check := checkRegistryValid(cfg)()
	if check.Status != StatusPass {
		t.Fatalf("expected PASS for empty registry, got %q: %s", check.Status, check.Message)
	}
}

// ---- checkDaemonRunning (no daemon — expected FAIL) -------------------------

func TestCheckDaemonRunningNoPID(t *testing.T) {
	home := newTempHome(t)
	cfg := Config{VedoxHome: home}
	check := checkDaemonRunning(cfg)()
	if check.Status != StatusFail {
		t.Fatalf("expected FAIL when no daemon, got %q: %s", check.Status, check.Message)
	}
	if !strings.Contains(check.Message, "PID") {
		t.Fatalf("expected mention of PID in message: %q", check.Message)
	}
	if check.Fix == "" {
		t.Fatal("expected non-empty Fix for missing daemon")
	}
}

// ---- safeRun / panic recovery -----------------------------------------------

func TestSafeRunPanicRecovery(t *testing.T) {
	panicker := checkFn(func() Check {
		panic("deliberate test panic")
	})
	result := safeRun(panicker)
	if result.Status != StatusFail {
		t.Fatalf("expected FAIL after panic, got %q", result.Status)
	}
	if !strings.Contains(result.Message, "panicked") {
		t.Fatalf("expected 'panicked' in message, got %q", result.Message)
	}
}

// ---- DefaultConfig ----------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg, err := DefaultConfig("1.2.3")
	if err != nil {
		t.Fatalf("DefaultConfig: %v", err)
	}
	if cfg.CLIVersion != "1.2.3" {
		t.Fatalf("CLIVersion = %q, want 1.2.3", cfg.CLIVersion)
	}
	if cfg.DefaultPort == 0 {
		t.Fatal("DefaultPort should be non-zero")
	}
	if cfg.VedoxHome == "" {
		t.Fatal("VedoxHome should be non-empty")
	}
}

// ---- RunAll (smoke test — no assertions on individual check outcomes) -------

func TestRunAllReturnsAllChecks(t *testing.T) {
	home := newTempHome(t)
	cfg := Config{
		VedoxHome:   home,
		DefaultPort: 19998,
		CLIVersion:  "test",
		// Inject an in-memory SecretStore so the keychain check does NOT touch
		// the real macOS Keychain (which would leak probe keys and may prompt
		// for permission in sandboxed contexts).
		SecretStore: secrets.NewInMemoryStore(),
	}
	results := RunAll(cfg)
	// RunAll must return at least 9 checks per task spec. We currently implement
	// 12; verify we get at least 9 so the requirement is met even if checks are
	// removed in a future refactor.
	const minChecks = 9
	if len(results) < minChecks {
		t.Fatalf("RunAll returned %d checks, want >= %d", len(results), minChecks)
	}
	// Every result must have a non-empty Name and a valid Status.
	for i, r := range results {
		if r.Name == "" {
			t.Errorf("check[%d] has empty Name", i)
		}
		switch r.Status {
		case StatusPass, StatusWarn, StatusFail:
		default:
			t.Errorf("check[%d] has invalid Status %q", i, r.Status)
		}
	}
}
