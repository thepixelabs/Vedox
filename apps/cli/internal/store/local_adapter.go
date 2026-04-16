package store

import (
	"bytes"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// watcherWarningThreshold is the number of watched filesystem entries at which
// VDX-007 is emitted. Moving this warning into Phase 1 was mandated by the CTO
// audit because dogfooding workspace scans (with node_modules) will breach the
// OS inotify/kqueue default limits on day one.
const watcherWarningThreshold = 1000

// secretBlocklist contains file name patterns that are never allowed through any
// read or write operation. Pattern matching uses filepath.Match (glob).
// These are checked against the base filename, not the full path, so
// ".env" matches both "root/.env" and "docs/.env".
var secretBlocklist = []string{
	".env",
	"*.pem",
	"*.key",
	"id_rsa",
	"*.p12",
	"credentials.json",
}

// LocalAdapter implements DocStore for a local filesystem workspace. Every
// operation is constrained to the workspace root via path traversal checks.
// Atomic writes use the temp-file → fsync → rename pattern so a crash mid-write
// never leaves a partially-written file.
//
// Logging policy: operation names and paths are logged; file contents are never
// logged. This is enforced structurally — we only call slog with the path string,
// never with content.
type LocalAdapter struct {
	// root is the absolute, cleaned workspace root path. All operations are
	// restricted to paths under this directory.
	root string

	// logger is the structured logger for this adapter. It always uses slog so
	// callers can wire in any slog.Handler.
	logger *slog.Logger

	// watchedCount tracks the number of paths currently registered with the
	// fsnotify watcher. It is accessed with atomic operations to allow safe
	// reads from any goroutine.
	watchedCount atomic.Int64
}

// NewLocalAdapter constructs a LocalAdapter rooted at workspaceRoot. The root
// is resolved to an absolute path; if it doesn't exist or can't be resolved,
// an error is returned. Pass nil for logger to use the default slog logger.
func NewLocalAdapter(workspaceRoot string, logger *slog.Logger) (*LocalAdapter, error) {
	abs, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("store.NewLocalAdapter: cannot resolve workspace root %q: %w", workspaceRoot, err)
	}
	// Clean eliminates any ".." components that slipped through before Abs.
	abs = filepath.Clean(abs)

	// Resolve symlinks on the root so Watch boundary checks compare against
	// the canonical path. Event paths are always EvalSymlinks-resolved before
	// the HasPrefix check in Watch; without this the boundary check rejects
	// every event in a tempdir-rooted workspace (e.g. macOS /var/folders is a
	// symlink to /private/var/folders).
	// If the directory does not exist yet, fall back to the cleaned abs path —
	// callers may construct the adapter before creating the workspace dir.
	if resolved, rerr := filepath.EvalSymlinks(abs); rerr == nil {
		abs = resolved
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &LocalAdapter{
		root:   abs,
		logger: logger,
	}, nil
}

// Root returns the absolute workspace root this adapter is constrained to.
func (a *LocalAdapter) Root() string {
	return a.root
}

// -- DocStore implementation --------------------------------------------------

// Read reads the file at path (workspace-relative or absolute) and returns a
// Doc with content and parsed frontmatter. The path undergoes security checks
// before any I/O occurs. Returns VDX-005 for path traversal, VDX-006 for
// blocked secret files.
func (a *LocalAdapter) Read(path string) (*Doc, error) {
	const op = "Read"

	abs, err := a.safePath(op, path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("store.LocalAdapter.Read: stat %s: %w", path, err)
	}

	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("store.LocalAdapter.Read: read %s: %w", path, err)
	}

	a.logger.Info("store.Read", slog.String("path", path))

	meta, content := parseFrontmatter(raw)

	return &Doc{
		Path:     path,
		Content:  content,
		Metadata: meta,
		ModTime:  info.ModTime(),
		Size:     info.Size(),
	}, nil
}

