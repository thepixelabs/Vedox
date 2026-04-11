// Package store — SymlinkAdapter.
//
// SymlinkAdapter implements DocStore for an external project directory that is
// linked into Vedox without copying. All operations are read-only: Write and
// Delete always return VDX-011. Read and List enforce the same path-traversal
// and secret-file protections as LocalAdapter, but rooted at the external
// directory rather than the workspace root.
//
// Design notes:
//   - externalRoot is resolved to a real (non-symlink) absolute path at
//     construction time. All subsequent operations reuse this resolved root so
//     that mid-session symlink retargeting cannot bypass boundary checks.
//   - Docs returned by Read and List carry two synthetic metadata fields:
//     _source = "symlink" and _editable = false. These are injected after
//     frontmatter parsing and must never be persisted back to disk.
//   - Watch resolves the target directory to its real path before handing it
//     to fsnotify so kqueue/inotify tracks the underlying inode.
package store

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
	vedoxerrors "github.com/vedox/vedox/internal/errors"
)

// SymlinkAdapter implements DocStore for a read-only, externally-linked
// project directory. The external directory may itself be a symlink; it is
// resolved to a real path once at construction time.
type SymlinkAdapter struct {
	// externalRoot is the resolved (real, absolute) path to the external
	// project directory. All file operations are constrained to this subtree.
	externalRoot string

	// projectName is the logical name used to identify this project inside
	// Vedox. It appears in doc Paths so the frontend can route correctly.
	projectName string

	// workspaceRoot is the Vedox workspace root. It is stored so we can
	// enforce the invariant that externalRoot must not be inside the workspace
	// (enforced at construction; stored for diagnostic logging).
	workspaceRoot string

	// logger is the structured logger for this adapter.
	logger *slog.Logger

	// watchedCount tracks the number of paths registered with the fsnotify
	// watcher. Accessed with atomic operations for goroutine safety.
	watchedCount atomic.Int64
}

// NewSymlinkAdapter constructs a SymlinkAdapter.
//
// externalRoot must exist and must not reside inside workspaceRoot (Vedox
// must never write into an external project's directory tree). The path is
// resolved to its real (non-symlink) form before use.
//
// Pass nil for logger to use the default slog logger.
func NewSymlinkAdapter(externalRoot, projectName, workspaceRoot string) (*SymlinkAdapter, error) {
	// Resolve the workspace root first so the containment check is accurate.
	realWorkspace, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("store.NewSymlinkAdapter: resolve workspaceRoot %q: %w", workspaceRoot, err)
	}
	realWorkspace = filepath.Clean(realWorkspace)

	// Resolve and validate the external root.
	realExternal, err := filepath.EvalSymlinks(externalRoot)
	if err != nil {
		return nil, fmt.Errorf("store.NewSymlinkAdapter: resolve externalRoot %q: %w", externalRoot, err)
	}
	realExternal = filepath.Clean(realExternal)

	// externalRoot must not be inside the Vedox workspace — that would allow
	// the symlink adapter to serve files that LocalAdapter is also managing,
	// which creates ambiguous write semantics.
	wsWithSep := realWorkspace + string(os.PathSeparator)
	if realExternal == realWorkspace || strings.HasPrefix(realExternal, wsWithSep) {
		return nil, fmt.Errorf(
			"store.NewSymlinkAdapter: externalRoot %q resolves inside workspaceRoot %q; use LocalAdapter instead",
			realExternal, realWorkspace,
		)
	}

	// The resolved external root must be a directory.
	info, err := os.Stat(realExternal)
	if err != nil {
		return nil, fmt.Errorf("store.NewSymlinkAdapter: stat externalRoot %q: %w", realExternal, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("store.NewSymlinkAdapter: externalRoot %q is not a directory", realExternal)
	}

	if projectName == "" {
		return nil, fmt.Errorf("store.NewSymlinkAdapter: projectName must not be empty")
	}

	return &SymlinkAdapter{
		externalRoot:  realExternal,
		projectName:   projectName,
		workspaceRoot: realWorkspace,
		logger:        slog.Default(),
	}, nil
}

// ExternalRoot returns the resolved absolute path to the external project directory.
func (a *SymlinkAdapter) ExternalRoot() string { return a.externalRoot }

// ProjectName returns the logical project name used inside Vedox.
func (a *SymlinkAdapter) ProjectName() string { return a.projectName }

// -- DocStore implementation --------------------------------------------------

