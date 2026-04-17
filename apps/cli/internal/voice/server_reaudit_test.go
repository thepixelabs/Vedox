package voice

// Re-audit tests for voice HTTP server — FIX-SEC-02 verification and new
// attacker vectors (rate limiting, oversize body).

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandlePTT_NoOriginHeader is the wave-0 FIX-SEC-02 acceptance test.
// Voice routes are now mounted on the chi router in production, so every PTT
// request flows through corsMiddleware. The raw Mount() variant (used here for
// test isolation) deliberately does NOT include CORS — we document the
// in-production behaviour via server_test.go against the api package.
//
// This test confirms that the handler itself still accepts a POST without an
// Origin header, because the CSRF check lives in the api package's
// corsMiddleware, not in the handler. Kept as a regression anchor.
func TestHandlePTT_NoOriginHeader_HandlerLevel(t *testing.T) {
	pipe := newTestPipeline(t)
	vs := NewVoiceServer(pipe)

	mux := http.NewServeMux()
	vs.Mount(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/voice/ptt",
		bytes.NewReader([]byte(`{"active":true}`)))
	// No Origin header — the raw mux has no CORS middleware.
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("handler-level PTT: got %d, want 204", rec.Code)
	}
}

// TestHandlePTT_RateLimit verifies that rapid-fire PTT activations are
// throttled with 429 Too Many Requests. Without the limiter a malicious local
// process can toggle the microphone thousands of times per second.
func TestHandlePTT_RateLimit(t *testing.T) {
	pipe := newTestPipeline(t)
	vs := NewVoiceServer(pipe)

	mux := http.NewServeMux()
	vs.Mount(mux)

	got429 := false
	// Send pttRateLimit+5 requests back-to-back. At least one of the last
	// few must receive 429.
	for i := 0; i < pttRateLimit+5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/voice/ptt",
			bytes.NewReader([]byte(`{"active":false}`)))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			got429 = true
			if !strings.Contains(rec.Body.String(), "VDX-429") {
				t.Errorf("429 body missing VDX-429: %s", rec.Body.String())
			}
		}
	}
	if !got429 {
		t.Fatalf("rate limiter never triggered 429 in %d attempts", pttRateLimit+5)
	}
}

// TestHandlePTT_BodyTooLarge confirms MaxBytesReader caps the body so a
// malicious caller cannot OOM the daemon with a multi-megabyte payload.
func TestHandlePTT_BodyTooLarge(t *testing.T) {
	pipe := newTestPipeline(t)
	vs := NewVoiceServer(pipe)

	mux := http.NewServeMux()
	vs.Mount(mux)

	// 8 KB body — above the 1 KB PTT limit but small enough to send in one shot.
	big := []byte(`{"active":true,"pad":"` + strings.Repeat("A", 8*1024) + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/voice/ptt",
		bytes.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code < 400 {
		t.Fatalf("oversized body: got %d, want 4xx", rec.Code)
	}
}

// newTestPipeline constructs the smallest viable Pipeline for HTTP tests.
// The pipeline is never started; we only need its SetPTT method to resolve
// and its OnActivity hook to not crash.
func newTestPipeline(t *testing.T) *Pipeline {
	t.Helper()
	p, err := NewPipeline(PipelineConfig{
		Source:      NewStubAudioSource(""),
		Transcriber: NewStubTranscriber(nil),
		DaemonURL:   "http://127.0.0.1:4711",
		DispatchFunc: func(_ context.Context, _ Intent, _ string) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	return p
}
