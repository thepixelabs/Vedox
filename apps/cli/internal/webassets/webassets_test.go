//go:build !release

package webassets_test

import (
	"testing"

	"github.com/vedox/vedox/internal/webassets"
)

// TestGetEditorFS_Stub verifies that GetEditorFS returns nil in non-release
// builds (i.e., when compiled without -tags=release).
//
// This test exercises the embed_stub.go path. It must not be annotated with
// //go:build !release — the package-level tag on this file handles exclusion
// from release builds. The test is intentionally simple: the stub's only
// invariant is that it returns nil, ensuring PR-time go test ./... passes
// without the SvelteKit build output present.
func TestGetEditorFS_Stub(t *testing.T) {
	got := webassets.GetEditorFS()
	if got != nil {
		t.Errorf("GetEditorFS() = %v, want nil in non-release build", got)
	}
}