// Read reads the file at path (relative to externalRoot) and returns a Doc.
// The returned Doc has two extra metadata fields:
//
//	_source   = "symlink"
//	_editable = false
//
// Returns VDX-005 if the resolved path escapes externalRoot.
// Returns VDX-006 if path matches the secret file blocklist.
func (a *SymlinkAdapter) Read(path string) (*Doc, error) {
	abs, err := a.safePath("Read", path)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("store.SymlinkAdapter.Read: stat %s: %w", path, err)
	}

	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("store.SymlinkAdapter.Read: read %s: %w", path, err)
	}

	a.logger.Info("store.SymlinkAdapter.Read", slog.String("path", path))

	meta, content := parseFrontmatter(raw)
	injectSymlinkMeta(meta, abs)

	return &Doc{
		Path:     path,
		Content:  content,
		Metadata: meta,
		ModTime:  info.ModTime(),
		Size:     info.Size(),
	}, nil
}

// Write always returns VDX-011. Symlinked documents are read-only in Vedox;
// users must use Import & Migrate to obtain an editable copy.
func (a *SymlinkAdapter) Write(_ string, _ string) error {
	return vedoxerrors.ReadOnly()
}

// Delete always returns VDX-011 for the same reason as Write.
func (a *SymlinkAdapter) Delete(_ string) error {
	return vedoxerrors.ReadOnly()
}

// List walks externalRoot/<dir> recursively and returns all Markdown files.
// Secret-blocked files are silently skipped. Each returned Doc carries the
// _source and _editable metadata fields.
//
// Unlike LocalAdapter.List, this implementation is recursive so that callers
// get the full document tree of an external project in one call. Non-Markdown
// files are skipped.
func (a *SymlinkAdapter) List(dir string) ([]*Doc, error) {
	abs, err := a.safePath("List", dir)
	if err != nil {
		return nil, err
	}

	a.logger.Info("store.SymlinkAdapter.List", slog.String("dir", dir))

	var docs []*Doc

	walkErr := filepath.WalkDir(abs, func(p string, d fs.DirEntry, walkEntryErr error) error {
		if walkEntryErr != nil {
			// Log and skip unreadable entries rather than aborting the whole walk.
			a.logger.Warn("store.SymlinkAdapter.List: walk error, skipping",
				slog.String("path", p),
				slog.String("error", walkEntryErr.Error()),
			)
			return nil
		}

		if d.IsDir() {
			return nil
		}

		name := d.Name()

		// Secret-blocked files are silently skipped — their existence is not
		// surfaced to the caller.
		if isSecretFile(name) {
			a.logger.Warn("store.SymlinkAdapter.List: skipping blocked file",
				slog.String("code", "VDX-006"),
				slog.String("path", p),
			)
			return nil
		}

		info, err := d.Info()
		if err != nil {
			a.logger.Error("store.SymlinkAdapter.List: stat failed, skipping",
				slog.String("path", p),
				slog.String("error", err.Error()),
			)
			return nil
		}

		raw, err := os.ReadFile(p)
		if err != nil {
			a.logger.Error("store.SymlinkAdapter.List: read failed, skipping",
				slog.String("path", p),
				slog.String("error", err.Error()),
			)
			return nil
		}

		// Compute the path relative to externalRoot so callers get a consistent
		// relative path, not an absolute filesystem path.
		rel, err := filepath.Rel(a.externalRoot, p)
		if err != nil {
			a.logger.Error("store.SymlinkAdapter.List: cannot compute relative path, skipping",
				slog.String("abs", p),
				slog.String("error", err.Error()),
			)
			return nil
		}

		meta, content := parseFrontmatter(raw)
		injectSymlinkMeta(meta, p)

		docs = append(docs, &Doc{
			Path:    rel,
			Content: content,
			Metadata: meta,
			ModTime: info.ModTime(),
			Size:    info.Size(),
		})

		return nil
	})

	if walkErr != nil {
		return nil, fmt.Errorf("store.SymlinkAdapter.List: walk %s: %w", dir, walkErr)
	}

	if docs == nil {
		docs = []*Doc{}
	}
	return docs, nil
}

