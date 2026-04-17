package api

// FIX-SEC-10 acceptance tests for NewServer's fail-closed constructor.
//
// Previously, passing a nil requireAgent silently substituted PassthroughAuth
// — an unauthenticated default that let wiring mistakes go unnoticed. After
// the fix, NewServer panics on nil, forcing callers to make an explicit
// choice (RequireAgent for production, RejectAllAuth for fail-closed
// degradation, PassthroughAuth for tests only).

import (
	"testing"

	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// TestNewServer_PanicsOnNilRequireAgent is the regression guard: if someone
// restores the old silent-fallback behaviour, this test fails.
func TestNewServer_PanicsOnNilRequireAgent(t *testing.T) {
	wsRoot := t.TempDir()
	adapter, err := store.NewLocalAdapter(wsRoot, nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}
	dbStore, err := db.Open(db.Options{WorkspaceRoot: wsRoot})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = dbStore.Close() })

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when requireAgent is nil; NewServer returned normally — "+
				"FIX-SEC-10 fail-closed constructor regressed",
			)
		}
	}()

	_ = NewServer(
		adapter,
		dbStore,
		wsRoot,
		scanner.NewJobStore(),
		ai.NewJobStore(3),
		store.NewProjectRegistry(),
		nil, // must panic — no silent PassthroughAuth substitution
	)
}

// TestNewServer_AcceptsPassthroughAuth confirms the escape hatch tests rely
// on still works when the caller makes an explicit choice.
func TestNewServer_AcceptsPassthroughAuth(t *testing.T) {
	wsRoot := t.TempDir()
	adapter, _ := store.NewLocalAdapter(wsRoot, nil)
	dbStore, err := db.Open(db.Options{WorkspaceRoot: wsRoot})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = dbStore.Close() })

	srv := NewServer(
		adapter,
		dbStore,
		wsRoot,
		scanner.NewJobStore(),
		ai.NewJobStore(3),
		store.NewProjectRegistry(),
		agentauth.PassthroughAuth(),
	)
	if srv == nil {
		t.Fatal("NewServer returned nil with an explicit PassthroughAuth middleware")
	}
}

// TestNewServer_AcceptsRejectAllAuth confirms the fail-closed middleware
// produced by RejectAllAuth is a valid argument — i.e. the daemon's
// keystore-failure branch constructs without panicking.
func TestNewServer_AcceptsRejectAllAuth(t *testing.T) {
	wsRoot := t.TempDir()
	adapter, _ := store.NewLocalAdapter(wsRoot, nil)
	dbStore, err := db.Open(db.Options{WorkspaceRoot: wsRoot})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = dbStore.Close() })

	srv := NewServer(
		adapter,
		dbStore,
		wsRoot,
		scanner.NewJobStore(),
		ai.NewJobStore(3),
		store.NewProjectRegistry(),
		agentauth.RejectAllAuth(),
	)
	if srv == nil {
		t.Fatal("NewServer returned nil with an explicit RejectAllAuth middleware")
	}
}
