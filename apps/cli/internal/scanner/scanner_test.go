package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

// mkGitDir creates a bare .git directory inside dir to simulate a project root.
func mkGitDir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkGitDir: %v", err)
	}
}

// mkDir creates a directory tree, failing the test on error.
func mkDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkDir %s: %v", path, err)
	}
}

// mkFile creates a file with empty content.
func mkFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkFile dir: %v", err)
	}
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("mkFile %s: %v", path, err)
	}
}

// TestScan_DiscoversSingleProject verifies that a project root directly under
// the workspace root is discovered.
func TestScan_DiscoversSingleProject(t *testing.T) {
	ws := t.TempDir()
	mkGitDir(t, filepath.Join(ws, "myproject"))

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "myproject" {
		t.Errorf("expected name 'myproject', got %q", projects[0].Name)
	}
}

// TestScan_MultipleProjects verifies multiple projects at depth 1 are all found.
func TestScan_MultipleProjects(t *testing.T) {
	ws := t.TempDir()
	names := []string{"alpha", "beta", "gamma"}
	for _, n := range names {
		mkGitDir(t, filepath.Join(ws, n))
	}

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(projects))
	}
	// Results must be sorted by Name.
	for i, want := range names { // names already sorted alphabetically
		if projects[i].Name != want {
			t.Errorf("projects[%d].Name = %q, want %q", i, projects[i].Name, want)
		}
	}
}

// TestScan_SortedByName confirms the sort invariant.
func TestScan_SortedByName(t *testing.T) {
	ws := t.TempDir()
	mkGitDir(t, filepath.Join(ws, "zz-last"))
	mkGitDir(t, filepath.Join(ws, "aa-first"))
	mkGitDir(t, filepath.Join(ws, "mm-middle"))

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := []string{"aa-first", "mm-middle", "zz-last"}
	for i, w := range want {
		if projects[i].Name != w {
			t.Errorf("projects[%d] = %q, want %q", i, projects[i].Name, w)
		}
	}
}

// TestScan_NestedProjectAtDepth2 verifies projects nested deeper than direct
// children of the workspace are still discovered (up to maxDepth).
func TestScan_NestedProjectAtDepth2(t *testing.T) {
	ws := t.TempDir()
	// ws/org/repo/.git
	mkGitDir(t, filepath.Join(ws, "org", "repo"))

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "repo" {
		t.Errorf("expected name 'repo', got %q", projects[0].Name)
	}
}

// TestScan_ExceedsMaxDepth verifies that projects buried deeper than maxDepth
// are NOT discovered.
func TestScan_ExceedsMaxDepth(t *testing.T) {
	ws := t.TempDir()

	// Build a path at depth maxDepth+1.
	// ws/d1/d2/d3/d4/d5/deep-project/.git
	deep := ws
	for i := 0; i <= maxDepth; i++ {
		deep = filepath.Join(deep, "d")
	}
	mkGitDir(t, filepath.Join(deep, "deep-project"))

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects beyond maxDepth, got %d", len(projects))
	}
}

// TestScan_SkipsNodeModules ensures node_modules directories are not traversed.
func TestScan_SkipsNodeModules(t *testing.T) {
	ws := t.TempDir()
	// Placing .git inside node_modules — should never be found.
	mkGitDir(t, filepath.Join(ws, "node_modules", "some-pkg"))

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d (node_modules should be skipped)", len(projects))
	}
}

// TestScan_SkipsVendor ensures vendor directories are not traversed.
func TestScan_SkipsVendor(t *testing.T) {
	ws := t.TempDir()
	mkGitDir(t, filepath.Join(ws, "vendor", "upstream-lib"))

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d (vendor should be skipped)", len(projects))
	}
}

// TestScan_SkipsHiddenDirs ensures hidden directories (. prefix) are not traversed.
func TestScan_SkipsHiddenDirs(t *testing.T) {
	ws := t.TempDir()
	// .hidden/project/.git should not be found.
	mkGitDir(t, filepath.Join(ws, ".hidden", "project"))

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d (hidden dirs should be skipped)", len(projects))
	}
}

// TestScan_DoesNotDescendIntoProjectSubdirs verifies that once a project root
// is found, its subdirectories are not scanned for nested projects.
func TestScan_DoesNotDescendIntoProjectSubdirs(t *testing.T) {
	ws := t.TempDir()
	// Outer project.
	outer := filepath.Join(ws, "outer")
	mkGitDir(t, outer)
	// Inner project nested inside outer (a git submodule scenario).
	mkGitDir(t, filepath.Join(outer, "submodule"))

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	// Only the outer project should be returned; we don't recurse into it.
	if len(projects) != 1 {
		t.Errorf("expected 1 project (outer), got %d", len(projects))
	}
	if projects[0].Name != "outer" {
		t.Errorf("expected 'outer', got %q", projects[0].Name)
	}
}

// TestScan_DocCount verifies that DocCount reflects the number of .md files
// found under the project.
func TestScan_DocCount(t *testing.T) {
	ws := t.TempDir()
	proj := filepath.Join(ws, "myproject")
	mkGitDir(t, proj)
	mkFile(t, filepath.Join(proj, "README.md"))
	mkFile(t, filepath.Join(proj, "docs", "guide.md"))
	mkFile(t, filepath.Join(proj, "docs", "api.md"))
	mkFile(t, filepath.Join(proj, "main.go")) // not a markdown file

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].DocCount != 3 {
		t.Errorf("expected DocCount=3, got %d", projects[0].DocCount)
	}
}

// TestScan_RelPath checks that RelPath is relative to the workspace root.
func TestScan_RelPath(t *testing.T) {
	ws := t.TempDir()
	mkGitDir(t, filepath.Join(ws, "org", "repo"))

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	want := filepath.Join("org", "repo")
	if projects[0].RelPath != want {
		t.Errorf("RelPath = %q, want %q", projects[0].RelPath, want)
	}
}

// TestScan_EmptyWorkspace verifies that an empty workspace returns an empty
// (non-nil) slice.
func TestScan_EmptyWorkspace(t *testing.T) {
	ws := t.TempDir()

	s := NewScanner()
	projects, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if projects == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

// TestScan_CacheHit verifies that a second scan returns cached results when
// the project root mtime has not changed.
func TestScan_CacheHit(t *testing.T) {
	ws := t.TempDir()
	proj := filepath.Join(ws, "myproject")
	mkGitDir(t, proj)
	mkFile(t, filepath.Join(proj, "README.md"))

	s := NewScanner()

	// First scan — populates cache.
	first, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("first Scan: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first Scan: expected 1 project, got %d", len(first))
	}
	firstScanned := first[0].LastScanned

	// Second scan — should hit cache (same mtime).
	second, err := s.Scan(ws)
	if err != nil {
		t.Fatalf("second Scan: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("second Scan: expected 1 project, got %d", len(second))
	}

	// The LastScanned timestamp should be from the first scan (cached).
	if !second[0].LastScanned.Equal(firstScanned) {
		t.Errorf("expected cache hit (same LastScanned), but timestamps differ: first=%v second=%v",
			firstScanned, second[0].LastScanned)
	}
}
