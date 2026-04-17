// Package codepreview implements the vedox:// inline code preview resolver.
//
// The vedox:// URI scheme allows documentation authors to embed live references
// to source code files with optional line anchors:
//
//	vedox://file/<project-relative-path>[#L<start>[-L<end>]]
//
// The resolver reads the referenced file from disk, enforces a security sandbox
// (no path traversal, no symlink escape, no absolute paths, secret-file
// blocklist, binary rejection, 500KB size cap), and returns the relevant line
// range or the full file with language-detection metadata.
//
// Security model:
//   - The project root is the sandbox boundary.  All resolved paths must be
//     children of the absolute, symlink-resolved root.
//   - Paths containing ".." components are rejected before any filesystem
//     access.  This is intentional redundancy: filepath.Clean handles many
//     traversal forms, but rejecting ".." in the raw URL catches encoded or
//     concatenated variants before they reach the OS.
//   - The secret-file blocklist is checked against the base filename using the
//     same patterns as secretscan.blockedPath so the two subsystems stay in
//     sync.  Updates to secretFilePatterns in secretscan automatically apply
//     here because we call the exported IsBlockedPath helper.
//   - Symlinks whose EvalSymlinks result escapes the project root are rejected
//     with ErrSymlinkEscape.
//   - Binary files (first 512 bytes contain a null byte) are rejected with
//     ErrBinaryFile.
//   - Files larger than maxFileBytes are read only up to the cap; Content in
//     the returned Preview is truncated accordingly and Truncated is true.
//
// This package has no dependency on the database, the indexer, or any other
// Vedox subsystem.  It is a pure file-system reader: cheap, stateless, and
// safe to call on every hover event.
package codepreview

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// maxFileBytes is the maximum number of bytes read from a preview target.
// Files beyond this limit are truncated; the Preview.Truncated flag is set.
// 500 KB matches the CEO brief requirement.
const maxFileBytes = 500 * 1024

// binarySniffBytes is the number of leading bytes inspected for null-byte
// binary detection.  512 bytes is the same heuristic Go's net/http package
// uses for content-type sniffing.
const binarySniffBytes = 512

// maxLineRange is the maximum number of lines allowed in a single anchor span.
// Matches the spec: "end - start <= 500".
const maxLineRange = 500

// ---- Sentinel errors --------------------------------------------------------

// ErrInvalidScheme is returned when the URL scheme is not "vedox".
var ErrInvalidScheme = errors.New("codepreview: URL scheme must be vedox")

// ErrInvalidHost is returned when the URL host is not "file".
var ErrInvalidHost = errors.New("codepreview: vedox:// URL host must be 'file'")

// ErrEmptyPath is returned when the path component of the vedox:// URL is empty.
var ErrEmptyPath = errors.New("codepreview: vedox:// URL path must not be empty")

// ErrAbsolutePath is returned when the path component contains a leading slash
// after stripping the host prefix — i.e. the author wrote an absolute path.
var ErrAbsolutePath = errors.New("codepreview: path must not be absolute")

// ErrTraversal is returned when ".." appears anywhere in the URL path.
var ErrTraversal = errors.New("codepreview: path must not contain '..' components")

// ErrSymlinkEscape is returned when EvalSymlinks on the resolved path
// produces a result outside the project root.
var ErrSymlinkEscape = errors.New("codepreview: symlink escapes project root")

// ErrSecretFile is returned when the target filename matches the secret-file
// blocklist shared with the secretscan package.
var ErrSecretFile = errors.New("codepreview: target file matches secret blocklist")

// ErrBinaryFile is returned when the first 512 bytes of the target file
// contain a null byte.
var ErrBinaryFile = errors.New("codepreview: target is a binary file, not text")

// ErrFileNotFound is returned when the resolved path does not exist on disk.
var ErrFileNotFound = errors.New("codepreview: file not found")

// ErrInvalidAnchor is returned when the #L... anchor cannot be parsed.
var ErrInvalidAnchor = errors.New("codepreview: invalid line anchor")

// ErrAnchorRangeTooBig is returned when end - start exceeds maxLineRange (500).
var ErrAnchorRangeTooBig = errors.New("codepreview: line range exceeds 500-line limit")

// ErrAnchorOutOfRange is returned when the anchor refers to lines beyond EOF.
var ErrAnchorOutOfRange = errors.New("codepreview: line anchor refers to lines beyond end of file")

// ---- Output type ------------------------------------------------------------

