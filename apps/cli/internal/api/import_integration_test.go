package api_test

// Integration tests for POST /api/import.
//
// The importer walks the source tree for .md files and copies them into the
// workspace via the DocStore. These tests verify that the HTTP handler wires
// together srcProjectRoot validation, path traversal guards, projectName
// sanitisation, and the underlying importer's silent-skip behaviour.

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// importReqBody is the JSON body shape for POST /api/import.
type importReqBody struct {
	SrcProjectRoot string `json:"srcProjectRoot"`
	ProjectName    string `json:"projectName"`
}

// importRespBody mirrors the success-path response — a subset is enough for
// the assertions below, so we avoid coupling to every field in api.importResponse.
type importRespBody struct {
	Imported []string `json:"imported"`
	Skipped  []string `json:"skipped"`
	Warnings []string `json:"warnings"`
}

// newSourceTree builds a small external source project outside the Vedox
// workspace so the importer's self-import guard does not fire.
//
// Layout:
//
//	<tmp>/
//	  src/
//	    README.md
//	    docs/
//	      guide.md
func newSourceTree(t *testing.T) string {
	t.Helper()
	src := filepath.Join(t.TempDir(), "src")
	if err := mkdirAll(filepath.Join(src, "docs")); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := writeFile(filepath.Join(src, "README.md"), "# src\n"); err != nil {
		t.Fatalf("write README: %v", err)
	}
	if err := writeFile(filepath.Join(src, "docs", "guide.md"), "# Guide\n"); err != nil {
		t.Fatalf("write guide: %v", err)
	}
	return src
}

func TestImportProject(t *testing.T) {
	f := newTestServer(t)
	src := newSourceTree(t)

	resp := f.do(t, http.MethodPost, "/api/import", importReqBody{
		SrcProjectRoot: src,
		ProjectName:    "myimport",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	var body importRespBody
	decodeJSON(t, resp, &body)

	if len(body.Imported) != 2 {
		t.Errorf("imported count = %d, want 2 (%v)", len(body.Imported), body.Imported)
	}

	// Verify the files landed in the workspace.
	for _, rel := range []string{"myimport/README.md", "myimport/docs/guide.md"} {
		abs := filepath.Join(f.workspaceRoot, rel)
		if _, err := readFile(abs); err != nil {
			t.Errorf("expected imported file %s on disk: %v", rel, err)
		}
	}
}

// TestImport_SilentlySkipsNonMarkdown documents the importer's current
// behaviour: non-.md files (including secret files like .env) are silently
// excluded from the walk rather than surfaced in the "skipped" list. This
// matches the importer's implementation and catches regressions where a
// future change might start leaking secret contents into the workspace.
//
// NOTE (finding for follow-up): the handler's docs say `.env` appears in the
// skipped list, but the importer only walks *.md files, so a plain `.env` in
// the source tree is silently dropped. Consider either (a) surfacing ignored
// non-.md files in `skipped` or (b) removing the mention from the handler docs.
func TestImport_SilentlySkipsNonMarkdown(t *testing.T) {
	f := newTestServer(t)
	src := newSourceTree(t)

	// Seed a .env file next to the .md files. It must NOT end up in the
	// workspace after import.
	if err := writeFile(filepath.Join(src, ".env"), "SECRET=abc"); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	resp := f.do(t, http.MethodPost, "/api/import", importReqBody{
		SrcProjectRoot: src,
		ProjectName:    "myimport",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, readBody(t, resp))
	}

	var body importRespBody
	decodeJSON(t, resp, &body)

	// .env must not be copied into the workspace under any path.
	envDest := filepath.Join(f.workspaceRoot, "myimport", ".env")
	if _, err := readFile(envDest); err == nil {
		t.Errorf(".env was copied into workspace at %s — secret leak", envDest)
	}

	// Imported list must not mention anything about .env.
	for _, p := range body.Imported {
		if strings.Contains(p, ".env") {
			t.Errorf("imported list contains .env reference: %q", p)
		}
	}
}

// TestImport_SelfReferentialBlocked ensures the importer refuses when the
// source path is inside the Vedox workspace — that would create a recursive
// copy loop and corrupt the workspace.
func TestImport_SelfReferentialBlocked(t *testing.T) {
	f := newTestServer(t)

	// Point srcProjectRoot at a subdirectory of the workspace root.
	inside := filepath.Join(f.workspaceRoot, "nested")
	if err := mkdirAll(inside); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	resp := f.do(t, http.MethodPost, "/api/import", importReqBody{
		SrcProjectRoot: inside,
		ProjectName:    "myimport",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	if !strings.Contains(readBody(t, resp), "VDX-005") {
		t.Errorf("expected VDX-005 in body")
	}
}

func TestImport_InvalidProjectName(t *testing.T) {
	f := newTestServer(t)
	src := newSourceTree(t)

	resp := f.do(t, http.MethodPost, "/api/import", importReqBody{
		SrcProjectRoot: src,
		ProjectName:    "bad/name",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	if !strings.Contains(readBody(t, resp), "VDX-201") {
		t.Errorf("expected VDX-201 in body")
	}
}

// TestImport_MissingSourceDir uses a path that does not exist on disk.
// The handler's os.Stat check should return VDX-200.
func TestImport_MissingSourceDir(t *testing.T) {
	f := newTestServer(t)

	resp := f.do(t, http.MethodPost, "/api/import", importReqBody{
		SrcProjectRoot: "/this/path/definitely/does/not/exist/abc123",
		ProjectName:    "whatever",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	if !strings.Contains(readBody(t, resp), "VDX-200") {
		t.Errorf("expected VDX-200 in body")
	}
}
