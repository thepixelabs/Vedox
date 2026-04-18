package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/store"
)

// testDeps groups the real store and db.Store instances needed by Import.
type testDeps struct {
	docStore store.DocStore
	dbStore  *db.Store
	// workspaceRoot is the absolute path to the destination workspace.
	workspaceRoot string
}

// newTestDeps creates real LocalAdapter and db.Store instances backed by
// temporary directories that are cleaned up when the test ends.
func newTestDeps(t *testing.T) *testDeps {
	t.Helper()

	workspaceRoot := t.TempDir()

	adapter, err := store.NewLocalAdapter(workspaceRoot, nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}

	dbStore, err := db.Open(db.Options{WorkspaceRoot: workspaceRoot})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = dbStore.Close() })

	return &testDeps{
		docStore:      adapter,
		dbStore:       dbStore,
		workspaceRoot: workspaceRoot,
	}
}

// writeSrcFile creates a file inside srcRoot with the given content.
// Intermediate directories are created as needed.
func writeSrcFile(t *testing.T, srcRoot, relPath, content string) {
	t.Helper()
	abs := filepath.Join(srcRoot, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(abs), err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", abs, err)
	}
}

// TestImport_SuccessfulMDFiles verifies that .md files in the source directory
// are copied to the destination workspace and indexed in SQLite.
func TestImport_SuccessfulMDFiles(t *testing.T) {
	deps := newTestDeps(t)
	srcRoot := t.TempDir()

	writeSrcFile(t, srcRoot, "docs/adr-001.md", "# ADR-001\n\nContent here.\n")
	writeSrcFile(t, srcRoot, "README.md", "# README\n")
	writeSrcFile(t, srcRoot, "sub/nested.md", "# Nested\n")

	result, err := Import(srcRoot, "my-project", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err != nil {
		t.Fatalf("Import: unexpected error: %v", err)
	}

	if len(result.Imported) != 3 {
		t.Errorf("Imported: got %d, want 3; paths: %v", len(result.Imported), result.Imported)
	}

	// Each imported path must start with "my-project/".
	for _, p := range result.Imported {
		if !strings.HasPrefix(p, "my-project"+string(os.PathSeparator)) {
			t.Errorf("imported path %q does not start with project prefix", p)
		}
	}

	// Skipped list should be empty — all source files are valid .md.
	if len(result.Skipped) != 0 {
		t.Errorf("Skipped: want 0, got %d: %v", len(result.Skipped), result.Skipped)
	}

	// At least one warning must exist: the git removal reminder.
	if len(result.Warnings) == 0 {
		t.Error("Warnings: expected at least one warning (git removal reminder), got none")
	}
	hasGitHint := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "git") {
			hasGitHint = true
			break
		}
	}
	if !hasGitHint {
		t.Errorf("Warnings: expected git removal hint, got: %v", result.Warnings)
	}

	// Verify that files were actually written to the workspace.
	destADR := filepath.Join(deps.workspaceRoot, "my-project", "docs", "adr-001.md")
	if _, err := os.Stat(destADR); err != nil {
		t.Errorf("destination file %s does not exist: %v", destADR, err)
	}
}

