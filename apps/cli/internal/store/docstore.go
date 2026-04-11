// Package store defines the DocStore abstraction for all file-system operations
// in Vedox. Implementations must satisfy the security requirements defined in
// EPIC-001: path traversal protection, secret file blocklist, atomic writes,
// and content-safe logging (paths only, never file contents).
package store

import "time"

// Doc is the in-memory representation of a Markdown document. Content holds
// the raw bytes as a string. Metadata contains any YAML frontmatter key/value
// pairs parsed from the leading "---" block; it is an empty (non-nil) map when
// no frontmatter is present.
type Doc struct {
	// Path is the workspace-relative path, e.g. "docs/architecture/adr-001.md".
	Path string

	// Content is the full raw text of the file, including frontmatter.
	Content string

	// Metadata contains parsed YAML frontmatter key/value pairs.
	// Always non-nil; empty when the file has no frontmatter block.
	Metadata map[string]interface{}

	// ModTime is the file's last-modified timestamp as reported by the OS.
	ModTime time.Time

	// Size is the file size in bytes.
	Size int64
}

// DocStore is the primary file-system abstraction for Vedox. All Markdown CRUD
// and file-watching flows use this interface. Implementations are responsible
// for enforcing security invariants (path traversal, secret blocklist) and
// write durability (atomic temp-file + fsync + rename).
//
// Error values returned by implementations use the VDX error codes documented
// in the CLI error taxonomy:
//
//	VDX-005 — path traversal attempt
//	VDX-006 — blocked secret file
//	VDX-007 — watcher file-count warning threshold (logged, not returned as error)
type DocStore interface {
	// Read returns the Doc at path. path is workspace-relative.
	// Returns VDX-005 if the resolved path escapes the workspace root.
	// Returns VDX-006 if path matches the secret file blocklist.
	Read(path string) (*Doc, error)

	// Write creates or overwrites the file at path with content.
	// Implementations must use the atomic temp-file → fsync → rename pattern.
	// Returns VDX-005 / VDX-006 on security violations.
	Write(path string, content string) error

	// Delete removes the file at path.
	// Returns VDX-005 / VDX-006 on security violations.
	Delete(path string) error

	// List returns all Docs directly inside dir (non-recursive).
	// dir is workspace-relative. Returns VDX-005 if dir escapes workspace root.
	List(dir string) ([]*Doc, error)

	// Watch starts a file-system watcher on dir and calls onChange with the
	// workspace-relative path of any file that is created, modified, or removed.
	// Callers should invoke Watch in a goroutine; it blocks until the adapter's
	// internal watcher is closed or an unrecoverable error occurs.
	// Symlinks are resolved to their real paths before being watched.
	// At 1000+ watched entries a VDX-007 warning is emitted to the log.
	Watch(dir string, onChange func(path string)) error
}