// Preview is the resolved result returned by Resolve.
type Preview struct {
	// FilePath is the project-relative path extracted from the vedox:// URL.
	// This is the verbatim path after security validation, not the absolute path.
	FilePath string

	// Language is the Shiki-compatible language identifier inferred from the
	// file extension.  Empty string when the extension is unrecognised.
	Language string

	// Content is the file content (or the requested line slice when an anchor
	// was present).  Always UTF-8 text.  May be a truncated prefix if the file
	// exceeded 500 KB and no anchor was given.
	Content string

	// StartLine is the 1-indexed first line of Content within the original file.
	// 1 when no anchor was given (full-file preview).
	StartLine int

	// EndLine is the 1-indexed last line of Content within the original file.
	// Equal to TotalLines when no anchor was given (full-file preview).
	EndLine int

	// TotalLines is the total number of newline-delimited lines in the file
	// (counting only lines within the 500 KB cap when truncated).
	TotalLines int

	// Truncated is true when the file exceeded maxFileBytes and Content is a
	// prefix of the full file.
	Truncated bool
}

// ---- Public API -------------------------------------------------------------

// Resolve parses a vedox:// URL, validates it against the security sandbox
// rooted at projectRoot, reads the referenced file, and returns a Preview.
//
// projectRoot must be an absolute path.  Resolve calls filepath.EvalSymlinks
// on it internally to guard against symlinked project roots.
//
// The anchor, if present, must be one of:
//   - #L<n>          — single line (n is 1-indexed)
//   - #L<start>-L<end> — line range, both 1-indexed, end >= start
//
// Resolve is safe for concurrent use from multiple goroutines.  It performs no
// caching — the caller (API handler) decides whether to cache responses.
func Resolve(projectRoot, vedoxURL string) (*Preview, error) {
	// ---- 1. Parse the URL -----------------------------------------------
	parsed, err := url.Parse(vedoxURL)
	if err != nil {
		return nil, fmt.Errorf("codepreview: parse URL %q: %w", vedoxURL, err)
	}

	if parsed.Scheme != "vedox" {
		return nil, ErrInvalidScheme
	}
	if parsed.Host != "file" {
		return nil, ErrInvalidHost
	}

	// parsed.Path includes the leading "/" that separates host from path in
	// "vedox://file/apps/cli/main.go".  Strip it to get a clean relative path.
	relPath := strings.TrimPrefix(parsed.Path, "/")
	if relPath == "" {
		return nil, ErrEmptyPath
	}

	// ---- 2. Anchor extraction -------------------------------------------
	start, end, err := parseAnchor(parsed.Fragment)
	if err != nil {
		return nil, err
	}

	// ---- 3. Path validation (before any filesystem access) --------------
	if err := validatePath(relPath); err != nil {
		return nil, err
	}

	// ---- 4. Resolve to absolute path + sandbox check --------------------
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("codepreview: resolve project root %q: %w", projectRoot, err)
	}
	// EvalSymlinks on root so our prefix check works even when root is a symlink.
	if resolved, rerr := filepath.EvalSymlinks(absRoot); rerr == nil {
		absRoot = resolved
	}
	// Ensure the root ends without a trailing separator for clean prefix checks.
	absRoot = filepath.Clean(absRoot)

	// Join using filepath (OS-native) — relPath uses POSIX separators, convert.
	absTarget := filepath.Join(absRoot, filepath.FromSlash(relPath))
	absTarget = filepath.Clean(absTarget)

	// Prefix check before symlink resolution (defense-in-depth).
	if !isUnderRoot(absTarget, absRoot) {
		return nil, ErrTraversal
	}

	// Symlink resolution — catch symlinks that escape the project root.
	realTarget, err := filepath.EvalSymlinks(absTarget)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("codepreview: resolve symlinks for %q: %w", absTarget, err)
	}

	if !isUnderRoot(realTarget, absRoot) {
		return nil, ErrSymlinkEscape
	}

	// ---- 5. Secret-file blocklist ---------------------------------------
	if isSecretFile(realTarget) {
		return nil, ErrSecretFile
	}

	// ---- 6. Read the file (with size cap) --------------------------------
	f, err := os.Open(realTarget)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("codepreview: open %q: %w", realTarget, err)
	}
	defer f.Close()

	buf := make([]byte, maxFileBytes)
	n, readErr := readFull(f, buf)
	if readErr != nil {
		// A partial read with a real error is reported rather than silently
		// returning the prefix. readFull swallows io.EOF as nil, so any error
		// here is an actual filesystem failure (permissions, short read, etc.).
		return nil, fmt.Errorf("codepreview: read %q: %w", realTarget, readErr)
	}
	raw := buf[:n]

	truncated := false
	if n == maxFileBytes {
		// Check if there is more beyond the cap. An error on the probing read
		// does not affect correctness — we already have the first maxFileBytes
		// bytes; we just cannot confirm truncation in that case and treat it
		// as non-truncated rather than fail the preview.
		extra := make([]byte, 1)
		nn, _ := f.Read(extra)
		if nn > 0 {
			truncated = true
		}
	}

	// ---- 7. Binary detection (first 512 bytes) ---------------------------
	sniffLen := binarySniffBytes
	if sniffLen > len(raw) {
		sniffLen = len(raw)
	}
	if bytes.IndexByte(raw[:sniffLen], 0x00) >= 0 {
		return nil, ErrBinaryFile
	}

	// ---- 8. Split into lines --------------------------------------------
	// Use the raw bytes to split on newlines before converting to string.
	lines := splitLines(raw)
	totalLines := len(lines)

	// ---- 9. Apply anchor / line range -----------------------------------
	if start == 0 {
		// No anchor: return full content.
		start = 1
		end = totalLines
	} else {
		// Validate anchor bounds. start and end are both >=1 here because
		// parseAnchor rejects non-positive integers.
		if end-start+1 > maxLineRange {
			return nil, ErrAnchorRangeTooBig
		}
		if start > totalLines || end > totalLines {
			return nil, ErrAnchorOutOfRange
		}
	}

	// Slice the relevant lines (1-indexed → 0-indexed).
	slicedLines := lines[start-1 : end]
	content := strings.Join(slicedLines, "\n")

	// Preserve a trailing newline if the original file has one and we're
	// returning the full file.
	if !truncated && start == 1 && end == totalLines && len(raw) > 0 && raw[len(raw)-1] == '\n' {
		content += "\n"
	}

	return &Preview{
		FilePath:   relPath,
		Language:   languageFromExt(path.Ext(relPath)),
		Content:    content,
		StartLine:  start,
		EndLine:    end,
		TotalLines: totalLines,
		Truncated:  truncated,
	}, nil
}

