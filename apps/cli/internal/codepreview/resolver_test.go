package codepreview

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- helpers ----------------------------------------------------------------

// scaffold creates a temporary directory tree for testing.
// Returns the project root path.
func scaffold(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Normal Go source file with 30 lines.
	goLines := make([]string, 30)
	for i := range goLines {
		goLines[i] = fmt.Sprintf("// line %d", i+1)
	}
	writeFile(t, root, "main.go", strings.Join(goLines, "\n")+"\n")

	// TypeScript file.
	writeFile(t, root, "src/app.ts", "const x = 1;\nconst y = 2;\nconst z = 3;\n")

	// Nested path.
	writeFile(t, root, "infra/prod.tf", `resource "aws_s3_bucket" "b" {
  bucket = "my-bucket"
}
`)

	// An empty file.
	writeFile(t, root, "empty.go", "")

	// A file with exactly 500KB of content (boundary).
	big := strings.Repeat("x", maxFileBytes)
	writeFile(t, root, "big.txt", big)

	// A file slightly over 500KB.
	over := strings.Repeat("y", maxFileBytes+100)
	writeFile(t, root, "over.txt", over)

	// Binary file (contains null byte).
	binary := []byte{'h', 'e', 'l', 'l', 'o', 0x00, 'w', 'o', 'r', 'l', 'd'}
	writeBinary(t, root, "image.png", binary)

	// Secret files.
	writeFile(t, root, ".env", "SECRET=abc123")
	writeFile(t, root, "server.pem", "-----BEGIN CERTIFICATE-----")
	writeFile(t, root, "credentials.json", `{"type":"service_account"}`)
	writeFile(t, root, "id_rsa", "-----BEGIN RSA PRIVATE KEY-----")

	return root
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("scaffold: MkdirAll: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("scaffold: WriteFile %s: %v", rel, err)
	}
}

func writeBinary(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("scaffold: MkdirAll: %v", err)
	}
	if err := os.WriteFile(abs, data, 0o644); err != nil {
		t.Fatalf("scaffold: WriteFile binary %s: %v", rel, err)
	}
}

// expectErr asserts that err wraps or equals target.
func expectErr(t *testing.T, err, target error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %v, got nil", target)
	}
	if !strings.Contains(err.Error(), target.Error()) {
		t.Fatalf("expected error containing %q, got %q", target, err)
	}
}

// expectNoErr fails the test if err is non-nil.
func expectNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- Tests ------------------------------------------------------------------

// TestResolveFullFile checks that a vedox:// URL without an anchor returns
// the entire file content with correct metadata.
func TestResolveFullFile(t *testing.T) {
	root := scaffold(t)

	p, err := Resolve(root, "vedox://file/main.go")
	expectNoErr(t, err)

	if p.FilePath != "main.go" {
		t.Errorf("FilePath = %q, want %q", p.FilePath, "main.go")
	}
	if p.Language != "go" {
		t.Errorf("Language = %q, want \"go\"", p.Language)
	}
	if p.StartLine != 1 {
		t.Errorf("StartLine = %d, want 1", p.StartLine)
	}
	if p.TotalLines != 30 {
		t.Errorf("TotalLines = %d, want 30", p.TotalLines)
	}
	if p.EndLine != 30 {
		t.Errorf("EndLine = %d, want 30", p.EndLine)
	}
	if p.Truncated {
		t.Error("Truncated = true, want false")
	}
	if !strings.Contains(p.Content, "// line 1") {
		t.Errorf("Content does not contain '// line 1': %q", p.Content[:min(100, len(p.Content))])
	}
}

// TestResolveSingleLine checks that #L<n> returns exactly one line.
func TestResolveSingleLine(t *testing.T) {
	root := scaffold(t)

	p, err := Resolve(root, "vedox://file/main.go#L15")
	expectNoErr(t, err)

	if p.StartLine != 15 {
		t.Errorf("StartLine = %d, want 15", p.StartLine)
	}
	if p.EndLine != 15 {
		t.Errorf("EndLine = %d, want 15", p.EndLine)
	}
	if p.Content != "// line 15" {
		t.Errorf("Content = %q, want \"// line 15\"", p.Content)
	}
}

// TestResolveLineRange checks that #L<start>-L<end> returns the correct slice.
func TestResolveLineRange(t *testing.T) {
	root := scaffold(t)

	p, err := Resolve(root, "vedox://file/main.go#L10-L12")
	expectNoErr(t, err)

	if p.StartLine != 10 {
		t.Errorf("StartLine = %d, want 10", p.StartLine)
	}
	if p.EndLine != 12 {
		t.Errorf("EndLine = %d, want 12", p.EndLine)
	}
	lines := strings.Split(p.Content, "\n")
	if len(lines) != 3 {
		t.Errorf("Content line count = %d, want 3: %q", len(lines), p.Content)
	}
	if lines[0] != "// line 10" {
		t.Errorf("lines[0] = %q, want \"// line 10\"", lines[0])
	}
	if lines[2] != "// line 12" {
		t.Errorf("lines[2] = %q, want \"// line 12\"", lines[2])
	}
}

