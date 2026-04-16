package analytics_test

// Integration tests for the analytics/events package. These are black-box
// tests (package analytics_test, not analytics) that verify observable
// behaviour through the public API.
//
// The existing events_test.go lives in the internal package; this file lives
// in the external test package so it can serve as executable documentation of
// the public contract.

import (
	"strings"
	"testing"
	"time"

	"github.com/vedox/vedox/internal/analytics"
)

// ---- helpers ----------------------------------------------------------------

// validEvent returns a well-formed Event using the given kind.
func validEvent(kind string) analytics.Event {
	return analytics.Event{
		Kind:      kind,
		Timestamp: time.Now(),
		SessionID: "integration-session-abc123",
	}
}

// validEventWithProps returns a well-formed Event with additional properties.
func validEventWithProps(kind string, props map[string]any) analytics.Event {
	e := validEvent(kind)
	e.Properties = props
	return e
}

// ---- Test: all 10 bootstrap event kinds validate successfully ---------------

// TestIntegration_AllBootstrapKinds_ValidatePass creates a valid Event for
// each of the 10 bootstrap EventKind* constants and asserts Validate() passes.
// This is the specification document for which events the system emits.
func TestIntegration_AllBootstrapKinds_ValidatePass(t *testing.T) {
	bootstrapKinds := []struct {
		name string
		kind string
	}{
		{"document.published", analytics.EventKindDocumentPublished},
		{"document.viewed", analytics.EventKindDocumentViewed},
		{"repo.registered", analytics.EventKindRepoRegistered},
		{"repo.created", analytics.EventKindRepoCreated},
		{"agent.triggered", analytics.EventKindAgentTriggered},
		{"agent.installed", analytics.EventKindAgentInstalled},
		{"onboarding.started", analytics.EventKindOnboardingStarted},
		{"onboarding.completed", analytics.EventKindOnboardingCompleted},
		{"search.executed", analytics.EventKindSearchExecuted},
		{"settings.changed", analytics.EventKindSettingsChanged},
	}

	for _, tc := range bootstrapKinds {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			e := validEvent(tc.kind)
			if err := e.Validate(); err != nil {
				t.Errorf("EventKind %q failed Validate: %v", tc.kind, err)
			}
		})
	}
}

// ---- Test: valid events with properties -------------------------------------

// TestIntegration_ValidEventWithProperties verifies that events with
// populated Properties maps validate correctly for representative kinds.
func TestIntegration_ValidEventWithProperties(t *testing.T) {
	cases := []struct {
		name  string
		kind  string
		props map[string]any
	}{
		{
			name:  "document.published with repo_id",
			kind:  analytics.EventKindDocumentPublished,
			props: map[string]any{"repo_id": "abc-123", "doc_id": "def-456", "word_count": 1420},
		},
		{
			name:  "agent.triggered with provider",
			kind:  analytics.EventKindAgentTriggered,
			props: map[string]any{"provider": "claude-code", "trigger": "voice", "routing": "private"},
		},
		{
			name:  "search.executed with query",
			kind:  analytics.EventKindSearchExecuted,
			props: map[string]any{"query": "terraform deployment", "result_count": 12},
		},
		{
			name:  "onboarding.completed with step count",
			kind:  analytics.EventKindOnboardingCompleted,
			props: map[string]any{"steps_completed": 5, "voice_configured": false},
		},
		{
			name:  "settings.changed with category",
			kind:  analytics.EventKindSettingsChanged,
			props: map[string]any{"category": "theme", "old_value": "graphite", "new_value": "ember"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			e := validEventWithProps(tc.kind, tc.props)
			if err := e.Validate(); err != nil {
				t.Errorf("event with properties failed Validate: %v", err)
			}
		})
	}
}

// ---- Test: nil Properties is valid ------------------------------------------

// TestIntegration_NilProperties_Valid verifies that a nil Properties map is
// accepted by Validate (it is explicitly documented as valid).
func TestIntegration_NilProperties_Valid(t *testing.T) {
	for _, kind := range []string{
		analytics.EventKindRepoCreated,
		analytics.EventKindAgentInstalled,
	} {
		e := analytics.Event{
			Kind:       kind,
			Timestamp:  time.Now(),
			SessionID:  "sess-nil-props",
			Properties: nil,
		}
		if err := e.Validate(); err != nil {
			t.Errorf("nil Properties should be valid for %q, got: %v", kind, err)
		}
	}
}