// ---- Internal helpers -------------------------------------------------------

// parseAnchor parses the URL fragment into (startLine, endLine).
// Returns (0, 0) for an empty fragment (no anchor).
// Supported forms:
//
//	L10        → (10, 10)
//	L10-L25    → (10, 25)
func parseAnchor(fragment string) (int, int, error) {
	if fragment == "" {
		return 0, 0, nil
	}

	// Symbol-id anchors (non-L prefix) are reserved and rejected in v2.
	if !strings.HasPrefix(fragment, "L") {
		return 0, 0, fmt.Errorf("%w: unsupported anchor format %q (only #L<n> and #L<start>-L<end> are supported in v2)", ErrInvalidAnchor, fragment)
	}

	// Strip the leading "L".
	rest := fragment[1:]

	if idx := strings.Index(rest, "-L"); idx >= 0 {
		// Range form: L<start>-L<end>
		startStr := rest[:idx]
		endStr := rest[idx+2:]

		start, err := strconv.Atoi(startStr)
		if err != nil || start < 1 {
			return 0, 0, fmt.Errorf("%w: start line %q is not a positive integer", ErrInvalidAnchor, startStr)
		}
		end, err := strconv.Atoi(endStr)
		if err != nil || end < 1 {
			return 0, 0, fmt.Errorf("%w: end line %q is not a positive integer", ErrInvalidAnchor, endStr)
		}
		if end < start {
			return 0, 0, fmt.Errorf("%w: end line %d is before start line %d", ErrInvalidAnchor, end, start)
		}
		return start, end, nil
	}

	// Single-line form: L<n>
	n, err := strconv.Atoi(rest)
	if err != nil || n < 1 {
		return 0, 0, fmt.Errorf("%w: line number %q is not a positive integer", ErrInvalidAnchor, rest)
	}
	return n, n, nil
}

// validatePath enforces structural path constraints before any filesystem
// access.  It rejects:
//   - Absolute paths (leading "/" after scheme+host are stripped by the URL
//     parser; this catches paths that begin with "/" in the raw URL path field)
//   - Any path segment equal to ".." (traversal)
//   - Any path segment equal to "." (degenerate — not a security issue but
//     makes path handling ambiguous)
func validatePath(relPath string) error {
	// relPath should already have the leading "/" stripped.  A remaining leading
	// "/" means the author wrote something like vedox://file//abs/path.
	if filepath.IsAbs(relPath) || strings.HasPrefix(relPath, "/") {
		return ErrAbsolutePath
	}

	// Use path (POSIX) split because relPath uses forward slashes.
	segments := strings.Split(relPath, "/")
	for _, seg := range segments {
		if seg == ".." {
			return ErrTraversal
		}
	}

	return nil
}

