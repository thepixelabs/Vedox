// Package scanner — framework detection heuristics.
//
// DetectFramework identifies the documentation framework used by a project by
// checking for the presence of well-known config files using os.Stat (no file
// reads). Detection is intentionally cheap: one or two stat calls per project.
//
// Detection results are memoised in a package-level cache keyed on
// (projectRoot, configFileMtime). The cache is invalidated automatically when
// the relevant config file changes.
package scanner

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Framework is the canonical name for a detected documentation framework.
type Framework = string

const (
	FrameworkAstro     Framework = "astro"
	FrameworkMkDocs    Framework = "mkdocs"
	FrameworkJekyll    Framework = "jekyll"
	FrameworkDocusaurus Framework = "docusaurus"
	FrameworkBare      Framework = "bare"
	FrameworkUnknown   Framework = "unknown"
)

// detectCacheKey combines a project root path and the mtime of the signal file
// that was used to identify the framework. If that file changes, the key
// changes and the cached result is implicitly bypassed.
type detectCacheKey struct {
	root  string
	mtime time.Time
}

var (
	detectMu    sync.Mutex
	detectCache = make(map[detectCacheKey]Framework)
)

// DetectFramework returns the documentation framework for the given project
// root directory. It performs only os.Stat calls — no file reads.
//
// Detection priority (first match wins):
//  1. astro.config.mjs or astro.config.ts  → "astro"
//  2. mkdocs.yml                            → "mkdocs"
//  3. _config.yml + Gemfile                → "jekyll"
//  4. .docusaurus/ directory               → "docusaurus"
//  5. README.md only (no other framework)  → "bare"
//  6. (nothing matched)                    → "unknown"
func DetectFramework(projectRoot string) Framework {
	// --- 1. Astro ---
	if f, mtime, ok := firstExisting(projectRoot, "astro.config.mjs", "astro.config.ts"); ok {
		return cachedFramework(detectCacheKey{root: projectRoot, mtime: mtime}, f, FrameworkAstro)
	}

	// --- 2. MkDocs ---
	if f, mtime, ok := firstExisting(projectRoot, "mkdocs.yml"); ok {
		return cachedFramework(detectCacheKey{root: projectRoot, mtime: mtime}, f, FrameworkMkDocs)
	}

	// --- 3. Jekyll (requires both _config.yml AND Gemfile) ---
	configInfo := statFile(filepath.Join(projectRoot, "_config.yml"))
	gemfileInfo := statFile(filepath.Join(projectRoot, "Gemfile"))
	if configInfo != nil && gemfileInfo != nil {
		// Key on the older of the two mtimes for cache stability.
		mtime := configInfo.ModTime().UTC()
		if t := gemfileInfo.ModTime().UTC(); t.Before(mtime) {
			mtime = t
		}
		return cachedFramework(detectCacheKey{root: projectRoot, mtime: mtime}, "", FrameworkJekyll)
	}

	// --- 4. Docusaurus ---
	if docuInfo := statFile(filepath.Join(projectRoot, ".docusaurus")); docuInfo != nil {
		mtime := docuInfo.ModTime().UTC()
		return cachedFramework(detectCacheKey{root: projectRoot, mtime: mtime}, "", FrameworkDocusaurus)
	}

	// --- 5. Bare (README.md present, no other framework detected) ---
	if readmeInfo := statFile(filepath.Join(projectRoot, "README.md")); readmeInfo != nil {
		mtime := readmeInfo.ModTime().UTC()
		return cachedFramework(detectCacheKey{root: projectRoot, mtime: mtime}, "", FrameworkBare)
	}

	return FrameworkUnknown
}

// firstExisting returns the name and mtime of the first file in names that
// exists in projectRoot. ok is false if none exist.
func firstExisting(projectRoot string, names ...string) (name string, mtime time.Time, ok bool) {
	for _, n := range names {
		if info := statFile(filepath.Join(projectRoot, n)); info != nil {
			return n, info.ModTime().UTC(), true
		}
	}
	return "", time.Time{}, false
}

// statFile calls os.Stat and returns the FileInfo, or nil on any error.
func statFile(path string) os.FileInfo {
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	return info
}

// cachedFramework returns the cached Framework for key if it exists; otherwise
// it stores result and returns it. name is unused at the moment but retained
// for future diagnostic logging.
func cachedFramework(key detectCacheKey, _ string, result Framework) Framework {
	detectMu.Lock()
	defer detectMu.Unlock()

	if cached, ok := detectCache[key]; ok {
		return cached
	}
	detectCache[key] = result
	return result
}
