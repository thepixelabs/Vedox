package db

import (
	"context"
	"path/filepath"
	"testing"
)

// openGlobalDB is a test helper that opens a GlobalDB in t.TempDir() and
// registers cleanup automatically.
func openGlobalDB(t *testing.T) *GlobalDB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "global.db")
	g, err := OpenGlobalDB(path)
	if err != nil {
		t.Fatalf("OpenGlobalDB: %v", err)
	}
	t.Cleanup(func() { _ = g.Close() })
	return g
}

// ---------------------------------------------------------------------------
// OpenGlobalDB — construction
// ---------------------------------------------------------------------------

// TestOpenGlobalDB_EmptyPath verifies that an empty path is rejected.
func TestOpenGlobalDB_EmptyPath(t *testing.T) {
	if _, err := OpenGlobalDB(""); err == nil {
		t.Error("expected error for empty path, got nil")
	}
}

// TestOpenGlobalDB_CreatesDir verifies that OpenGlobalDB creates the parent
// directory if it does not exist.
func TestOpenGlobalDB_CreatesDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "global.db")
	g, err := OpenGlobalDB(path)
	if err != nil {
		t.Fatalf("OpenGlobalDB with nested dir: %v", err)
	}
	defer g.Close()
	if g.Path() != path {
		t.Errorf("Path() = %q, want %q", g.Path(), path)
	}
}

// TestOpenGlobalDB_Idempotent verifies that reopening an existing global.db
// does not fail (migrations are idempotent via IF NOT EXISTS).
func TestOpenGlobalDB_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "global.db")
	g1, err := OpenGlobalDB(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	_ = g1.Close()

	g2, err := OpenGlobalDB(path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer g2.Close()
}

// ---------------------------------------------------------------------------
// Repo CRUD
// ---------------------------------------------------------------------------

func sampleRepo(id string) Repo {
	return Repo{
		ID:        id,
		Name:      "docs-" + id,
		Type:      "private",
		RootPath:  "/home/user/docs-" + id,
		RemoteURL: "https://github.com/user/docs-" + id,
		Status:    "active",
	}
}

// TestUpsertRepo_EmptyID verifies that UpsertRepo rejects an empty ID.
func TestUpsertRepo_EmptyID(t *testing.T) {
	g := openGlobalDB(t)
	if err := g.UpsertRepo(context.Background(), Repo{}); err == nil {
		t.Error("expected error for empty ID, got nil")
	}
}

// TestUpsertRepo_EmptyName verifies that UpsertRepo rejects an empty Name.
func TestUpsertRepo_EmptyName(t *testing.T) {
	g := openGlobalDB(t)
	r := sampleRepo("r1")
	r.Name = ""
	if err := g.UpsertRepo(context.Background(), r); err == nil {
		t.Error("expected error for empty Name, got nil")
	}
}

// TestUpsertRepo_RoundTrip inserts a repo and verifies GetRepo returns it.
func TestUpsertRepo_RoundTrip(t *testing.T) {
	ctx := context.Background()
	g := openGlobalDB(t)

	in := sampleRepo("abc-123")
	if err := g.UpsertRepo(ctx, in); err != nil {
		t.Fatalf("UpsertRepo: %v", err)
	}

	out, err := g.GetRepo(ctx, "abc-123")
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if out == nil {
		t.Fatal("expected repo, got nil")
	}
	if out.Name != in.Name {
		t.Errorf("Name = %q, want %q", out.Name, in.Name)
	}
	if out.Type != "private" {
		t.Errorf("Type = %q, want private", out.Type)
	}
	if out.RemoteURL != in.RemoteURL {
		t.Errorf("RemoteURL = %q, want %q", out.RemoteURL, in.RemoteURL)
	}
}

// TestGetRepo_Nonexistent verifies GetRepo returns nil for unknown IDs.
func TestGetRepo_Nonexistent(t *testing.T) {
	g := openGlobalDB(t)
	r, err := g.GetRepo(context.Background(), "ghost")
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if r != nil {
		t.Errorf("expected nil for unknown ID, got %+v", r)
	}
}

// TestListRepos_Empty verifies that ListRepos returns a non-nil empty slice.
func TestListRepos_Empty(t *testing.T) {
	g := openGlobalDB(t)
	repos, err := g.ListRepos(context.Background(), "")
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(repos))
	}
}