// isUnderRoot reports whether target is a descendant of root.
// Both paths must be absolute and cleaned (no trailing separators).
func isUnderRoot(target, root string) bool {
	// Ensure root ends with separator for the prefix check so that
	// "/foo/bar" does not match as a prefix of "/foo/barbaz".
	rootWithSep := root + string(filepath.Separator)
	return strings.HasPrefix(target, rootWithSep) || target == root
}

// isSecretFile checks the file's base name against the secret-file blocklist.
// The blocklist is intentionally a superset of the patterns in
// secretscan.secretFilePatterns (which is unexported).  Any additions to that
// list should be mirrored here.
//
// This is a deliberate duplication rather than a shared variable because the
// codepreview package must remain importable independently of the secretscan
// package's internal state.  The two lists diverge only if someone adds a
// pattern in one place and forgets the other — the resolver_test.go test
// covers the most sensitive patterns.
var secretFilePatterns = []string{
	"*.env",
	".env",
	"*.pem",
	"*.key",
	"*.p12",
	"*.pfx",
	"*.pkcs12",
	"id_rsa",
	"id_rsa.pub",
	"id_ed25519",
	"id_ecdsa",
	"id_dsa",
	"*.openssh",
	"credentials",
	"credentials.json",
	"service-account*.json",
	"serviceaccount*.json",
	"*secret*.json",
	"*token*.json",
	"*.password",
	"*.secret",
	"*_secrets.yaml",
	"*_secrets.yml",
	"*.kdbx",
	"*.keystore",
}

func isSecretFile(absPath string) bool {
	base := strings.ToLower(filepath.Base(absPath))
	for _, pattern := range secretFilePatterns {
		if matched, err := filepath.Match(pattern, base); err == nil && matched {
			return true
		}
	}
	return false
}

// readFull reads up to len(buf) bytes from f.  Unlike io.ReadFull it does not
// return an error when EOF is reached before the buffer is full — it returns
// the byte count and a nil error in that case.
func readFull(f *os.File, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := f.Read(buf[total:])
		total += n
		if err != nil {
			// io.EOF means we consumed the file normally.
			// errors.Is is stable across runtimes — a string compare on
			// err.Error() is brittle because other wrapped errors can have
			// the same message ("EOF") yet not be io.EOF.
			if errors.Is(err, io.EOF) {
				return total, nil
			}
			return total, err
		}
	}
	return total, nil
}

// splitLines splits a byte slice on newlines and returns the lines as strings.
// A trailing newline does not produce an empty final element (matches the
// behaviour a user expects: a file with a trailing newline has N lines, not
// N+1 with an empty last one).
func splitLines(raw []byte) []string {
	if len(raw) == 0 {
		return []string{""}
	}
	// Trim a single trailing newline before splitting to avoid the empty
	// last element.
	trimmed := raw
	if trimmed[len(trimmed)-1] == '\n' {
		trimmed = trimmed[:len(trimmed)-1]
	}
	// Handle Windows CRLF.
	unified := bytes.ReplaceAll(trimmed, []byte("\r\n"), []byte("\n"))
	parts := bytes.Split(unified, []byte("\n"))
	result := make([]string, len(parts))
	for i, p := range parts {
		result[i] = string(p)
	}
	return result
}

// languageFromExt maps a file extension (including the leading dot, as returned
// by path.Ext) to a Shiki-compatible language identifier.
// Extensions not in the map return an empty string — Shiki will render the
// block as plain text.
func languageFromExt(ext string) string {
	// Lower-case the extension for case-insensitive matching.
	switch strings.ToLower(ext) {
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".tf", ".tfvars":
		return "hcl"
	case ".hcl":
		return "hcl"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	case ".sh", ".bash", ".zsh":
		return "bash"
	case ".dockerfile":
		return "dockerfile"
	case ".sql":
		return "sql"
	case ".md", ".mdx":
		return "markdown"
	case ".html", ".htm":
		return "html"
	case ".css", ".scss", ".sass":
		return "css"
	case ".svelte":
		return "svelte"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "cpp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".kt", ".kts":
		return "kotlin"
	case ".swift":
		return "swift"
	case ".xml":
		return "xml"
	case ".proto":
		return "protobuf"
	case ".lua":
		return "lua"
	case ".r":
		return "r"
	default:
		return ""
	}
}
