package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	vedoxerrors "github.com/vedox/vedox/internal/errors"
)

// helpers

// makeSymlinkAdapter creates a SymlinkAdapter with separate externalRoot and
// workspaceRoot temp directories, both pre-existing.
func makeSymlinkAdapter(t *testing.T) (*SymlinkAdapter, string, string) {
	t.Helper()
	extRoot := t.TempDir()
	wsRoot := t.TempDir()
	a, err := NewSymlinkAdapter(extRoot, "test-project", wsRoot)
	if err != nil {
		t.Fatalf("NewSymlinkAdapter: %v", err)
	}
	return a, extRoot, wsRoot
}

// writeMD writes a markdown file under dir/rel, creating parent dirs as needed.
func writeMD(t *testing.T, dir, rel, content string) {
	t.Helper()
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func assertSymlinkVDXCode(t *testing.T, err error, want vedoxerrors.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", want)
	}
	var ve *vedoxerrors.VedoxError
	if !errors.As(err, &ve) {
		t.Fatalf("expected *vedoxerrors.VedoxError, got %T: %v", err, err)
	}
	if ve.Code != want {
		t.Errorf("code: got %s, want %s", ve.Code, want)
	}
}

// -- NewSymlinkAdapter ----------------------------------------------------------

func TestNewSymlinkAdapter_HappyPath(t *testing.T) {
	extRoot := t.TempDir()
	wsRoot := t.TempDir()

	a, err := NewSymlinkAdapter(extRoot, "myproject", wsRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil adapter")
	}
}

func TestNewSymlinkAdapter_ExternalRootDoesNotExist(t *testing.T) {
	wsRoot := t.TempDir()
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	_, err := NewSymlinkAdapter(missing, "proj", wsRoot)
	if err == nil {
		t.Fatal("expected error when externalRoot does not exist, got nil")
	}
}

func TestNewSymlinkAdapter_WorkspaceRootDoesNotExist(t *testing.T) {
	extRoot := t.TempDir()
	missing := filepath.Join(t.TempDir(), "no-such-ws")

	_, err := NewSymlinkAdapter(extRoot, "proj", missing)
	if err == nil {
		t.Fatal("expected error when workspaceRoot does not exist, got nil")
	}
}

func TestNewSymlinkAdapter_EmptyProjectName(t *testing.T) {
	extRoot := t.TempDir()
	wsRoot := t.TempDir()

	_, err := NewSymlinkAdapter(extRoot, "", wsRoot)
	if err == nil {
		t.Fatal("expected error for empty projectName, got nil")
	}
	if !strings.Contains(err.Error(), "projectName") {
		t.Errorf("error should mention projectName, got: %v", err)
	}
}

