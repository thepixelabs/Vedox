// Package logging sets up the Vedox structured logger.
//
// We use the Go stdlib slog package (introduced in Go 1.21) with a JSON
// handler writing to a daily-rotated log file at ~/.vedox/logs/vedox-YYYY-MM-DD.log.
//
// SECURITY INVARIANT: File *contents* are NEVER written to logs. Only paths,
// file sizes, and operation names may appear in log records. This is enforced
// by convention and tested in the test suite. See LogFileOp for the canonical
// safe logging helper.
//
// Rotation: a new log file is opened on the first log call each calendar day.
// Files older than 7 days are pruned on startup. We do not rely on an external
// log rotation daemon (logrotate, etc.) — the binary manages its own retention.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	logDirName    = ".vedox"
	logSubDir     = "logs"
	logFilePrefix = "vedox-"
	logFileSuffix = ".log"
	retentionDays = 7
)

// rotatingWriter is an io.Writer that opens a new log file each calendar day
// and prunes files older than retentionDays.
type rotatingWriter struct {
	dir         string
	currentDate string
	file        *os.File
}

// Write implements io.Writer. If the calendar date has changed since the last
// write, it closes the current file, opens a new one, and prunes old files.
func (w *rotatingWriter) Write(p []byte) (n int, err error) {
	today := time.Now().Format("2006-01-02")
	if w.currentDate != today || w.file == nil {
		if err := w.rotate(today); err != nil {
			// Fallback: write to stderr so structured logs are not silently lost.
			return os.Stderr.Write(p)
		}
	}
	return w.file.Write(p)
}

// rotate opens the log file for today and prunes files beyond the retention window.
func (w *rotatingWriter) rotate(today string) error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}

	path := filepath.Join(w.dir, logFilePrefix+today+logFileSuffix)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("logging: could not open log file %s: %w", path, err)
	}

	w.file = f
	w.currentDate = today
	w.pruneOldLogs()
	return nil
}

// pruneOldLogs removes log files older than retentionDays. Errors are silently
// ignored — a failed prune is not worth crashing the process over.
func (w *rotatingWriter) pruneOldLogs() {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	// Collect all log file names that match our naming convention.
	var logFiles []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, logFilePrefix) && strings.HasSuffix(name, logFileSuffix) {
			logFiles = append(logFiles, name)
		}
	}
	sort.Strings(logFiles)

	for _, name := range logFiles {
		// Extract the date portion: "vedox-2006-01-02.log" → "2006-01-02"
		datePart := strings.TrimPrefix(name, logFilePrefix)
		datePart = strings.TrimSuffix(datePart, logFileSuffix)

		t, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			continue // unknown file format, leave it alone
		}
		if t.Before(cutoff) {
			_ = os.Remove(filepath.Join(w.dir, name))
		}
	}
}

// Setup initialises the global slog logger and returns a cleanup function.
//
// logLevel should be slog.LevelInfo normally, or slog.LevelDebug when
// --debug is active. The returned cleanup func flushes and closes the log file.
//
// If the log directory cannot be created or the log file cannot be opened,
// Setup falls back to stderr and returns a no-op cleanup so the caller never
// needs to handle a fatal Setup error.
func Setup(logLevel slog.Level) (cleanup func(), err error) {
	logDir, err := resolveLogDir()
	if err != nil {
		// Non-fatal: fall back to stderr.
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: logLevel,
		})))
		return func() {}, fmt.Errorf("logging: could not resolve log directory, falling back to stderr: %w", err)
	}

	if err := os.MkdirAll(logDir, 0o700); err != nil {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: logLevel,
		})))
		return func() {}, fmt.Errorf("logging: could not create log directory %s, falling back to stderr: %w", logDir, err)
	}

	rw := &rotatingWriter{dir: logDir}

	// Write structured JSON to both the log file and stderr (stderr only in
	// debug mode to avoid noise in normal operation).
	var writer io.Writer = rw
	if logLevel == slog.LevelDebug {
		writer = io.MultiWriter(rw, os.Stderr)
	}

	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: logLevel,
		// AddSource adds the caller's file+line to every record. Useful in
		// debug mode; omitted at INFO to keep logs compact.
		AddSource: logLevel == slog.LevelDebug,
	})

	slog.SetDefault(slog.New(handler))

	cleanup = func() {
		if rw.file != nil {
			_ = rw.file.Close()
		}
	}
	return cleanup, nil
}

// resolveLogDir returns the absolute path to the Vedox log directory,
// typically ~/.vedox/logs.
func resolveLogDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, logDirName, logSubDir), nil
}

// LogFileOp is the canonical helper for logging file operations without
// recording file contents. Callers MUST use this instead of logging file
// data directly.
//
// op should be a short operation name like "read", "write", "delete".
// path should be the absolute path to the file.
// size is the file size in bytes; pass -1 if not available.
func LogFileOp(op, path string, size int64) {
	if size >= 0 {
		slog.Debug("file_op", "op", op, "path", path, "size_bytes", size)
	} else {
		slog.Debug("file_op", "op", op, "path", path)
	}
}
