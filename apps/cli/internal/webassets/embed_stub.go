//go:build !release

package webassets

import "io/fs"

// EditorFS is nil in non-release builds.
//
// The daemon server must check for nil and either:
//   - redirect / proxy to the Vite dev server on port 5151 (dev mode), or
//   - return an informative HTTP 503 with the message:
//     "editor not embedded; rebuild with -tags=release or run in dev mode".
//
// This stub ensures go build ./... and go test -race ./... succeed on PR
// runners without requiring the SvelteKit build output to exist. The embed
// directive in embed.go is only compiled when -tags=release is set (goreleaser
// release builds). See embed.go doc comment for the full dual-file rationale.
var EditorFS fs.FS = nil //nolint:revive // nil is intentional; callers must check

// GetEditorFS returns nil in non-release builds.
// Callers must treat a nil return as "editor not embedded" and handle
// accordingly (dev-mode proxy or error response).
func GetEditorFS() fs.FS {
	return EditorFS
}