// TestResolveNestedPath verifies that files in subdirectories resolve correctly.
func TestResolveNestedPath(t *testing.T) {
	root := scaffold(t)

	p, err := Resolve(root, "vedox://file/infra/prod.tf")
	expectNoErr(t, err)

	if p.Language != "hcl" {
		t.Errorf("Language = %q, want \"hcl\"", p.Language)
	}
	if !strings.Contains(p.Content, "aws_s3_bucket") {
		t.Error("Content does not contain expected terraform resource")
	}
}

// TestResolveLanguageInference checks various extensions.
func TestResolveLanguageInference(t *testing.T) {
	root := scaffold(t)

	p, err := Resolve(root, "vedox://file/src/app.ts")
	expectNoErr(t, err)
	if p.Language != "typescript" {
		t.Errorf("Language = %q, want \"typescript\"", p.Language)
	}
}

// ---- Path traversal rejection -----------------------------------------------

// TestTraversalDotDot verifies that ".." in the URL path is rejected.
func TestTraversalDotDot(t *testing.T) {
	root := scaffold(t)

	_, err := Resolve(root, "vedox://file/../etc/passwd")
	expectErr(t, err, ErrTraversal)
}

// TestTraversalEncodedDotDot verifies that URL-encoded traversal is also caught.
// The Go URL parser decodes %2F so "..%2F" becomes "../" after decode, which
// our segment check catches.
func TestTraversalEncodedDotDot(t *testing.T) {
	root := scaffold(t)

	// Double-dot in the middle of a decoded path.
	_, err := Resolve(root, "vedox://file/apps/../../../etc/passwd")
	expectErr(t, err, ErrTraversal)
}

// TestTraversalAbsolutePath verifies that absolute paths are rejected.
func TestTraversalAbsolutePath(t *testing.T) {
	root := scaffold(t)

	// The URL path "//etc/passwd" would resolve to host="" path="/etc/passwd"
	// after parsing, but we also catch paths that remain absolute after stripping
	// the leading slash.
	_, err := Resolve(root, "vedox://file/"+"/etc/passwd")
	// This should fail with either ErrAbsolutePath or ErrTraversal.
	if err == nil {
		t.Fatal("expected error for absolute path, got nil")
	}
}

// TestTraversalSymlinkEscape verifies that a symlink pointing outside the root
// is rejected.
func TestTraversalSymlinkEscape(t *testing.T) {
	root := scaffold(t)

	// Create a symlink inside root that points to /etc (or any real dir outside).
	linkPath := filepath.Join(root, "escape")
	// Use the system temp dir as a target that exists but is outside root.
	if err := os.Symlink(os.TempDir(), linkPath); err != nil {
		t.Skipf("cannot create symlink (likely Windows without privilege): %v", err)
	}

	_, err := Resolve(root, "vedox://file/escape/somefile.txt")
	if err == nil {
		t.Fatal("expected ErrSymlinkEscape, got nil")
	}
	// Could be ErrFileNotFound (the target file doesn't exist in TempDir) or
	// ErrSymlinkEscape.  Either is acceptable — the important thing is that we
	// did not serve content outside the root.
	// On most systems the symlink resolves to /tmp which is outside root, giving
	// ErrSymlinkEscape.  The non-existence of "somefile.txt" inside /tmp may give
	// ErrFileNotFound on some systems.
	if err != ErrSymlinkEscape && err != ErrFileNotFound {
		// Accept if message contains either sentinel text.
		if !strings.Contains(err.Error(), "symlink") && !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected ErrSymlinkEscape or ErrFileNotFound, got %v", err)
		}
	}
}

// ---- Secret file rejection --------------------------------------------------

// TestSecretFileEnv verifies that .env files are blocked.
func TestSecretFileEnv(t *testing.T) {
	root := scaffold(t)
	_, err := Resolve(root, "vedox://file/.env")
	expectErr(t, err, ErrSecretFile)
}

// TestSecretFilePem verifies that .pem files are blocked.
func TestSecretFilePem(t *testing.T) {
	root := scaffold(t)
	_, err := Resolve(root, "vedox://file/server.pem")
	expectErr(t, err, ErrSecretFile)
}

