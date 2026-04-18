package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	vedoxerrors "github.com/vedox/vedox/internal/errors"
)

// newTestAdapter creates a LocalAdapter rooted at a temporary directory that is
// cleaned up when the test ends.
func newTestAdapter(t *testing.T) (*LocalAdapter, string) {
	t.Helper()
	root := t.TempDir()
	a, err := NewLocalAdapter(root, nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}
	return a, root
}

// writeRaw writes bytes directly into the temp workspace, bypassing the adapter
// (useful for test setup of pre-existing files).
func writeRaw(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
		t.Fatalf("writeRaw: %v", err)
	}
}

// assertVDXCode asserts that err is (or wraps) a *vedoxerrors.VedoxError with
// the expected code.
func assertVDXCode(t *testing.T, err error, wantCode vedoxerrors.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s, got nil", wantCode)
	}
	var vdxErr *vedoxerrors.VedoxError
	if !errors.As(err, &vdxErr) {
		t.Fatalf("expected *vedoxerrors.VedoxError, got %T: %v", err, err)
	}
	if vdxErr.Code != wantCode {
		t.Errorf("expected code %s, got %s", wantCode, vdxErr.Code)
	}
}

// -- Atomic write tests -------------------------------------------------------

// TestWrite_AtomicNoTempFileLeft verifies that after a successful Write the only
// file in the target directory is the intended output — no orphaned .vedox-write-*
// temp files remain.
func TestWrite_AtomicNoTempFileLeft(t *testing.T) {
	a, root := newTestAdapter(t)

	if err := a.Write("doc.md", "# Hello"); err != nil {
		t.Fatalf("Write: %v", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".vedox-write-") {
			t.Errorf("leftover temp file found: %s", e.Name())
		}
	}

	abs := filepath.Join(root, "doc.md")
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("ReadFile after Write: %v", err)
	}
	if string(got) != "# Hello" {
		t.Errorf("content mismatch: got %q, want %q", string(got), "# Hello")
	}
}

// TestWrite_Overwrite checks that a second Write to the same path replaces the
// previous content and leaves no temp files.
func TestWrite_Overwrite(t *testing.T) {
	a, root := newTestAdapter(t)

	if err := a.Write("doc.md", "first"); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	if err := a.Write("doc.md", "second"); err != nil {
		t.Fatalf("second Write: %v", err)
	}

	abs := filepath.Join(root, "doc.md")
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "second" {
		t.Errorf("expected %q, got %q", "second", string(got))
	}
}

// TestWrite_CreatesParentDirs verifies that Write creates intermediate directories
// rather than failing when they don't exist.
func TestWrite_CreatesParentDirs(t *testing.T) {
	a, _ := newTestAdapter(t)

	if err := a.Write("deep/nested/doc.md", "content"); err != nil {
		t.Fatalf("Write nested: %v", err)
	}
}

// -- Path traversal tests -----------------------------------------------------

// TestSafePath_TraversalBlocked verifies that paths with ".." components that
// resolve outside the workspace root are rejected with VDX-005.
func TestSafePath_TraversalBlocked(t *testing.T) {
	a, _ := newTestAdapter(t)

	cases := []string{
		"../escape.md",
		"../../etc/passwd",
		"docs/../../secret",
		"docs/../../../outside",
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc, func(t *testing.T) {
			err := a.Write(tc, "payload")
			assertVDXCode(t, err, vedoxerrors.ErrPathTraversal)
		})
	}
}

// TestSafePath_TraversalBlockedRead checks Read.
func TestSafePath_TraversalBlockedRead(t *testing.T) {
	a, _ := newTestAdapter(t)
	_, err := a.Read("../escape.md")
	assertVDXCode(t, err, vedoxerrors.ErrPathTraversal)
}

