package analytics

import (
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Event.Validate — happy path
// ---------------------------------------------------------------------------

// TestValidate_ValidEvent verifies that a correctly formed event passes Validate.
func TestValidate_ValidEvent(t *testing.T) {
	e := Event{
		Kind:      EventKindDocumentPublished,
		Timestamp: time.Now(),
		SessionID: "sess-abc-123",
	}
	if err := e.Validate(); err != nil {
		t.Errorf("expected no error for valid event, got: %v", err)
	}
}

// TestValidate_WithProperties verifies that a non-nil Properties map is valid.
func TestValidate_WithProperties(t *testing.T) {
	e := Event{
		Kind:       EventKindSearchExecuted,
		Timestamp:  time.Now(),
		SessionID:  "sess-xyz",
		Properties: map[string]any{"query": "terraform", "results": 5},
	}
	if err := e.Validate(); err != nil {
		t.Errorf("expected no error with properties, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Event.Validate — error paths
// ---------------------------------------------------------------------------

// TestValidate_EmptyKind verifies that an empty Kind is rejected.
func TestValidate_EmptyKind(t *testing.T) {
	e := Event{
		Kind:      "",
		Timestamp: time.Now(),
		SessionID: "s",
	}
	if err := e.Validate(); err == nil {
		t.Error("expected error for empty Kind, got nil")
	}
}

// TestValidate_InvalidKind verifies that kinds with illegal characters fail.
func TestValidate_InvalidKind(t *testing.T) {
	illegal := []string{
		"Document.Published",    // uppercase
		"document published",    // space
		"document/published",    // slash
		"doc:pub",               // colon
		"",                      // empty (also tested above)
	}
	for _, k := range illegal {
		e := Event{Kind: k, Timestamp: time.Now(), SessionID: "s"}
		if err := e.Validate(); err == nil {
			t.Errorf("expected error for Kind %q, got nil", k)
		}
	}
}

// TestValidate_KindTooLong verifies that a kind exceeding 48 chars is rejected.
func TestValidate_KindTooLong(t *testing.T) {
	// 49 lowercase alpha chars.
	long := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if len(long) != 49 {
		t.Fatalf("test setup: expected 49 chars, got %d", len(long))
	}
	e := Event{Kind: long, Timestamp: time.Now(), SessionID: "s"}
	if err := e.Validate(); err == nil {
		t.Errorf("expected error for 49-char kind, got nil")
	}
}

// TestValidate_KindExactly48 verifies that a 48-char kind is accepted.
func TestValidate_KindExactly48(t *testing.T) {
	// 48 lowercase alpha chars.
	max48 := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if len(max48) != 48 {
		t.Fatalf("test setup: expected 48 chars, got %d", len(max48))
	}
	e := Event{Kind: max48, Timestamp: time.Now(), SessionID: "s"}
	if err := e.Validate(); err != nil {
		t.Errorf("expected no error for 48-char kind, got: %v", err)
	}
}

// TestValidate_ZeroTimestamp verifies that a zero Timestamp is rejected.
func TestValidate_ZeroTimestamp(t *testing.T) {
	e := Event{
		Kind:      EventKindRepoCreated,
		Timestamp: time.Time{},
		SessionID: "s",
	}
	if err := e.Validate(); err == nil {
		t.Error("expected error for zero Timestamp, got nil")
	}
}

// TestValidate_EmptySessionID verifies that an empty SessionID is rejected.
func TestValidate_EmptySessionID(t *testing.T) {
	e := Event{
		Kind:      EventKindAgentInstalled,
		Timestamp: time.Now(),
		SessionID: "",
	}
	if err := e.Validate(); err == nil {
		t.Error("expected error for empty SessionID, got nil")
	}
}

// ---------------------------------------------------------------------------
// Event kind constants — verify all 10 constants pass Validate individually
// ---------------------------------------------------------------------------

// TestAllEventKindsValid verifies that every EventKind* constant satisfies
// the Validate regex (i.e. no typos or illegal characters crept in).
func TestAllEventKindsValid(t *testing.T) {
	allKinds := []string{
		EventKindDocumentPublished,
		EventKindDocumentViewed,
		EventKindRepoRegistered,
		EventKindRepoCreated,
		EventKindAgentTriggered,
		EventKindAgentInstalled,
		EventKindOnboardingStarted,
		EventKindOnboardingCompleted,
		EventKindSearchExecuted,
		EventKindSettingsChanged,
	}
	for _, k := range allKinds {
		e := Event{Kind: k, Timestamp: time.Now(), SessionID: "test-session"}
		if err := e.Validate(); err != nil {
			t.Errorf("EventKind constant %q failed Validate: %v", k, err)
		}
	}
}

// TestAllEventKindsDistinct verifies that no two EventKind* constants share
// the same wire value (catches copy-paste duplicates).
func TestAllEventKindsDistinct(t *testing.T) {
	allKinds := []string{
		EventKindDocumentPublished,
		EventKindDocumentViewed,
		EventKindRepoRegistered,
		EventKindRepoCreated,
		EventKindAgentTriggered,
		EventKindAgentInstalled,
		EventKindOnboardingStarted,
		EventKindOnboardingCompleted,
		EventKindSearchExecuted,
		EventKindSettingsChanged,
	}
	seen := make(map[string]bool, len(allKinds))
	for _, k := range allKinds {
		if seen[k] {
			t.Errorf("duplicate EventKind constant value %q", k)
		}
		seen[k] = true
	}
	if len(seen) != 10 {
		t.Errorf("expected 10 distinct event kinds, got %d", len(seen))
	}
}

// ---------------------------------------------------------------------------
// validKind regex — boundary cases
// ---------------------------------------------------------------------------

// TestValidKindRegex_Boundaries exercises the regex directly for edge cases
// that the EventKind constants do not cover.
func TestValidKindRegex_Boundaries(t *testing.T) {
	valid := []string{
		"a",
		"a.b",
		"document.published",
		"index.started",
		"onboarding.step.completed",
		"a1.b2.c3",
		"x_y",           // underscore is allowed
	}
	for _, k := range valid {
		if !validKind.MatchString(k) {
			t.Errorf("expected valid, got invalid for %q", k)
		}
	}

	invalid := []string{
		"",
		"A",
		"Document",
		"doc-pub",        // hyphen not allowed
		"doc pub",        // space
		"doc/pub",        // slash
	}
	for _, k := range invalid {
		if validKind.MatchString(k) {
			t.Errorf("expected invalid, got valid for %q", k)
		}
	}
}
