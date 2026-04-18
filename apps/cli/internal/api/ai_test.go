package api

// Tests for the three AI handlers in ai.go (previously 0% coverage):
//
//   - handleAIProviders          GET  /api/ai/providers
//   - handleGenerateNames        POST /api/ai/generate-names
//   - handleGenerateNamesStatus  GET  /api/ai/generate-names/{jobId}
//
// The tests reuse coverageFixture from handlers_coverage_test.go for the full
// chi-mounted server stack (CORS + logging middleware applied), so these are
// real HTTP round-trips rather than in-process handler invocations.
//
// Job execution is exercised end-to-end against a non-existent provider binary
// (ProviderClaude with PATH cleared) so RunGeneration fails fast without any
// real AI subprocess. We then assert observable status + error fields on the
// completed job.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/vedox/vedox/internal/ai"
)

// ── Test helpers ──────────────────────────────────────────────────────────────

// providersResponseBody is the JSON shape returned by GET /api/ai/providers.
// Mirrors the anonymous struct in handleAIProviders so tests can decode it.
type providersResponseBody struct {
	Providers []ai.ProviderInfo `json:"providers"`
	Altergo   ai.AltergoInfo    `json:"altergo"`
}

// generateNamesAcceptedBody is the JSON shape returned by 202 from
// handleGenerateNames.
type generateNamesAcceptedBody struct {
	JobID string `json:"jobId"`
}