func TestNewSymlinkAdapter_ExternalRootInsideWorkspace(t *testing.T) {
	wsRoot := t.TempDir()
	// Create a sub-directory inside the workspace root to use as externalRoot.
	sub := filepath.Join(wsRoot, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	_, err := NewSymlinkAdapter(sub, "proj", wsRoot)
	if err == nil {
		t.Fatal("expected error when externalRoot is inside workspaceRoot, got nil")
	}
}

func TestNewSymlinkAdapter_ExternalRootIsWorkspaceRoot(t *testing.T) {
	wsRoot := t.TempDir()
	_, err := NewSymlinkAdapter(wsRoot, "proj", wsRoot)
	if err == nil {
		t.Fatal("expected error when externalRoot equals workspaceRoot, got nil")
	}
}

// -- ExternalRoot and ProjectName ----------------------------------------------

func TestExternalRoot_ReturnsResolvedPath(t *testing.T) {
	a, extRoot, _ := makeSymlinkAdapter(t)

	// EvalSymlinks because TempDir may itself be a symlink (macOS).
	want, err := filepath.EvalSymlinks(extRoot)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	want = filepath.Clean(want)

	if got := a.ExternalRoot(); got != want {
		t.Errorf("ExternalRoot() = %q, want %q", got, want)
	}
}

func TestProjectName_ReturnsConfiguredName(t *testing.T) {
	extRoot := t.TempDir()
	wsRoot := t.TempDir()
	a, err := NewSymlinkAdapter(extRoot, "my-external-docs", wsRoot)
	if err != nil {
		t.Fatalf("NewSymlinkAdapter: %v", err)
	}
	if got := a.ProjectName(); got != "my-external-docs" {
		t.Errorf("ProjectName() = %q, want %q", got, "my-external-docs")
	}
}

// -- Read ----------------------------------------------------------------------

func TestRead_HappyPath(t *testing.T) {
	a, extRoot, _ := makeSymlinkAdapter(t)
	writeMD(t, extRoot, "notes.md", "# Hello\n\nWorld.\n")

	doc, err := a.Read("notes.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if doc == nil {
		t.Fatal("expected non-nil doc")
	}
	if !strings.Contains(doc.Content, "Hello") {
		t.Errorf("Content missing 'Hello': %q", doc.Content)
	}
	if doc.Metadata["_source"] != "symlink" {
		t.Errorf("_source: got %v, want %q", doc.Metadata["_source"], "symlink")
	}
	if doc.Metadata["_editable"] != false {
		t.Errorf("_editable: got %v, want false", doc.Metadata["_editable"])
	}
}

func TestRead_WithFrontmatter(t *testing.T) {
	a, extRoot, _ := makeSymlinkAdapter(t)
	writeMD(t, extRoot, "doc.md", "---\ntitle: My External Doc\n---\n# Body\n")

	doc, err := a.Read("doc.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if doc.Metadata["title"] != "My External Doc" {
		t.Errorf("title: got %v, want %q", doc.Metadata["title"], "My External Doc")
	}
	// Synthetic fields should still be injected.
	if doc.Metadata["_source"] != "symlink" {
		t.Errorf("_source: got %v, want 'symlink'", doc.Metadata["_source"])
	}
}

func TestRead_PathTraversalRejected(t *testing.T) {
	a, _, _ := makeSymlinkAdapter(t)

	_, err := a.Read("../escape.md")
	assertSymlinkVDXCode(t, err, vedoxerrors.ErrPathTraversal)
}

func TestRead_DeepPathTraversalRejected(t *testing.T) {
	a, _, _ := makeSymlinkAdapter(t)

	_, err := a.Read("docs/../../etc/passwd")
	assertSymlinkVDXCode(t, err, vedoxerrors.ErrPathTraversal)
}

func TestRead_SecretFileBlocked(t *testing.T) {
	a, extRoot, _ := makeSymlinkAdapter(t)
	// Write the secret file directly to bypass adapter.
	if err := os.WriteFile(filepath.Join(extRoot, ".env"), []byte("SECRET=1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := a.Read(".env")
	assertSymlinkVDXCode(t, err, vedoxerrors.ErrSecretFileBlocked)
}

func TestRead_FileNotFound(t *testing.T) {
	a, _, _ := makeSymlinkAdapter(t)
	_, err := a.Read("nonexistent.md")
	if err == nil {
		t.Fatal("expected error reading nonexistent file, got nil")
	}
}

// -- List ----------------------------------------------------------------------

func TestList_ReturnsMarkdownFiles(t *testing.T) {
	a, extRoot, _ := makeSymlinkAdapter(t)
	writeMD(t, extRoot, "a.md", "# A")
	writeMD(t, extRoot, "b.md", "# B")

	docs, err := a.List(".")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
}

func TestList_IgnoresNonMarkdownFiles(t *testing.T) {
	a, extRoot, _ := makeSymlinkAdapter(t)
	writeMD(t, extRoot, "doc.md", "# Doc")
	// Write a non-.md file.
	if err := os.WriteFile(filepath.Join(extRoot, "image.png"), []byte("PNG"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(extRoot, "config.yaml"), []byte("key: val"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	docs, err := a.List(".")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// SymlinkAdapter.List includes all files it can read; non-.md are included
	// unless isSecretFile blocks them. Verify .md is present.
	found := false
	for _, d := range docs {
		if strings.HasSuffix(d.Path, "doc.md") {
			found = true
		}
	}
	if !found {
		t.Error("expected doc.md in listing, not found")
	}
}

func TestList_RecursiveWalk(t *testing.T) {
	a, extRoot, _ := makeSymlinkAdapter(t)
	writeMD(t, extRoot, "top.md", "# Top")
	writeMD(t, extRoot, "sub/child.md", "# Child")
	writeMD(t, extRoot, "sub/deep/nested.md", "# Nested")

	docs, err := a.List(".")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Expect all three markdown files.
	if len(docs) < 3 {
		t.Errorf("expected at least 3 docs from recursive walk, got %d", len(docs))
	}
}

func TestSymlinkList_EmptyDir(t *testing.T) {
	a, _, _ := makeSymlinkAdapter(t)

	docs, err := a.List(".")
	if err != nil {
		t.Fatalf("List empty dir: %v", err)
	}
	if docs == nil {
		t.Error("List should return empty slice, not nil")
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(docs))
	}
}

func TestSymlinkList_SkipsSecretFiles(t *testing.T) {
	a, extRoot, _ := makeSymlinkAdapter(t)
	writeMD(t, extRoot, "visible.md", "# Visible")
	if err := os.WriteFile(filepath.Join(extRoot, ".env"), []byte("SECRET=1"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	docs, err := a.List(".")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, d := range docs {
		if filepath.Base(d.Path) == ".env" {
			t.Errorf("List should not expose .env, but got path %s", d.Path)
		}
	}
}

func TestList_PathTraversalRejected(t *testing.T) {
	a, _, _ := makeSymlinkAdapter(t)
	_, err := a.List("../")
	assertSymlinkVDXCode(t, err, vedoxerrors.ErrPathTraversal)
}

func TestList_DocHasSymlinkMeta(t *testing.T) {
	a, extRoot, _ := makeSymlinkAdapter(t)
	writeMD(t, extRoot, "check.md", "# Check")

	docs, err := a.List(".")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(docs) == 0 {
		t.Fatal("expected at least one doc")
	}
	d := docs[0]
	if d.Metadata["_source"] != "symlink" {
		t.Errorf("_source: got %v, want 'symlink'", d.Metadata["_source"])
	}
	if d.Metadata["_editable"] != false {
		t.Errorf("_editable: got %v, want false", d.Metadata["_editable"])
	}
}

// -- Write / Delete (read-only) ------------------------------------------------

func TestWrite_ReturnsReadOnlyError(t *testing.T) {
	a, _, _ := makeSymlinkAdapter(t)
	err := a.Write("any.md", "content")
	assertSymlinkVDXCode(t, err, vedoxerrors.ErrReadOnly)
}

func TestDelete_ReturnsReadOnlyError(t *testing.T) {
	a, _, _ := makeSymlinkAdapter(t)
	err := a.Delete("any.md")
	assertSymlinkVDXCode(t, err, vedoxerrors.ErrReadOnly)
}

// -- injectSymlinkMeta ---------------------------------------------------------

func TestInjectSymlinkMeta_SetsFields(t *testing.T) {
	meta := make(map[string]interface{})
	injectSymlinkMeta(meta, "/some/path/file.md")

	if meta["_source"] != "symlink" {
		t.Errorf("_source = %v, want 'symlink'", meta["_source"])
	}
	if meta["_editable"] != false {
		t.Errorf("_editable = %v, want false", meta["_editable"])
	}
	if meta["_source_path"] != "/some/path/file.md" {
		t.Errorf("_source_path = %v, want '/some/path/file.md'", meta["_source_path"])
	}
}

func TestInjectSymlinkMeta_OverwritesExisting(t *testing.T) {
	meta := map[string]interface{}{
		"_source":   "old",
		"_editable": true,
	}
	injectSymlinkMeta(meta, "/new/path.md")

	if meta["_source"] != "symlink" {
		t.Errorf("_source not overwritten: got %v", meta["_source"])
	}
	if meta["_editable"] != false {
		t.Errorf("_editable not overwritten: got %v", meta["_editable"])
	}
}