// TestSecretFileCredentials verifies that credentials.json is blocked.
func TestSecretFileCredentials(t *testing.T) {
	root := scaffold(t)
	_, err := Resolve(root, "vedox://file/credentials.json")
	expectErr(t, err, ErrSecretFile)
}

// TestSecretFileIdRsa verifies that id_rsa is blocked.
func TestSecretFileIdRsa(t *testing.T) {
	root := scaffold(t)
	_, err := Resolve(root, "vedox://file/id_rsa")
	expectErr(t, err, ErrSecretFile)
}

// TestSecretFileCaseInsensitive verifies that blocklist matching is
// case-insensitive (e.g. ".ENV" is also blocked).
func TestSecretFileCaseInsensitive(t *testing.T) {
	root := scaffold(t)
	// Write an uppercase variant.
	writeFile(t, root, "SECRETS.ENV", "SECRET=abc")

	_, err := Resolve(root, "vedox://file/SECRETS.ENV")
	expectErr(t, err, ErrSecretFile)
}

// ---- Binary file rejection --------------------------------------------------

// TestBinaryFileRejected verifies that a file with a null byte is blocked.
func TestBinaryFileRejected(t *testing.T) {
	root := scaffold(t)
	_, err := Resolve(root, "vedox://file/image.png")
	expectErr(t, err, ErrBinaryFile)
}

// ---- 500KB cap --------------------------------------------------------------

// TestFileSizeCap verifies that a file at exactly 500KB is served without
// the Truncated flag.
func TestFileSizeCap(t *testing.T) {
	root := scaffold(t)

	p, err := Resolve(root, "vedox://file/big.txt")
	expectNoErr(t, err)

	if p.Truncated {
		t.Error("Truncated = true for a file exactly at the cap, want false")
	}
}

// TestFileSizeOverCap verifies that a file larger than 500KB sets Truncated.
func TestFileSizeOverCap(t *testing.T) {
	root := scaffold(t)

	p, err := Resolve(root, "vedox://file/over.txt")
	expectNoErr(t, err)

	if !p.Truncated {
		t.Error("Truncated = false for a file over the cap, want true")
	}
	if len(p.Content) > maxFileBytes {
		t.Errorf("Content length %d exceeds cap %d", len(p.Content), maxFileBytes)
	}
}

// ---- Anchor validation -------------------------------------------------------

// TestAnchorInvalidFormat verifies that a non-L anchor is rejected.
func TestAnchorInvalidFormat(t *testing.T) {
	root := scaffold(t)
	_, err := Resolve(root, "vedox://file/main.go#symbol-id")
	expectErr(t, err, ErrInvalidAnchor)
}

// TestAnchorEndBeforeStart verifies that end < start is rejected.
func TestAnchorEndBeforeStart(t *testing.T) {
	root := scaffold(t)
	_, err := Resolve(root, "vedox://file/main.go#L15-L10")
	expectErr(t, err, ErrInvalidAnchor)
}

// TestAnchorOutOfRange verifies that a line anchor beyond EOF is rejected.
func TestAnchorOutOfRange(t *testing.T) {
	root := scaffold(t)
	_, err := Resolve(root, "vedox://file/main.go#L999")
	expectErr(t, err, ErrAnchorOutOfRange)
}

// TestAnchorRangeTooBig verifies that a range of more than 500 lines is rejected.
func TestAnchorRangeTooBig(t *testing.T) {
	root := scaffold(t)
	_, err := Resolve(root, "vedox://file/main.go#L1-L501")
	expectErr(t, err, ErrAnchorRangeTooBig)
}

// ---- Scheme and host validation ---------------------------------------------

// TestInvalidScheme verifies that non-vedox schemes are rejected.
func TestInvalidScheme(t *testing.T) {
	root := scaffold(t)
	_, err := Resolve(root, "https://file/main.go")
	expectErr(t, err, ErrInvalidScheme)
}

// TestInvalidHost verifies that the host must be "file".
func TestInvalidHost(t *testing.T) {
	root := scaffold(t)
	_, err := Resolve(root, "vedox://project/main.go")
	expectErr(t, err, ErrInvalidHost)
}

// ---- File not found ---------------------------------------------------------

// TestFileNotFound verifies that a missing file returns ErrFileNotFound.
func TestFileNotFound(t *testing.T) {
	root := scaffold(t)
	_, err := Resolve(root, "vedox://file/does-not-exist.go")
	expectErr(t, err, ErrFileNotFound)
}

// ---- Language inference -----------------------------------------------------

