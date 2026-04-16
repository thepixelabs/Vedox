package providers

// atomic.go — shared file-write primitives used by all provider adapters.
//
// This mirrors the pattern in api/providers.go but lives here so the
// providers package has no import dependency on the api package.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// atomicFileWrite writes data to absPath atomically via temp file + fsync +
// rename. It first verifies that absPath is inside boundary with no symlinks
// on the path (same defence as api/providers.go's atomicWrite).
//
// dirMode is applied to any directories created by MkdirAll. fileMode is
// applied to the temp file before rename.
func atomicFileWrite(boundary, absPath string, data []byte, dirMode, fileMode os.FileMode) error {
	if err := assertNoSymlink(boundary, absPath); err != nil {
		return err
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	// Re-check after MkdirAll — a racing actor could introduce a symlink.
	if err := assertNoSymlink(boundary, absPath); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".vedox-provider-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("fsync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, fileMode); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpPath, absPath); err != nil {
		cleanup()
		return fmt.Errorf("rename temp → target: %w", err)
	}
	return nil
}

// assertNoSymlink walks every path component from boundary down to target and
// returns an error if any component — or the target itself — is a symlink.
// Both inputs must be absolute paths.
func assertNoSymlink(boundary, target string) error {
	boundary = filepath.Clean(boundary)
	target = filepath.Clean(target)

	rel, err := filepath.Rel(boundary, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		slog.Warn("providers: target escapes boundary",
			"boundary", boundary, "target", target)
		return fmt.Errorf("target path escapes boundary")
	}

	// Check boundary itself.
	if err := lstatRejectSymlink(boundary); err != nil {
		return err
	}
	if rel == "." {
		return nil
	}

	parts := strings.Split(rel, string(os.PathSeparator))
	cur := boundary
	for _, part := range parts {
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil // component will be created — safe
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			slog.Warn("providers: symlink in path rejected", "component", cur)
			return fmt.Errorf("symlink ancestor rejected at %s", cur)
		}
	}
	return nil
}

func lstatRejectSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlink boundary rejected at %s", path)
	}
	return nil
}

// sha256Hex returns the hex-encoded SHA-256 hash of data.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// executeFileOps runs a slice of FileOp values in order. All create/update
// ops use atomicFileWrite; delete ops call os.Remove after a symlink check.
// Returns an error on the first failure; no rollback is attempted — callers
// should treat a partial execution as requiring Repair.
func executeFileOps(ops []FileOp) (map[string]string, error) {
	hashes := make(map[string]string, len(ops))
	for _, op := range ops {
		switch op.Action {
		case OpCreate, OpUpdate:
			if err := atomicFileWrite(op.Boundary, op.Path, op.Content, 0o755, 0o644); err != nil {
				return nil, fmt.Errorf("file op %s %s: %w", op.Action, op.Path, err)
			}
			hashes[op.Path] = sha256Hex(op.Content)

		case OpDelete:
			if err := assertNoSymlink(op.Boundary, op.Path); err != nil {
				return nil, fmt.Errorf("file op delete %s: %w", op.Path, err)
			}
			if err := os.Remove(op.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("file op delete %s: %w", op.Path, err)
			}

		default:
			return nil, fmt.Errorf("unknown op action %q for path %s", op.Action, op.Path)
		}
	}
	return hashes, nil
}