// TestSafePath_TraversalBlockedDelete checks Delete.
func TestSafePath_TraversalBlockedDelete(t *testing.T) {
	a, _ := newTestAdapter(t)
	err := a.Delete("../escape.md")
	assertVDXCode(t, err, vedoxerrors.ErrPathTraversal)
}

// TestSafePath_TraversalBlockedList checks List.
func TestSafePath_TraversalBlockedList(t *testing.T) {
	a, _ := newTestAdapter(t)
	_, err := a.List("../")
	assertVDXCode(t, err, vedoxerrors.ErrPathTraversal)
}

// TestSafePath_ValidPathsAllowed verifies that well-formed paths within the root
// are not falsely rejected.
func TestSafePath_ValidPathsAllowed(t *testing.T) {
	a, _ := newTestAdapter(t)

	paths := []string{
		"doc.md",
		"docs/arch/adr-001.md",
		"README.md",
	}
	for _, p := range paths {
		if err := a.Write(p, "content"); err != nil {
			t.Errorf("Write(%q) unexpectedly failed: %v", p, err)
		}
	}
}

// -- Secret file blocklist tests ----------------------------------------------

func TestSecretBlocklist_Write(t *testing.T) {
	a, _ := newTestAdapter(t)

	blocked := []string{
		".env",
		"server.pem",
		"server.key",
		"id_rsa",
		"keystore.p12",
		"credentials.json",
	}

	for _, name := range blocked {
		name := name
		t.Run(name, func(t *testing.T) {
			err := a.Write(name, "secret content")
			assertVDXCode(t, err, vedoxerrors.ErrSecretFileBlocked)
		})
	}
}

func TestSecretBlocklist_Read(t *testing.T) {
	a, root := newTestAdapter(t)

	// Create the file directly (bypassing the adapter) so we can test Read.
	writeRaw(t, root, ".env", "SECRET=abc")

	_, err := a.Read(".env")
	assertVDXCode(t, err, vedoxerrors.ErrSecretFileBlocked)
}

func TestSecretBlocklist_Delete(t *testing.T) {
	a, root := newTestAdapter(t)
	writeRaw(t, root, "id_rsa", "PRIVATE KEY")

	err := a.Delete("id_rsa")
	assertVDXCode(t, err, vedoxerrors.ErrSecretFileBlocked)
}

// TestSecretBlocklist_Nested verifies that blocklisted files are rejected even
// when nested inside a subdirectory.
func TestSecretBlocklist_Nested(t *testing.T) {
	a, _ := newTestAdapter(t)

	err := a.Write("config/.env", "DB_PASS=hunter2")
	assertVDXCode(t, err, vedoxerrors.ErrSecretFileBlocked)
}

// -- Frontmatter parsing tests ------------------------------------------------

