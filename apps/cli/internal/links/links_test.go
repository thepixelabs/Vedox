package links

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeLinksFile writes raw bytes to .vedox/links.json inside root.
func writeLinksFile(t *testing.T, root string, content []byte) {
	t.Helper()
	dir := filepath.Join(root, ".vedox")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "links.json"), content, 0o644); err != nil {
		t.Fatalf("WriteFile links.json: %v", err)
	}
}

// loadedProjects is a convenience that calls Load and fails the test on error.
func loadedProjects(t *testing.T, root string) []LinkedProject {
	t.Helper()
	projects, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return projects
}

// ── Load ──────────────────────────────────────────────────────────────────────

func TestLoad_MissingFile_ReturnsEmptySlice(t *testing.T) {
	root := t.TempDir()

	projects, err := Load(root)
	if err != nil {
		t.Fatalf("Load on missing file should succeed, got: %v", err)
	}
	if projects == nil {
		t.Fatal("Load should return non-nil slice on missing file")
	}
	if len(projects) != 0 {
		t.Errorf("expected empty slice, got %d items", len(projects))
	}
}

func TestLoad_ValidFile_ParsesEntries(t *testing.T) {
	root := t.TempDir()
	content := `{"links":[{"projectName":"my-api","externalRoot":"/projects/my-api"},{"projectName":"ui","externalRoot":"/projects/ui"}]}`
	writeLinksFile(t, root, []byte(content))

	projects := loadedProjects(t, root)
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	if projects[0].ProjectName != "my-api" {
		t.Errorf("projects[0].ProjectName = %q, want %q", projects[0].ProjectName, "my-api")
	}
	if projects[0].ExternalRoot != "/projects/my-api" {
		t.Errorf("projects[0].ExternalRoot = %q, want %q", projects[0].ExternalRoot, "/projects/my-api")
	}
}

func TestLoad_EmptyLinksArray_ReturnsEmptySlice(t *testing.T) {
	root := t.TempDir()
	writeLinksFile(t, root, []byte(`{"links":[]}`))

	projects := loadedProjects(t, root)
	if len(projects) != 0 {
		t.Errorf("expected empty slice for empty links array, got %d", len(projects))
	}
}

func TestLoad_NullLinksField_ReturnsEmptySlice(t *testing.T) {
	// Covers the reg.Links == nil guard in Load.
	root := t.TempDir()
	writeLinksFile(t, root, []byte(`{"links":null}`))

	projects := loadedProjects(t, root)
	if projects == nil {
		t.Fatal("Load should return non-nil slice for null links field")
	}
	if len(projects) != 0 {
		t.Errorf("expected empty slice for null links, got %d", len(projects))
	}
}

func TestLoad_InvalidJSON_ReturnsError(t *testing.T) {
	root := t.TempDir()
	writeLinksFile(t, root, []byte(`{not valid json`))

	_, err := Load(root)
	if err == nil {
		t.Fatal("Load on invalid JSON should return an error")
	}
}

// ── Save ──────────────────────────────────────────────────────────────────────

func TestSave_CreatesDirectoryAndFile(t *testing.T) {
	root := t.TempDir()
	projects := []LinkedProject{
		{ProjectName: "alpha", ExternalRoot: "/srv/alpha"},
	}

	if err := Save(root, projects); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := filepath.Join(root, ".vedox", "links.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Save should create .vedox/links.json")
	}
}