// postRaw sends a POST whose body is already a JSON byte slice. Unlike
// coverageFixture.post it does not re-marshal, which lets us send malformed
// JSON to exercise the VDX-000 branch.
func postRaw(t *testing.T, f *coverageFixture, path string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, f.server.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest POST %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// pollJobUntil polls GET /api/ai/generate-names/{jobId} until the response
// status field equals want or the deadline expires. Returns the final response
// body. We poll the HTTP layer (not the JobStore) so that polling itself goes
// through Snapshot — exercising the handler we are testing.
func pollJobUntil(t *testing.T, f *coverageFixture, jobID string, want ai.JobStatus, timeout time.Duration) ai.GenerationJob {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last ai.GenerationJob
	for time.Now().Before(deadline) {
		resp := f.get(t, "/api/ai/generate-names/"+jobID)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("poll: status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("poll: read body: %v", err)
		}
		if err := json.Unmarshal(body, &last); err != nil {
			t.Fatalf("poll: decode JSON: %v (body=%s)", err, body)
		}
		if last.Status == want {
			return last
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("pollJobUntil(%s): timed out after %s; final status = %q", want, timeout, last.Status)
	return last
}

// validGenerateNamesBody returns a minimal request body that passes validation
// (provider known, count in range). The Provider is "claude" so the job will
// fail with "binary not found on PATH" rather than actually shelling out — as
// long as the test isolates PATH (see isolatePATH).
func validGenerateNamesBody() map[string]interface{} {
	return map[string]interface{}{
		"provider": "claude",
		"account":  "",
		"count":    5,
		"params": map[string]interface{}{
			"categories": []string{"developer-tools"},
			"tone":       "playful",
		},
	}
}

// isolatePATH points PATH at a fresh empty directory and HOME at a fresh
// temp directory. This guarantees:
//   - DetectAvailableProviders returns Available=false for every provider
//     (no binaries on PATH)
//   - DiscoverAltergo returns Available=false (no ~/.altergo dir under HOME)
//   - RunGeneration fails immediately with "AI CLI not found on PATH"
//
// Returns nothing; relies on t.Setenv for automatic cleanup.
func isolatePATH(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	t.Setenv("HOME", t.TempDir())
}

// ── handleAIProviders ─────────────────────────────────────────────────────────

// TestAIProviders_HappyPath checks the response shape: Providers slice has one
// entry per known provider, every entry has a stable ID + Name, and the Altergo
// section is present. PATH is isolated so the assertion does not depend on
// what binaries the developer has installed AND so that no real `claude
// --version` (which prompts for auth) ever runs and hangs the test.
func TestAIProviders_HappyPath(t *testing.T) {
	isolatePATH(t)
	f := newCoverageServer(t)

	resp := f.get(t, "/api/ai/providers")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}

	var got providersResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	// All four known providers must appear, in declaration order.
	wantIDs := []ai.ProviderID{ai.ProviderClaude, ai.ProviderGemini, ai.ProviderCodex, ai.ProviderCopilot}
	if len(got.Providers) != len(wantIDs) {
		t.Fatalf("len(providers) = %d, want %d (got=%+v)", len(got.Providers), len(wantIDs), got.Providers)
	}
	for i, want := range wantIDs {
		if got.Providers[i].ID != want {
			t.Errorf("providers[%d].ID = %q, want %q", i, got.Providers[i].ID, want)
		}
		if got.Providers[i].Name == "" {
			t.Errorf("providers[%d].Name is empty for %s", i, want)
		}
	}
}

// TestAIProviders_NoBinariesOnPATH verifies that a host with zero AI CLIs
// installed returns Available=false for every provider AND a non-error 200
// response. This is the regression guard for "fresh laptop with nothing
// installed" — the most common first-run state.
func TestAIProviders_NoBinariesOnPATH(t *testing.T) {
	isolatePATH(t)
	f := newCoverageServer(t)

	resp := f.get(t, "/api/ai/providers")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}

	var got providersResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	for _, p := range got.Providers {
		if p.Available {
			t.Errorf("provider %q reported Available=true with empty PATH; binary lookup leaked: %+v", p.ID, p)
		}
		if p.Version != "" {
			t.Errorf("provider %q reported Version=%q with empty PATH (expected blank)", p.ID, p.Version)
		}
	}

	// AlterGo must be Available=false because HOME points at a fresh tmpdir
	// with no .altergo directory.
	if got.Altergo.Available {
		t.Errorf("Altergo.Available = true with isolated HOME; want false")
	}
}

// ── handleGenerateNames ───────────────────────────────────────────────────────

// TestGenerateNames_ValidBody returns 202 + a job ID for a well-formed request.
// We isolate PATH so the background goroutine cannot actually exec the AI CLI
// — only the HTTP-layer contract (status, jobId, content-type) is asserted.
func TestGenerateNames_ValidBody(t *testing.T) {
	isolatePATH(t)
	f := newCoverageServer(t)

	resp := postRaw(t, f, "/api/ai/generate-names", mustJSON(t, validGenerateNamesBody()))
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got generateNamesAcceptedBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if got.JobID == "" {
		t.Error("jobId is empty in 202 response")
	}
	if len(got.JobID) < 16 {
		t.Errorf("jobId %q looks too short — newAIJobID returns 32-char hex", got.JobID)
	}
}

// TestGenerateNames_MalformedJSON returns 400 VDX-000 when the body is not
// valid JSON (e.g. trailing junk, empty body).
func TestGenerateNames_MalformedJSON(t *testing.T) {
	f := newCoverageServer(t)

	resp := postRaw(t, f, "/api/ai/generate-names", []byte("{not json"))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
	body := bodyStr(t, resp)
	if !strings.Contains(body, "VDX-000") {
		t.Errorf("expected VDX-000 in body, got %s", body)
	}
}

// TestGenerateNames_UnknownProvider returns 400 VDX-400 for a provider ID that
// is not in the known providers map. This guards the trust boundary: if the
// handler ever forwarded an unknown provider to RunGeneration, it would shell
// out to a binary chosen by the client (RCE if we're unlucky with PATH).
func TestGenerateNames_UnknownProvider(t *testing.T) {
	f := newCoverageServer(t)

	body := validGenerateNamesBody()
	body["provider"] = "evil-rce-binary"

	resp := postRaw(t, f, "/api/ai/generate-names", mustJSON(t, body))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
	respBody := bodyStr(t, resp)
	if !strings.Contains(respBody, "VDX-400") {
		t.Errorf("expected VDX-400 in body, got %s", respBody)
	}
	if !strings.Contains(respBody, "unknown provider") {
		t.Errorf("expected 'unknown provider' message in body, got %s", respBody)
	}
}

// TestGenerateNames_MissingProvider returns 400 VDX-400 when provider is empty.
// Empty string maps to BinaryForProvider("") == "", which is the same code path
// as an unknown provider but is the more common client mistake (forgot to
// include the field).
func TestGenerateNames_MissingProvider(t *testing.T) {
	f := newCoverageServer(t)

	body := validGenerateNamesBody()
	delete(body, "provider")

	resp := postRaw(t, f, "/api/ai/generate-names", mustJSON(t, body))
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
	if !strings.Contains(bodyStr(t, resp), "VDX-400") {
		t.Errorf("expected VDX-400 for missing provider, got %s", bodyStr(t, resp))
	}
}

// TestGenerateNames_CountOutOfRange covers both ends of the validation:
// negative counts and counts above the documented max (20).
func TestGenerateNames_CountOutOfRange(t *testing.T) {
	cases := []struct {
		name  string
		count int
	}{
		{"negative", -1},
		{"too_large", 21},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newCoverageServer(t)

			body := validGenerateNamesBody()
			body["count"] = c.count

			resp := postRaw(t, f, "/api/ai/generate-names", mustJSON(t, body))
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body=%s)", resp.StatusCode, bodyStr(t, resp))
			}
			respBody := bodyStr(t, resp)
			if !strings.Contains(respBody, "VDX-400") {
				t.Errorf("expected VDX-400 for count=%d, got %s", c.count, respBody)
			}
			if !strings.Contains(respBody, "between 1 and 20") {
				t.Errorf("expected count-range message, got %s", respBody)
			}
		})
	}
}

