// Package importer implements the Import & Migrate flow for Vedox Phase 2.
//
// Import copies Markdown documents from an external project root into the
// Vedox workspace and indexes them in SQLite. It is deliberately kept
// separate from the DocStore abstraction: the *source* side performs raw OS
// reads (the source repo is not a Vedox workspace), while the *destination*
// side goes through store.DocStore for atomic writes and the secret blocklist.
//
// Security model
//   - srcProjectRoot is verified to be an absolute path that exists.
//   - srcProjectRoot must not be inside destWorkspaceRoot (no self-import).
//   - Every derived destination path is checked via the store's own safePath
//     logic (path traversal + secret blocklist) by calling store.Write.
//   - Source files that match the secret blocklist are silently skipped
//     (same patterns as LocalAdapter) — their existence is not returned to
//     callers.
//   - File contents from the source are never written to logs.
package importer

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
	"strings"
	"time"

	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/store"
)

// secretBlocklist mirrors the patterns in store.LocalAdapter. We duplicate
// rather than export the private slice from the store package to keep the
// importer self-contained and avoid coupling to unexported internals.
// Patterns use filepath.Match (glob), matched against the base filename only.
var secretBlocklist = []string{
	".env",
	"*.pem",
	"*.key",
	"id_rsa",
	"*.p12",
	"credentials.json",
}

// skipDirs lists directory names that are always excluded during source walks.
// Mirrors the exclusions in scanner.countMarkdownFiles.
var skipDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
}

// ImportResult is the outcome of a single Import call.
type ImportResult struct {
	// Imported holds the workspace-relative destination paths of every file
	// successfully written and indexed (e.g. "my-project/docs/adr-001.md").
	Imported []string `json:"imported"`

	// Skipped holds source-relative paths of files that were not imported.
	// This covers: non-.md files are not included (we only walk .md), and
	// files that matched the secret blocklist. The reason is embedded as a
	// parenthetical so the UI can display it without a separate map.
	Skipped []string `json:"skipped"`

	// Warnings contains advisory messages the user should act on. Always
	// includes the Git removal reminder when at least one file was imported.
	Warnings []string `json:"warnings"`
}