func TestSave_ContentIsValidJSON(t *testing.T) {
	root := t.TempDir()
	projects := []LinkedProject{
		{ProjectName: "beta", ExternalRoot: "/srv/beta"},
	}

	if err := Save(root, projects); err != nil {
		t.Fatalf("Save: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, ".vedox", "links.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var reg registry
	if err := json.Unmarshal(raw, &reg); err != nil {
		t.Fatalf("saved file is not valid JSON: %v", err)
	}
	if len(reg.Links) != 1 || reg.Links[0].ProjectName != "beta" {
		t.Errorf("saved content mismatch: %+v", reg)
	}
}

func TestSave_NilSlice_WritesEmptyArray(t *testing.T) {
	root := t.TempDir()

	if err := Save(root, nil); err != nil {
		t.Fatalf("Save with nil slice: %v", err)
	}

	loaded := loadedProjects(t, root)
	if len(loaded) != 0 {
		t.Errorf("nil save should round-trip to empty slice, got %d", len(loaded))
	}
}

func TestSave_IsIdempotent(t *testing.T) {
	root := t.TempDir()
	projects := []LinkedProject{{ProjectName: "svc", ExternalRoot: "/srv/svc"}}

	for i := 0; i < 3; i++ {
		if err := Save(root, projects); err != nil {
			t.Fatalf("Save iteration %d: %v", i, err)
		}
	}

	loaded := loadedProjects(t, root)
	if len(loaded) != 1 {
		t.Errorf("repeated Save should not duplicate entries, got %d", len(loaded))
	}
}

// ── Round-trip: Save → Load ────────────────────────────────────────────────

func TestSaveLoad_RoundTrip(t *testing.T) {
	root := t.TempDir()
	want := []LinkedProject{
		{ProjectName: "docs", ExternalRoot: "/workspace/docs"},
		{ProjectName: "api", ExternalRoot: "/workspace/api"},
	}

	if err := Save(root, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got := loadedProjects(t, root)
	if len(got) != len(want) {
		t.Fatalf("round-trip length: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ProjectName != want[i].ProjectName || got[i].ExternalRoot != want[i].ExternalRoot {
			t.Errorf("round-trip[%d]: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

// ── Add ───────────────────────────────────────────────────────────────────────

func TestAdd_AppendsNewEntry(t *testing.T) {
	root := t.TempDir()

	entry := LinkedProject{ProjectName: "new-svc", ExternalRoot: "/srv/new-svc"}
	if err := Add(root, entry); err != nil {
		t.Fatalf("Add: %v", err)
	}

	projects := loadedProjects(t, root)
	if len(projects) != 1 {
		t.Fatalf("expected 1 project after Add, got %d", len(projects))
	}
	if projects[0].ProjectName != "new-svc" {
		t.Errorf("ProjectName = %q, want %q", projects[0].ProjectName, "new-svc")
	}
}

func TestAdd_ReplacesExistingEntry(t *testing.T) {
	root := t.TempDir()

	original := LinkedProject{ProjectName: "my-svc", ExternalRoot: "/old/path"}
	if err := Add(root, original); err != nil {
		t.Fatalf("Add original: %v", err)
	}

	updated := LinkedProject{ProjectName: "my-svc", ExternalRoot: "/new/path"}
	if err := Add(root, updated); err != nil {
		t.Fatalf("Add updated: %v", err)
	}

	projects := loadedProjects(t, root)
	if len(projects) != 1 {
		t.Fatalf("re-add should replace, not duplicate; got %d entries", len(projects))
	}
	if projects[0].ExternalRoot != "/new/path" {
		t.Errorf("ExternalRoot = %q, want /new/path", projects[0].ExternalRoot)
	}
}

func TestAdd_MultipleDistinctProjects(t *testing.T) {
	root := t.TempDir()

	entries := []LinkedProject{
		{ProjectName: "a", ExternalRoot: "/srv/a"},
		{ProjectName: "b", ExternalRoot: "/srv/b"},
		{ProjectName: "c", ExternalRoot: "/srv/c"},
	}
	for _, e := range entries {
		if err := Add(root, e); err != nil {
			t.Fatalf("Add %s: %v", e.ProjectName, err)
		}
	}

	projects := loadedProjects(t, root)
	if len(projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(projects))
	}
}

// ── Remove ────────────────────────────────────────────────────────────────────

func TestRemove_ExistingEntry_RemovesIt(t *testing.T) {
	root := t.TempDir()

	entries := []LinkedProject{
		{ProjectName: "keep", ExternalRoot: "/srv/keep"},
		{ProjectName: "delete-me", ExternalRoot: "/srv/delete-me"},
	}
	if err := Save(root, entries); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := Remove(root, "delete-me"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	projects := loadedProjects(t, root)
	if len(projects) != 1 {
		t.Fatalf("expected 1 project after Remove, got %d", len(projects))
	}
	if projects[0].ProjectName != "keep" {
		t.Errorf("remaining project = %q, want %q", projects[0].ProjectName, "keep")
	}
}

func TestRemove_NonexistentName_IsNoOp(t *testing.T) {
	root := t.TempDir()

	existing := []LinkedProject{{ProjectName: "svc", ExternalRoot: "/srv/svc"}}
	if err := Save(root, existing); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := Remove(root, "does-not-exist"); err != nil {
		t.Fatalf("Remove of nonexistent name should not error: %v", err)
	}

	projects := loadedProjects(t, root)
	if len(projects) != 1 {
		t.Errorf("Remove of unknown name should leave list unchanged, got %d", len(projects))
	}
}

func TestRemove_EmptyRegistry_IsNoOp(t *testing.T) {
	root := t.TempDir()
	// No prior Save — links.json does not exist yet.

	if err := Remove(root, "anything"); err != nil {
		t.Fatalf("Remove on empty registry should not error: %v", err)
	}

	projects := loadedProjects(t, root)
	if len(projects) != 0 {
		t.Errorf("Remove on empty registry should leave empty list, got %d", len(projects))
	}
}

func TestRemove_AllEntries_LeavesEmptyList(t *testing.T) {
	root := t.TempDir()

	if err := Save(root, []LinkedProject{{ProjectName: "only", ExternalRoot: "/srv/only"}}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := Remove(root, "only"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	projects := loadedProjects(t, root)
	if len(projects) != 0 {
		t.Errorf("expected empty list after removing last entry, got %d", len(projects))
	}
}