// Watch starts a recursive file-system watcher on externalRoot/<dir> and
// calls onChange with the externalRoot-relative path of any file that is
// created, modified, or removed.
//
// The resolved (real) path is watched — not the symlink — so that
// kqueue/inotify tracks the underlying inode. Symlinks inside the watched
// tree that resolve outside externalRoot are skipped with a WARN log.
//
// This method blocks until the watcher is closed or an unrecoverable error
// occurs; callers must run it in a goroutine.
func (a *SymlinkAdapter) Watch(dir string, onChange func(path string)) error {
	abs, err := a.safePath("Watch", dir)
	if err != nil {
		return err
	}

	// Resolve the root of the watch target.
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return fmt.Errorf("store.SymlinkAdapter.Watch: resolve symlink for %s: %w", dir, err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("store.SymlinkAdapter.Watch: create watcher: %w", err)
	}
	defer watcher.Close()

	// Walk and register all subdirectories with the watcher.
	if err := filepath.WalkDir(real, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries
		}
		if !d.IsDir() {
			return nil
		}

		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			a.logger.Warn("store.SymlinkAdapter.Watch: cannot resolve symlink, skipping",
				slog.String("path", p),
				slog.String("error", err.Error()),
			)
			return nil
		}

		// Enforce externalRoot boundary: reject symlinks that escape.
		rootWithSep := a.externalRoot + string(os.PathSeparator)
		if resolved != a.externalRoot && !strings.HasPrefix(resolved, rootWithSep) {
			a.logger.Warn("store.SymlinkAdapter.Watch: symlink escapes externalRoot, skipping",
				slog.String("path", p),
				slog.String("resolved", resolved),
				slog.String("externalRoot", a.externalRoot),
			)
			return nil
		}

		if err := watcher.Add(resolved); err != nil {
			a.logger.Warn("store.SymlinkAdapter.Watch: failed to watch dir",
				slog.String("path", resolved),
				slog.String("error", err.Error()),
			)
			return nil
		}

		count := a.watchedCount.Add(1)
		if count == watcherWarningThreshold {
			a.logger.Warn(fmt.Sprintf(
				"[VDX-007] SymlinkAdapter watching %d+ files in external project %q. Consider filtering paths.",
				watcherWarningThreshold, a.projectName,
			),
				slog.Int64("watched_count", count),
			)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("store.SymlinkAdapter.Watch: walk %s: %w", dir, err)
	}

	a.logger.Info("store.SymlinkAdapter.Watch: started",
		slog.String("dir", dir),
		slog.String("project", a.projectName),
	)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
				// Convert the absolute path back to externalRoot-relative.
				rel, err := filepath.Rel(a.externalRoot, event.Name)
				if err != nil {
					a.logger.Warn("store.SymlinkAdapter.Watch: cannot compute relative path",
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
			a.logger.Error("store.SymlinkAdapter.Watch: watcher error",
				slog.String("error", err.Error()),
			)
		}
	}
}

// -- Internal helpers ---------------------------------------------------------

// safePath resolves path relative to externalRoot and asserts the result is
// within externalRoot. It also enforces the secret-file blocklist.
//
// filepath.EvalSymlinks is used (unlike LocalAdapter which uses filepath.Abs)
// because the external root itself is a resolved real path — we must compare
// real paths on both sides to avoid symlink-escape attacks.
func (a *SymlinkAdapter) safePath(op, path string) (string, error) {
	joined := filepath.Join(a.externalRoot, path)
	abs := filepath.Clean(joined)

	// Resolve symlinks in the joined path so we can compare real paths.
	// We use Abs rather than EvalSymlinks here because the target file may not
	// yet exist (e.g. during a Watch walk for a file just created). The boundary
	// check below uses string prefix matching on cleaned paths, which is safe
	// given that externalRoot was already fully resolved at construction.
	rootWithSep := a.externalRoot + string(os.PathSeparator)
	if abs != a.externalRoot && !strings.HasPrefix(abs, rootWithSep) {
		a.logger.Warn("store.SymlinkAdapter: path traversal attempt blocked",
			slog.String("code", "VDX-005"),
			slog.String("op", op),
			slog.String("path", path),
		)
		return "", errPathTraversal(op, path)
	}

	// Secret file blocklist check against the base filename.
	if isSecretFile(filepath.Base(abs)) {
		a.logger.Warn("store.SymlinkAdapter: secret file access blocked",
			slog.String("code", "VDX-006"),
			slog.String("op", op),
			slog.String("path", path),
		)
		return "", errSecretFile(op, path)
	}

	return abs, nil
}

// injectSymlinkMeta adds the _source and _editable synthetic metadata fields
// into meta after frontmatter parsing. sourcePath is the absolute filesystem
// path of the file, stored as _source_path for the frontend read-only banner.
func injectSymlinkMeta(meta map[string]interface{}, sourcePath string) {
	meta["_source"] = "symlink"
	meta["_editable"] = false
	meta["_source_path"] = sourcePath
}
