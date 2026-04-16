// Package webassets embeds the compiled SvelteKit editor static export into
// the Vedox binary at release time.
//
// # Dual-file pattern
//
// This package uses two mutually exclusive build-tagged files:
//
//   - embed.go (this file): compiled only when -tags=release is set.
//     It contains the //go:embed directive and populates EditorFS with the
//     real SvelteKit build output copied into editorassets/ by goreleaser's
//     before.hooks step before compilation begins.
//
//   - embed_stub.go: compiled in all other cases (//go:build !release).
//     It sets EditorFS = nil so that go build ./... and go test ./... succeed
//     on PR runners without requiring the SvelteKit build output to exist.
//
// The daemon server checks EditorFS == nil at startup and either proxies to
// the Vite dev server (dev mode) or returns an informative error (e.g.,
// "editor not embedded; rebuild with -tags=release or run in dev mode").
//
// # goreleaser integration
//
// goreleaser's before.hooks runs:
//
//	cp -r apps/editor/build/* apps/cli/internal/webassets/editorassets/
//
// This copies the SvelteKit static export into editorassets/ before the Go
// compiler runs. The editorassets/ directory contains only a .gitkeep in
// version control; real assets are never committed. The directory is listed
// in .gitignore (contents excluded, directory kept via .gitkeep).
//
// The -tags=release flag in goreleaser's builds.flags activates this file and
// deactivates embed_stub.go. PR CI never sets -tags=release and never touches
// the editorassets/ directory.

//go:build release

package webassets

import (
	"embed"
	"io/fs"
)

// editorFiles is the raw embedded filesystem containing the SvelteKit static
// export. The embed path is relative to this source file; editorassets/ is
// the directory populated by goreleaser's before.hooks cp step.
//
//go:embed all:editorassets
var editorFiles embed.FS

// EditorFS is the sub-filesystem rooted at the editorassets/ directory.
// The daemon's HTTP file server serves this filesystem at the editor route.
// In release builds this is never nil. In non-release builds (embed_stub.go),
// this is nil — the daemon must handle that case explicitly.
var EditorFS fs.FS

func init() {
	sub, err := fs.Sub(editorFiles, "editorassets")
	if err != nil {
		// Panic is intentional: a malformed embed at release time means the
		// binary is broken. A loud immediate failure is far better than a
		// daemon that silently serves nothing.
		panic("webassets: failed to create sub-filesystem: " + err.Error())
	}
	EditorFS = sub
}

// GetEditorFS returns the embedded editor filesystem.
// In release builds this is always non-nil.
// In non-release builds (embed_stub.go) this returns nil.
func GetEditorFS() fs.FS {
	return EditorFS
}