// ---- Test: empty Kind is rejected -------------------------------------------

// TestIntegration_EmptyKind_Rejected verifies that an empty Kind string is
// rejected with a descriptive error.
func TestIntegration_EmptyKind_Rejected(t *testing.T) {
	e := analytics.Event{
		Kind:      "",
		Timestamp: time.Now(),
		SessionID: "sess-empty-kind",
	}
	err := e.Validate()
	if err == nil {
		t.Fatal("expected error for empty Kind, got nil")
	}
	if !strings.Contains(err.Error(), "Kind") {
		t.Errorf("error message should mention 'Kind', got: %v", err)
	}
}

// ---- Test: invalid Kind characters ------------------------------------------

// TestIntegration_InvalidKindCharacters_Rejected verifies that Kind strings
// with disallowed characters (uppercase, spaces, hyphens, slashes) are
// rejected. The valid charset is [a-z0-9_.].
func TestIntegration_InvalidKindCharacters_Rejected(t *testing.T) {
	badKinds := []struct {
		desc string
		kind string
	}{
		{"uppercase letter", "Document.Published"},
		{"space in kind", "document published"},
		{"hyphen", "document-published"},
		{"slash", "document/published"},
		{"colon", "document:published"},
		{"exclamation mark", "document!published"},
		{"leading dot", ".document.published"},
		{"trailing dot", "document.published."},
	}

	for _, tc := range badKinds {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			e := analytics.Event{
				Kind:      tc.kind,
				Timestamp: time.Now(),
				SessionID: "s",
			}
			if err := e.Validate(); err == nil {
				t.Errorf("expected Validate to reject kind %q (%s), got nil", tc.kind, tc.desc)
			}
		})
	}
}

// ---- Test: Kind exactly 48 chars is the boundary limit ----------------------

// TestIntegration_KindLength_Boundary exercises the boundary condition for
// Kind length: 48 chars must pass, 49 must fail.
func TestIntegration_KindLength_Boundary(t *testing.T) {
	// Build a 48-char kind that is syntactically valid: "a.b" repeated until 48.
	// Simplest approach: 24 "a." pairs → 48 chars.
	kind48 := strings.Repeat("a", 24) + "." + strings.Repeat("b", 23) // "aaa...a.bbb...b" = 24+1+23 = 48
	if len(kind48) != 48 {
		t.Fatalf("test setup: expected 48 chars, got %d", len(kind48))
	}

	t.Run("exactly-48-chars-valid", func(t *testing.T) {
		e := analytics.Event{Kind: kind48, Timestamp: time.Now(), SessionID: "s"}
		if err := e.Validate(); err != nil {
			t.Errorf("48-char kind should be valid, got: %v", err)
		}
	})

	kind49 := kind48 + "x" // 49 chars
	t.Run("49-chars-invalid", func(t *testing.T) {
		e := analytics.Event{Kind: kind49, Timestamp: time.Now(), SessionID: "s"}
		if err := e.Validate(); err == nil {
			t.Error("49-char kind should be rejected, got nil error")
		}
	})
}

// ---- Test: zero Timestamp is rejected ---------------------------------------

// TestIntegration_ZeroTimestamp_Rejected verifies that a zero-value Timestamp
// is rejected for any event kind.
func TestIntegration_ZeroTimestamp_Rejected(t *testing.T) {
	for _, kind := range []string{
		analytics.EventKindDocumentPublished,
		analytics.EventKindSearchExecuted,
		analytics.EventKindOnboardingStarted,
	} {
		e := analytics.Event{
			Kind:      kind,
			Timestamp: time.Time{}, // zero value
			SessionID: "s",
		}
		if err := e.Validate(); err == nil {
			t.Errorf("expected error for zero Timestamp on kind %q, got nil", kind)
		}
	}
}

// ---- Test: empty SessionID is rejected --------------------------------------

// TestIntegration_EmptySessionID_Rejected verifies that an empty SessionID is
// rejected. Every event must be attributed to a daemon session.
func TestIntegration_EmptySessionID_Rejected(t *testing.T) {
	for _, kind := range []string{
		analytics.EventKindAgentTriggered,
		analytics.EventKindRepoRegistered,
		analytics.EventKindSettingsChanged,
	} {
		e := analytics.Event{
			Kind:      kind,
			Timestamp: time.Now(),
			SessionID: "",
		}
		if err := e.Validate(); err == nil {
			t.Errorf("expected error for empty SessionID on kind %q, got nil", kind)
		}
	}
}

