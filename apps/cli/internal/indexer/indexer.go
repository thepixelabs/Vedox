// Package indexer implements a background file-system watcher that keeps the
// SQLite FTS5 index in sync as Markdown files change on disk.
//
// Design constraints:
//   - Never block the HTTP server: all indexing runs inside the goroutine
//     started by Start; the caller's goroutine is unaffected.
//   - Debounce rapid writes: editors that auto-save frequently would otherwise
//     thrash the DB. A 300ms per-path timer resets on each new event.
//   - inotify/kqueue limits: track watched path count with an atomic. Warn at
//     1000, refuse to add more beyond 2000.
//   - Draft exclusion: paths under .vedox/drafts/ are never indexed.
//   - Secret blocklist: store.Read enforces VDX-006; we log at DEBUG and skip.
package indexer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/store"
)

const (
	// debounceDuration is how long to wait after the last event on a path
	// before processing it. 300ms is well under the typical editor auto-save
	// interval (800ms–1s) so we debounce without adding perceptible lag.
	debounceDuration = 300 * time.Millisecond

	// watchWarnThreshold matches LocalAdapter's VDX-007 threshold.
	watchWarnThreshold = 1000

	// watchHardLimit is the absolute ceiling on watched paths. Beyond this we
	// stop adding watchers and log an ERROR — staying silent would cause silent
	// indexing gaps which are harder to diagnose than a loud failure.
	watchHardLimit = 2000

	// maxWatchDepth matches scanner.maxDepth so we watch the same tree the
	// initial scan covers.
	maxWatchDepth = 5
)

// Indexer watches a workspace directory tree and upserts/deletes documents in
// the SQLite index as files change. Create with New; call Start in a goroutine.
type Indexer struct {
	store         store.DocStore
	db            *db.Store
	workspaceRoot string // absolute, cleaned path
	logger        *slog.Logger

	watcher    *fsnotify.Watcher
	watchCount atomic.Int64

	// debounce state: one timer per path, reset on repeated events.
	mu      sync.Mutex
	timers  map[string]*time.Timer
	pending map[string]fsnotify.Op // the most recent op for a path

	stopOnce sync.Once
	stopCh   chan struct{}

	// runDone is closed when runLoop has fully exited (including drainTimers).
	// Stop waits on it rather than on a WaitGroup because using wg.Add inside
	// Start would race with wg.Wait in a caller who calls Stop before Start's
	// goroutine has had a chance to run. The channel is created in New so it
	// exists regardless of whether Start is ever called — a Stop-before-Start
	// caller sees it as a closed-on-its-own pseudo-state by observing
	// started=false.
	runDone chan struct{}

	// started is set to true by Start before it enters runLoop. Stop reads it
	// (under startedMu) to decide whether to wait on runDone or return early.
	// atomic.Bool would work too; mutex keeps the init-ordering guarantees
	// easy to reason about.
	startedMu sync.Mutex
	started   bool

	// afterFuncWG tracks in-flight debounce AfterFunc callbacks (NOT runLoop).
	// It is only ever Add()ed from inside runLoop and Wait()ed after runLoop
	// has exited, so there is no Add/Wait race.
	afterFuncWG sync.WaitGroup
}

// New creates an Indexer. workspaceRoot must be a valid, accessible directory.
// Pass nil for logger to use the default slog logger.
func New(s store.DocStore, dbStore *db.Store, workspaceRoot string) *Indexer {
	abs, err := filepath.Abs(workspaceRoot)
	if err != nil {
		abs = workspaceRoot
	}
	abs = filepath.Clean(abs)
	// Resolve symlinks on the workspace root so the boundary check in addDirTree
	// (which compares EvalSymlinks-resolved event paths against ix.workspaceRoot
	// via HasPrefix) doesn't reject legitimate paths on platforms where the temp
	// directory is itself a symlink (e.g. macOS /var/folders -> /private/var/folders).
	// If the directory does not exist yet, fall back to the cleaned abs path.
	if resolved, rerr := filepath.EvalSymlinks(abs); rerr == nil {
		abs = resolved
	}

	return &Indexer{
		store:         s,
		db:            dbStore,
		workspaceRoot: abs,
		logger:        slog.Default(),
		timers:        make(map[string]*time.Timer),
		pending:       make(map[string]fsnotify.Op),
		stopCh:        make(chan struct{}),
		runDone:       make(chan struct{}),
	}
}