func TestParseFrontmatter_WithFrontmatter(t *testing.T) {
	raw := []byte("---\ntitle: My Doc\nstatus: draft\ndate: 2026-04-07\n---\n# Body\n")
	meta, content := parseFrontmatter(raw)

	if meta["title"] != "My Doc" {
		t.Errorf("title: got %v, want %q", meta["title"], "My Doc")
	}
	if meta["status"] != "draft" {
		t.Errorf("status: got %v, want %q", meta["status"], "draft")
	}
	// content must equal the full raw input for round-trip fidelity.
	if content != string(raw) {
		t.Errorf("content should equal raw input for round-trip fidelity")
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	raw := []byte("# Just a heading\n\nNo frontmatter here.\n")
	meta, content := parseFrontmatter(raw)

	if len(meta) != 0 {
		t.Errorf("expected empty meta, got %v", meta)
	}
	if content != string(raw) {
		t.Errorf("content mismatch")
	}
}

func TestParseFrontmatter_EmptyFrontmatter(t *testing.T) {
	raw := []byte("---\n---\n# Body\n")
	meta, content := parseFrontmatter(raw)

	if len(meta) != 0 {
		t.Errorf("expected empty meta for empty frontmatter block, got %v", meta)
	}
	if content != string(raw) {
		t.Errorf("content mismatch")
	}
}

func TestParseFrontmatter_UnclosedBlock(t *testing.T) {
	// An opening "---" with no closing "---" should not parse as frontmatter.
	raw := []byte("---\ntitle: Oops\n# forgot to close\n")
	meta, content := parseFrontmatter(raw)

	if len(meta) != 0 {
		t.Errorf("unclosed block: expected empty meta, got %v", meta)
	}
	if content != string(raw) {
		t.Errorf("content mismatch")
	}
}

func TestParseFrontmatter_InvalidYAML(t *testing.T) {
	// Invalid YAML in frontmatter should not crash; meta returns empty non-nil map.
	raw := []byte("---\n: invalid: yaml: {{\n---\n# Body\n")
	meta, _ := parseFrontmatter(raw)

	if meta == nil {
		t.Errorf("meta must never be nil")
	}
}

func TestParseFrontmatter_NestedValues(t *testing.T) {
	raw := []byte("---\ntitle: Nested\ntags:\n  - go\n  - docs\n---\n")
	meta, _ := parseFrontmatter(raw)

	tags, ok := meta["tags"]
	if !ok {
		t.Fatalf("expected 'tags' key in meta")
	}
	tagList, ok := tags.([]interface{})
	if !ok {
		t.Fatalf("expected tags to be []interface{}, got %T", tags)
	}
	if len(tagList) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tagList))
	}
}

// -- Read / Write / Delete round-trip tests -----------------------------------

func TestReadWrite_RoundTrip(t *testing.T) {
	a, _ := newTestAdapter(t)

	content := "---\ntitle: ADR-001\n---\n# Decision\n\nWe chose Go.\n"
	if err := a.Write("adr-001.md", content); err != nil {
		t.Fatalf("Write: %v", err)
	}

	doc, err := a.Read("adr-001.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if doc.Content != content {
		t.Errorf("content mismatch:\ngot:  %q\nwant: %q", doc.Content, content)
	}
	if doc.Metadata["title"] != "ADR-001" {
		t.Errorf("title: got %v, want %q", doc.Metadata["title"], "ADR-001")
	}
	if doc.Size <= 0 {
		t.Errorf("Size should be positive, got %d", doc.Size)
	}
	if doc.ModTime.IsZero() {
		t.Errorf("ModTime should not be zero")
	}
}

func TestDelete_RemovesFile(t *testing.T) {
	a, root := newTestAdapter(t)

	writeRaw(t, root, "to-delete.md", "bye")

	if err := a.Delete("to-delete.md"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "to-delete.md")); !os.IsNotExist(err) {
		t.Errorf("file should not exist after Delete")
	}
}

// -- List tests ---------------------------------------------------------------

func TestList_ReturnsFiles(t *testing.T) {
	a, root := newTestAdapter(t)

	writeRaw(t, root, "a.md", "# A")
	writeRaw(t, root, "b.md", "# B")

	docs, err := a.List(".")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
}

func TestList_SkipsSecretFiles(t *testing.T) {
	a, root := newTestAdapter(t)

	writeRaw(t, root, "visible.md", "# Visible")
	writeRaw(t, root, ".env", "SECRET=hidden")

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

func TestList_EmptyDir(t *testing.T) {
	a, _ := newTestAdapter(t)

	docs, err := a.List(".")
	if err != nil {
		t.Fatalf("List empty dir: %v", err)
	}
	if docs == nil {
		t.Errorf("List should return empty slice, not nil")
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(docs))
	}
}

// -- Watch tests --------------------------------------------------------------