// ── handleGenerateNamesStatus ─────────────────────────────────────────────────

// TestGenerateNamesStatus_UnknownJobID returns 404 VDX-404 for an ID that
// was never submitted (or that the server cleared on restart).
func TestGenerateNamesStatus_UnknownJobID(t *testing.T) {
	f := newCoverageServer(t)

	resp := f.get(t, "/api/ai/generate-names/0000000000000000deadbeef00000000")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body=%s)", resp.StatusCode, bodyStr(t, resp))
	}
	body := bodyStr(t, resp)
	if !strings.Contains(body, "VDX-404") {
		t.Errorf("expected VDX-404 in body, got %s", body)
	}
}

// TestGenerateNamesStatus_RunningOrPending submits a job and immediately
// queries the status endpoint. We accept either pending or running because the
// transition is asynchronous and timing-dependent — the contract the handler
// owes is "200 + a valid GenerationJob payload", not which non-terminal state.
func TestGenerateNamesStatus_RunningOrPending(t *testing.T) {
	isolatePATH(t)
	f := newCoverageServer(t)

	// Submit a job so we have a real ID in the store.
	submitResp := postRaw(t, f, "/api/ai/generate-names", mustJSON(t, validGenerateNamesBody()))
	if submitResp.StatusCode != http.StatusAccepted {
		t.Fatalf("submit: status = %d, want 202 (body=%s)", submitResp.StatusCode, bodyStr(t, submitResp))
	}
	var sub generateNamesAcceptedBody
	if err := json.NewDecoder(submitResp.Body).Decode(&sub); err != nil {
		t.Fatalf("decode submit: %v", err)
	}

	// Query status. The job will likely be running or already finished by now;
	// we just need to confirm the handler returns 200 + a valid payload that
	// echoes the same job ID. Snapshot ensures reading mid-flight is race-safe
	// (the job goroutine may still be mutating fields).
	statusResp := f.get(t, "/api/ai/generate-names/"+sub.JobID)
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("status: status = %d, want 200 (body=%s)", statusResp.StatusCode, bodyStr(t, statusResp))
	}
	var job ai.GenerationJob
	if err := json.NewDecoder(statusResp.Body).Decode(&job); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if job.ID != sub.JobID {
		t.Errorf("status: job.ID = %q, want %q (Snapshot must echo the original ID)", job.ID, sub.JobID)
	}
	// Status must be one of the four known values — never empty.
	switch job.Status {
	case ai.JobPending, ai.JobRunning, ai.JobDone, ai.JobError:
		// ok
	default:
		t.Errorf("job.Status = %q; want one of pending/running/done/error", job.Status)
	}
	if job.StartedAt.IsZero() {
		t.Error("job.StartedAt must be set immediately on submit")
	}
}

// TestGenerateNamesStatus_CompletedJob submits a job, polls until it reaches
// the terminal JobError state (the AI CLI binary is not on PATH), then asserts
// the terminal-state invariants documented on GenerationJob: Error is set,
// CompletedAt is non-nil. This is the only path that exercises the full
// pending → running → error lifecycle through the HTTP handler.
func TestGenerateNamesStatus_CompletedJob(t *testing.T) {
	isolatePATH(t)
	f := newCoverageServer(t)

	submitResp := postRaw(t, f, "/api/ai/generate-names", mustJSON(t, validGenerateNamesBody()))
	if submitResp.StatusCode != http.StatusAccepted {
		t.Fatalf("submit: status = %d, want 202 (body=%s)", submitResp.StatusCode, bodyStr(t, submitResp))
	}
	var sub generateNamesAcceptedBody
	if err := json.NewDecoder(submitResp.Body).Decode(&sub); err != nil {
		t.Fatalf("decode submit: %v", err)
	}

	// 5s is generous: the failure path is "exec.LookPath returns ENOENT" —
	// nanoseconds — but the goroutine scheduler may take a few ms to pick up
	// the job under -race.
	job := pollJobUntil(t, f, sub.JobID, ai.JobError, 5*time.Second)

	if job.Error == "" {
		t.Error("terminal job: Error must be non-empty when Status == error")
	}
	if job.CompletedAt == nil {
		t.Error("terminal job: CompletedAt must be set")
	}
	if !strings.Contains(job.Error, "not found") && !strings.Contains(job.Error, "PATH") {
		// The exact wording comes from RunGeneration; we only require that the
		// message mentions the missing-binary failure mode so a regression to
		// "AI CLI error: exit 1" (which would mean we somehow exec'd something)
		// would be caught.
		t.Errorf("Error = %q; want a missing-binary message", job.Error)
	}
}

// (mustJSON lives in providers_test.go and is reused here.)
