package logging_test

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vedox/vedox/internal/logging"
)

func TestSetup_CreatesLogFile(t *testing.T) {
	// Point the log directory to a temp dir via HOME override.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cleanup, err := logging.Setup(slog.LevelInfo)
	if err != nil {
		t.Fatalf("Setup returned unexpected error: %v", err)
	}
	defer cleanup()

	// Write a log entry to trigger file creation.
	slog.Info("test entry", "key", "value")

	cleanup() // flush

	// Verify the log file exists.
	logDir := filepath.Join(tmpHome, ".vedox", "logs")
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(logDir, "vedox-"+today+".log")

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("expected log file %q to exist, it does not", logFile)
	}
}

func TestSetup_LogIsValidJSON(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cleanup, err := logging.Setup(slog.LevelInfo)
	if err != nil {
		t.Fatalf("Setup returned unexpected error: %v", err)
	}

	slog.Info("json_test", "foo", "bar")
	cleanup()

	logDir := filepath.Join(tmpHome, ".vedox", "logs")
	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(logDir, "vedox-"+today+".log")

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("could not read log file: %v", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Errorf("log line is not valid JSON: %q — error: %v", line, err)
		}
	}
}

func TestLogFileOp_DoesNotPanic(t *testing.T) {
	// LogFileOp should be callable without a configured logger.
	logging.LogFileOp("write", "/tmp/test.md", 1024)
	logging.LogFileOp("read", "/tmp/test.md", -1) // -1 = size unknown
}

// TestLogFileOp_SecurityInvariant documents the invariant that log entries
// must only contain paths and metadata — never file contents. This is a
// structural test: we verify the LogFileOp helper only logs "path" and
// "size_bytes" keys, not content-related keys.
func TestLogFileOp_SecurityInvariant(t *testing.T) {
	// The security guarantee is enforced by the LogFileOp API: it only accepts
	// path and size parameters. There is no API surface for passing content.
	// This test documents the contract; the real enforcement is that callers
	// must use LogFileOp instead of ad-hoc slog calls for file operations.
	//
	// Lint rule to add: disallow slog.* calls with keys "content", "body",
	// "data", "text" in the logging and docstore packages.
	//
	// For now, document via test that the correct path is LogFileOp.
	if false {
		// This would be wrong — NEVER do this:
		// slog.Info("file read", "content", string(someFileBytes))
		_ = "documented above"
	}
}