// Start begins watching the workspace. It blocks until ctx is cancelled, then
// performs a graceful shutdown: closes the watcher, drains and cancels all
// pending debounce timers, and returns nil.
//
// Start is designed to run in a goroutine:
//
//	go func() { _ = ix.Start(ctx) }()
func (ix *Indexer) Start(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("indexer: create watcher: %w", err)
	}
	ix.watcher = watcher

	// Walk the workspace tree and register every directory up to maxWatchDepth.
	if err := ix.addDirTree(ix.workspaceRoot, 0); err != nil {
		_ = watcher.Close()
		return fmt.Errorf("indexer: initial watch setup: %w", err)
	}

	ix.logger.Info("indexer: started", slog.String("root", ix.workspaceRoot))

	// Mark as started BEFORE runLoop begins so any concurrent Stop() will
	// correctly wait on runDone instead of returning early.
	ix.startedMu.Lock()
	ix.started = true
	ix.startedMu.Unlock()

	// Run the event loop in this goroutine. The caller put us in a goroutine.
	// runDone is closed in runLoop's defer after every cleanup step (including
	// afterFuncWG.Wait), so observing it closed is a strong shutdown signal.
	ix.runLoop(ctx)

	return nil
}

// Stop signals the indexer to shut down and blocks until every goroutine it
// spawned (the runLoop and every in-flight debounce callback) has exited.
// Safe to call multiple times; subsequent calls return once the first has
// completed. Safe to call before or without Start — in that case only stopCh
// is closed and Stop returns immediately because no runLoop was ever entered.
func (ix *Indexer) Stop() {
	ix.stopOnce.Do(func() {
		close(ix.stopCh)
	})

	// Snapshot started state. If Start never ran there is no runLoop to wait
	// for; returning now is correct.
	ix.startedMu.Lock()
	started := ix.started
	ix.startedMu.Unlock()
	if !started {
		return
	}
	<-ix.runDone
}

// runLoop is the core event dispatch loop. It exits when ctx is done or
// stopCh is closed, then cleans up.
func (ix *Indexer) runLoop(ctx context.Context) {
	defer func() {
		// Ensure stopCh is closed on every exit path (including ctx
		// cancellation) so any AfterFunc callback that fires after this point
		// can see the shutdown signal and bail before touching ix.db.
		ix.stopOnce.Do(func() { close(ix.stopCh) })
		_ = ix.watcher.Close()
		ix.drainTimers()
		// Wait for any AfterFunc callback that had already fired and was
		// racing us to complete — drainTimers cannot cancel a running one.
		ix.afterFuncWG.Wait()
		ix.logger.Info("indexer: stopped")
		close(ix.runDone)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ix.stopCh:
			return

		case event, ok := <-ix.watcher.Events:
			if !ok {
				return
			}
			ix.handleEvent(ctx, event)

		case err, ok := <-ix.watcher.Errors:
			if !ok {
				return
			}
			ix.logger.Error("indexer: watcher error", slog.String("error", err.Error()))
		}
	}
}

// handleEvent classifies the fsnotify event and schedules debounced processing.
func (ix *Indexer) handleEvent(ctx context.Context, event fsnotify.Event) {
	// Ignore chmod — no content change.
	if event.Has(fsnotify.Chmod) && !event.Has(fsnotify.Write) &&
		!event.Has(fsnotify.Create) && !event.Has(fsnotify.Remove) &&
		!event.Has(fsnotify.Rename) {
		return
	}

	path := event.Name

	// If a new directory was created, add it to the watcher immediately
	// (no debounce — directories don't need content indexing).
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			// Compute depth relative to workspace root so we respect maxWatchDepth.
			rel, _ := filepath.Rel(ix.workspaceRoot, path)
			depth := len(strings.Split(filepath.ToSlash(rel), "/"))
			_ = ix.addDirTree(path, depth)
			return // directory itself doesn't need FTS indexing
		}
	}

	// Only index .md files.
	if filepath.Ext(path) != ".md" {
		return
	}

	// Skip draft auto-saves.
	if ix.isDraft(path) {
		ix.logger.Debug("indexer: skipping draft path", slog.String("path", path))
		return
	}

	// Determine the canonical op to use if multiple events arrive during debounce.
	// Remove/Rename wins over Write/Create (last-write-wins is wrong if file is gone).
	op := event.Op
	ix.scheduleDebounce(ctx, path, op)
}

// isDraft reports whether path is under the .vedox/drafts/ directory.
func (ix *Indexer) isDraft(path string) bool {
	rel, err := filepath.Rel(ix.workspaceRoot, path)
	if err != nil {
		return false
	}
	// Normalise to forward slashes for reliable prefix matching on all platforms.
	rel = filepath.ToSlash(rel)
	return strings.HasPrefix(rel, ".vedox/drafts/")
}