// Write atomically writes content to the file at path. The atomic pattern is:
//  1. Create a temp file in the same directory (guarantees same filesystem → rename is atomic).
//  2. Write all content to the temp file.
//  3. fsync the temp file (durability: survives crash before rename completes).
//  4. os.Rename the temp file onto the target path (atomic at the kernel level).
//
// If any step fails the temp file is cleaned up and an error is returned.
// The target path is never touched until the rename succeeds.
func (a *LocalAdapter) Write(path string, content string) error {
	const op = "Write"

	abs, err := a.safePath(op, path)
	if err != nil {
		return err
	}

	// Ensure the parent directory exists before we try to create a temp file in it.
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("store.LocalAdapter.Write: mkdir %s: %w", dir, err)
	}

	// Step 1: create temp file in the same directory as the target. Using the
	// same directory guarantees the rename is on the same filesystem and
	// therefore atomic. The "*" prefix keeps the name recognizable in crash dumps.
	tmp, err := os.CreateTemp(dir, ".vedox-write-*")
	if err != nil {
		return fmt.Errorf("store.LocalAdapter.Write: create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	// Always try to clean up the temp file on any failure path.
	success := false
	defer func() {
		if !success {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	// Step 2: write content.
	if _, err := tmp.WriteString(content); err != nil {
		return fmt.Errorf("store.LocalAdapter.Write: write to temp file: %w", err)
	}

	// Step 3: fsync — flush kernel buffers to disk. This is the durability
	// guarantee: even if the machine crashes between Write and Rename, the data
	// is on disk and the original file (if any) is untouched.
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("store.LocalAdapter.Write: fsync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("store.LocalAdapter.Write: close temp file: %w", err)
	}

	// Step 4: atomic rename. On POSIX systems rename(2) is atomic — readers
	// either see the old file or the new file, never a partial write.
	if err := os.Rename(tmpName, abs); err != nil {
		return fmt.Errorf("store.LocalAdapter.Write: rename temp to %s: %w", path, err)
	}

	success = true
	a.logger.Info("store.Write", slog.String("path", path))
	return nil
}

// Delete removes the file at path. Returns VDX-005 / VDX-006 on security
// violations. Returns a wrapped os error if the file does not exist.
func (a *LocalAdapter) Delete(path string) error {
	const op = "Delete"

	abs, err := a.safePath(op, path)
	if err != nil {
		return err
	}

	if err := os.Remove(abs); err != nil {
		return fmt.Errorf("store.LocalAdapter.Delete: remove %s: %w", path, err)
	}

	a.logger.Info("store.Delete", slog.String("path", path))
	return nil
}

// List returns all files (non-recursive) directly inside dir. dir is
// workspace-relative. Only regular files are returned; subdirectories and
// symlinks are skipped. Returns VDX-005 if dir escapes the workspace root.
func (a *LocalAdapter) List(dir string) ([]*Doc, error) {
	const op = "List"

	abs, err := a.safePath(op, dir)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("store.LocalAdapter.List: readdir %s: %w", dir, err)
	}

	a.logger.Info("store.List", slog.String("dir", dir))

	var docs []*Doc
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}

		name := entry.Name()
		filePath := filepath.Join(dir, name)

		// Secret-blocked files in a directory listing are silently skipped —
		// their existence is not surfaced to the caller per the blocklist spec.
		if isSecretFile(name) {
			a.logger.Warn("store.List: skipping blocked file",
				slog.String("code", "VDX-006"),
				slog.String("path", filePath),
			)
			continue
		}

		info, err := entry.Info()
		if err != nil {
			// Log and skip rather than failing the whole listing.
			a.logger.Error("store.List: stat failed, skipping",
				slog.String("path", filePath),
				slog.String("error", err.Error()),
			)
			continue
		}

		raw, err := os.ReadFile(filepath.Join(abs, name))
		if err != nil {
			a.logger.Error("store.List: read failed, skipping",
				slog.String("path", filePath),
				slog.String("error", err.Error()),
			)
			continue
		}

		meta, content := parseFrontmatter(raw)
		docs = append(docs, &Doc{
			Path:     filePath,
			Content:  content,
			Metadata: meta,
			ModTime:  info.ModTime(),
			Size:     info.Size(),
		})
	}

	if docs == nil {
		docs = []*Doc{}
	}
	return docs, nil
}

