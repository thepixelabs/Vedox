package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newTestVoiceServer builds a VoiceServer backed by a real (stub) Pipeline,
// starts the pipeline, and returns both.  The caller is responsible for
// stopping the pipeline.
func newTestVoiceServer(t *testing.T) (*VoiceServer, *Pipeline, context.CancelFunc) {
	t.Helper()

	trans := NewStubTranscriber(nil)
	src := NewStubAudioSource("")

	p, err := NewPipeline(PipelineConfig{
		Source:      src,
		Transcriber: trans,
		DaemonURL:   "http://127.0.0.1:4711",
		DispatchFunc: func(_ context.Context, _ Intent, _ string) error {
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}

	// VoiceServer registers OnActivity before pipeline starts.
	vs := NewVoiceServer(p)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := p.Start(ctx); err != nil {
		cancel()
		t.Fatalf("pipeline Start: %v", err)
	}

	return vs, p, cancel
}

// ---------------------------------------------------------------------------
// GET /api/voice/status
// ---------------------------------------------------------------------------

func TestVoiceServerStatusIdle(t *testing.T) {
	t.Parallel()

	vs, p, cancel := newTestVoiceServer(t)
	defer cancel()
	defer p.Stop() //nolint:errcheck

	// Wait for the pipeline to reach idle.
	time.Sleep(20 * time.Millisecond)

	mux := http.NewServeMux()
	vs.Mount(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/voice/status", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body statusResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Enabled {
		t.Error("enabled = false, want true")
	}
	if body.State != "idle" {
		t.Errorf("state = %q, want %q", body.State, "idle")
	}
}

func TestVoiceServerStatusListening(t *testing.T) {
	t.Parallel()

	vs, p, cancel := newTestVoiceServer(t)
	defer cancel()
	defer p.Stop() //nolint:errcheck

	mux := http.NewServeMux()
	vs.Mount(mux)

	// Activate PTT — pipeline should go to listening.
	p.SetPTT(true)

	// Poll until the server reports "listening".
	deadline := time.Now().Add(2 * time.Second)
	var lastState string
	for time.Now().Before(deadline) {
		req := httptest.NewRequest(http.MethodGet, "/api/voice/status", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		var body statusResponse
		if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		lastState = body.State
		if body.State == "listening" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("state = %q after PTT activate, want %q", lastState, "listening")
}

func TestVoiceServerStatusLastTranscriptCommand(t *testing.T) {
	t.Parallel()

	vs, p, cancel := newTestVoiceServer(t)
	defer cancel()
	defer p.Stop() //nolint:errcheck

	mux := http.NewServeMux()
	vs.Mount(mux)

	// Directly set last transcript and command (simulating what glue code would do).
	vs.SetLastTranscript("vedox document everything")
	vs.SetLastCommand(string(CommandDocumentEverything))

	req := httptest.NewRequest(http.MethodGet, "/api/voice/status", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body statusResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.LastTranscript != "vedox document everything" {
		t.Errorf("lastTranscript = %q, want %q", body.LastTranscript, "vedox document everything")
	}
	if body.LastCommand != string(CommandDocumentEverything) {
		t.Errorf("lastCommand = %q, want %q", body.LastCommand, CommandDocumentEverything)
	}
}

// ---------------------------------------------------------------------------
// POST /api/voice/ptt
// ---------------------------------------------------------------------------

func TestVoiceServerPTTActivate(t *testing.T) {
	t.Parallel()

	vs, p, cancel := newTestVoiceServer(t)
	defer cancel()
	defer p.Stop() //nolint:errcheck

	rec := &stateRecorder{}
	// Re-register the activity callback — VoiceServer already registered one,
	// but pipelines accept only one callback.  We test the pipeline state
	// indirectly via the VoiceServer's status endpoint instead.
	_ = rec // unused; we poll via HTTP

	mux := http.NewServeMux()
	vs.Mount(mux)

	body := bytes.NewBufferString(`{"active":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/voice/ptt", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Errorf("POST /api/voice/ptt (active=true): status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	// Deactivate.
	body2 := bytes.NewBufferString(`{"active":false}`)
	req2 := httptest.NewRequest(http.MethodPost, "/api/voice/ptt", body2)
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNoContent {
		t.Errorf("POST /api/voice/ptt (active=false): status = %d, want %d", rr2.Code, http.StatusNoContent)
	}

	// Verify the pipeline PTT field is now false via internal state.
	p.pttMu.Lock()
	active := p.pttActive
	p.pttMu.Unlock()
	if active {
		t.Error("pttActive = true after sending active=false, want false")
	}
}

func TestVoiceServerPTTBadBody(t *testing.T) {
	t.Parallel()

	vs, p, cancel := newTestVoiceServer(t)
	defer cancel()
	defer p.Stop() //nolint:errcheck

	mux := http.NewServeMux()
	vs.Mount(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/voice/ptt", bytes.NewBufferString(`not json`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("bad body: status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// Mount registers both routes
// ---------------------------------------------------------------------------

func TestVoiceServerMountRoutes(t *testing.T) {
	t.Parallel()

	vs, p, cancel := newTestVoiceServer(t)
	defer cancel()
	defer p.Stop() //nolint:errcheck

	mux := http.NewServeMux()
	vs.Mount(mux)

	// GET /api/voice/status
	req := httptest.NewRequest(http.MethodGet, "/api/voice/status", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /api/voice/status: status = %d, want %d", rr.Code, http.StatusOK)
	}

	// POST /api/voice/ptt
	req2 := httptest.NewRequest(http.MethodPost, "/api/voice/ptt", bytes.NewBufferString(`{"active":false}`))
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusNoContent {
		t.Errorf("POST /api/voice/ptt: status = %d, want %d", rr2.Code, http.StatusNoContent)
	}
}

// ---------------------------------------------------------------------------
// End-to-end: PTT via HTTP endpoint drives the pipeline to dispatch
// ---------------------------------------------------------------------------

func TestVoiceServerEndToEnd(t *testing.T) {
	t.Parallel()

	responses := make(chan string, 1)
	responses <- "vedox document everything"

	trans := NewStubTranscriber(responses)
	src := newControlledSource()

	var dispatchCount int32
	dispatchFn := func(_ context.Context, intent Intent, _ string) error {
		if intent.Command == CommandDocumentEverything {
			atomic.AddInt32(&dispatchCount, 1)
		}
		return nil
	}

	p, err := NewPipeline(PipelineConfig{
		Source:       src,
		Transcriber:  trans,
		DaemonURL:    "http://127.0.0.1:4711",
		DispatchFunc: dispatchFn,
	})
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}

	// VoiceServer.NewVoiceServer calls p.OnActivity internally.  We use the
	// VoiceServer's status endpoint to detect the listening state rather than
	// a separate stateRecorder (pipeline supports only one activity callback).
	vs := NewVoiceServer(p)
	mux := http.NewServeMux()
	vs.Mount(mux)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop() //nolint:errcheck

	// POST PTT active.
	pttOn := bytes.NewBufferString(`{"active":true}`)
	r1 := httptest.NewRequest(http.MethodPost, "/api/voice/ptt", pttOn)
	mux.ServeHTTP(httptest.NewRecorder(), r1)

	// Wait until VoiceServer reports "listening" before feeding audio.
	// This ensures the pipeline loop has processed the PTT=true signal.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		req := httptest.NewRequest(http.MethodGet, "/api/voice/status", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		var body statusResponse
		if err := json.NewDecoder(rr.Body).Decode(&body); err == nil && body.State == "listening" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Feed audio while listening.
	src.push(silenceChunk(DefaultChunkSamples))

	// POST PTT inactive — triggers transcribe + dispatch.
	pttOff := bytes.NewBufferString(`{"active":false}`)
	r2 := httptest.NewRequest(http.MethodPost, "/api/voice/ptt", pttOff)
	mux.ServeHTTP(httptest.NewRecorder(), r2)

	// Wait for dispatch.
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&dispatchCount) >= 1 {
			return // success
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("dispatch count = %d after PTT on+off via HTTP, want >= 1", atomic.LoadInt32(&dispatchCount))
}
