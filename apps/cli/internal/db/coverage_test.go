package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers shared across coverage tests
// ---------------------------------------------------------------------------

// minimalDoc builds a valid Doc with the given id and body. All required
// fields are populated so UpsertDoc succeeds without callers repeating the
// boilerplate.
func minimalDoc(id, body string) *Doc {
	return &Doc{
		ID:          id,
		Project:     "test-project",
		Title:       id,
		Type:        "how-to",
		Status:      "published",
		Date:        "2026-04-13",
		ContentHash: fmt.Sprintf("%x", id),
		ModTime:     "2026-04-13T00:00:00Z",
		Size:        int64(len(body)),
		Body:        body,
	}
}

// ---------------------------------------------------------------------------
// Store.Path
// ---------------------------------------------------------------------------

// TestStore_Path verifies that Path() returns the database file location.
func TestStore_Path(t *testing.T) {
	s := openStore(t, t.TempDir())
	p := s.Path()
	if p == "" {
		t.Error("Path() returned empty string")
	}
	if !strings.Contains(p, "index.db") {
		t.Errorf("Path() %q does not contain index.db", p)
	}
}

// ---------------------------------------------------------------------------
// UpsertDoc — error paths
// ---------------------------------------------------------------------------

// TestUpsertDoc_NilDoc verifies that passing nil returns an error.
func TestUpsertDoc_NilDoc(t *testing.T) {
	s := openStore(t, t.TempDir())
	if err := s.UpsertDoc(context.Background(), nil); err == nil {
		t.Error("expected error for nil doc, got nil")
	}
}

// TestUpsertDoc_EmptyID verifies that a doc with an empty ID is rejected.
func TestUpsertDoc_EmptyID(t *testing.T) {
	s := openStore(t, t.TempDir())
	if err := s.UpsertDoc(context.Background(), &Doc{}); err == nil {
		t.Error("expected error for empty doc.ID, got nil")
	}
}

