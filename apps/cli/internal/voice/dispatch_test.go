package voice

// Tests for Dispatch — the intent → daemon API bridge. Previously 0% covered
// despite being the entire end-to-end user-visible voice feature. Every
// command routes to a different endpoint; every HTTP error path produces a
// DispatchError. All must be pinned.

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// recordedCall captures one request the test server received so assertions
// about method, path, and body can be made after the fact.
type recordedCall struct {
	Method string
	Path   string
	Body   []byte
}

// newDaemonStub returns an httptest.Server that records every request and
// responds with the given status code. If status == 0 it responds 204.
func newDaemonStub(t *testing.T, status int) (*httptest.Server, *[]recordedCall) {
	t.Helper()
	var calls []recordedCall
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		calls = append(calls, recordedCall{
			Method: r.Method,
			Path:   r.URL.Path,
			Body:   body,
		})
		if status == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte("stub body"))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// TestDispatch_DocumentEverything verifies the trigger call for the
// "document everything" command has the correct endpoint, method, and body.
// A regression where the command key is wrong makes the daemon reject the
// trigger but the voice UI still shows success.
func TestDispatch_DocumentEverything(t *testing.T) {
	srv, calls := newDaemonStub(t, 0)

	err := Dispatch(context.Background(), Intent{Command: CommandDocumentEverything}, srv.URL)
	if err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(*calls))
	}
	c := (*calls)[0]
	if c.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", c.Method)
	}
	if c.Path != "/v1/agent/trigger" {
		t.Errorf("path = %q, want /v1/agent/trigger", c.Path)
	}
	var payload triggerRequest
	if err := json.Unmarshal(c.Body, &payload); err != nil {
		t.Fatalf("unmarshal body: %v (body=%s)", err, c.Body)
	}
	if payload.Command != "document_everything" {
		t.Errorf("payload.Command = %q, want document_everything", payload.Command)
	}
	if payload.Target != "" {
		t.Errorf("payload.Target = %q, want empty", payload.Target)
	}
}

// TestDispatch_DocumentFolder passes a target folder through to the payload.
func TestDispatch_DocumentFolder(t *testing.T) {
	srv, calls := newDaemonStub(t, 0)

	err := Dispatch(context.Background(), Intent{
		Command: CommandDocumentFolder,
		Target:  "docs/architecture",
	}, srv.URL)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(*calls))
	}
	var payload triggerRequest
	_ = json.Unmarshal((*calls)[0].Body, &payload)
	if payload.Command != "document_folder" {
		t.Errorf("Command = %q, want document_folder", payload.Command)
	}
	if payload.Target != "docs/architecture" {
		t.Errorf("Target = %q, want docs/architecture", payload.Target)
	}
}

// TestDispatch_DocumentChanges carries no target.
func TestDispatch_DocumentChanges(t *testing.T) {
	srv, calls := newDaemonStub(t, 0)

	err := Dispatch(context.Background(), Intent{Command: CommandDocumentChanges}, srv.URL)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	var payload triggerRequest
	_ = json.Unmarshal((*calls)[0].Body, &payload)
	if payload.Command != "document_changes" {
		t.Errorf("Command = %q, want document_changes", payload.Command)
	}
	if payload.Target != "" {
		t.Errorf("Target = %q, want empty for document_changes", payload.Target)
	}
}

// TestDispatch_DocumentFile passes the path argument through.
func TestDispatch_DocumentFile(t *testing.T) {
	srv, calls := newDaemonStub(t, 0)

	err := Dispatch(context.Background(), Intent{
		Command: CommandDocumentFile,
		Target:  "docs/api/readme.md",
	}, srv.URL)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	var payload triggerRequest
	_ = json.Unmarshal((*calls)[0].Body, &payload)
	if payload.Command != "document_file" {
		t.Errorf("Command = %q, want document_file", payload.Command)
	}
	if payload.Target != "docs/api/readme.md" {
		t.Errorf("Target = %q, want docs/api/readme.md", payload.Target)
	}
}

// TestDispatch_Status hits /healthz via GET.
func TestDispatch_Status(t *testing.T) {
	srv, calls := newDaemonStub(t, http.StatusOK)

	err := Dispatch(context.Background(), Intent{Command: CommandStatus}, srv.URL)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(*calls))
	}
	if (*calls)[0].Method != http.MethodGet {
		t.Errorf("method = %q, want GET", (*calls)[0].Method)
	}
	if (*calls)[0].Path != "/healthz" {
		t.Errorf("path = %q, want /healthz", (*calls)[0].Path)
	}
}

// TestDispatch_Stop hits /v1/agent/cancel via POST.
func TestDispatch_Stop(t *testing.T) {
	srv, calls := newDaemonStub(t, 0)

	err := Dispatch(context.Background(), Intent{Command: CommandStop}, srv.URL)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if (*calls)[0].Method != http.MethodPost {
		t.Errorf("method = %q, want POST", (*calls)[0].Method)
	}
	if (*calls)[0].Path != "/v1/agent/cancel" {
		t.Errorf("path = %q, want /v1/agent/cancel", (*calls)[0].Path)
	}
}

// TestDispatch_Unknown returns a DispatchError with no HTTP call made.
func TestDispatch_Unknown(t *testing.T) {
	srv, calls := newDaemonStub(t, 0)

	err := Dispatch(context.Background(), Intent{
		Command: CommandUnknown,
		RawText: "vedox do the needful",
	}, srv.URL)
	if err == nil {
		t.Fatalf("Dispatch returned nil, want DispatchError for unknown command")
	}
	dispErr, ok := err.(*DispatchError)
	if !ok {
		t.Fatalf("error type = %T, want *DispatchError", err)
	}
	if dispErr.Command != CommandUnknown {
		t.Errorf("err.Command = %q, want CommandUnknown", dispErr.Command)
	}
	// Raw text of the transcript must be embedded in the error so the UI can
	// echo it to the user for debugging.
	if !strings.Contains(dispErr.Error(), "vedox do the needful") {
		t.Errorf("error message missing raw transcript: %q", dispErr.Error())
	}
	if len(*calls) != 0 {
		t.Errorf("unknown command made %d HTTP calls, want 0", len(*calls))
	}
}