// ---- Test: all 10 event kinds are distinct (no copy-paste duplicates) -------

// TestIntegration_AllKindsDistinct verifies no two bootstrap EventKind*
// constants share the same wire value. A duplicate would cause events to be
// misrouted in the analytics pipeline.
func TestIntegration_AllKindsDistinct(t *testing.T) {
	allKinds := []string{
		analytics.EventKindDocumentPublished,
		analytics.EventKindDocumentViewed,
		analytics.EventKindRepoRegistered,
		analytics.EventKindRepoCreated,
		analytics.EventKindAgentTriggered,
		analytics.EventKindAgentInstalled,
		analytics.EventKindOnboardingStarted,
		analytics.EventKindOnboardingCompleted,
		analytics.EventKindSearchExecuted,
		analytics.EventKindSettingsChanged,
	}

	seen := make(map[string]bool, len(allKinds))
	for _, k := range allKinds {
		if seen[k] {
			t.Errorf("duplicate EventKind constant value: %q", k)
		}
		seen[k] = true
	}
	if len(seen) != 10 {
		t.Errorf("expected exactly 10 distinct event kinds, got %d", len(seen))
	}
}

// ---- Test: event kinds follow subject.verb naming convention ----------------

// TestIntegration_KindNamingConvention verifies that all bootstrap event kinds
// follow the "subject.verb" pattern: must contain exactly one dot, no
// consecutive dots, and non-empty segments on both sides.
func TestIntegration_KindNamingConvention(t *testing.T) {
	allKinds := []struct {
		name string
		kind string
	}{
		{"document.published", analytics.EventKindDocumentPublished},
		{"document.viewed", analytics.EventKindDocumentViewed},
		{"repo.registered", analytics.EventKindRepoRegistered},
		{"repo.created", analytics.EventKindRepoCreated},
		{"agent.triggered", analytics.EventKindAgentTriggered},
		{"agent.installed", analytics.EventKindAgentInstalled},
		{"onboarding.started", analytics.EventKindOnboardingStarted},
		{"onboarding.completed", analytics.EventKindOnboardingCompleted},
		{"search.executed", analytics.EventKindSearchExecuted},
		{"settings.changed", analytics.EventKindSettingsChanged},
	}

	for _, tc := range allKinds {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Must contain at least one dot.
			if !strings.Contains(tc.kind, ".") {
				t.Errorf("%q: event kind must contain a dot (subject.verb convention)", tc.kind)
			}
			// Must not contain consecutive dots.
			if strings.Contains(tc.kind, "..") {
				t.Errorf("%q: event kind must not contain consecutive dots", tc.kind)
			}
			// Must not start or end with a dot.
			if strings.HasPrefix(tc.kind, ".") || strings.HasSuffix(tc.kind, ".") {
				t.Errorf("%q: event kind must not start or end with a dot", tc.kind)
			}
			// Wire value must equal the test case name (convention check).
			if tc.kind != tc.name {
				t.Errorf("EventKind constant wire value %q does not match expected %q", tc.kind, tc.name)
			}
		})
	}
}

// ---- Test: Validate error messages are descriptive --------------------------

// TestIntegration_ValidateErrorMessages verifies that Validate() returns error
// messages that name the specific field that failed. This ensures the caller
// (the event-writing goroutine) can produce actionable log output.
func TestIntegration_ValidateErrorMessages(t *testing.T) {
	cases := []struct {
		desc      string
		event     analytics.Event
		wantInMsg string
	}{
		{
			desc:      "empty Kind",
			event:     analytics.Event{Kind: "", Timestamp: time.Now(), SessionID: "s"},
			wantInMsg: "Kind",
		},
		{
			desc:      "invalid Kind",
			event:     analytics.Event{Kind: "BAD KIND!", Timestamp: time.Now(), SessionID: "s"},
			wantInMsg: "Kind",
		},
		{
			desc:      "zero Timestamp",
			event:     analytics.Event{Kind: "doc.ok", Timestamp: time.Time{}, SessionID: "s"},
			wantInMsg: "Timestamp",
		},
		{
			desc:      "empty SessionID",
			event:     analytics.Event{Kind: "doc.ok", Timestamp: time.Now(), SessionID: ""},
			wantInMsg: "SessionID",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.event.Validate()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantInMsg) {
				t.Errorf("error message should contain %q, got: %v", tc.wantInMsg, err)
			}
		})
	}
}