// TestUpsertDoc_Update verifies that upserting the same ID twice updates the
// existing row rather than inserting a duplicate.
func TestUpsertDoc_Update(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())

	if err := s.UpsertDoc(ctx, minimalDoc("doc.md", "original body")); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	// Update with a different body — doc count must stay 1.
	if err := s.UpsertDoc(ctx, minimalDoc("doc.md", "updated body")); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	n, err := s.CountDocs(ctx)
	if err != nil {
		t.Fatalf("CountDocs: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 doc after two upserts of same ID, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// DeleteDoc — edge cases
// ---------------------------------------------------------------------------

// TestDeleteDoc_EmptyPath verifies that an empty path is rejected.
func TestDeleteDoc_EmptyPath(t *testing.T) {
	s := openStore(t, t.TempDir())
	if err := s.DeleteDoc(context.Background(), ""); err == nil {
		t.Error("expected error for empty path, got nil")
	}
}

// TestDeleteDoc_Nonexistent verifies that deleting a doc that does not exist
// succeeds (no error — idempotent DELETE).
func TestDeleteDoc_Nonexistent(t *testing.T) {
	s := openStore(t, t.TempDir())
	if err := s.DeleteDoc(context.Background(), "does-not-exist.md"); err != nil {
		t.Errorf("expected no error deleting nonexistent doc, got %v", err)
	}
}

// TestDeleteDoc_RemovesFromFTS verifies that after deletion the doc body is
// no longer returned by Search.
func TestDeleteDoc_RemovesFromFTS(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())

	d := minimalDoc("fts-doc.md", "uniquetoken-deletetest quantum flux")
	if err := s.UpsertDoc(ctx, d); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Sanity: it is searchable before deletion.
	hits, err := s.Search(ctx, "uniquetoken-deletetest", SearchFilters{})
	if err != nil {
		t.Fatalf("pre-delete search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected doc to be searchable before deletion")
	}

	if err := s.DeleteDoc(ctx, "fts-doc.md"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// After deletion the FTS row should be gone too.
	hits, err = s.Search(ctx, "uniquetoken-deletetest", SearchFilters{})
	if err != nil {
		t.Fatalf("post-delete search: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("expected 0 hits after deletion, got %d", len(hits))
	}
}

// ---------------------------------------------------------------------------
// GetDoc
// ---------------------------------------------------------------------------

// TestGetDoc_Nonexistent verifies that GetDoc returns (nil, nil) for an
// unknown path.
func TestGetDoc_Nonexistent(t *testing.T) {
	s := openStore(t, t.TempDir())
	d, err := s.GetDoc(context.Background(), "not-there.md")
	if err != nil {
		t.Fatalf("GetDoc: %v", err)
	}
	if d != nil {
		t.Errorf("expected nil for unknown doc, got %+v", d)
	}
}

// TestGetDoc_Exists verifies that GetDoc returns the metadata for an upserted
// document.
func TestGetDoc_Exists(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())

	in := minimalDoc("readme.md", "hello world")
	in.Author = "alice"
	if err := s.UpsertDoc(ctx, in); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	out, err := s.GetDoc(ctx, "readme.md")
	if err != nil {
		t.Fatalf("GetDoc: %v", err)
	}
	if out == nil {
		t.Fatal("expected doc, got nil")
	}
	if out.ID != "readme.md" {
		t.Errorf("ID = %q, want %q", out.ID, "readme.md")
	}
	if out.Author != "alice" {
		t.Errorf("Author = %q, want %q", out.Author, "alice")
	}
}

// TestGetDoc_SlugRoundTrip verifies that a non-empty Slug is stored and
// returned correctly by GetDoc.
func TestGetDoc_SlugRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())

	in := minimalDoc("slug-doc.md", "body text")
	in.Slug = "my-custom-slug"
	if err := s.UpsertDoc(ctx, in); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	out, err := s.GetDoc(ctx, "slug-doc.md")
	if err != nil {
		t.Fatalf("GetDoc: %v", err)
	}
	if out == nil {
		t.Fatal("expected doc, got nil")
	}
	if out.Slug != "my-custom-slug" {
		t.Errorf("Slug = %q, want %q", out.Slug, "my-custom-slug")
	}
}

// ---------------------------------------------------------------------------
// Search — edge cases
// ---------------------------------------------------------------------------

// TestSearch_EmptyQuery verifies that an empty query returns nil (not an error).
func TestSearch_EmptyQuery(t *testing.T) {
	s := openStore(t, t.TempDir())
	hits, err := s.Search(context.Background(), "", SearchFilters{})
	if err != nil {
		t.Fatalf("Search with empty query: %v", err)
	}
	if hits != nil {
		t.Errorf("expected nil for empty query, got %v", hits)
	}
}

// TestSearch_WhitespaceQuery verifies that an all-whitespace query is treated
// the same as empty (sanitizeFTSQuery reduces it to nothing).
func TestSearch_WhitespaceQuery(t *testing.T) {
	s := openStore(t, t.TempDir())
	hits, err := s.Search(context.Background(), "   \t  ", SearchFilters{})
	if err != nil {
		t.Fatalf("Search with whitespace query: %v", err)
	}
	if hits != nil {
		t.Errorf("expected nil for whitespace query, got %v", hits)
	}
}

// TestSearch_SpecialCharsInQuery verifies that FTS5 special characters in the
// query do not cause a SQL error — sanitizeFTSQuery must strip them.
func TestSearch_SpecialCharsInQuery(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())
	if err := s.UpsertDoc(ctx, minimalDoc("a.md", "simple body text")); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Characters that could break raw FTS5 query syntax.
	problematic := []string{
		`AND OR NOT`,
		`"quoted"`,
		`token-with-hyphens`,
		`col:filter`,
		`NEAR(foo bar)`,
		`*suffix`,
	}
	for _, q := range problematic {
		if _, err := s.Search(ctx, q, SearchFilters{}); err != nil {
			t.Errorf("Search(%q) returned error: %v", q, err)
		}
	}
}