// TestDispatch_DefaultBranch triggers the `default:` branch of the switch by
// passing an unrecognised Command constant. This path returns a DispatchError
// without contacting the daemon — callers rely on it to surface programmer
// error rather than succeed silently.
func TestDispatch_DefaultBranch(t *testing.T) {
	srv, calls := newDaemonStub(t, 0)

	err := Dispatch(context.Background(), Intent{Command: Command("invented")}, srv.URL)
	if err == nil {
		t.Fatalf("Dispatch returned nil, want error for bogus command constant")
	}
	if _, ok := err.(*DispatchError); !ok {
		t.Errorf("error type = %T, want *DispatchError", err)
	}
	if len(*calls) != 0 {
		t.Errorf("default branch made %d HTTP calls, want 0", len(*calls))
	}
}

// TestDispatch_HTTP5xx_ReturnsDispatchError covers the error-body capture
// branch in postTrigger. The UI relies on DispatchError.StatusCode and
// DispatchError.Body to render a diagnostic to the user.
func TestDispatch_HTTP5xx_ReturnsDispatchError(t *testing.T) {
	srv, _ := newDaemonStub(t, http.StatusInternalServerError)

	err := Dispatch(context.Background(), Intent{Command: CommandDocumentEverything}, srv.URL)
	if err == nil {
		t.Fatalf("Dispatch returned nil, want error for 500 response")
	}
	dispErr, ok := err.(*DispatchError)
	if !ok {
		t.Fatalf("error type = %T, want *DispatchError", err)
	}
	if dispErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", dispErr.StatusCode)
	}
	if dispErr.Body == "" {
		t.Errorf("Body is empty, want captured server body")
	}
	// DispatchError.Error() format for HTTP errors must include the status
	// code and body; Unwrap() returns nil (the error is not wrapped).
	errText := dispErr.Error()
	if !strings.Contains(errText, "HTTP 500") {
		t.Errorf("Error() = %q; want substring HTTP 500", errText)
	}
	if dispErr.Unwrap() != nil {
		t.Errorf("Unwrap() = %v, want nil for HTTP-level error", dispErr.Unwrap())
	}
}

// TestDispatch_ConnectionRefused returns a DispatchError whose Unwrap yields
// the underlying network error.
func TestDispatch_ConnectionRefused(t *testing.T) {
	// Point at an almost-certainly-dead port. We do not use t.Deadline()
	// because httpClient has its own 10s timeout; connection-refused should
	// return in milliseconds.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := Dispatch(ctx, Intent{Command: CommandDocumentEverything}, "http://127.0.0.1:1")
	if err == nil {
		t.Fatalf("Dispatch returned nil, want connection error")
	}
	dispErr, ok := err.(*DispatchError)
	if !ok {
		t.Fatalf("error type = %T, want *DispatchError", err)
	}
	if dispErr.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0 (non-HTTP error)", dispErr.StatusCode)
	}
	if dispErr.Unwrap() == nil {
		t.Errorf("Unwrap() = nil, want the underlying net error")
	}
}

// TestDispatch_TrimsTrailingSlash verifies the guard that normalises the
// daemon URL. A daemonURL with a trailing slash would previously produce
// "http://host//v1/agent/trigger" which some HTTP stacks reject.
func TestDispatch_TrimsTrailingSlash(t *testing.T) {
	srv, calls := newDaemonStub(t, 0)

	// Supply a URL with a trailing slash.
	err := Dispatch(context.Background(), Intent{Command: CommandDocumentEverything}, srv.URL+"/")
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("got %d calls, want 1", len(*calls))
	}
	if (*calls)[0].Path != "/v1/agent/trigger" {
		t.Errorf("path = %q (double slash?) want /v1/agent/trigger", (*calls)[0].Path)
	}
}

// TestDispatch_Status_Non2xx returns a DispatchError when /healthz reports
// an error status.
func TestDispatch_Status_Non2xx(t *testing.T) {
	srv, _ := newDaemonStub(t, http.StatusServiceUnavailable)

	err := Dispatch(context.Background(), Intent{Command: CommandStatus}, srv.URL)
	if err == nil {
		t.Fatalf("Dispatch returned nil, want DispatchError for 503")
	}
	dispErr, ok := err.(*DispatchError)
	if !ok {
		t.Fatalf("error type = %T, want *DispatchError", err)
	}
	if dispErr.Command != CommandStatus {
		t.Errorf("Command = %q, want CommandStatus", dispErr.Command)
	}
	if dispErr.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("StatusCode = %d, want 503", dispErr.StatusCode)
	}
}

// TestDispatch_Stop_Non2xx mirrors the status-error test for the /cancel
// endpoint so the cancel error path is also covered.
func TestDispatch_Stop_Non2xx(t *testing.T) {
	srv, _ := newDaemonStub(t, http.StatusBadGateway)

	err := Dispatch(context.Background(), Intent{Command: CommandStop}, srv.URL)
	dispErr, ok := err.(*DispatchError)
	if !ok {
		t.Fatalf("error type = %T, want *DispatchError", err)
	}
	if dispErr.Command != CommandStop {
		t.Errorf("Command = %q, want CommandStop", dispErr.Command)
	}
	if dispErr.StatusCode != http.StatusBadGateway {
		t.Errorf("StatusCode = %d, want 502", dispErr.StatusCode)
	}
}
