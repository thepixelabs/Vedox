package voice

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// collectStates records every VoiceState transition emitted by the pipeline.
type stateRecorder struct {
	mu     sync.Mutex
	states []VoiceState
}

func (r *stateRecorder) record(s VoiceState) {
	r.mu.Lock()
	r.states = append(r.states, s)
	r.mu.Unlock()
}

func (r *stateRecorder) snapshot() []VoiceState {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]VoiceState, len(r.states))
	copy(out, r.states)
	return out
}

// waitFor blocks until the stateRecorder has seen the target state at least
// minOccurrences times, or the deadline is exceeded.
func (r *stateRecorder) waitFor(t *testing.T, want VoiceState, deadline time.Duration) {
	t.Helper()
	r.waitForN(t, want, 1, deadline)
}

// waitForN blocks until the stateRecorder has seen the target state at least n
// times, or the deadline is exceeded.
func (r *stateRecorder) waitForN(t *testing.T, want VoiceState, n int, deadline time.Duration) {
	t.Helper()
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		count := 0
		for _, s := range r.snapshot() {
			if s == want {
				count++
			}
		}
		if count >= n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("state %v not seen %d time(s) within %v; states: %v", want, n, deadline, r.snapshot())
}

// controlledSource is a minimal AudioSource that lets tests push chunks on demand.
type controlledSource struct {
	outCh  chan []float32
	stopCh chan struct{}
}

func newControlledSource() *controlledSource {
	return &controlledSource{
		outCh:  make(chan []float32, 16),
		stopCh: make(chan struct{}),
	}
}

func (c *controlledSource) Start(_ context.Context) (<-chan []float32, error) {
	return c.outCh, nil
}

func (c *controlledSource) Stop() error {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
	return nil
}

// push sends a chunk to the pipeline's audio channel.
func (c *controlledSource) push(samples []float32) {
	c.outCh <- samples
}

// silenceChunk returns a chunk of silence of length n.
func silenceChunk(n int) []float32 {
	return make([]float32, n)
}

// ---------------------------------------------------------------------------
// TestPipelineBasicDispatch
// Tests the happy path: start → PTT on → audio → PTT off → transcribe → dispatch
// ---------------------------------------------------------------------------

func TestPipelineBasicDispatch(t *testing.T) {
	t.Parallel()

	var dispatchedIntent Intent
	var dispatchCount int32

	responses := make(chan string, 1)
	responses <- "vedox document everything"

	trans := NewStubTranscriber(responses)
	src := newControlledSource()
	rec := &stateRecorder{}

	var dispatchMu sync.Mutex
	dispatchFn := func(ctx context.Context, intent Intent, _ string) error {
		dispatchMu.Lock()
		dispatchedIntent = intent
		dispatchMu.Unlock()
		atomic.AddInt32(&dispatchCount, 1)
		return nil
	}

	p, err := NewPipeline(PipelineConfig{
		Source:        src,
		Transcriber:   trans,
		DaemonURL:     "http://127.0.0.1:4711",
		MinConfidence: 0.5,
		DispatchFunc:  dispatchFn,
	})
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	p.OnActivity(rec.record)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop() //nolint:errcheck

	// Activate PTT.
	p.SetPTT(true)
	rec.waitFor(t, VoiceStateListening, time.Second)

	// Feed some silence — just to populate the buffer.
	for i := 0; i < 5; i++ {
		src.push(silenceChunk(DefaultChunkSamples))
	}

	// Release PTT — pipeline should transcribe and dispatch.
	p.SetPTT(false)

	// Wait for the dispatching state to appear.
	rec.waitFor(t, VoiceStateDispatching, 3*time.Second)
	rec.waitFor(t, VoiceStateIdle, 3*time.Second)

	if n := atomic.LoadInt32(&dispatchCount); n != 1 {
		t.Errorf("dispatch called %d times, want 1", n)
	}

	dispatchMu.Lock()
	got := dispatchedIntent
	dispatchMu.Unlock()

	if got.Command != CommandDocumentEverything {
		t.Errorf("dispatched command = %q, want %q", got.Command, CommandDocumentEverything)
	}
	if got.Confidence < 0.99 {
		t.Errorf("dispatched confidence = %v, want >= 0.99", got.Confidence)
	}
}