// scheduleDebounce arms or resets the per-path debounce timer. When the timer
// fires (300ms after the last event on that path) processPath is called.
func (ix *Indexer) scheduleDebounce(ctx context.Context, path string, op fsnotify.Op) {
	ix.mu.Lock()
	defer ix.mu.Unlock()

	// Merge ops: if a remove is already pending, keep it even if a write arrives.
	// This handles the editor "write then immediately delete temp file" pattern.
	existing, hasPending := ix.pending[path]
	if hasPending && (existing.Has(fsnotify.Remove) || existing.Has(fsnotify.Rename)) {
		// Keep the destructive op regardless of the new one.
	} else {
		ix.pending[path] = op
	}

	if t, exists := ix.timers[path]; exists {
		t.Reset(debounceDuration)
		return
	}

	// First event for this path: arm a new timer. Track the pending callback
	// on afterFuncWG so runLoop's defer can wait for any callback that had
	// already fired before drainTimers got to it — timer.Stop() does NOT
	// wait for an already-running AfterFunc goroutine.
	ix.afterFuncWG.Add(1)
	t := time.AfterFunc(debounceDuration, func() {
		defer ix.afterFuncWG.Done()

		// Bail out if shutdown has begun. drainTimers clears any pending
		// entry we would have read, and skipping the DB write here avoids
		// touching ix.db after the caller considers the indexer stopped.
		select {
		case <-ix.stopCh:
			return
		default:
		}

		ix.mu.Lock()
		op, ok := ix.pending[path]
		delete(ix.timers, path)
		delete(ix.pending, path)
		ix.mu.Unlock()

		// If drainTimers raced us and cleared the pending op there is
		// nothing to do — a zero Op would misroute to upsertDoc.
		if !ok {
			return
		}

		ix.processPath(ctx, path, op)
	})
	ix.timers[path] = t
}

// processPath runs the actual index update for a path after the debounce fires.
func (ix *Indexer) processPath(ctx context.Context, path string, op fsnotify.Op) {
	rel, err := filepath.Rel(ix.workspaceRoot, path)
	if err != nil {
		ix.logger.Error("indexer: cannot compute rel path",
			slog.String("path", path),
			slog.String("error", err.Error()),
		)
		return
	}
	rel = filepath.ToSlash(rel)

	if op.Has(fsnotify.Remove) || op.Has(fsnotify.Rename) {
		ix.deleteDoc(ctx, rel)
		return
	}

	// Write or Create.
	ix.upsertDoc(ctx, rel)
}

// upsertDoc reads the file via the store and inserts/updates the FTS index.
func (ix *Indexer) upsertDoc(ctx context.Context, relPath string) {
	storDoc, err := ix.store.Read(relPath)
	if err != nil {
		// VDX-006: secret file — log at debug and skip (expected, not an error).
		// VDX-005: path traversal — should not happen since the watcher is scoped
		//          to workspaceRoot, but log and skip defensively.
		ix.logger.Debug("indexer: store.Read skipped",
			slog.String("path", relPath),
			slog.String("error", err.Error()),
		)
		return
	}

	doc := storeDocToDBDoc(storDoc)

	if err := ix.db.UpsertDoc(ctx, doc); err != nil {
		ix.logger.Error("indexer: UpsertDoc failed",
			slog.String("path", relPath),
			slog.String("error", err.Error()),
		)
		return
	}

	ix.logger.Debug("indexer: upserted", slog.String("path", relPath))
}

// deleteDoc removes a document from the FTS index.
func (ix *Indexer) deleteDoc(ctx context.Context, relPath string) {
	if err := ix.db.DeleteDoc(ctx, relPath); err != nil {
		ix.logger.Error("indexer: DeleteDoc failed",
			slog.String("path", relPath),
			slog.String("error", err.Error()),
		)
		return
	}
	ix.logger.Debug("indexer: deleted", slog.String("path", relPath))
}

// addDirTree walks path recursively (up to maxWatchDepth - currentDepth levels)
// and adds each directory to the fsnotify watcher. Symlinks are resolved.
// Skips .vedox/drafts and hidden directories.
func (ix *Indexer) addDirTree(root string, currentDepth int) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries
		}
		if !d.IsDir() {
			return nil
		}

		name := d.Name()

		// Skip hidden dirs (but allow the workspace root itself which may be "." in some paths).
		if p != root && len(name) > 0 && name[0] == '.' {
			return fs.SkipDir
		}

		// Skip common large dirs.
		if name == "node_modules" || name == "vendor" {
			return fs.SkipDir
		}

		// Compute depth relative to the indexer's workspace root, not to root
		// (which may be a sub-directory if we're adding a newly created dir).
		rel, _ := filepath.Rel(ix.workspaceRoot, p)
		relParts := strings.Split(filepath.ToSlash(rel), "/")
		depth := len(relParts)
		if rel == "." {
			depth = 0
		}
		if depth > maxWatchDepth {
			return fs.SkipDir
		}

		// Resolve symlinks before watching.
		resolved, err := filepath.EvalSymlinks(p)
		if err != nil {
			ix.logger.Warn("indexer: cannot resolve symlink, skipping",
				slog.String("path", p),
				slog.String("error", err.Error()),
			)
			return nil
		}

		// Enforce workspace boundary for symlink targets.
		rootWithSep := ix.workspaceRoot + string(os.PathSeparator)
		if resolved != ix.workspaceRoot && !strings.HasPrefix(resolved, rootWithSep) {
			ix.logger.Warn("indexer: symlink escapes workspace, skipping",
				slog.String("path", p),
				slog.String("resolved", resolved),
			)
			return nil
		}

		count := ix.watchCount.Load()
		if count >= watchHardLimit {
			ix.logger.Error(fmt.Sprintf("[VDX-007] Watch limit reached (%d). Not adding more watchers. Add paths to .vedoxignore to reduce the watched file count.", watchHardLimit))
			return fs.SkipAll
		}

		if err := ix.watcher.Add(resolved); err != nil {
			ix.logger.Warn("indexer: failed to watch dir",
				slog.String("path", resolved),
				slog.String("error", err.Error()),
			)
			return nil
		}

		newCount := ix.watchCount.Add(1)
		if newCount == watchWarnThreshold {
			ix.logger.Warn(fmt.Sprintf("[VDX-007] Watching %d+ files. Consider adding paths to .vedoxignore to avoid hitting OS watcher limits.", watchWarnThreshold),
				slog.Int64("watched_count", newCount),
			)
		}

		return nil
	})
}

