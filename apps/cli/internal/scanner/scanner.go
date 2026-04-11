// Package scanner implements the Vedox workspace scanner.
//
// The scanner walks a directory tree looking for Git project roots (directories
// that contain a ".git" entry). For each discovered project it runs framework
// detection (see detect.go) and counts Markdown documents.
//
// Results are cached to <workspaceRoot>/.vedox/scan-cache.json so that
// subsequent scans skip projects whose root directory mtime has not changed.
//
// Async scanning with progress reporting is handled by the JobStore in
// progress.go. The HTTP handlers in internal/api/scan.go use that interface.
package scanner

import (
	"encoding/json"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	// maxDepth is the maximum directory depth the scanner will descend into
	// relative to the workspace root. Directories deeper than this are skipped.
	// Depth 0 = workspace root itself; depth 1 = immediate children, etc.
	maxDepth = 5

	// cacheFile is the path relative to workspaceRoot where scan results are
	// persisted between runs.
	cacheFile = ".vedox/scan-cache.json"
)

// Project is the scanner's representation of a discovered Git project root.
type Project struct {
	// Name is the base directory name of the project.
	Name string `json:"name"`

	// AbsPath is the absolute path to the project root.
	AbsPath string `json:"absPath"`

	// RelPath is the path relative to the workspace root.
	RelPath string `json:"relPath"`

	// DocCount is the number of Markdown (.md) files found under this project.
	DocCount int `json:"docCount"`

	// DetectedFramework is the documentation framework detected for this project.
	// Possible values: "astro", "mkdocs", "jekyll", "docusaurus", "bare", "unknown".
	DetectedFramework string `json:"detectedFramework"`

	// LastScanned is the UTC timestamp when this project was last scanned.
	LastScanned time.Time `json:"lastScanned"`
}

// cacheEntry is one record inside scan-cache.json.
type cacheEntry struct {
	// DirMtime is the mtime of the project root directory at the time of the
	// last scan. If the directory's current mtime differs, the entry is stale.
	DirMtime time.Time `json:"dirMtime"`

	// Project is the cached result for this project root.
	Project *Project `json:"project"`
}

// scanCache maps absPath → cacheEntry.
type scanCache map[string]cacheEntry

// Scanner performs workspace scans. The zero value is ready to use.
// It is safe to call Scan from multiple goroutines; each call is independent.
type Scanner struct {
	// mu protects cacheByRoot so concurrent scans of the same workspace root
	// don't race on the in-memory cache map.
	mu           sync.Mutex
	cacheByRoot  map[string]scanCache // workspaceRoot → loaded cache
}

// NewScanner returns a new Scanner.
func NewScanner() *Scanner {
	return &Scanner{
		cacheByRoot: make(map[string]scanCache),
	}
}

// Scan walks workspaceRoot looking for Git project roots and returns them
// sorted by Name ascending.
//
// Excluded during the walk:
//   - Hidden directories (names starting with ".") except ".git" is checked
//     for existence but never recursed into.
//   - "node_modules" directories.
//   - "vendor" directories.
//   - Directories deeper than maxDepth levels below workspaceRoot.
//
// After the walk, results are persisted to cacheFile inside the workspace.
func (s *Scanner) Scan(workspaceRoot string) ([]*Project, error) {
	cache := s.loadCache(workspaceRoot)

	projects := make([]*Project, 0)

	err := walkDir(workspaceRoot, workspaceRoot, 0, func(absPath string, depth int) error {
		// Check whether this directory contains ".git".
		gitPath := filepath.Join(absPath, ".git")
		gitInfo, statErr := os.Stat(gitPath)
		if statErr != nil {
			// .git not present — let the walk descend into this directory.
			return nil
		}
		// .git exists; accept both a directory (.git dir) and a plain file
		// (.git file used by Git worktrees).
		if !gitInfo.IsDir() && !gitInfo.Mode().IsRegular() {
			// Something unusual — skip this path, don't treat it as a project.
			return nil
		}

		// We found a project root. Check the cache.
		dirInfo, dirErr := os.Stat(absPath)
		if dirErr != nil {
			slog.Warn("scanner: could not stat project dir, skipping",
				"path", absPath, "error", dirErr.Error())
			return fs.SkipDir
		}

		dirMtime := dirInfo.ModTime().UTC()
		if entry, ok := cache[absPath]; ok {
			if entry.DirMtime.Equal(dirMtime) {
				// Cache hit — return a copy so callers can't mutate the cache entry.
				cp := *entry.Project
				projects = append(projects, &cp)
				slog.Debug("scanner: cache hit", "path", absPath)
				// Don't recurse into this project's subdirectories.
				return fs.SkipDir
			}
		}

		// Cache miss — scan this project.
		relPath, relErr := filepath.Rel(workspaceRoot, absPath)
		if relErr != nil {
			relPath = absPath
		}

		framework := DetectFramework(absPath)
		docCount := countMarkdownFiles(absPath)

		p := &Project{
			Name:              filepath.Base(absPath),
			AbsPath:           absPath,
			RelPath:           relPath,
			DocCount:          docCount,
			DetectedFramework: framework,
			LastScanned:       time.Now().UTC(),
		}

		cache[absPath] = cacheEntry{
			DirMtime: dirMtime,
			Project:  p,
		}

		projects = append(projects, p)
		slog.Debug("scanner: found project", "name", p.Name, "framework", framework)

		// Don't recurse into the project's own subdirectories — the project
		// root is the unit of discovery, not individual files inside it.
		return fs.SkipDir
	})
	if err != nil {
		return nil, err
	}

	// Persist the updated cache.
	s.saveCache(workspaceRoot, cache)

	// Sort by Name ascending for deterministic output.
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return projects, nil
}