// TestListRepos_StatusFilter verifies that the status filter narrows results.
func TestListRepos_StatusFilter(t *testing.T) {
	ctx := context.Background()
	g := openGlobalDB(t)

	active := sampleRepo("r1")
	active.Status = "active"
	archived := sampleRepo("r2")
	archived.Status = "archived"

	for _, r := range []Repo{active, archived} {
		if err := g.UpsertRepo(ctx, r); err != nil {
			t.Fatalf("UpsertRepo %s: %v", r.ID, err)
		}
	}

	got, err := g.ListRepos(ctx, "active")
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(got) != 1 || got[0].ID != "r1" {
		t.Errorf("expected 1 active repo, got %+v", got)
	}
}

// TestUpsertRepo_Update verifies that upserting the same ID updates the row.
func TestUpsertRepo_Update(t *testing.T) {
	ctx := context.Background()
	g := openGlobalDB(t)

	r := sampleRepo("upd-1")
	if err := g.UpsertRepo(ctx, r); err != nil {
		t.Fatalf("initial upsert: %v", err)
	}

	r.Name = "updated-name"
	r.Status = "archived"
	if err := g.UpsertRepo(ctx, r); err != nil {
		t.Fatalf("update upsert: %v", err)
	}

	out, err := g.GetRepo(ctx, "upd-1")
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if out.Name != "updated-name" {
		t.Errorf("Name = %q, want updated-name", out.Name)
	}
	if out.Status != "archived" {
		t.Errorf("Status = %q, want archived", out.Status)
	}

	// Count: must still be exactly 1 row.
	all, err := g.ListRepos(ctx, "")
	if err != nil {
		t.Fatalf("ListRepos: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 row after two upserts of same ID, got %d", len(all))
	}
}

// TestDeleteRepo verifies that DeleteRepo removes the row.
func TestDeleteRepo(t *testing.T) {
	ctx := context.Background()
	g := openGlobalDB(t)

	if err := g.UpsertRepo(ctx, sampleRepo("del-1")); err != nil {
		t.Fatalf("UpsertRepo: %v", err)
	}
	if err := g.DeleteRepo(ctx, "del-1"); err != nil {
		t.Fatalf("DeleteRepo: %v", err)
	}
	r, err := g.GetRepo(ctx, "del-1")
	if err != nil {
		t.Fatalf("GetRepo after delete: %v", err)
	}
	if r != nil {
		t.Errorf("expected nil after delete, got %+v", r)
	}
}