// TestWatch_FiresOnWrite verifies that a Write through the adapter triggers the
// onChange callback. Rather than a fixed sleep + single write (fragile on slow
// CI where inotify setup can lag), we retry writes until events start flowing.
// Each attempt creates a fresh temp file + rename, so each one is a valid
// opportunity for the watcher to fire.
func TestWatch_FiresOnWrite(t *testing.T) {
	a, _ := newTestAdapter(t)

	var (
		mu      sync.Mutex
		changed []string
		done    = make(chan struct{}, 1)
	)

	go func() {
		// Watch blocks; ignore the error returned on watcher close.
		_ = a.Watch(".", func(path string) {
			mu.Lock()
			changed = append(changed, path)
			mu.Unlock()
			select {
			case done <- struct{}{}:
			default:
			}
		})
	}()

	// Retry writes until the watcher starts delivering events. On macOS/kqueue
	// this fires on the first attempt; on Linux/inotify, watcher goroutine
	// init can take a few hundred ms on busy CI runners.
	deadline := time.Now().Add(5 * time.Second)
	fired := false
	for time.Now().Before(deadline) && !fired {
		if err := a.Write("watched.md", "# content"); err != nil {
			t.Fatalf("Write: %v", err)
		}
		select {
		case <-done:
			fired = true
		case <-time.After(200 * time.Millisecond):
			// No event yet — watcher may still be initialising. Try again.
		}
	}
	if !fired {
		t.Error("Watch: onChange never fired after repeated writes")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(changed) == 0 {
		t.Error("Watch: expected at least one changed path, got none")
	}
}

// -- isSecretFile unit tests --------------------------------------------------

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
		{"store.p12", true},
		{"credentials.json", true},
		{"README.md", false},
		{"adr-001.md", false},
		{"config.yaml", false},
		{"my.env.md", false},     // ".env" pattern doesn't match "my.env.md"
		{"my_id_rsa.pub", false}, // "id_rsa" exact match only

		// --- draft-variant cases: WS-R hotfix (changelog item 31) ---
		// A .draft.md suffix must be stripped before the blocklist check so
		// secret files saved as drafts are still blocked.
		{".env.draft.md", true},    // core case: bug was here — first-dot strip yielded ".md", not ".env"
		{".env.draft.md.2", true},  // numbered draft variant
		{".env.draft", true},       // .draft-only suffix (no .md extension)
		{"server.pem.draft.md", true},  // pem blocklist entry with draft suffix
		{"server.key.draft.md", true},  // key blocklist entry with draft suffix
		{"id_rsa.draft.md", true},      // exact-match entry with draft suffix

		// --- false-positive traps: non-secret files must NOT be blocked ---
		// A regular doc saved as a draft is fine; it must not be falsely blocked.
		{"config.draft.md", false},   // "config" is not in the blocklist
		{"adr-001.draft.md", false},  // normal ADR draft — not a secret file
		// "secrets.draft.md": "secrets" does not match any blocklist pattern
		// (blocklist uses ".env", "*.pem", "*.key", "id_rsa", "*.p12", "credentials.json")
		// so this must be false.
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

// -- NewLocalAdapter edge-case tests ------------------------------------------

func TestNewLocalAdapter_ResolvesRelativePath(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(tmp); err != nil {
		t.Skip("cannot chdir:", err)
	}

	a, err := NewLocalAdapter(".", nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}
	if !filepath.IsAbs(a.Root()) {
		t.Errorf("Root() must be absolute, got %q", a.Root())
	}
}

func TestNewLocalAdapter_RootMatchesAbs(t *testing.T) {
	tmp := t.TempDir()
	a, err := NewLocalAdapter(tmp, nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}
	// NewLocalAdapter resolves symlinks on the workspace root via EvalSymlinks
	// so the Watch boundary check works on platforms where the temp directory
	// itself is a symlink (e.g. macOS /var/folders -> /private/var/folders).
	// The test expectation must therefore also resolve symlinks before comparing.
	want, err := filepath.EvalSymlinks(filepath.Clean(tmp))
	if err != nil {
		t.Fatalf("EvalSymlinks(tmp): %v", err)
	}
	if a.Root() != want {
		t.Errorf("Root() = %q, want %q", a.Root(), want)
	}
}