// TestSearch_TypeFilter verifies that the Type filter narrows results correctly.
func TestSearch_TypeFilter(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())

	adr := minimalDoc("adr.md", "zebra keyword")
	adr.Type = "adr"
	howto := minimalDoc("howto.md", "zebra keyword")
	howto.Type = "how-to"

	for _, d := range []*Doc{adr, howto} {
		if err := s.UpsertDoc(ctx, d); err != nil {
			t.Fatalf("upsert %s: %v", d.ID, err)
		}
	}

	hits, err := s.Search(ctx, "zebra", SearchFilters{Type: "adr"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].Type != "adr" {
		t.Errorf("type filter failed: %+v", hits)
	}
}

// TestSearch_StatusFilter verifies that the Status filter works.
func TestSearch_StatusFilter(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())

	pub := minimalDoc("published.md", "rhino keyword")
	pub.Status = "published"
	draft := minimalDoc("draft.md", "rhino keyword")
	draft.Status = "draft"

	for _, d := range []*Doc{pub, draft} {
		if err := s.UpsertDoc(ctx, d); err != nil {
			t.Fatalf("upsert %s: %v", d.ID, err)
		}
	}

	hits, err := s.Search(ctx, "rhino", SearchFilters{Status: "draft"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].Status != "draft" {
		t.Errorf("status filter failed: %+v", hits)
	}
}

// TestSearch_TagFilter verifies that the Tag filter works with JSON-stored tags.
func TestSearch_TagFilter(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())

	tagged := minimalDoc("tagged.md", "mongoose keyword")
	tagged.Tags = []string{"featured", "guide"}
	untagged := minimalDoc("untagged.md", "mongoose keyword")
	untagged.Tags = []string{"other"}

	for _, d := range []*Doc{tagged, untagged} {
		if err := s.UpsertDoc(ctx, d); err != nil {
			t.Fatalf("upsert %s: %v", d.ID, err)
		}
	}

	hits, err := s.Search(ctx, "mongoose", SearchFilters{Tag: "featured"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].ID != "tagged.md" {
		t.Errorf("tag filter failed: %+v", hits)
	}
}

// ---------------------------------------------------------------------------
// Reindex — edge cases
// ---------------------------------------------------------------------------

// TestReindex_NilDocStore verifies that passing nil returns an error.
func TestReindex_NilDocStore(t *testing.T) {
	s := openStore(t, t.TempDir())
	if err := s.Reindex(context.Background(), nil, ""); err == nil {
		t.Error("expected error for nil DocStore, got nil")
	}
}

// TestReindex_EmptyDocStore verifies that reindexing with a store that has
// no documents results in zero rows.
func TestReindex_EmptyDocStore(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())

	empty := &memDocStore{docs: nil}
	if err := s.Reindex(ctx, empty, ""); err != nil {
		t.Fatalf("Reindex empty: %v", err)
	}
	n, _ := s.CountDocs(ctx)
	if n != 0 {
		t.Errorf("expected 0 docs after empty reindex, got %d", n)
	}
}

// TestReindex_ReplacesExisting verifies that a full reindex replaces previously
// indexed documents (truncate + rebuild).
func TestReindex_ReplacesExisting(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())

	// First index: 3 docs.
	v1 := &memDocStore{docs: []*Doc{
		minimalDoc("a.md", "first body a"),
		minimalDoc("b.md", "first body b"),
		minimalDoc("c.md", "first body c"),
	}}
	if err := s.Reindex(ctx, v1, ""); err != nil {
		t.Fatalf("first reindex: %v", err)
	}
	n, _ := s.CountDocs(ctx)
	if n != 3 {
		t.Fatalf("expected 3 docs, got %d", n)
	}

	// Second index: 2 different docs — old 3 must be gone.
	v2 := &memDocStore{docs: []*Doc{
		minimalDoc("x.md", "second body x"),
		minimalDoc("y.md", "second body y"),
	}}
	if err := s.Reindex(ctx, v2, ""); err != nil {
		t.Fatalf("second reindex: %v", err)
	}
	n, _ = s.CountDocs(ctx)
	if n != 2 {
		t.Errorf("expected 2 docs after second reindex, got %d", n)
	}
}

