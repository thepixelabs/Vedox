// Package analytics defines the canonical event payload struct and kind
// constants used by every event-emitting workstream in Vedox.
//
// Event kind naming follows the subject.verb convention ratified in
// FINAL_PLAN.md OQ-K: lowercase, dot-separated, max 48 characters,
// alphabet [a-z0-9_.]. The taxonomy here is the first-10 bootstrap set;
// additional kinds are added by workstream owners importing this package
// and declaring new EventKind* constants in their own files.
//
// Constraint: every workstream that emits events (WS-A, WS-B, WS-C,
// WS-L, WS-F, WS-G, WS-H, WS-I, WS-J, WS-M, WS-E per FINAL_PLAN.md
// changelog item 4) MUST import this package and use its Event struct.
// Do not define parallel event structs elsewhere.
package analytics

import (
	"fmt"
	"regexp"
	"time"
)

// Event is the canonical payload for every analytics event in Vedox.
// Events are written to the per-workspace events table via the writer
// goroutine and aggregated daily into global.db::events_daily by the
// background aggregator (R1, SQLite-tail pattern).
//
// Events never leave the machine (zero outbound network calls rule).
type Event struct {
	// Kind is a dot-separated subject.verb identifier (e.g. "document.published").
	// Maximum 48 characters. Valid characters: [a-z0-9_.].
	// Use the EventKind* constants defined in this package.
	Kind string

	// Timestamp is when the event occurred. Callers should set this to
	// time.Now() at the moment the action is observed, not deferred.
	Timestamp time.Time

	// SessionID is an opaque per-daemon-start identifier (UUID v4). Set
	// once by the daemon on startup and threaded through to every event
	// emitter. Allows session-level analytics (e.g. "commands per session").
	SessionID string

	// Properties is a free-form map of additional event attributes. Each
	// workstream documents the properties it writes; the analytics reader
	// trusts their shape after Validate() passes. Nil is valid (no properties).
	Properties map[string]any
}

// Validate returns an error if the event is malformed. Callers MUST call
// Validate before writing an event to the database so the reader can trust
// the shape of stored rows.
//
// Rules enforced:
//   - Kind must not be empty.
//   - Kind must match [a-z0-9_.]{1,48}.
//   - Timestamp must not be the zero value.
//   - SessionID must not be empty.
func (e Event) Validate() error {
	if e.Kind == "" {
		return fmt.Errorf("analytics: event Kind must not be empty")
	}
	if !validKind.MatchString(e.Kind) {
		return fmt.Errorf("analytics: event Kind %q is invalid (must match [a-z0-9_.]{1,48})", e.Kind)
	}
	if e.Timestamp.IsZero() {
		return fmt.Errorf("analytics: event Timestamp must not be zero")
	}
	if e.SessionID == "" {
		return fmt.Errorf("analytics: event SessionID must not be empty")
	}
	return nil
}

// validKind enforces the subject.verb naming contract.
//
// Rules:
//   - Allowed characters: [a-z0-9_.]
//   - Total length: 1–48 characters.
//   - Must not start or end with a dot (empty leading/trailing segment).
//   - Must not contain consecutive dots (empty interior segment).
//
// The pattern anchors on a non-dot start and non-dot end, with optional
// dot-separated interior segments.
var validKind = regexp.MustCompile(`^[a-z0-9_][a-z0-9_.]{0,46}[a-z0-9_]$|^[a-z0-9_]$`)

// ---------------------------------------------------------------------------
// Event kind constants — first 10 bootstrap events.
//
// Naming convention: EventKind<Subject><Verb> in Go; "subject.verb" on wire.
// Subjects: document, repo, agent, onboarding, search, settings
// ---------------------------------------------------------------------------

const (
	// EventKindDocumentPublished fires when a document transitions to
	// status=published, either via the agent or a manual save.
	EventKindDocumentPublished = "document.published"

	// EventKindDocumentViewed fires when a document is opened in the editor
	// or fetched via GET /api/docs/:id.
	EventKindDocumentViewed = "document.viewed"

	// EventKindRepoRegistered fires when an existing Git repo is registered
	// with Vedox (the "register" path in onboarding step 2 or settings).
	EventKindRepoRegistered = "repo.registered"

	// EventKindRepoCreated fires when a new documentation repo is created
	// via gh CLI or the bare-local inbox path during onboarding.
	EventKindRepoCreated = "repo.created"

	// EventKindAgentTriggered fires when the Doc Agent trigger phrase is
	// detected (voice or inline), before the agent actually runs.
	EventKindAgentTriggered = "agent.triggered"

	// EventKindAgentInstalled fires when the Doc Agent pack is successfully
	// installed into a provider (claude-code, codex, copilot, or gemini).
	EventKindAgentInstalled = "agent.installed"

	// EventKindOnboardingStarted fires at the first step of the onboarding
	// flow, whether on first run or re-triggered from Settings.
	EventKindOnboardingStarted = "onboarding.started"

	// EventKindOnboardingCompleted fires when the user reaches the final
	// "you're ready" screen of the onboarding flow (step 5).
	EventKindOnboardingCompleted = "onboarding.completed"

	// EventKindSearchExecuted fires when the user submits a search query
	// via the command palette or the search bar.
	EventKindSearchExecuted = "search.executed"

	// EventKindSettingsChanged fires when any user preference is mutated
	// via PUT /api/settings (partial-merge semantics per R3).
	EventKindSettingsChanged = "settings.changed"
)
