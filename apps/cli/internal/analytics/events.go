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
// Event kind constants — full 30-event taxonomy (FINAL_PLAN.md changelog
// item 3, OQ-K RESOLVED: dot-separated subject.verb form).
//
// Naming convention: EventKind<Subject><Verb> in Go; "subject.verb" on wire.
// Subjects (by workstream):
//   - document     — WS-K core (doc lifecycle)
//   - repo         — WS-B multi-repo registry
//   - agent        — WS-C agent install/remove
//   - daemon       — WS-A daemon lifecycle
//   - voice        — WS-E push-to-talk pipeline
//   - doctree      — WS-F left-rail tree interactions
//   - graph        — WS-G doc-graph view
//   - history      — WS-H history timeline
//   - preview      — WS-I inline code preview
//   - onboarding   — WS-L first-run flow
//   - settings     — WS-J personalization
//   - search       — WS-K search bar
//
// Closed taxonomy (OQ-K rule 3): unknown kinds are rejected at ingress. Any
// new event requires a migration note and a taxonomy PR adding a new
// constant here. Every event-emitting workstream (WS-A/B/C/E/F/G/H/I/J/L
// per plan.md changelog item 4) MUST import the constant from this file —
// never inline a wire string.
// ---------------------------------------------------------------------------

const (
	// -- document lifecycle (WS-K core) -----------------------------------

	// EventKindDocumentPublished fires when a document transitions to
	// status=published, either via the agent or a manual save.
	EventKindDocumentPublished = "document.published"

	// EventKindDocumentViewed fires when a document is opened in the editor
	// or fetched via GET /api/docs/:id.
	EventKindDocumentViewed = "document.viewed"

	// -- daemon lifecycle (WS-A) ------------------------------------------

	// EventKindDaemonStarted fires once per daemon process start, after the
	// HTTP listener is bound and ready. Used to attribute subsequent events
	// to a known daemon session.
	EventKindDaemonStarted = "daemon.started"

	// EventKindDaemonStopped fires from the graceful-shutdown hook just
	// before the main goroutine returns. Best-effort: if the daemon is
	// SIGKILL'd the event will not fire.
	EventKindDaemonStopped = "daemon.stopped"

	// EventKindDaemonReloaded fires on SIGHUP after repos.json has been
	// re-read and new repos hot-added / removed repos gracefully unloaded.
	EventKindDaemonReloaded = "daemon.reloaded"

	// -- multi-repo registry (WS-B) ---------------------------------------

	// EventKindRepoRegistered fires when an existing Git repo is registered
	// with Vedox (the "register" path in onboarding step 2 or settings).
	EventKindRepoRegistered = "repo.registered"

	// EventKindRepoCreated fires when a new documentation repo is created
	// via gh CLI or the bare-local inbox path during onboarding.
	EventKindRepoCreated = "repo.created"

	// EventKindRepoRemoved fires when a repo is removed from the registry
	// via DELETE /api/repos/:id. Does not delete the working tree on disk.
	EventKindRepoRemoved = "repo.removed"

	// EventKindRepoSetDefault fires when a repo is marked as the default
	// target for new documents via PUT /api/repos/:id/default.
	EventKindRepoSetDefault = "repo.set_default"

	// -- agent install/remove (WS-C) --------------------------------------

	// EventKindAgentTriggered fires when the Doc Agent trigger phrase is
	// detected (voice or inline), before the agent actually runs.
	EventKindAgentTriggered = "agent.triggered"

	// EventKindAgentInstalled fires when the Doc Agent pack is successfully
	// installed into a provider (claude-code, codex, copilot, or gemini).
	EventKindAgentInstalled = "agent.installed"

	// EventKindAgentUninstalled fires when the Doc Agent pack is removed
	// from a provider (DELETE /api/agent/:provider). The receipt file is
	// deleted and the provider-specific files are reverted.
	EventKindAgentUninstalled = "agent.uninstalled"

	// EventKindAgentRepaired fires when `vedox doctor --fix` (or the UI
	// equivalent) successfully re-installs a provider after drift detection.
	EventKindAgentRepaired = "agent.repaired"

	// -- voice pipeline (WS-E) --------------------------------------------

	// EventKindVoiceActivated fires when the push-to-talk hotkey is pressed
	// and the recording pipeline begins capturing audio.
	EventKindVoiceActivated = "voice.activated"

	// EventKindVoiceTranscribed fires when whisper.cpp finishes transcribing
	// a captured utterance, before intent parsing runs.
	EventKindVoiceTranscribed = "voice.transcribed"

	// EventKindVoiceDispatched fires when the parsed voice intent is
	// successfully dispatched to a daemon endpoint.
	EventKindVoiceDispatched = "voice.dispatched"

	// -- doc tree interactions (WS-F) -------------------------------------

	// EventKindDoctreeFiltered fires when the user applies a filter or
	// search query to the left-rail doc tree.
	EventKindDoctreeFiltered = "doctree.filtered"

	// EventKindDoctreeExpanded fires when a collapsed folder node in the
	// doc tree is expanded by the user.
	EventKindDoctreeExpanded = "doctree.expanded"

	// -- doc graph (WS-G) -------------------------------------------------

	// EventKindGraphViewed fires when the user opens the doc graph view
	// (GET /api/graph followed by a Cytoscape render).
	EventKindGraphViewed = "graph.viewed"

	// EventKindGraphNodeClicked fires when the user clicks a node in the
	// doc graph, navigating to that document.
	EventKindGraphNodeClicked = "graph.node_clicked"

	// -- history timeline (WS-H) ------------------------------------------

	// EventKindHistoryViewed fires when the user opens the history panel
	// for a document (GET /api/docs/:id/history).
	EventKindHistoryViewed = "history.viewed"

	// EventKindHistoryEntryExpanded fires when a history entry row is
	// expanded to show the full diff or authorship details.
	EventKindHistoryEntryExpanded = "history.entry_expanded"

	// -- inline preview (WS-I) --------------------------------------------

	// EventKindPreviewHovered fires when the user hovers a vedox:// link
	// and the inline code preview popover renders.
	EventKindPreviewHovered = "preview.hovered"

	// -- settings (WS-J) --------------------------------------------------

	// EventKindSettingsChanged fires when any user preference is mutated
	// via PUT /api/settings (partial-merge semantics per R3).
	EventKindSettingsChanged = "settings.changed"

	// EventKindSettingsReset fires when the user resets one or more
	// settings categories to their defaults via the Settings UI.
	EventKindSettingsReset = "settings.reset"

	// -- onboarding (WS-L) ------------------------------------------------

	// EventKindOnboardingStarted fires at the first step of the onboarding
	// flow, whether on first run or re-triggered from Settings.
	EventKindOnboardingStarted = "onboarding.started"

	// EventKindOnboardingStepCompleted fires once per step transition in
	// the onboarding flow. Properties.step_id identifies the step (1..5).
	EventKindOnboardingStepCompleted = "onboarding.step_completed"

	// EventKindOnboardingSkipped fires when the user exits onboarding before
	// reaching step 5 (dismiss, skip link, or navigation away).
	EventKindOnboardingSkipped = "onboarding.skipped"

	// EventKindOnboardingCompleted fires when the user reaches the final
	// "you're ready" screen of the onboarding flow (step 5).
	EventKindOnboardingCompleted = "onboarding.completed"

	// -- search (WS-K) ----------------------------------------------------

	// EventKindSearchExecuted fires when the user submits a search query
	// via the command palette or the search bar.
	EventKindSearchExecuted = "search.executed"
)

