package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("writeFile mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

func makeDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("makeDir %s: %v", path, err)
	}
}

// TestDetect_Astro_mjs verifies astro.config.mjs detection.
func TestDetect_Astro_mjs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "astro.config.mjs"))

	if got := DetectFramework(dir); got != FrameworkAstro {
		t.Errorf("got %q, want %q", got, FrameworkAstro)
	}
}

// TestDetect_Astro_ts verifies astro.config.ts detection.
func TestDetect_Astro_ts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "astro.config.ts"))

	if got := DetectFramework(dir); got != FrameworkAstro {
		t.Errorf("got %q, want %q", got, FrameworkAstro)
	}
}

// TestDetect_MkDocs verifies mkdocs.yml detection.
func TestDetect_MkDocs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mkdocs.yml"))

	if got := DetectFramework(dir); got != FrameworkMkDocs {
		t.Errorf("got %q, want %q", got, FrameworkMkDocs)
	}
}

// TestDetect_Jekyll verifies that both _config.yml AND Gemfile must be present.
func TestDetect_Jekyll(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "_config.yml"))
	writeFile(t, filepath.Join(dir, "Gemfile"))

	if got := DetectFramework(dir); got != FrameworkJekyll {
		t.Errorf("got %q, want %q", got, FrameworkJekyll)
	}
}

// TestDetect_Jekyll_OnlyConfig verifies that _config.yml alone is not enough
// to trigger Jekyll detection.
func TestDetect_Jekyll_OnlyConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "_config.yml"))
	// No Gemfile.

	got := DetectFramework(dir)
	if got == FrameworkJekyll {
		t.Errorf("expected non-jekyll result when Gemfile is absent, got %q", got)
	}
}

// TestDetect_Jekyll_OnlyGemfile verifies that Gemfile alone is not enough.
func TestDetect_Jekyll_OnlyGemfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Gemfile"))
	// No _config.yml.

	got := DetectFramework(dir)
	if got == FrameworkJekyll {
		t.Errorf("expected non-jekyll result when _config.yml is absent, got %q", got)
	}
}

// TestDetect_Docusaurus verifies .docusaurus directory detection.
func TestDetect_Docusaurus(t *testing.T) {
	dir := t.TempDir()
	makeDir(t, filepath.Join(dir, ".docusaurus"))

	if got := DetectFramework(dir); got != FrameworkDocusaurus {
		t.Errorf("got %q, want %q", got, FrameworkDocusaurus)
	}
}

// TestDetect_Bare verifies that a project with only README.md gets "bare".
func TestDetect_Bare(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "README.md"))

	if got := DetectFramework(dir); got != FrameworkBare {
		t.Errorf("got %q, want %q", got, FrameworkBare)
	}
}

// TestDetect_Unknown verifies that an empty directory returns "unknown".
func TestDetect_Unknown(t *testing.T) {
	dir := t.TempDir()

	if got := DetectFramework(dir); got != FrameworkUnknown {
		t.Errorf("got %q, want %q", got, FrameworkUnknown)
	}
}

// TestDetect_Priority_AstroOverBare verifies that astro beats bare README.
func TestDetect_Priority_AstroOverBare(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "README.md"))
	writeFile(t, filepath.Join(dir, "astro.config.mjs"))

	if got := DetectFramework(dir); got != FrameworkAstro {
		t.Errorf("expected astro to win over bare, got %q", got)
	}
}

// TestDetect_Priority_MkDocsOverBare verifies mkdocs beats bare README.
func TestDetect_Priority_MkDocsOverBare(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "README.md"))
	writeFile(t, filepath.Join(dir, "mkdocs.yml"))

	if got := DetectFramework(dir); got != FrameworkMkDocs {
		t.Errorf("expected mkdocs to win over bare, got %q", got)
	}
}