// ---------------------------------------------------------------------------
// TestPipelineMultipleCommands
// PTT on/off twice in sequence — both transcriptions should dispatch.
// ---------------------------------------------------------------------------

func TestPipelineMultipleCommands(t *testing.T) {
	t.Parallel()

	responses := make(chan string, 2)
	responses <- "vedox document everything"
	responses <- "vedox stop"

	trans := NewStubTranscriber(responses)
	src := newControlledSource()

	var dispatched []Command
	var mu sync.Mutex

	dispatchFn := func(_ context.Context, intent Intent, _ string) error {
		mu.Lock()
		dispatched = append(dispatched, intent.Command)
		mu.Unlock()
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop() //nolint:errcheck

	// First PTT cycle.
	p.SetPTT(true)
	time.Sleep(20 * time.Millisecond)
	src.push(silenceChunk(DefaultChunkSamples))
	p.SetPTT(false)
	time.Sleep(200 * time.Millisecond)

	// Second PTT cycle.
	p.SetPTT(true)
	time.Sleep(20 * time.Millisecond)
	src.push(silenceChunk(DefaultChunkSamples))
	p.SetPTT(false)
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	got := append([]Command(nil), dispatched...)
	mu.Unlock()

	if len(got) != 2 {
		t.Fatalf("dispatch called %d times, want 2; got: %v", len(got), got)
	}
	if got[0] != CommandDocumentEverything {
		t.Errorf("dispatch[0] = %q, want %q", got[0], CommandDocumentEverything)
	}
	if got[1] != CommandStop {
		t.Errorf("dispatch[1] = %q, want %q", got[1], CommandStop)
	}
}

// ---------------------------------------------------------------------------
// TestPipelinePTTTimeout
// If PTT is held for PTTMaxDuration, the pipeline auto-releases.
// We shorten the max via a custom test-only pipeline that overrides the
// timeout via a monkeypatched approach, instead we use a very short fake
// timer by starting PTT and letting the timer fire, then checking dispatch.
//
// Because PTTMaxDuration is 30s (too long for a unit test), this test builds
// a pipeline with a custom dispatchFn and verifies the timeout code path
// using a shortened TTL by embedding a channel trick: the test sends PTT=true,
// then sends a time signal by calling the internal loop indirectly.
//
// Practical approach: inject a fast-timeout pipeline variant.
// ---------------------------------------------------------------------------

func TestPipelinePTTTimeoutPath(t *testing.T) {
	t.Parallel()

	// We cannot easily shorten PTTMaxDuration without making it a field.
	// However, we CAN test the auto-release code path by verifying that
	// a pipeline whose PTT is set true — and then the context cancelled
	// while PTT is active — does not dispatch (graceful shutdown wins over
	// timeout in that race), AND separately verify the path exists via
	// compilation and logic inspection.
	//
	// For an integration-style timeout test: build a pipeline, set PTT true,
	// wait slightly longer than PTTMaxDuration, then check dispatch was called.
	// This is gated with t.Skip in short mode.

	if testing.Short() {
		t.Skip("PTT timeout test skipped in short mode (requires 30s wait)")
	}

	responses := make(chan string, 1)
	responses <- "vedox document everything"

	trans := NewStubTranscriber(responses)
	src := newControlledSource()
	rec := &stateRecorder{}

	var dispatched int32
	dispatchFn := func(_ context.Context, _ Intent, _ string) error {
		atomic.AddInt32(&dispatched, 1)
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
	p.OnActivity(rec.record)

	ctx, cancel := context.WithTimeout(context.Background(), PTTMaxDuration+5*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop() //nolint:errcheck

	// Feed silence while PTT is held — the timer should fire after 30s.
	p.SetPTT(true)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				src.push(silenceChunk(DefaultChunkSamples))
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	// Wait for auto-release dispatch.
	rec.waitFor(t, VoiceStateDispatching, PTTMaxDuration+3*time.Second)

	if n := atomic.LoadInt32(&dispatched); n < 1 {
		t.Errorf("dispatch count = %d after PTT timeout, want >= 1", n)
	}
}

// ---------------------------------------------------------------------------
// TestPipelineContextCancellation
// Cancelling the context should cleanly stop the pipeline.
// ---------------------------------------------------------------------------

func TestPipelineContextCancellation(t *testing.T) {
	t.Parallel()

	trans := NewStubTranscriber(nil)
	src := newControlledSource()
	rec := &stateRecorder{}

	dispatched := int32(0)
	dispatchFn := func(_ context.Context, _ Intent, _ string) error {
		atomic.AddInt32(&dispatched, 1)
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
	p.OnActivity(rec.record)

	ctx, cancel := context.WithCancel(context.Background())

	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Let the pipeline settle.
	rec.waitFor(t, VoiceStateIdle, time.Second)

	// Cancel the context — should stop the pipeline cleanly.
	cancel()

	// Stop must return (not hang).
	stopDone := make(chan error, 1)
	go func() { stopDone <- p.Stop() }()

	select {
	case err := <-stopDone:
		if err != nil {
			t.Errorf("Stop returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Stop did not return within 3s after context cancellation")
	}

	// doneCh must be closed.
	select {
	case <-p.doneCh:
		// ok
	default:
		t.Error("pipeline doneCh not closed after Stop")
	}

	// No dispatch should have fired (PTT was never activated).
	if n := atomic.LoadInt32(&dispatched); n != 0 {
		t.Errorf("dispatch count = %d, want 0", n)
	}
}

// ---------------------------------------------------------------------------
// TestPipelineStopWithoutStart
// Stop before Start must not panic or hang.
// ---------------------------------------------------------------------------

func TestPipelineStopWithoutStart(t *testing.T) {
	t.Parallel()

	trans := NewStubTranscriber(nil)
	src := newControlledSource()

	p, err := NewPipeline(PipelineConfig{
		Source:      src,
		Transcriber: trans,
		DaemonURL:   "http://127.0.0.1:4711",
	})
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}

	// Stop before Start must not panic or hang. The started flag gates the
	// <-p.doneCh wait so this returns promptly.
	done := make(chan error, 1)
	go func() { done <- p.Stop() }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Stop before Start returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Stop before Start hung — possible deadlock")
	}
}

// ---------------------------------------------------------------------------
// TestPipelineLowConfidenceTranscript
// A transcript that produces an intent below MinConfidence should cause
// VoiceStateError, not dispatch.
// ---------------------------------------------------------------------------

func TestPipelineLowConfidenceTranscript(t *testing.T) {
	t.Parallel()

	// "hello world" produces CommandUnknown, Confidence = 0.0 — below any
	// reasonable MinConfidence threshold.
	responses := make(chan string, 1)
	responses <- "hello world"

	trans := NewStubTranscriber(responses)
	src := newControlledSource()
	rec := &stateRecorder{}

	dispatched := int32(0)
	dispatchFn := func(_ context.Context, _ Intent, _ string) error {
		atomic.AddInt32(&dispatched, 1)
		return nil
	}

	p, err := NewPipeline(PipelineConfig{
		Source:        src,
		Transcriber:   trans,
		DaemonURL:     "http://127.0.0.1:4711",
		MinConfidence: 0.5,
		DispatchFunc:  dispatchFn,
	})
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	p.OnActivity(rec.record)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop() //nolint:errcheck

	p.SetPTT(true)
	time.Sleep(20 * time.Millisecond)
	src.push(silenceChunk(DefaultChunkSamples))
	p.SetPTT(false)

	// Expect error state.
	rec.waitFor(t, VoiceStateError, 3*time.Second)

	if n := atomic.LoadInt32(&dispatched); n != 0 {
		t.Errorf("dispatch count = %d, want 0 for low-confidence transcript", n)
	}
}

// ---------------------------------------------------------------------------
// TestPipelineDispatchError
// If Dispatch returns an error, the pipeline signals VoiceStateError but
// continues running.
// ---------------------------------------------------------------------------

func TestPipelineDispatchError(t *testing.T) {
	t.Parallel()

	responses := make(chan string, 2)
	responses <- "vedox document everything"
	responses <- "vedox stop"

	trans := NewStubTranscriber(responses)
	src := newControlledSource()
	rec := &stateRecorder{}

	callCount := int32(0)
	dispatchFn := func(_ context.Context, _ Intent, _ string) error {
		n := atomic.AddInt32(&callCount, 1)
		if n == 1 {
			return errors.New("daemon unreachable")
		}
		return nil
	}

	p, err := NewPipeline(PipelineConfig{
		Source:        src,
		Transcriber:   trans,
		DaemonURL:     "http://127.0.0.1:4711",
		MinConfidence: 0.5,
		DispatchFunc:  dispatchFn,
	})
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	p.OnActivity(rec.record)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop() //nolint:errcheck

	// First PTT cycle — dispatch will error.
	p.SetPTT(true)
	rec.waitForN(t, VoiceStateListening, 1, time.Second)
	src.push(silenceChunk(DefaultChunkSamples))
	p.SetPTT(false)
	rec.waitFor(t, VoiceStateError, 3*time.Second)

	// Brief settle to ensure the loop has returned to its select before we
	// kick off the second cycle.
	time.Sleep(50 * time.Millisecond)

	// Second PTT cycle — dispatch succeeds; pipeline is still alive.
	// Wait for the 2nd occurrence of VoiceStateListening.
	p.SetPTT(true)
	rec.waitForN(t, VoiceStateListening, 2, time.Second)
	src.push(silenceChunk(DefaultChunkSamples))
	p.SetPTT(false)
	// Wait for 2nd dispatching and 2nd idle occurrences.
	rec.waitForN(t, VoiceStateDispatching, 1, 3*time.Second)
	rec.waitForN(t, VoiceStateIdle, 2, 3*time.Second)

	if n := atomic.LoadInt32(&callCount); n != 2 {
		t.Errorf("dispatch call count = %d, want 2", n)
	}
}

// ---------------------------------------------------------------------------
// TestPipelineIdleChunksDiscarded
// While PTT is not active, audio chunks arriving from the source must be
// silently discarded — the buffer must remain empty.
// ---------------------------------------------------------------------------

func TestPipelineIdleChunksDiscarded(t *testing.T) {
	t.Parallel()

	trans := NewStubTranscriber(nil) // returns "" → CommandUnknown
	src := newControlledSource()
	rec := &stateRecorder{}

	dispatched := int32(0)
	dispatchFn := func(_ context.Context, _ Intent, _ string) error {
		atomic.AddInt32(&dispatched, 1)
		return nil
	}

	p, err := NewPipeline(PipelineConfig{
		Source:        src,
		Transcriber:   trans,
		DaemonURL:     "http://127.0.0.1:4711",
		MinConfidence: 0.5,
		DispatchFunc:  dispatchFn,
	})
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	p.OnActivity(rec.record)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop() //nolint:errcheck

	rec.waitFor(t, VoiceStateIdle, time.Second)

	// Push 50 chunks with PTT inactive.
	for i := 0; i < 50; i++ {
		src.push(silenceChunk(DefaultChunkSamples))
	}

	// Short wait — no dispatch should fire.
	time.Sleep(100 * time.Millisecond)

	if n := atomic.LoadInt32(&dispatched); n != 0 {
		t.Errorf("dispatch count = %d, want 0 while PTT inactive", n)
	}
	// Pipeline should still be in idle state throughout.
	for _, s := range rec.snapshot() {
		if s == VoiceStateListening || s == VoiceStateTranscribing || s == VoiceStateDispatching {
			t.Errorf("unexpected state %v while PTT was never activated", s)
		}
	}
}

// ---------------------------------------------------------------------------
// TestNewPipelineMissingConfig
// NewPipeline must reject configs missing required fields.
// ---------------------------------------------------------------------------

func TestNewPipelineMissingConfig(t *testing.T) {
	t.Parallel()

	trans := NewStubTranscriber(nil)
	src := newControlledSource()

	cases := []struct {
		name string
		cfg  PipelineConfig
	}{
		{
			name: "missing source",
			cfg: PipelineConfig{
				Transcriber: trans,
				DaemonURL:   "http://127.0.0.1:4711",
			},
		},
		{
			name: "missing transcriber",
			cfg: PipelineConfig{
				Source:    src,
				DaemonURL: "http://127.0.0.1:4711",
			},
		},
		{
			name: "missing daemon URL",
			cfg: PipelineConfig{
				Source:      src,
				Transcriber: trans,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewPipeline(tc.cfg)
			if err == nil {
				t.Errorf("NewPipeline(%s) = nil error, want non-nil", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestVoiceStateString
// Verify String() on every VoiceState value.
// ---------------------------------------------------------------------------

func TestVoiceStateString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		state VoiceState
		want  string
	}{
		{VoiceStateIdle, "idle"},
		{VoiceStateListening, "listening"},
		{VoiceStateTranscribing, "transcribing"},
		{VoiceStateDispatching, "dispatching"},
		{VoiceStateError, "error"},
		{VoiceState(99), "VoiceState(99)"},
	}

	for _, tc := range cases {
		if got := tc.state.String(); got != tc.want {
			t.Errorf("VoiceState(%d).String() = %q, want %q", int(tc.state), got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// TestStubTranscriberLifecycle
// Verify StubTranscriber respects Close and context cancellation.
// ---------------------------------------------------------------------------

func TestStubTranscriberLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("returns responses in order", func(t *testing.T) {
		t.Parallel()
		ch := make(chan string, 2)
		ch <- "first"
		ch <- "second"
		s := NewStubTranscriber(ch)

		ctx := context.Background()
		got1, err := s.Transcribe(ctx, silenceChunk(16))
		if err != nil || got1 != "first" {
			t.Errorf("first Transcribe = %q, %v; want first, nil", got1, err)
		}
		got2, err := s.Transcribe(ctx, silenceChunk(16))
		if err != nil || got2 != "second" {
			t.Errorf("second Transcribe = %q, %v; want second, nil", got2, err)
		}
		// Drained — falls back to FallbackText.
		got3, err := s.Transcribe(ctx, silenceChunk(16))
		if err != nil || got3 != "" {
			t.Errorf("third Transcribe = %q, %v; want empty, nil", got3, err)
		}
	})

	t.Run("empty audio returns empty string", func(t *testing.T) {
		t.Parallel()
		s := NewStubTranscriber(nil)
		got, err := s.Transcribe(context.Background(), nil)
		if err != nil || got != "" {
			t.Errorf("Transcribe(nil) = %q, %v; want empty, nil", got, err)
		}
	})

	t.Run("cancelled context returns error", func(t *testing.T) {
		t.Parallel()
		s := NewStubTranscriber(nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := s.Transcribe(ctx, silenceChunk(16))
		if err == nil {
			t.Error("Transcribe with cancelled context returned nil error")
		}
	})

	t.Run("after Close returns error", func(t *testing.T) {
		t.Parallel()
		s := NewStubTranscriber(nil)
		if err := s.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		_, err := s.Transcribe(context.Background(), silenceChunk(16))
		if err == nil {
			t.Error("Transcribe after Close returned nil error")
		}
	})
}

// ---------------------------------------------------------------------------
// TestStubAudioSourceSilence
// StubAudioSource in silence mode must emit chunks until stopped.
// ---------------------------------------------------------------------------

func TestStubAudioSourceSilence(t *testing.T) {
	t.Parallel()

	src := NewStubAudioSource("")
	ctx, cancel := context.WithCancel(context.Background())

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Read a few chunks and verify they are all-zero.
	for i := 0; i < 3; i++ {
		chunk := <-ch
		if len(chunk) == 0 {
			t.Errorf("chunk %d: empty", i)
		}
		for j, v := range chunk {
			if v != 0 {
				t.Errorf("chunk %d sample %d = %v, want 0 (silence)", i, j, v)
			}
		}
	}

	// Stop via context cancellation — channel must close.
	cancel()
	timeout := time.After(time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // channel closed — correct
			}
		case <-timeout:
			t.Fatal("audio channel not closed after context cancellation")
		}
	}
}

// ---------------------------------------------------------------------------
// TestStubAudioSourceStop
// StubAudioSource.Stop must close the channel promptly.
// ---------------------------------------------------------------------------

func TestStubAudioSourceStop(t *testing.T) {
	t.Parallel()

	src := NewStubAudioSource("")
	ctx := context.Background()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Drain one chunk to confirm it is running.
	<-ch

	if err := src.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	timeout := time.After(time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // closed — correct
			}
		case <-timeout:
			t.Fatal("audio channel not closed after Stop")
		}
	}
}