// Watch starts a recursive file-system watcher on dir and calls onChange with
// the workspace-relative path whenever a file is written or removed. It blocks
// until an unrecoverable watcher error occurs; callers should run it in a
// goroutine.
//
// Symlinks are resolved to their real path before being added to the watcher so
// kqueue/inotify watches the underlying inode, not the symlink entry.
//
// VDX-007: when the watched file count crosses watcherWarningThreshold, a WARN
// is logged. This was moved into Phase 1 per the CTO audit because dogfooding
// with a workspace that has node_modules will hit OS watcher limits on day one.
func (a *LocalAdapter) Watch(dir string, onChange func(path string)) error {
	const op = "Watch"

	abs, err := a.safePath(op, dir)
	if err != nil {
		return err
	}

	// Resolve symlinks on the watch root itself.
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return fmt.Errorf("store.LocalAdapter.Watch: resolve symlink for %s: %w", dir, err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("store.LocalAdapter.Watch: create watcher: %w", err)
	}
	defer watcher.Close()

	// Walk the directory tree and register every sub-directory with the watcher.
	// fsnotify watches directories, not individual files; any event inside a
	// watched directory fires on that directory's watcher.
	if err := filepath.WalkDir(real, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // Skip unreadable entries rather than aborting the whole walk.
		}
		if !d.IsDir() {
			return nil
		}

		// Resolve any per-entry symlinks (subdirectory symlinks).
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			a.logger.Warn("store.Watch: cannot resolve symlink, skipping",
				slog.String("path", p),
				slog.String("error", err.Error()),
			)
			return nil
		}

		// Enforce workspace boundary even for resolved symlink targets.
		if !strings.HasPrefix(resolved, a.root) {
			a.logger.Warn("store.Watch: symlink escapes workspace root, skipping",
				slog.String("path", p),
				slog.String("resolved", resolved),
			)
			return nil
		}

		if err := watcher.Add(resolved); err != nil {
			a.logger.Warn("store.Watch: failed to watch dir",
				slog.String("path", resolved),
				slog.String("error", err.Error()),
			)
			return nil
		}

		count := a.watchedCount.Add(1)
		if count == watcherWarningThreshold {
			a.logger.Warn(fmt.Sprintf("[VDX-007] Watching %d+ files. Consider adding paths to .vedoxignore to avoid hitting OS watcher limits.", watcherWarningThreshold),
				slog.Int64("watched_count", count),
			)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("store.LocalAdapter.Watch: walk %s: %w", dir, err)
	}

	a.logger.Info("store.Watch: started", slog.String("dir", dir))

	// Event loop: translate fsnotify events to workspace-relative paths and call
	// onChange for Write and Remove events. Create events map to Write semantics.
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil // Watcher was closed.
			}

			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
				// Convert absolute path back to workspace-relative.
				rel, err := filepath.Rel(a.root, event.Name)
				if err != nil {
					a.logger.Warn("store.Watch: cannot compute relative path",
						slog.String("abs", event.Name),
						slog.String("error", err.Error()),
					)
					continue
				}
				onChange(rel)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			a.logger.Error("store.Watch: watcher error",
				slog.String("error", err.Error()),
			)
		}
	}
}

// -- Internal helpers ---------------------------------------------------------

// safePath resolves path to an absolute, cleaned path and asserts it is within
// the workspace root. It also checks the secret file blocklist. Returns the
// absolute path if safe, or a VDX-005/VDX-006 error if not.
//
// This function is called at the top of every exported operation — it is the
// single, centralised enforcement point for path security.
func (a *LocalAdapter) safePath(op, path string) (string, error) {
	// Join with root if relative; Abs handles the case where path is already absolute.
	joined := filepath.Join(a.root, path)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("store.LocalAdapter.%s: resolve path %q: %w", op, path, err)
	}
	abs = filepath.Clean(abs)

	// Path traversal check: the resolved path must be inside the workspace root.
	// We append os.PathSeparator to the root so that "/workspace-root-extra"
	// does not match "/workspace-root" as a prefix.
	rootWithSep := a.root + string(os.PathSeparator)
	if abs != a.root && !strings.HasPrefix(abs, rootWithSep) {
		a.logger.Warn("store: path traversal attempt blocked",
			slog.String("code", "VDX-005"),
			slog.String("op", op),
			slog.String("path", path),
		)
		return "", errPathTraversal(op, path)
	}

	// Secret file blocklist check: applies to the base filename only.
	if isSecretFile(filepath.Base(abs)) {
		a.logger.Warn("store: secret file access blocked",
			slog.String("code", "VDX-006"),
			slog.String("op", op),
			slog.String("path", path),
		)
		return "", errSecretFile(op, path)
	}

	return abs, nil
}