// drainTimers cancels all pending debounce timers at shutdown. We intentionally
// do NOT flush them — a file written 50ms before shutdown doesn't need to be
// indexed; the next startup's reindex will catch it.
//
// For every timer we successfully cancel before it fires (t.Stop returns
// true), we must call afterFuncWG.Done to balance the Add done when the timer
// was armed — the AfterFunc callback's own deferred Done will not run because
// the callback never executes. If Stop returns false the callback is already
// running (or about to run) and will Done itself via its deferred call.
func (ix *Indexer) drainTimers() {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	for path, t := range ix.timers {
		if t.Stop() {
			ix.afterFuncWG.Done()
		}
		delete(ix.timers, path)
	}
	for path := range ix.pending {
		delete(ix.pending, path)
	}
}

// slugFromFilename derives an index-only slug from a base filename. It strips
// the extension, lowercases, replaces non-alphanumerics with hyphens, collapses
// runs, and trims leading/trailing hyphens. Never written back to disk.
var slugNonAlnumRE = regexp.MustCompile(`[^a-z0-9]+`)

func slugFromFilename(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	base = strings.ToLower(base)
	base = slugNonAlnumRE.ReplaceAllString(base, "-")
	return strings.Trim(base, "-")
}

// storeDocToDBDoc converts a store.Doc (file-layer view) to a db.Doc (index view).
// The db.Doc.ID is the workspace-relative slash-normalised path.
//
// Fields that the store layer doesn't track (Project, Type, Status, Date, Author)
// are extracted from the document's YAML frontmatter when present; otherwise
// sensible zero-value defaults are used. The FTS Body is the full raw content.
func storeDocToDBDoc(s *store.Doc) *db.Doc {
	body := s.Content

	sum := sha256.Sum256([]byte(s.Content))
	hash := hex.EncodeToString(sum[:])

	id := filepath.ToSlash(s.Path)

	// Pull well-known frontmatter fields if present.
	strField := func(key string) string {
		if v, ok := s.Metadata[key]; ok {
			if str, ok := v.(string); ok {
				return str
			}
		}
		return ""
	}

	var tags []string
	if v, ok := s.Metadata["tags"]; ok {
		switch tv := v.(type) {
		case []interface{}:
			for _, item := range tv {
				if str, ok := item.(string); ok {
					tags = append(tags, str)
				}
			}
		case []string:
			tags = tv
		}
	}

	// Serialise the entire metadata map as JSON for raw_frontmatter storage.
	rawFM := ""
	if len(s.Metadata) > 0 {
		if b, err := json.Marshal(s.Metadata); err == nil {
			rawFM = string(b)
		}
	}

	// Slug: prefer frontmatter, else derive from filename. Index-only — we
	// never write this back to the Markdown file.
	slug := strField("slug")
	if slug == "" {
		slug = slugFromFilename(filepath.Base(s.Path))
	}

	title := strField("title")
	if title == "" {
		// Fall back to filename stem.
		base := filepath.Base(s.Path)
		title = strings.TrimSuffix(base, filepath.Ext(base))
	}

	return &db.Doc{
		ID:             id,
		Project:        strField("project"),
		Slug:           slug,
		Title:          title,
		Type:           strField("type"),
		Status:         strField("status"),
		Date:           strField("date"),
		Tags:           tags,
		Author:         strField("author"),
		ContentHash:    hash,
		ModTime:        s.ModTime.UTC().Format(time.RFC3339),
		Size:           s.Size,
		RawFrontmatter: rawFM,
		Body:           body,
	}
}