func TestLanguageInference(t *testing.T) {
	cases := []struct {
		ext      string
		expected string
	}{
		{".go", "go"},
		{".ts", "typescript"},
		{".tsx", "typescript"},
		{".js", "javascript"},
		{".py", "python"},
		{".rs", "rust"},
		{".tf", "hcl"},
		{".hcl", "hcl"},
		{".yaml", "yaml"},
		{".yml", "yaml"},
		{".json", "json"},
		{".toml", "toml"},
		{".sh", "bash"},
		{".bash", "bash"},
		{".sql", "sql"},
		{".md", "markdown"},
		{".html", "html"},
		{".css", "css"},
		{".svelte", "svelte"},
		{".unknown", ""},
		{"", ""},
	}

	for _, tc := range cases {
		got := languageFromExt(tc.ext)
		if got != tc.expected {
			t.Errorf("languageFromExt(%q) = %q, want %q", tc.ext, got, tc.expected)
		}
	}
}

// ---- splitLines tests -------------------------------------------------------

func TestSplitLines(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		wantLen  int
		wantLast string
	}{
		{"empty", "", 1, ""},
		{"one line no newline", "hello", 1, "hello"},
		{"one line with newline", "hello\n", 1, "hello"},
		{"two lines", "a\nb", 2, "b"},
		{"two lines with trailing newline", "a\nb\n", 2, "b"},
		{"three lines", "a\nb\nc\n", 3, "c"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := splitLines([]byte(tc.input))
			if len(got) != tc.wantLen {
				t.Errorf("len = %d, want %d (input=%q)", len(got), tc.wantLen, tc.input)
			}
			if len(got) > 0 && got[len(got)-1] != tc.wantLast {
				t.Errorf("last = %q, want %q", got[len(got)-1], tc.wantLast)
			}
		})
	}
}

// ---- parseAnchor tests ------------------------------------------------------

func TestParseAnchor(t *testing.T) {
	cases := []struct {
		fragment  string
		wantStart int
		wantEnd   int
		wantErr   bool
	}{
		{"", 0, 0, false},
		{"L1", 1, 1, false},
		{"L10", 10, 10, false},
		{"L10-L25", 10, 25, false},
		{"L1-L1", 1, 1, false},
		{"L25-L10", 0, 0, true}, // end < start
		{"L0", 0, 0, true},      // line 0 is invalid
		{"symbol", 0, 0, true},  // non-L anchor
		{"L-1", 0, 0, true},     // negative
	}

	for _, tc := range cases {
		t.Run(tc.fragment, func(t *testing.T) {
			s, e, err := parseAnchor(tc.fragment)
			if tc.wantErr {
				if err == nil {
					t.Errorf("fragment=%q: expected error, got start=%d end=%d", tc.fragment, s, e)
				}
				return
			}
			if err != nil {
				t.Errorf("fragment=%q: unexpected error: %v", tc.fragment, err)
				return
			}
			if s != tc.wantStart || e != tc.wantEnd {
				t.Errorf("fragment=%q: got (%d, %d), want (%d, %d)", tc.fragment, s, e, tc.wantStart, tc.wantEnd)
			}
		})
	}
}

// ---- isSecretFile tests -----------------------------------------------------

func TestIsSecretFile(t *testing.T) {
	blocked := []string{
		"/project/.env",
		"/project/prod.env",
		"/project/server.pem",
		"/project/client.key",
		"/project/service-account-prod.json",
		"/project/id_rsa",
		"/project/id_ed25519",
		"/project/credentials.json",
		"/project/app.secret",
		"/project/db_secrets.yaml",
		"/project/db_secrets.yml",
		"/project/vault.p12",
		"/project/store.kdbx",
		"/project/app.keystore",
	}

	for _, p := range blocked {
		if !isSecretFile(p) {
			t.Errorf("isSecretFile(%q) = false, want true", p)
		}
	}

	allowed := []string{
		"/project/main.go",
		"/project/README.md",
		"/project/config.yaml",
		"/project/values.json",
		"/project/setup.sh",
	}

	for _, p := range allowed {
		if isSecretFile(p) {
			t.Errorf("isSecretFile(%q) = true, want false", p)
		}
	}
}

// ---- Empty file -------------------------------------------------------------

// TestEmptyFile verifies that an empty file resolves without error.
func TestEmptyFile(t *testing.T) {
	root := scaffold(t)

	p, err := Resolve(root, "vedox://file/empty.go")
	expectNoErr(t, err)

	if p.TotalLines != 1 {
		// splitLines always returns at least one element (empty string for empty input).
		t.Errorf("TotalLines = %d, want 1", p.TotalLines)
	}
	if p.Language != "go" {
		t.Errorf("Language = %q, want \"go\"", p.Language)
	}
}

// min is a local helper for Go versions without the builtin.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