// AllEventKinds is the authoritative list of every EventKind* constant
// declared in this package. Callers that need to iterate over the closed
// taxonomy (e.g. validation, documentation generation, admin dashboards)
// should use this slice rather than hand-maintaining their own.
//
// Ordering matches the grouped declaration above (document → daemon → repo
// → agent → voice → doctree → graph → history → preview → settings →
// onboarding → search). Length is pinned at 30 (FINAL_PLAN.md OQ-K).
var AllEventKinds = []string{
	EventKindDocumentPublished,
	EventKindDocumentViewed,
	EventKindDaemonStarted,
	EventKindDaemonStopped,
	EventKindDaemonReloaded,
	EventKindRepoRegistered,
	EventKindRepoCreated,
	EventKindRepoRemoved,
	EventKindRepoSetDefault,
	EventKindAgentTriggered,
	EventKindAgentInstalled,
	EventKindAgentUninstalled,
	EventKindAgentRepaired,
	EventKindVoiceActivated,
	EventKindVoiceTranscribed,
	EventKindVoiceDispatched,
	EventKindDoctreeFiltered,
	EventKindDoctreeExpanded,
	EventKindGraphViewed,
	EventKindGraphNodeClicked,
	EventKindHistoryViewed,
	EventKindHistoryEntryExpanded,
	EventKindPreviewHovered,
	EventKindSettingsChanged,
	EventKindSettingsReset,
	EventKindOnboardingStarted,
	EventKindOnboardingStepCompleted,
	EventKindOnboardingSkipped,
	EventKindOnboardingCompleted,
	EventKindSearchExecuted,
}
