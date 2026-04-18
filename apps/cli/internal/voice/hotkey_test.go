package voice

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// StubHotkeyListener
// ---------------------------------------------------------------------------

func TestStubHotkeyListenerStartStop(t *testing.T) {
	t.Parallel()

	l := NewStubHotkeyListener(DefaultHotkeyConfig())
	ctx := context.Background()

	if err := l.Start(ctx, func() {}, func() {}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := l.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	// Second Stop must be idempotent.
	if err := l.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

func TestStubHotkeyListenerDoubleStart(t *testing.T) {
	t.Parallel()

	l := NewStubHotkeyListener(DefaultHotkeyConfig())
	ctx := context.Background()

	if err := l.Start(ctx, func() {}, func() {}); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	defer l.Stop() //nolint:errcheck

	if err := l.Start(ctx, func() {}, func() {}); err == nil {
		t.Error("second Start returned nil error, want error")
	}
}

func TestStubHotkeyListenerCallbacks(t *testing.T) {
	t.Parallel()

	var pressCount, releaseCount int32

	l := NewStubHotkeyListener(DefaultHotkeyConfig())
	ctx := context.Background()

	if err := l.Start(ctx,
		func() { atomic.AddInt32(&pressCount, 1) },
		func() { atomic.AddInt32(&releaseCount, 1) },
	); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer l.Stop() //nolint:errcheck

	l.SimulatePress()
	l.SimulatePress()
	l.SimulateRelease()

	// Give callbacks a moment to complete (they are synchronous, so this is
	// belt-and-suspenders).
	time.Sleep(10 * time.Millisecond)

	if n := atomic.LoadInt32(&pressCount); n != 2 {
		t.Errorf("pressCount = %d, want 2", n)
	}
	if n := atomic.LoadInt32(&releaseCount); n != 1 {
		t.Errorf("releaseCount = %d, want 1", n)
	}
}

func TestStubHotkeyListenerSimulateBeforeStart(t *testing.T) {
	t.Parallel()

	// SimulatePress / SimulateRelease before Start must not panic.
	l := NewStubHotkeyListener(DefaultHotkeyConfig())
	l.SimulatePress()
	l.SimulateRelease()
}

func TestStubHotkeyListenerContextCancellation(t *testing.T) {
	t.Parallel()

	l := NewStubHotkeyListener(DefaultHotkeyConfig())
	ctx, cancel := context.WithCancel(context.Background())

	if err := l.Start(ctx, func() {}, func() {}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Cancel the context — the listener should stop itself.
	cancel()

	// Allow the internal goroutine to detect the cancellation.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		l.mu.Lock()
		stopped := l.stopped
		l.mu.Unlock()
		if stopped {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Error("listener did not stop after context cancellation")
}

// ---------------------------------------------------------------------------
// NativeHotkeyListener
// ---------------------------------------------------------------------------

func TestNativeHotkeyListenerReturnsUnavailable(t *testing.T) {
	t.Parallel()

	l := NewNativeHotkeyListener(DefaultHotkeyConfig())
	err := l.Start(context.Background(), func() {}, func() {})
	if err == nil {
		t.Error("NativeHotkeyListener.Start returned nil error, want ErrNativeHotkeyUnavailable")
	}
}

func TestNativeHotkeyListenerStopIsNoOp(t *testing.T) {
	t.Parallel()

	l := NewNativeHotkeyListener(DefaultHotkeyConfig())
	if err := l.Stop(); err != nil {
		t.Errorf("Stop returned error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// NewBestHotkeyListener
// ---------------------------------------------------------------------------

func TestNewBestHotkeyListenerReturnsFallback(t *testing.T) {
	t.Parallel()

	// NativeHotkeyListener always returns unavailable in this build, so
	// NewBestHotkeyListener must return a stub with stubFallback = true.
	l, isFallback := NewBestHotkeyListener(DefaultHotkeyConfig())
	if !isFallback {
		t.Error("expected stubFallback = true in this build")
	}
	if l == nil {
		t.Fatal("returned listener is nil")
	}

	// The fallback listener must be usable.
	ctx := context.Background()
	if err := l.Start(ctx, func() {}, func() {}); err != nil {
		t.Fatalf("fallback listener Start: %v", err)
	}
	if err := l.Stop(); err != nil {
		t.Fatalf("fallback listener Stop: %v", err)
	}
}

// ---------------------------------------------------------------------------
// HotkeyConfig defaults
// ---------------------------------------------------------------------------

func TestDefaultHotkeyConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultHotkeyConfig()
	if cfg.Hotkey != DefaultHotkey {
		t.Errorf("DefaultHotkeyConfig().Hotkey = %q, want %q", cfg.Hotkey, DefaultHotkey)
	}
}

// ---------------------------------------------------------------------------
// Integration: hotkey drives Pipeline PTT
// ---------------------------------------------------------------------------

// TestHotkeyDrivesPipeline verifies that SimulatePress/SimulateRelease on a
// StubHotkeyListener correctly activate and deactivate PTT on the Pipeline,
// resulting in a dispatch call.
func TestHotkeyDrivesPipeline(t *testing.T) {
	t.Parallel()

	responses := make(chan string, 1)
	responses <- "vedox document everything"

	trans := NewStubTranscriber(responses)
	src := newControlledSource()

	var dispatched int32
	dispatchFn := func(_ context.Context, intent Intent, _ string) error {
		if intent.Command == CommandDocumentEverything {
			atomic.AddInt32(&dispatched, 1)
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

	rec := &stateRecorder{}
	p.OnActivity(rec.record)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop() //nolint:errcheck

	// Wire the hotkey listener to the pipeline.
	l := NewStubHotkeyListener(DefaultHotkeyConfig())
	if err := l.Start(ctx,
		func() { p.SetPTT(true) },
		func() { p.SetPTT(false) },
	); err != nil {
		t.Fatalf("listener Start: %v", err)
	}
	defer l.Stop() //nolint:errcheck

	// Press — audio flows.
	l.SimulatePress()
	rec.waitFor(t, VoiceStateListening, time.Second)

	src.push(silenceChunk(DefaultChunkSamples))

	// Release — pipeline should transcribe + dispatch.
	l.SimulateRelease()
	rec.waitFor(t, VoiceStateDispatching, 3*time.Second)
	rec.waitFor(t, VoiceStateIdle, 3*time.Second)

	if n := atomic.LoadInt32(&dispatched); n != 1 {
		t.Errorf("dispatch count = %d, want 1", n)
	}
}