// walkDir is a recursive helper that calls fn for every directory entry under
// root, subject to depth and exclusion rules. fn may return fs.SkipDir to
// prevent recursion into that directory.
//
// depth is the current level below workspaceRoot (0 = workspaceRoot itself).
func walkDir(workspaceRoot, dir string, depth int, fn func(absPath string, depth int) error) error {
	if depth > maxDepth {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		// Log and skip unreadable directories rather than aborting the whole scan.
		slog.Warn("scanner: could not read directory", "path", dir, "error", err.Error())
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Skip hidden directories. We never need to descend into .git, .vedox,
		// .tasks, etc. We detect .git by its *presence* in the parent, not by
		// entering it.
		if len(name) > 0 && name[0] == '.' {
			continue
		}

		// Skip common large dependency/vendor directories.
		if name == "node_modules" || name == "vendor" {
			continue
		}

		absPath := filepath.Join(dir, name)

		// Call the visitor. If it returns fs.SkipDir, don't recurse.
		if visitErr := fn(absPath, depth+1); visitErr != nil {
			if visitErr == fs.SkipDir {
				continue
			}
			return visitErr
		}

		// Recurse.
		if recurseErr := walkDir(workspaceRoot, absPath, depth+1, fn); recurseErr != nil {
			return recurseErr
		}
	}

	return nil
}

// countMarkdownFiles returns the number of .md files found anywhere under root.
func countMarkdownFiles(root string) int {
	count := 0
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		// Skip large dependency dirs even during the count walk.
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == "vendor" {
				return fs.SkipDir
			}
			if len(name) > 0 && name[0] == '.' {
				return fs.SkipDir
			}
		}
		if !d.IsDir() && filepath.Ext(d.Name()) == ".md" {
			count++
		}
		return nil
	})
	return count
}

// --- Cache persistence ---

// loadCache reads the scan cache from disk for the given workspaceRoot.
// If the file doesn't exist or is unreadable, an empty cache is returned.
// The in-memory copy is kept in s.cacheByRoot to avoid redundant reads
// within a single process lifetime.
func (s *Scanner) loadCache(workspaceRoot string) scanCache {
	s.mu.Lock()
	defer s.mu.Unlock()

	if c, ok := s.cacheByRoot[workspaceRoot]; ok {
		// Return a shallow copy so the caller's mutations don't corrupt the
		// shared map until we explicitly write back in saveCache.
		copied := make(scanCache, len(c))
		for k, v := range c {
			copied[k] = v
		}
		return copied
	}

	path := filepath.Join(workspaceRoot, cacheFile)
	data, err := os.ReadFile(path)
	if err != nil {
		// Cache file absent or unreadable — start fresh.
		return make(scanCache)
	}

	var c scanCache
	if jsonErr := json.Unmarshal(data, &c); jsonErr != nil {
		slog.Warn("scanner: corrupt scan cache, starting fresh",
			"path", path, "error", jsonErr.Error())
		return make(scanCache)
	}

	return c
}

// saveCache writes cache to disk and updates the in-memory copy.
func (s *Scanner) saveCache(workspaceRoot string, cache scanCache) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update in-memory store.
	s.cacheByRoot[workspaceRoot] = cache

	cacheDir := filepath.Join(workspaceRoot, ".vedox")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		slog.Warn("scanner: could not create .vedox dir, cache not persisted",
			"error", err.Error())
		return
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		slog.Warn("scanner: could not marshal scan cache", "error", err.Error())
		return
	}

	path := filepath.Join(workspaceRoot, cacheFile)
	// Write atomically: temp file → fsync → rename.
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		slog.Warn("scanner: could not write scan cache temp file", "error", err.Error())
		return
	}

	if _, writeErr := f.Write(data); writeErr != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		slog.Warn("scanner: could not write scan cache data", "error", writeErr.Error())
		return
	}

	if syncErr := f.Sync(); syncErr != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		slog.Warn("scanner: could not fsync scan cache", "error", syncErr.Error())
		return
	}

	_ = f.Close()

	if renameErr := os.Rename(tmp, path); renameErr != nil {
		slog.Warn("scanner: could not rename scan cache temp file", "error", renameErr.Error())
	}
}