// Import walks srcProjectRoot for .md files, copies each one into the Vedox
// workspace under destWorkspaceRoot/<projectName>/<relPath>, indexes each doc
// via db.UpsertDoc, and returns a summary.
//
// Parameters:
//   - srcProjectRoot: absolute path to the source project. Must exist and must
//     not be inside destWorkspaceRoot.
//   - projectName: the sub-directory name under destWorkspaceRoot to write
//     files into. Treated as a single path segment (no slashes allowed).
//   - destWorkspaceRoot: absolute path to the Vedox workspace root.
//   - docStore: the Vedox DocStore for atomic destination writes.
//   - dbStore: the metadata/FTS index store for indexing imported docs.
func Import(
	srcProjectRoot string,
	projectName string,
	destWorkspaceRoot string,
	docStore store.DocStore,
	dbStore *db.Store,
) (*ImportResult, error) {
	// Validate srcProjectRoot: must be absolute and must exist.
	if !filepath.IsAbs(srcProjectRoot) {
		return nil, fmt.Errorf("importer: srcProjectRoot must be an absolute path, got %q", srcProjectRoot)
	}
	srcClean := filepath.Clean(srcProjectRoot)
	if _, err := os.Stat(srcClean); err != nil {
		return nil, fmt.Errorf("importer: srcProjectRoot %q does not exist: %w", srcProjectRoot, err)
	}

	// projectName must be a single path segment — no slashes.
	if strings.ContainsAny(projectName, `/\`) || projectName == "" || projectName == "." || projectName == ".." {
		return nil, fmt.Errorf("importer: projectName must be a single non-empty path segment, got %q", projectName)
	}

	// Validate destWorkspaceRoot.
	if !filepath.IsAbs(destWorkspaceRoot) {
		return nil, fmt.Errorf("importer: destWorkspaceRoot must be an absolute path, got %q", destWorkspaceRoot)
	}
	destClean := filepath.Clean(destWorkspaceRoot)

	// Self-import guard: srcProjectRoot must not be inside destWorkspaceRoot.
	// We resolve symlinks on both sides so symlink tricks don't bypass the check.
	srcReal, err := filepath.EvalSymlinks(srcClean)
	if err != nil {
		// If EvalSymlinks fails the path probably doesn't exist; Stat above
		// already checked existence, so this shouldn't happen — but be safe.
		srcReal = srcClean
	}
	destReal, err := filepath.EvalSymlinks(destClean)
	if err != nil {
		destReal = destClean
	}
	destWithSep := destReal + string(os.PathSeparator)
	if srcReal == destReal || strings.HasPrefix(srcReal, destWithSep) {
		return nil, fmt.Errorf("importer: srcProjectRoot is inside the Vedox workspace — cannot import a workspace into itself")
	}

	result := &ImportResult{
		Imported: []string{},
		Skipped:  []string{},
		Warnings: []string{},
	}

	// Walk the source directory tree collecting .md files.
	walkErr := filepath.WalkDir(srcClean, func(absPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Log unreadable entries and skip rather than aborting.
			slog.Warn("importer: walk error, skipping entry",
				slog.String("path", absPath),
				slog.String("error", walkErr.Error()),
			)
			return nil
		}

		if d.IsDir() {
			name := d.Name()
			// Skip hidden directories and known large dep dirs.
			if (len(name) > 0 && name[0] == '.') || skipDirs[name] {
				return fs.SkipDir
			}
			return nil
		}

		// We only handle regular .md files.
		if !d.Type().IsRegular() || filepath.Ext(d.Name()) != ".md" {
			return nil
		}

		// Secret blocklist check on the source base filename.
		base := d.Name()
		if isSecretFile(base) {
			relSrc, _ := filepath.Rel(srcClean, absPath)
			slog.Warn("importer: skipping secret-blocked source file",
				slog.String("code", "VDX-006"),
				slog.String("path", relSrc),
			)
			result.Skipped = append(result.Skipped, relSrc+" (secret-blocked)")
			return nil
		}

		// Compute the source-relative path for use as the destination sub-path.
		relSrc, relErr := filepath.Rel(srcClean, absPath)
		if relErr != nil {
			slog.Warn("importer: cannot compute rel path, skipping",
				slog.String("abs", absPath),
				slog.String("error", relErr.Error()),
			)
			return nil
		}

		// Stat the source file to capture modTime and size for indexing.
		// d.Info() re-uses the DirEntry's cached info without an extra syscall.
		fileInfo, infoErr := d.Info()
		if infoErr != nil {
			slog.Warn("importer: cannot stat source file, skipping",
				slog.String("path", relSrc),
				slog.String("error", infoErr.Error()),
			)
			result.Skipped = append(result.Skipped, relSrc+" (stat error)")
			return nil
		}

		// Read source file directly from OS (not through the store — the source
		// is an external path, not a Vedox workspace).
		raw, readErr := os.ReadFile(absPath)
		if readErr != nil {
			slog.Warn("importer: cannot read source file, skipping",
				slog.String("path", relSrc),
				slog.String("error", readErr.Error()),
			)
			result.Skipped = append(result.Skipped, relSrc+" (read error)")
			return nil
		}

		// Build the destination path: <projectName>/<relSrc>
		// filepath.Join keeps it clean and avoids double-slash.
		destRelPath := filepath.Join(projectName, relSrc)

		// Write through DocStore — this enforces atomic writes, path traversal
		// protection, and the secret blocklist on the destination side.
		if writeErr := docStore.Write(destRelPath, string(raw)); writeErr != nil {
			slog.Warn("importer: store.Write failed, skipping",
				slog.String("dest", destRelPath),
				slog.String("error", writeErr.Error()),
			)
			result.Skipped = append(result.Skipped, relSrc+" (write failed: "+writeErr.Error()+")")
			return nil
		}

		// Index the document in SQLite.
		dbDoc := buildDBDoc(destRelPath, projectName, string(raw), fileInfo.ModTime(), fileInfo.Size())
		if upsertErr := dbStore.UpsertDoc(context.Background(), dbDoc); upsertErr != nil {
			// Indexing failure is non-fatal: the file is already on disk and the
			// user can run `vedox reindex` to recover. We warn rather than abort.
			slog.Warn("importer: db.UpsertDoc failed, file written but not indexed",
				slog.String("path", destRelPath),
				slog.String("error", upsertErr.Error()),
			)
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("File %s was copied but could not be indexed — run `vedox reindex` to fix.", destRelPath),
			)
		}

		result.Imported = append(result.Imported, destRelPath)
		slog.Info("importer: imported file",
			slog.String("src", relSrc),
			slog.String("dest", destRelPath),
		)
		return nil
	})

	if walkErr != nil {
		return nil, fmt.Errorf("importer: walk %s: %w", srcClean, walkErr)
	}

	// Add Git removal reminder only when files were actually imported.
	if len(result.Imported) > 0 {
		// Build a flat list of source-relative paths for the git rm command.
		srcRelPaths := make([]string, 0, len(result.Imported))
		for _, dest := range result.Imported {
			// Strip the leading projectName/ prefix to recover the original src-relative path.
			srcRel := strings.TrimPrefix(dest, projectName+string(os.PathSeparator))
			srcRelPaths = append(srcRelPaths, srcRel)
		}
		gitCmd := fmt.Sprintf(
			"git -C %s rm %s && git -C %s commit -m 'chore: migrate docs to Vedox'",
			srcProjectRoot,
			strings.Join(srcRelPaths, " "),
			srcProjectRoot,
		)
		result.Warnings = append(result.Warnings,
			fmt.Sprintf(
				"TITLE: Migration cleanup required\nBODY: Your documents have been successfully copied into the Vedox workspace. To complete the migration and maintain a single source of truth, you should now remove the original files from the source repository.\n\nRun this command in your terminal to commit the removal:\nCOMMAND: %s",
				gitCmd,
			),
		)
	}

	return result, nil
}

// isSecretFile reports whether the given base filename matches any pattern in
// the secret blocklist. Mirrors LocalAdapter.isSecretFile.
func isSecretFile(name string) bool {
	for _, pattern := range secretBlocklist {
		matched, err := filepath.Match(pattern, name)
		if err != nil {
			// Malformed pattern — fail safe by treating as a match.
			return true
		}
		if matched {
			return true
		}
	}
	return false
}

// buildDBDoc constructs a db.Doc suitable for UpsertDoc from a raw Markdown
// string and the file's stat info. It does a lightweight frontmatter extraction
// for title/type/status fields; anything not found falls back to safe defaults.
// The full-text body is the raw file content (FTS5 handles tokenisation).
//
// This mirrors the logic in indexer.storeDocToDBDoc but operates on raw bytes
// rather than a store.Doc, because the source file is outside the workspace.
func buildDBDoc(destRelPath, project, rawContent string, modTime time.Time, size int64) *db.Doc {
	// Extract frontmatter key/value pairs with a simple line scan.
	// We intentionally avoid importing the full YAML parser here to keep the
	// dependency surface of the importer package minimal; the store package
	// already pulls in gopkg.in/yaml.v3 but we don't want to couple importer
	// to store internals. A line-based scan is sufficient for the fields we need.
	fm := extractSimpleFrontmatter(rawContent)

	title := stringVal(fm, "title")
	if title == "" {
		// Fall back to the filename stem.
		base := filepath.Base(destRelPath)
		title = strings.TrimSuffix(base, filepath.Ext(base))
	}

	docType := stringVal(fm, "type")
	if docType == "" {
		docType = "readme"
	}

	status := stringVal(fm, "status")
	if status == "" {
		status = "draft"
	}

	date := stringVal(fm, "date")
	author := stringVal(fm, "author")

	// Build a JSON-encoded frontmatter blob so raw_frontmatter matches the
	// format written by indexer.storeDocToDBDoc (JSON object, not raw YAML).
	rawFM := ""
	if len(fm) > 0 {
		// Convert map[string]string to map[string]interface{} for json.Marshal.
		fmIface := make(map[string]interface{}, len(fm))
		for k, v := range fm {
			fmIface[k] = v
		}
		if b, err := json.Marshal(fmIface); err == nil {
			rawFM = string(b)
		}
	}

	// SHA-256 content hash, consistent with indexer.storeDocToDBDoc.
	sum := sha256.Sum256([]byte(rawContent))
	hash := hex.EncodeToString(sum[:])

	return &db.Doc{
		ID:             filepath.ToSlash(destRelPath),
		Project:        project,
		Title:          title,
		Type:           docType,
		Status:         status,
		Date:           date,
		Author:         author,
		ContentHash:    hash,
		ModTime:        modTime.UTC().Format(time.RFC3339),
		Size:           size,
		RawFrontmatter: rawFM,
		Body:           rawContent,
	}
}

// extractSimpleFrontmatter does a minimal line-scan of a YAML frontmatter
// block (delimited by "---") and returns a map of key → value strings.
// Only scalar string values on lines of the form "key: value" are captured.
// Complex YAML (nested maps, lists) is ignored — the full reindex path handles
// those correctly.
func extractSimpleFrontmatter(content string) map[string]string {
	result := make(map[string]string)

	if !strings.HasPrefix(content, "---\n") {
		return result
	}

	rest := content[4:] // skip "---\n"
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return result
	}

	block := rest[:end]
	for _, line := range strings.Split(block, "\n") {
		idx := strings.IndexByte(line, ':')
		if idx < 1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Strip inline YAML quotes.
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if key != "" && val != "" {
			result[key] = val
		}
	}
	return result
}

func stringVal(m map[string]string, key string) string {
	if m == nil {
		return ""
	}
	return m[key]
}
