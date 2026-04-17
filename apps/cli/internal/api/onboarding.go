package api

// POST /api/onboarding/complete
//
// A narrow fire-and-forget endpoint the SvelteKit onboarding flow posts to
// once the user reaches the final "you're ready" screen (AllDone.svelte).
// The server response itself carries no data — the endpoint exists so the
// analytics Collector can emit onboarding.completed with whatever context
// the client chose to include in the body.
//
// Design rationale: we deliberately did NOT overload POST /api/settings for
// this (settings is PATCH-merge of user prefs, not an event sink) and we did
// not try to infer "completed" from other endpoints. A single explicit
// event point keeps the wire contract obvious and the server handler
// trivial to test.

import (
	"encoding/json"
	"net/http"
)

// onboardingCompleteRequest is the optional JSON body for
// POST /api/onboarding/complete. Every field is optional; an empty body
// is equivalent to a body of `{}`. The fields are echoed verbatim into the
// event properties so the dashboard can break out completion rates by
// the slice the user actually went through (e.g. skipped steps, installed
// providers). The server does not validate values — analytics is
// intentionally schema-loose; the aggregator trusts the shape.
type onboardingCompleteRequest struct {
	// SkippedSteps is the list of step indices the user skipped (1-5).
	SkippedSteps []int `json:"skippedSteps,omitempty"`
	// SelectedProviders is the list of agent provider IDs the user chose
	// during the install-agent step (e.g. ["claude-code", "codex"]).
	SelectedProviders []string `json:"selectedProviders,omitempty"`
	// RegisteredRepos is the count of repos the user registered during
	// onboarding. We ship a count, not the paths, so we never echo
	// arbitrary filesystem paths back through the analytics pipeline.
	RegisteredRepos int `json:"registeredRepos,omitempty"`
}

// handleOnboardingComplete handles POST /api/onboarding/complete.
//
// It emits onboarding.completed and returns 204 No Content. A missing or
// malformed body still emits the event (with empty properties) — the user
// finishing onboarding is what matters; the telemetry detail is a bonus.
func (s *Server) handleOnboardingComplete(w http.ResponseWriter, r *http.Request) {
	// Cap body at 16 KiB — the request is a small analytics blob; anything
	// bigger is either malformed or a malicious probe.
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)

	var req onboardingCompleteRequest
	// Tolerate an empty or malformed body: analytics must never gate on
	// client-side JSON quirks. We only decode when there is something to
	// decode; a decode error is swallowed and the event fires with an
	// empty payload.
	if r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	props := map[string]any{}
	if len(req.SkippedSteps) > 0 {
		props["skipped_steps"] = req.SkippedSteps
	}
	if len(req.SelectedProviders) > 0 {
		props["selected_providers"] = req.SelectedProviders
	}
	if req.RegisteredRepos > 0 {
		props["registered_repos"] = req.RegisteredRepos
	}
	// Pass nil instead of an empty map so event rows stay NULL-valued when
	// the client sent no properties (matches existing analytics rows).
	if len(props) == 0 {
		props = nil
	}

	s.emitEvent("onboarding.completed", props)

	w.WriteHeader(http.StatusNoContent)
}