// TestDeleteRepo_Nonexistent verifies that deleting an unknown ID is a no-op.
func TestDeleteRepo_Nonexistent(t *testing.T) {
	g := openGlobalDB(t)
	if err := g.DeleteRepo(context.Background(), "ghost"); err != nil {
		t.Errorf("expected no error deleting nonexistent repo, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// AgentInstall
// ---------------------------------------------------------------------------

// TestUpsertAgentInstall_RoundTrip inserts and retrieves an agent install.
func TestUpsertAgentInstall_RoundTrip(t *testing.T) {
	ctx := context.Background()
	g := openGlobalDB(t)

	a := AgentInstall{
		ID:           "ai-001",
		Provider:     "claude-code",
		Version:      "1.0.0",
		HealthStatus: "healthy",
	}
	if err := g.UpsertAgentInstall(ctx, a); err != nil {
		t.Fatalf("UpsertAgentInstall: %v", err)
	}

	installs, err := g.ListAgentInstalls(ctx, "claude-code")
	if err != nil {
		t.Fatalf("ListAgentInstalls: %v", err)
	}
	if len(installs) != 1 {
		t.Fatalf("expected 1 install, got %d", len(installs))
	}
	if installs[0].Provider != "claude-code" {
		t.Errorf("Provider = %q, want claude-code", installs[0].Provider)
	}
	if installs[0].HealthStatus != "healthy" {
		t.Errorf("HealthStatus = %q, want healthy", installs[0].HealthStatus)
	}
}

// TestUpsertAgentInstall_EmptyID verifies empty ID is rejected.
func TestUpsertAgentInstall_EmptyID(t *testing.T) {
	g := openGlobalDB(t)
	if err := g.UpsertAgentInstall(context.Background(), AgentInstall{}); err == nil {
		t.Error("expected error for empty ID, got nil")
	}
}

// TestListAgentInstalls_ProviderFilter verifies provider filter narrows results.
func TestListAgentInstalls_ProviderFilter(t *testing.T) {
	ctx := context.Background()
	g := openGlobalDB(t)

	for _, a := range []AgentInstall{
		{ID: "a1", Provider: "claude-code", Version: "1.0.0"},
		{ID: "a2", Provider: "codex", Version: "2.0.0"},
	} {
		if err := g.UpsertAgentInstall(ctx, a); err != nil {
			t.Fatalf("UpsertAgentInstall %s: %v", a.ID, err)
		}
	}

	got, err := g.ListAgentInstalls(ctx, "codex")
	if err != nil {
		t.Fatalf("ListAgentInstalls: %v", err)
	}
	if len(got) != 1 || got[0].Provider != "codex" {
		t.Errorf("expected 1 codex install, got %+v", got)
	}
}

// ---------------------------------------------------------------------------
// Daily event roll-up
// ---------------------------------------------------------------------------

// TestIncrementDailyEvent_Basic verifies that counts accumulate correctly.
func TestIncrementDailyEvent_Basic(t *testing.T) {
	ctx := context.Background()
	g := openGlobalDB(t)

	if err := g.IncrementDailyEvent(ctx, "2026-04-15", "document.published", 3); err != nil {
		t.Fatalf("IncrementDailyEvent: %v", err)
	}
	if err := g.IncrementDailyEvent(ctx, "2026-04-15", "document.published", 2); err != nil {
		t.Fatalf("IncrementDailyEvent (second): %v", err)
	}

	count, err := g.GetDailyEventCount(ctx, "2026-04-15", "document.published")
	if err != nil {
		t.Fatalf("GetDailyEventCount: %v", err)
	}
	if count != 5 {
		t.Errorf("count = %d, want 5", count)
	}
}

// TestGetDailyEventCount_Missing verifies that a missing row returns 0, not error.
func TestGetDailyEventCount_Missing(t *testing.T) {
	g := openGlobalDB(t)
	count, err := g.GetDailyEventCount(context.Background(), "2026-01-01", "search.executed")
	if err != nil {
		t.Fatalf("GetDailyEventCount: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 for missing row, got %d", count)
	}
}

// TestIncrementDailyEvent_EmptyArgs verifies that empty date/kind is rejected.
func TestIncrementDailyEvent_EmptyArgs(t *testing.T) {
	g := openGlobalDB(t)
	if err := g.IncrementDailyEvent(context.Background(), "", "document.published", 1); err == nil {
		t.Error("expected error for empty date, got nil")
	}
	if err := g.IncrementDailyEvent(context.Background(), "2026-04-15", "", 1); err == nil {
		t.Error("expected error for empty kind, got nil")
	}
}

// TestIncrementDailyEvent_DifferentDates verifies that the same kind on
// different dates is stored as separate rows.
func TestIncrementDailyEvent_DifferentDates(t *testing.T) {
	ctx := context.Background()
	g := openGlobalDB(t)

	if err := g.IncrementDailyEvent(ctx, "2026-04-14", "agent.triggered", 10); err != nil {
		t.Fatalf("day 1: %v", err)
	}
	if err := g.IncrementDailyEvent(ctx, "2026-04-15", "agent.triggered", 20); err != nil {
		t.Fatalf("day 2: %v", err)
	}

	c1, err := g.GetDailyEventCount(ctx, "2026-04-14", "agent.triggered")
	if err != nil {
		t.Fatalf("day 1 count: %v", err)
	}
	c2, err := g.GetDailyEventCount(ctx, "2026-04-15", "agent.triggered")
	if err != nil {
		t.Fatalf("day 2 count: %v", err)
	}
	if c1 != 10 || c2 != 20 {
		t.Errorf("counts = (%d, %d), want (10, 20)", c1, c2)
	}
}
