package agentauth

// Tests for RejectAllAuth — the fail-closed replacement for PassthroughAuth
// used when the daemon cannot load its HMAC key store. FIX-SEC-10.
//
// Acceptance contract:
//   - Every request, regardless of method or path, returns HTTP 503.
//   - The inner handler is never invoked.
//   - The response body carries the VDX-302 code + a short message.
//   - No detail about the underlying keystore failure leaks to the client.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// invokeReject wraps a sentinel handler with RejectAllAuth and invokes it with
// the supplied method/path/body. It returns the recorder so callers can assert
// on status, body, and the "inner was called" flag.
func invokeReject(method, path, body string) (*httptest.ResponseRecorder, *bool) {
	innerCalled := new(bool)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*innerCalled = true
		w.WriteHeader(http.StatusOK)
	})
	mw := RejectAllAuth()
	handler := mw(inner)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	handler.ServeHTTP(rec, req)
	return rec, innerCalled
}

// TestRejectAllAuth_Returns503_POST pins the primary contract: a mutating
// request is rejected with 503 before the inner handler runs.
func TestRejectAllAuth_Returns503_POST(t *testing.T) {
	rec, innerCalled := invokeReject(http.MethodPost, "/api/docs/whatever", `{"body":"x"}`)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if *innerCalled {
		t.Error("inner handler was invoked — RejectAllAuth must short-circuit")
	}
}

// TestRejectAllAuth_Returns503_GET verifies that read-only verbs are also
// rejected. A key-store outage means we cannot authenticate an agent for
// any verb — GET included.
func TestRejectAllAuth_Returns503_GET(t *testing.T) {
	rec, innerCalled := invokeReject(http.MethodGet, "/api/docs/foo", "")

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if *innerCalled {
		t.Error("inner handler was invoked for GET — RejectAllAuth must short-circuit every verb")
	}
}

// TestRejectAllAuth_ResponseBody_Shape verifies the JSON payload carries the
// VDX-302 error code and an operator-friendly message, without leaking
// underlying failure details.
func TestRejectAllAuth_ResponseBody_Shape(t *testing.T) {
	rec, _ := invokeReject(http.MethodPost, "/api/docs/x", "{}")

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var payload map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["code"] != "VDX-302" {
		t.Errorf("code = %q, want VDX-302", payload["code"])
	}
	if payload["message"] == "" {
		t.Error("message must not be empty")
	}
	// Guard against detail leakage: the body must not contain "keychain"
	// internals, stack traces, or file paths.
	lower := strings.ToLower(payload["message"])
	forbidden := []string{"keychain", "traceback", "/users/", "/home/", ".vedox"}
	for _, f := range forbidden {
		if strings.Contains(lower, f) {
			t.Errorf("message leaks internal detail containing %q: %s", f, payload["message"])
		}
	}
}

// TestRejectAllAuth_MultipleInvocations verifies the middleware is stateless
// — every invocation returns the same 503 and never lets a request through
// on retry.
func TestRejectAllAuth_MultipleInvocations(t *testing.T) {
	mw := RejectAllAuth()
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := mw(inner)

	for i := 0; i < 5; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/docs", strings.NewReader("{}"))
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("iteration %d: status = %d, want 503", i, rec.Code)
		}
	}
}