// TestImport_NonMDFilesSkipped verifies that non-.md files are silently skipped
// and do not appear in the Imported or Skipped lists.
func TestImport_NonMDFilesSkipped(t *testing.T) {
	deps := newTestDeps(t)
	srcRoot := t.TempDir()

	writeSrcFile(t, srcRoot, "valid.md", "# Valid\n")
	writeSrcFile(t, srcRoot, "ignored.txt", "plain text")
	writeSrcFile(t, srcRoot, "image.png", "\x89PNG")
	writeSrcFile(t, srcRoot, "script.sh", "#!/bin/sh")
	writeSrcFile(t, srcRoot, "config.json", "{}")

	result, err := Import(srcRoot, "proj", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err != nil {
		t.Fatalf("Import: unexpected error: %v", err)
	}

	if len(result.Imported) != 1 {
		t.Errorf("Imported: got %d, want 1; paths: %v", len(result.Imported), result.Imported)
	}
	// Non-.md files are not reported as Skipped — they are simply not walked.
	// Only secret-blocked and stat/read errors appear in Skipped.
	for _, s := range result.Skipped {
		t.Errorf("unexpected Skipped entry: %q", s)
	}
}

// TestImport_SelfImportRejected verifies that srcProjectRoot inside
// destWorkspaceRoot returns an error.
func TestImport_SelfImportRejected(t *testing.T) {
	deps := newTestDeps(t)

	// Create a sub-directory inside the workspace to use as the src root.
	srcInsideWorkspace := filepath.Join(deps.workspaceRoot, "sub-project")
	if err := os.MkdirAll(srcInsideWorkspace, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err := Import(srcInsideWorkspace, "proj", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err == nil {
		t.Fatal("Import: expected error for self-import, got nil")
	}
	if !strings.Contains(err.Error(), "inside the Vedox workspace") {
		t.Errorf("error message does not mention self-import: %v", err)
	}
}

// TestImport_SrcEqualsDestRejected verifies that srcProjectRoot == destWorkspaceRoot
// is also rejected.
func TestImport_SrcEqualsDestRejected(t *testing.T) {
	deps := newTestDeps(t)

	_, err := Import(deps.workspaceRoot, "proj", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err == nil {
		t.Fatal("Import: expected error when src == dest workspace, got nil")
	}
}

// TestImport_ProjectNameWithSlashRejected verifies that projectName containing
// a forward slash is rejected.
func TestImport_ProjectNameWithSlashRejected(t *testing.T) {
	deps := newTestDeps(t)
	srcRoot := t.TempDir()

	_, err := Import(srcRoot, "parent/child", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err == nil {
		t.Fatal("Import: expected error for projectName with '/', got nil")
	}
	if !strings.Contains(err.Error(), "projectName") {
		t.Errorf("error should mention projectName, got: %v", err)
	}
}

// TestImport_ProjectNameWithBackslashRejected verifies that projectName containing
// a backslash is rejected.
func TestImport_ProjectNameWithBackslashRejected(t *testing.T) {
	deps := newTestDeps(t)
	srcRoot := t.TempDir()

	_, err := Import(srcRoot, `parent\child`, deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err == nil {
		t.Fatal("Import: expected error for projectName with '\\', got nil")
	}
}

// TestImport_ProjectNameEmptyRejected verifies that an empty projectName is rejected.
func TestImport_ProjectNameEmptyRejected(t *testing.T) {
	deps := newTestDeps(t)
	srcRoot := t.TempDir()

	_, err := Import(srcRoot, "", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err == nil {
		t.Fatal("Import: expected error for empty projectName, got nil")
	}
}

// TestImport_ProjectNameDotRejected verifies that "." is rejected as a
// projectName (it is not a valid single-segment name).
func TestImport_ProjectNameDotRejected(t *testing.T) {
	deps := newTestDeps(t)
	srcRoot := t.TempDir()

	_, err := Import(srcRoot, ".", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err == nil {
		t.Fatal("Import: expected error for projectName '.', got nil")
	}
}

// TestImport_ProjectNameDotDotRejected verifies that ".." is rejected.
func TestImport_ProjectNameDotDotRejected(t *testing.T) {
	deps := newTestDeps(t)
	srcRoot := t.TempDir()

	_, err := Import(srcRoot, "..", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err == nil {
		t.Fatal("Import: expected error for projectName '..', got nil")
	}
}

// TestImport_EmptySourceDirectory verifies that importing an empty directory
// succeeds with zero imports and zero skips.
func TestImport_EmptySourceDirectory(t *testing.T) {
	deps := newTestDeps(t)
	srcRoot := t.TempDir()

	result, err := Import(srcRoot, "empty-proj", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err != nil {
		t.Fatalf("Import: unexpected error: %v", err)
	}

	if len(result.Imported) != 0 {
		t.Errorf("Imported: expected 0, got %d: %v", len(result.Imported), result.Imported)
	}
	if len(result.Skipped) != 0 {
		t.Errorf("Skipped: expected 0, got %d: %v", len(result.Skipped), result.Skipped)
	}
	// No files imported means no git warning.
	for _, w := range result.Warnings {
		if strings.Contains(w, "git") {
			t.Errorf("unexpected git warning when no files were imported: %q", w)
		}
	}
}

// TestImport_SourceNotAbsolute verifies that a relative srcProjectRoot is rejected.
func TestImport_SourceNotAbsolute(t *testing.T) {
	deps := newTestDeps(t)

	_, err := Import("relative/path", "proj", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err == nil {
		t.Fatal("Import: expected error for relative srcProjectRoot, got nil")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Errorf("error should mention 'absolute', got: %v", err)
	}
}

// TestImport_DestNotAbsolute verifies that a relative destWorkspaceRoot is rejected.
func TestImport_DestNotAbsolute(t *testing.T) {
	srcRoot := t.TempDir()
	deps := newTestDeps(t)

	_, err := Import(srcRoot, "proj", "relative/dest", deps.docStore, deps.dbStore)
	if err == nil {
		t.Fatal("Import: expected error for relative destWorkspaceRoot, got nil")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Errorf("error should mention 'absolute', got: %v", err)
	}
}

// TestImport_SourceDoesNotExist verifies that a srcProjectRoot that does not
// exist on disk is rejected.
func TestImport_SourceDoesNotExist(t *testing.T) {
	deps := newTestDeps(t)

	_, err := Import("/absolutely/does/not/exist/ever", "proj", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err == nil {
		t.Fatal("Import: expected error for non-existent srcProjectRoot, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error should mention non-existence, got: %v", err)
	}
}

// TestImport_SkipsDirWithLeadingDot verifies that directories whose name starts
// with '.' (hidden directories) are not descended into.
func TestImport_SkipsDirWithLeadingDot(t *testing.T) {
	deps := newTestDeps(t)
	srcRoot := t.TempDir()

	writeSrcFile(t, srcRoot, ".hidden/secret.md", "should be skipped")
	writeSrcFile(t, srcRoot, "visible.md", "# Visible\n")

	result, err := Import(srcRoot, "proj", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err != nil {
		t.Fatalf("Import: unexpected error: %v", err)
	}

	if len(result.Imported) != 1 {
		t.Errorf("Imported: got %d, want 1 (hidden dir should be skipped); paths: %v", len(result.Imported), result.Imported)
	}
	for _, p := range result.Imported {
		if strings.Contains(p, ".hidden") {
			t.Errorf("hidden directory file should not have been imported: %q", p)
		}
	}
}

// TestImport_SkipsNodeModules verifies that node_modules directories are excluded.
func TestImport_SkipsNodeModules(t *testing.T) {
	deps := newTestDeps(t)
	srcRoot := t.TempDir()

	writeSrcFile(t, srcRoot, "node_modules/dep/README.md", "dep readme")
	writeSrcFile(t, srcRoot, "docs/api.md", "# API\n")

	result, err := Import(srcRoot, "proj", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err != nil {
		t.Fatalf("Import: unexpected error: %v", err)
	}

	if len(result.Imported) != 1 {
		t.Errorf("Imported: got %d, want 1 (node_modules should be skipped); paths: %v", len(result.Imported), result.Imported)
	}
	for _, p := range result.Imported {
		if strings.Contains(p, "node_modules") {
			t.Errorf("node_modules file should not have been imported: %q", p)
		}
	}
}

// TestImport_WithFrontmatter verifies that a file with YAML frontmatter is
// imported without error and the content is preserved verbatim.
func TestImport_WithFrontmatter(t *testing.T) {
	deps := newTestDeps(t)
	srcRoot := t.TempDir()

	content := "---\ntitle: My ADR\ntype: adr\nstatus: published\ndate: 2026-01-15\nauthor: alice\n---\n# Body\n\nDecision made.\n"
	writeSrcFile(t, srcRoot, "adr.md", content)

	result, err := Import(srcRoot, "fm-proj", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err != nil {
		t.Fatalf("Import: unexpected error: %v", err)
	}

	if len(result.Imported) != 1 {
		t.Fatalf("Imported: got %d, want 1", len(result.Imported))
	}

	// Verify the written file content matches the source.
	destPath := filepath.Join(deps.workspaceRoot, result.Imported[0])
	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("ReadFile dest: %v", err)
	}
	if string(got) != content {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", string(got), content)
	}
}

// TestImport_UnreadableFileSkipped verifies that a .md file the process cannot
// read is recorded in Skipped and does not abort the walk.
// This test is skipped when running as root (root ignores file permissions).
func TestImport_UnreadableFileSkipped(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root: file permissions are not enforced")
	}

	deps := newTestDeps(t)
	srcRoot := t.TempDir()

	writeSrcFile(t, srcRoot, "readable.md", "# OK\n")
	writeSrcFile(t, srcRoot, "locked.md", "# Locked\n")

	// Remove read permission so os.ReadFile fails.
	if err := os.Chmod(filepath.Join(srcRoot, "locked.md"), 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() {
		// Restore permissions so t.TempDir() cleanup can delete the file.
		_ = os.Chmod(filepath.Join(srcRoot, "locked.md"), 0o600)
	})

	result, err := Import(srcRoot, "locked-proj", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err != nil {
		t.Fatalf("Import: unexpected error: %v", err)
	}

	// The readable file should be imported; the locked one should be skipped.
	if len(result.Imported) != 1 {
		t.Errorf("Imported: got %d, want 1; paths: %v", len(result.Imported), result.Imported)
	}
	foundSkipped := false
	for _, s := range result.Skipped {
		if strings.Contains(s, "locked") {
			foundSkipped = true
		}
	}
	if !foundSkipped {
		t.Errorf("Skipped: expected locked.md to appear; got: %v", result.Skipped)
	}
}

// TestImport_MultipleCallsSameProject verifies that importing into the same
// projectName twice (e.g. a re-import) overwrites without error and reflects
// the latest file count.
func TestImport_MultipleCallsSameProject(t *testing.T) {
	deps := newTestDeps(t)
	srcRoot := t.TempDir()

	writeSrcFile(t, srcRoot, "doc.md", "# v1\n")

	result1, err := Import(srcRoot, "repeat", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err != nil {
		t.Fatalf("first Import: %v", err)
	}
	if len(result1.Imported) != 1 {
		t.Errorf("first Import: want 1 imported, got %d", len(result1.Imported))
	}

	// Overwrite the source file and add a new one.
	writeSrcFile(t, srcRoot, "doc.md", "# v2\n")
	writeSrcFile(t, srcRoot, "new.md", "# New\n")

	result2, err := Import(srcRoot, "repeat", deps.workspaceRoot, deps.docStore, deps.dbStore)
	if err != nil {
		t.Fatalf("second Import: %v", err)
	}
	if len(result2.Imported) != 2 {
		t.Errorf("second Import: want 2 imported, got %d", len(result2.Imported))
	}
}

// -- Internal helper unit tests -----------------------------------------------
// These are in the same package (package importer) and exercise the private
// helpers directly to drive coverage of branches that are difficult to reach
// through the Import() entry point.

// TestExtractSimpleFrontmatter_NoPrefix verifies that content not starting with
// "---\n" returns an empty map immediately.
func TestExtractSimpleFrontmatter_NoPrefix(t *testing.T) {
	got := extractSimpleFrontmatter("# Just a heading\n")
	if len(got) != 0 {
		t.Errorf("expected empty map for non-frontmatter content, got %v", got)
	}
}

// TestExtractSimpleFrontmatter_UnclosedBlock verifies that content with an
// opening "---\n" but no closing delimiter returns an empty map.
func TestExtractSimpleFrontmatter_UnclosedBlock(t *testing.T) {
	got := extractSimpleFrontmatter("---\ntitle: Oops\n# no closing delimiter\n")
	if len(got) != 0 {
		t.Errorf("expected empty map for unclosed frontmatter, got %v", got)
	}
}

// TestExtractSimpleFrontmatter_QuotedValues verifies that both single-quoted and
// double-quoted values are stripped of their surrounding quotes.
func TestExtractSimpleFrontmatter_QuotedValues(t *testing.T) {
	content := "---\ntitle: \"My Title\"\nauthor: 'Alice'\nstatus: draft\n---\n"
	got := extractSimpleFrontmatter(content)

	if got["title"] != "My Title" {
		t.Errorf("double-quoted value: got %q, want %q", got["title"], "My Title")
	}
	if got["author"] != "Alice" {
		t.Errorf("single-quoted value: got %q, want %q", got["author"], "Alice")
	}
	if got["status"] != "draft" {
		t.Errorf("unquoted value: got %q, want %q", got["status"], "draft")
	}
}

// TestExtractSimpleFrontmatter_LineWithNoColon verifies that lines without a
// colon are skipped gracefully.
func TestExtractSimpleFrontmatter_LineWithNoColon(t *testing.T) {
	content := "---\nno colon here\ntitle: Valid\n---\n"
	got := extractSimpleFrontmatter(content)

	if got["title"] != "Valid" {
		t.Errorf("expected 'title' = 'Valid', got %q", got["title"])
	}
	if _, ok := got["no colon here"]; ok {
		t.Error("line without colon should not appear as a key")
	}
}

// TestExtractSimpleFrontmatter_EmptyValue verifies that key-value pairs where
// the value is empty (e.g. "key: ") are not included in the result.
func TestExtractSimpleFrontmatter_EmptyValue(t *testing.T) {
	content := "---\ntitle: \nstatus: published\n---\n"
	got := extractSimpleFrontmatter(content)

	if _, ok := got["title"]; ok {
		t.Error("key with empty value should not appear in result")
	}
	if got["status"] != "published" {
		t.Errorf("status: got %q, want %q", got["status"], "published")
	}
}

// TestStringVal_NilMap verifies that stringVal returns "" when given a nil map
// (the nil-guard branch at the top of the function).
func TestStringVal_NilMap(t *testing.T) {
	got := stringVal(nil, "any-key")
	if got != "" {
		t.Errorf("stringVal(nil, ...): expected empty string, got %q", got)
	}
}

// TestStringVal_MissingKey verifies that a key not present in the map returns "".
func TestStringVal_MissingKey(t *testing.T) {
	m := map[string]string{"a": "1"}
	got := stringVal(m, "missing")
	if got != "" {
		t.Errorf("stringVal missing key: expected empty string, got %q", got)
	}
}

// TestIsSecretFile verifies the secret blocklist matcher against a table of
// file names. This covers the isSecretFile function in the importer package
// (which mirrors the one in store.LocalAdapter but is package-private here).
func TestIsSecretFile(t *testing.T) {
	cases := []struct {
		name    string
		blocked bool
	}{
		// --- existing blocklist cases (must stay green) ---
		{".env", true},
		{"server.pem", true},
		{"server.key", true},
		{"id_rsa", true},
		{"keystore.p12", true},
		{"credentials.json", true},
		{"README.md", false},
		{"config.yaml", false},
		{"adr-001.md", false},

		// --- draft-variant cases: WS-R hotfix (changelog item 31) ---
		// A .draft.md suffix must be stripped before the blocklist check so
		// secret files saved as drafts are still blocked.
		{".env.draft.md", true},       // core case: bug was here
		{".env.draft.md.2", true},     // numbered draft variant
		{".env.draft", true},          // .draft-only suffix (no .md)
		{"server.pem.draft.md", true}, // pem entry with draft suffix
		{"server.key.draft.md", true}, // key entry with draft suffix
		{"id_rsa.draft.md", true},     // exact-match entry with draft suffix

		// --- false-positive traps: non-secret files must NOT be blocked ---
		{"config.draft.md", false},  // "config" is not in the blocklist
		{"adr-001.draft.md", false}, // normal ADR draft — not a secret
		// "secrets" is not a blocklist entry, so this must be false.
		{"secrets.draft.md", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := isSecretFile(tc.name)
			if got != tc.blocked {
				t.Errorf("isSecretFile(%q) = %v, want %v", tc.name, got, tc.blocked)
			}
		})
	}
}