// stripDraftSuffixes removes draft-variant suffixes from a filename so the
// residual basename can be compared against the secret blocklist.
//
// Recognised suffixes (longest match wins, applied once):
//   - ".draft.md.<N>"  — numbered draft (e.g. ".env.draft.md.2")
//   - ".draft.md"      — standard draft (e.g. ".env.draft.md")
//   - ".draft"         — draft without .md extension (e.g. ".env.draft")
//
// If no recognised suffix is present the original name is returned unchanged.
// The match is case-sensitive to match the rest of the blocklist logic.
func stripDraftSuffixes(name string) string {
	// Numbered draft: ends with ".draft.md." followed by one or more digits.
	// Walk from the right: find the last ".draft.md." occurrence and check that
	// the tail after it is all digits.
	const draftMD = ".draft.md"
	if idx := strings.LastIndex(name, draftMD+"."); idx >= 0 {
		tail := name[idx+len(draftMD)+1:]
		allDigits := len(tail) > 0
		for _, ch := range tail {
			if ch < '0' || ch > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return name[:idx]
		}
	}
	// Standard .draft.md suffix.
	if strings.HasSuffix(name, draftMD) {
		return strings.TrimSuffix(name, draftMD)
	}
	// Bare .draft suffix (no .md extension).
	if strings.HasSuffix(name, ".draft") {
		return strings.TrimSuffix(name, ".draft")
	}
	return name
}

// isSecretFile reports whether name matches any pattern in the secret blocklist.
// name should be the base filename (no directory components). Patterns use
// filepath.Match glob syntax.
//
// Draft-variant suffixes (.draft.md, .draft.md.<N>, .draft) are stripped from
// name before the blocklist check so that ".env.draft.md" is correctly treated
// as a secret file (FINAL_PLAN changelog item 31).
func isSecretFile(name string) bool {
	// Strip any draft suffix before the blocklist check. This ensures that
	// ".env.draft.md" is seen as ".env" and correctly blocked.
	stripped := stripDraftSuffixes(name)

	for _, pattern := range secretBlocklist {
		matched, err := filepath.Match(pattern, stripped)
		if err != nil {
			// Only happens if pattern is malformed — our patterns are constants, so
			// this is a programming error; treat as a match to fail safe.
			return true
		}
		if matched {
			return true
		}
	}
	return false
}

// parseFrontmatter splits raw Markdown bytes into a metadata map and the
// content string (the full raw text including any frontmatter, to preserve
// round-trip fidelity). If the file does not begin with "---\n" the returned
// map is empty and content is the full raw string.
//
// The returned content always equals string(raw) so callers get the full file
// contents for editor round-trips. Metadata is the *parsed* view of the
// frontmatter block only.
func parseFrontmatter(raw []byte) (meta map[string]interface{}, content string) {
	content = string(raw)
	meta = make(map[string]interface{})

	// Front-matter delimiter check: must start with "---" followed by a newline.
	const delim = "---"
	if !bytes.HasPrefix(raw, []byte(delim+"\n")) {
		return meta, content
	}

	// Find the closing "---" on its own line.
	rest := raw[len(delim)+1:] // skip opening "---\n"
	end := bytes.Index(rest, []byte("\n"+delim))
	if end < 0 {
		// Unclosed frontmatter block — treat as no frontmatter rather than error.
		return meta, content
	}

	yamlBlock := rest[:end]

	// We intentionally discard the parse error rather than returning it: a file
	// with malformed frontmatter should still be readable; callers get an empty
	// map and we log the parse failure without emitting the content.
	if err := yaml.Unmarshal(yamlBlock, &meta); err != nil {
		// Log path is not available here; the caller logs the path. We log
		// a short message so a slog handler can correlate via request context.
		slog.Warn("store.parseFrontmatter: invalid YAML frontmatter; ignoring",
			slog.String("error", err.Error()),
		)
		meta = make(map[string]interface{})
	}

	return meta, content
}