// TestReindex_WalkError verifies that a WalkDocs error propagates through
// Reindex and is returned to the caller.
func TestReindex_WalkError(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())

	boom := errors.New("walk failed")
	failing := &errDocStore{err: boom}

	if err := s.Reindex(ctx, failing, ""); err == nil {
		t.Error("expected error from failing DocStore, got nil")
	}
}

// errDocStore is a DocStore that always returns the configured error.
type errDocStore struct{ err error }

func (e *errDocStore) WalkDocs(_ string, _ func(*Doc) error) error {
	return e.err
}

// ---------------------------------------------------------------------------
// sanitizeFTSQuery — internal helper
// ---------------------------------------------------------------------------

// TestSanitizeFTSQuery_Empty returns empty for empty input.
func TestSanitizeFTSQuery_Empty(t *testing.T) {
	if got := sanitizeFTSQuery(""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// TestSanitizeFTSQuery_SingleToken wraps a single word in double quotes.
func TestSanitizeFTSQuery_SingleToken(t *testing.T) {
	got := sanitizeFTSQuery("hello")
	if got != `"hello"` {
		t.Errorf("sanitizeFTSQuery(hello) = %q, want %q", got, `"hello"`)
	}
}

// TestSanitizeFTSQuery_MultipleTokens joins tokens with spaces (implicit AND).
func TestSanitizeFTSQuery_MultipleTokens(t *testing.T) {
	got := sanitizeFTSQuery("quantum flux")
	if got != `"quantum" "flux"` {
		t.Errorf("sanitizeFTSQuery(quantum flux) = %q", got)
	}
}

// TestSanitizeFTSQuery_StripsPunctuation verifies that FTS5 operator characters
// are removed from the query.
func TestSanitizeFTSQuery_StripsPunctuation(t *testing.T) {
	got := sanitizeFTSQuery(`AND "quoted" *glob`)
	// Each token after stripping is a plain alphanumeric word.
	for _, tok := range []string{"AND", "quoted", "glob"} {
		if !strings.Contains(got, `"`+tok+`"`) {
			t.Errorf("expected token %q to be present in sanitized %q", tok, got)
		}
	}
}

// ---------------------------------------------------------------------------
// levenshtein / titleSimilarity / min3 — dupcheck.go
// ---------------------------------------------------------------------------

// TestLevenshtein_Table exercises levenshtein across a range of known inputs.
func TestLevenshtein_Table(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"kitten", "sitting", 3},
		{"a", "b", 1},
		{"abc", "abd", 1},
		{"abc", "aXc", 1},
	}
	for _, tc := range tests {
		got := levenshtein(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestTitleSimilarity_Identical verifies that identical strings score 1.0.
func TestTitleSimilarity_Identical(t *testing.T) {
	if got := titleSimilarity("hello", "hello"); got != 1.0 {
		t.Errorf("identical strings: got %f, want 1.0", got)
	}
}

// TestTitleSimilarity_BothEmpty verifies the empty-string convention (1.0).
func TestTitleSimilarity_BothEmpty(t *testing.T) {
	if got := titleSimilarity("", ""); got != 1.0 {
		t.Errorf("both empty: got %f, want 1.0", got)
	}
}

// TestTitleSimilarity_CompletelyDifferent verifies a low score for unrelated strings.
func TestTitleSimilarity_CompletelyDifferent(t *testing.T) {
	got := titleSimilarity("abc", "xyz")
	if got >= 1.0 {
		t.Errorf("expected < 1.0 for different strings, got %f", got)
	}
	if got < 0.0 {
		t.Errorf("similarity must be >= 0, got %f", got)
	}
}

// TestMin3_Table exercises all three branches of min3.
func TestMin3_Table(t *testing.T) {
	tests := []struct {
		a, b, c int
		want    int
	}{
		{1, 2, 3, 1},
		{3, 1, 2, 1},
		{3, 2, 1, 1},
		{5, 5, 5, 5},
		{0, 0, 0, 0},
	}
	for _, tc := range tests {
		got := min3(tc.a, tc.b, tc.c)
		if got != tc.want {
			t.Errorf("min3(%d,%d,%d) = %d, want %d", tc.a, tc.b, tc.c, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Tasks CRUD
// ---------------------------------------------------------------------------

func insertTask(t *testing.T, s *Store, id, project, title, status string, pos float64) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	if err := s.InsertTask(context.Background(), Task{
		ID:        id,
		Project:   project,
		Title:     title,
		Status:    status,
		Position:  pos,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertTask %s: %v", id, err)
	}
}

// TestListTasks_Empty verifies that ListTasks returns an empty (non-nil) slice
// when no tasks exist for the project.
func TestListTasks_Empty(t *testing.T) {
	s := openStore(t, t.TempDir())
	tasks, err := s.ListTasks(context.Background(), "no-project")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if tasks == nil {
		t.Error("expected non-nil slice, got nil")
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

// TestListTasks_OrderedByPosition verifies ascending position order.
func TestListTasks_OrderedByPosition(t *testing.T) {
	s := openStore(t, t.TempDir())
	insertTask(t, s, "t3", "p", "third", "todo", 3)
	insertTask(t, s, "t1", "p", "first", "todo", 1)
	insertTask(t, s, "t2", "p", "second", "todo", 2)

	tasks, err := s.ListTasks(context.Background(), "p")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
	for i, want := range []string{"t1", "t2", "t3"} {
		if tasks[i].ID != want {
			t.Errorf("tasks[%d].ID = %q, want %q", i, tasks[i].ID, want)
		}
	}
}

// TestGetTask_NotFound verifies that ErrTaskNotFound is returned for unknown IDs.
func TestGetTask_NotFound(t *testing.T) {
	s := openStore(t, t.TempDir())
	_, err := s.GetTask(context.Background(), "proj", "ghost")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

// TestGetTask_Exists verifies that an inserted task can be retrieved.
func TestGetTask_Exists(t *testing.T) {
	s := openStore(t, t.TempDir())
	insertTask(t, s, "task-1", "alpha", "My Task", "todo", 1.0)

	task, err := s.GetTask(context.Background(), "alpha", "task-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if task.Title != "My Task" {
		t.Errorf("Title = %q, want %q", task.Title, "My Task")
	}
}

// TestNextTaskPosition_NoTasks verifies that 1.0 is returned when no tasks exist.
func TestNextTaskPosition_NoTasks(t *testing.T) {
	s := openStore(t, t.TempDir())
	pos, err := s.NextTaskPosition(context.Background(), "empty-proj")
	if err != nil {
		t.Fatalf("NextTaskPosition: %v", err)
	}
	if pos != 1.0 {
		t.Errorf("expected 1.0, got %f", pos)
	}
}

// TestNextTaskPosition_WithTasks verifies max(position)+1 when tasks exist.
func TestNextTaskPosition_WithTasks(t *testing.T) {
	s := openStore(t, t.TempDir())
	insertTask(t, s, "t1", "proj", "a", "todo", 5.0)
	insertTask(t, s, "t2", "proj", "b", "todo", 3.0)

	pos, err := s.NextTaskPosition(context.Background(), "proj")
	if err != nil {
		t.Fatalf("NextTaskPosition: %v", err)
	}
	if pos != 6.0 {
		t.Errorf("expected 6.0, got %f", pos)
	}
}

// TestUpdateTask_NoFields verifies that passing all-nil fields is a no-op
// that returns the current task unchanged.
func TestUpdateTask_NoFields(t *testing.T) {
	s := openStore(t, t.TempDir())
	insertTask(t, s, "t", "proj", "original", "todo", 1)

	updated, err := s.UpdateTask(context.Background(), "proj", "t", nil, nil, nil)
	if err != nil {
		t.Fatalf("UpdateTask no-op: %v", err)
	}
	if updated.Title != "original" {
		t.Errorf("Title = %q, want %q", updated.Title, "original")
	}
}

// TestUpdateTask_TitleAndStatus verifies partial update with title and status.
func TestUpdateTask_TitleAndStatus(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())
	insertTask(t, s, "t", "proj", "old title", "todo", 1)

	newTitle := "new title"
	newStatus := "in-progress"
	updated, err := s.UpdateTask(ctx, "proj", "t", &newTitle, &newStatus, nil)
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	if updated.Title != newTitle {
		t.Errorf("Title = %q, want %q", updated.Title, newTitle)
	}
	if updated.Status != newStatus {
		t.Errorf("Status = %q, want %q", updated.Status, newStatus)
	}
}

// TestUpdateTask_Position verifies that position can be updated independently.
func TestUpdateTask_Position(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())
	insertTask(t, s, "t", "proj", "title", "todo", 1.0)

	newPos := 99.5
	updated, err := s.UpdateTask(ctx, "proj", "t", nil, nil, &newPos)
	if err != nil {
		t.Fatalf("UpdateTask position: %v", err)
	}
	if updated.Position != newPos {
		t.Errorf("Position = %f, want %f", updated.Position, newPos)
	}
}

// TestUpdateTask_NotFound verifies ErrTaskNotFound is returned when the task
// does not exist.
func TestUpdateTask_NotFound(t *testing.T) {
	s := openStore(t, t.TempDir())
	newTitle := "irrelevant"
	_, err := s.UpdateTask(context.Background(), "proj", "ghost", &newTitle, nil, nil)
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

// TestDeleteTask_Existing verifies that a known task can be deleted.
func TestDeleteTask_Existing(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())
	insertTask(t, s, "t", "proj", "task", "todo", 1)

	if err := s.DeleteTask(ctx, "proj", "t"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	_, err := s.GetTask(ctx, "proj", "t")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound after delete, got %v", err)
	}
}

// TestDeleteTask_NotFound verifies ErrTaskNotFound for a non-existent task.
func TestDeleteTask_NotFound(t *testing.T) {
	s := openStore(t, t.TempDir())
	if err := s.DeleteTask(context.Background(), "proj", "ghost"); !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

// TestRenumberTasks verifies that RenumberTasks assigns sequential positions
// starting at 1.
func TestRenumberTasks(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, t.TempDir())
	insertTask(t, s, "t1", "proj", "a", "todo", 0.001)
	insertTask(t, s, "t2", "proj", "b", "todo", 0.002)
	insertTask(t, s, "t3", "proj", "c", "todo", 0.003)

	tasks, err := s.ListTasks(ctx, "proj")
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	renumbered, err := s.RenumberTasks(ctx, "proj", tasks)
	if err != nil {
		t.Fatalf("RenumberTasks: %v", err)
	}
	for i, task := range renumbered {
		want := float64(i + 1)
		if task.Position != want {
			t.Errorf("renumbered[%d].Position = %f, want %f", i, task.Position, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Open — WorkspaceRoot validation
// ---------------------------------------------------------------------------

// TestOpen_EmptyWorkspaceRoot verifies that Open rejects an empty WorkspaceRoot.
func TestOpen_EmptyWorkspaceRoot(t *testing.T) {
	if _, err := Open(Options{WorkspaceRoot: ""}); err == nil {
		t.Error("expected error for empty WorkspaceRoot, got nil")
	}
}

// TestOpen_WithLogger verifies that a Logger option is accepted and called
// during schema migration.
func TestOpen_WithLogger(t *testing.T) {
	var logged []string
	s, err := Open(Options{
		WorkspaceRoot: t.TempDir(),
		Logger: func(msg string) {
			logged = append(logged, msg)
		},
	})
	if err != nil {
		t.Fatalf("Open with logger: %v", err)
	}
	defer s.Close()
	// At least one migration should have been logged on a fresh database.
	if len(logged) == 0 {
		t.Error("expected at least one log message from migrations, got none")
	}
}
