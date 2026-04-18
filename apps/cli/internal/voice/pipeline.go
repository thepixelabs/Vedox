// Package voice — pipeline.go
//
// Pipeline orchestrates the full voice capture → transcribe → parse → dispatch
// flow.  It is the public entry point for callers (e.g. cmd/vedox and the
// daemon) that want to enable voice input.
//
// Architecture:
//
//	┌─────────────────────────────────────────────────────────────────────┐
//	│  AudioSource                                                         │
//	│   (StubAudioSource / CoreAudioSource / ALSASource)                  │
//	│   emits []float32 chunks on a channel                                │
//	└────────────────────────────┬────────────────────────────────────────┘
//	                             │ audio chunks
//	                             ▼
//	┌─────────────────────────────────────────────────────────────────────┐
//	│  pipeline.loop goroutine                                             │
//	│   • idle: discards audio chunks                                      │
//	│   • listening (PTT active): appends chunks to buffer                 │
//	│   • 30 s hard limit: auto-releases PTT and proceeds                  │
//	└────────────────────────────┬────────────────────────────────────────┘
//	                             │ on PTT release / timeout
//	                             ▼
//	┌─────────────────────────────────────────────────────────────────────┐
//	│  Transcriber.Transcribe(ctx, buffer)                                 │
//	│   (StubTranscriber / WhisperTranscriber)                             │
//	└────────────────────────────┬────────────────────────────────────────┘
//	                             │ text
//	                             ▼
//	┌─────────────────────────────────────────────────────────────────────┐
//	│  ParseIntent(text)  →  Intent                                        │
//	└────────────────────────────┬────────────────────────────────────────┘
//	                             │ Intent
//	                             ▼
//	┌─────────────────────────────────────────────────────────────────────┐
//	│  Dispatch(ctx, intent, daemonURL)                                    │
//	└─────────────────────────────────────────────────────────────────────┘
package voice

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// VoiceState enumerates the observable states of the Pipeline.  Callers
// register an activity callback via OnActivity to receive transitions.
type VoiceState int

const (
	// VoiceStateIdle means the pipeline is running but PTT is not active.
	VoiceStateIdle VoiceState = iota

	// VoiceStateListening means PTT is active and audio is being buffered.
	VoiceStateListening

	// VoiceStateTranscribing means a transcription call is in progress.
	VoiceStateTranscribing

	// VoiceStateDispatching means Dispatch is being called with the parsed intent.
	VoiceStateDispatching

	// VoiceStateError means the last pipeline cycle ended with an error.
	// The pipeline continues listening; this state is transient.
	VoiceStateError
)

// String returns a human-readable name for the state.
func (s VoiceState) String() string {
	switch s {
	case VoiceStateIdle:
		return "idle"
	case VoiceStateListening:
		return "listening"
	case VoiceStateTranscribing:
		return "transcribing"
	case VoiceStateDispatching:
		return "dispatching"
	case VoiceStateError:
		return "error"
	default:
		return fmt.Sprintf("VoiceState(%d)", int(s))
	}
}

// PTTMaxDuration is the maximum time the pipeline will buffer audio while
// PTT is held before auto-releasing and proceeding to transcription.
const PTTMaxDuration = 30 * time.Second

// PipelineConfig holds constructor parameters for Pipeline.
type PipelineConfig struct {
	// Source is the audio capture backend.  Required.
	Source AudioSource

	// Transcriber converts PCM to text.  Required.
	Transcriber Transcriber

	// DaemonURL is the base URL of the Vedox daemon (e.g.
	// "http://127.0.0.1:4711").  Required.
	DaemonURL string

	// MinConfidence is the minimum Intent.Confidence required to dispatch.
	// Intents below this threshold are treated as unknown and an error state
	// is signalled.  Defaults to 0.5 (partial-match threshold) when zero.
	MinConfidence float64

	// DispatchFunc, if non-nil, overrides the package-level Dispatch function.
	// Used in tests to intercept dispatch calls without starting a daemon.
	DispatchFunc func(ctx context.Context, intent Intent, daemonURL string) error
}

// Pipeline is the voice pipeline orchestrator.  It owns the audio capture
// goroutine and the processing loop goroutine.  Create one with NewPipeline
// and call Start to begin.
//
// Pipeline is safe for concurrent calls to SetPTT, OnActivity, Start and
// Stop.  The lifecycle fields (cancel, activityCb) are protected by
// lifecycleMu; the loop goroutine reads activityCb via the notify() helper
// which takes the same lock, so a late OnActivity never races with a state
// transition already in flight.
type Pipeline struct {
	cfg PipelineConfig

	// PTT state — written by SetPTT (any goroutine), read by loop goroutine.
	pttMu     sync.Mutex
	pttActive bool

	// lifecycleMu protects the mutable lifecycle fields (activityCb, cancel,
	// started). OnActivity may now be called from any goroutine at any time
	// without racing the loop goroutine's notify() reads.
	lifecycleMu sync.Mutex
	activityCb  func(VoiceState)
	cancel      context.CancelFunc
	started     bool

	// pttCh carries PTT transitions into the loop goroutine.
	pttCh chan bool

	// doneCh is closed by the loop goroutine when it exits. Allocated once
	// in NewPipeline; never reassigned afterwards, so concurrent reads are
	// safe.
	doneCh chan struct{}
}

// NewPipeline constructs a Pipeline.  cfg.Source, cfg.Transcriber, and
// cfg.DaemonURL are required.
func NewPipeline(cfg PipelineConfig) (*Pipeline, error) {
	if cfg.Source == nil {
		return nil, fmt.Errorf("pipeline: Source is required")
	}
	if cfg.Transcriber == nil {
		return nil, fmt.Errorf("pipeline: Transcriber is required")
	}
	if cfg.DaemonURL == "" {
		return nil, fmt.Errorf("pipeline: DaemonURL is required")
	}
	if cfg.MinConfidence == 0 {
		cfg.MinConfidence = 0.5
	}
	if cfg.DispatchFunc == nil {
		cfg.DispatchFunc = Dispatch
	}

	return &Pipeline{
		cfg:    cfg,
		pttCh:  make(chan bool, 8),
		doneCh: make(chan struct{}),
	}, nil
}

// OnActivity registers a callback that is invoked every time the pipeline
// transitions to a new VoiceState.  The callback is called from the pipeline's
// internal goroutine; it must not block. Safe to call before or after Start
// from any goroutine.
func (p *Pipeline) OnActivity(cb func(VoiceState)) {
	p.lifecycleMu.Lock()
	p.activityCb = cb
	p.lifecycleMu.Unlock()
}

// SetPTT activates or deactivates push-to-talk.  May be called from any
// goroutine (e.g. a hotkey handler).  Has no effect if the pipeline has not
// been started or has been stopped.
func (p *Pipeline) SetPTT(active bool) {
	p.pttMu.Lock()
	p.pttActive = active
	p.pttMu.Unlock()

	select {
	case p.pttCh <- active:
	default:
		// channel is full — the loop will read the latest pttActive value
		// directly via the mutex, so dropping the notification is safe.
	}
}

// Start launches the audio capture and processing loop.  The provided context
// governs the lifetime of the pipeline — cancelling it is equivalent to
// calling Stop.
//
// Start returns after the audio source has been started.  The returned error
// is from AudioSource.Start; pipeline processing errors are reported via the
// OnActivity callback (VoiceStateError).
func (p *Pipeline) Start(ctx context.Context) error {
	loopCtx, cancel := context.WithCancel(ctx)

	p.lifecycleMu.Lock()
	p.cancel = cancel
	p.started = true
	p.lifecycleMu.Unlock()

	audioCh, err := p.cfg.Source.Start(loopCtx)
	if err != nil {
		cancel()
		// Revert lifecycle state so a subsequent Stop does not wait on a
		// doneCh that no loop goroutine will ever close.
		p.lifecycleMu.Lock()
		p.cancel = nil
		p.started = false
		p.lifecycleMu.Unlock()
		return fmt.Errorf("pipeline: start audio source: %w", err)
	}

	go p.loop(loopCtx, audioCh)
	return nil
}

// Stop halts the pipeline.  It cancels the context, stops the audio source,
// and waits for the loop goroutine to exit.  Stop is safe to call multiple
// times and from any goroutine.
func (p *Pipeline) Stop() error {
	p.lifecycleMu.Lock()
	cancel := p.cancel
	p.cancel = nil
	started := p.started
	p.lifecycleMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if err := p.cfg.Source.Stop(); err != nil {
		return fmt.Errorf("pipeline: stop audio source: %w", err)
	}
	if started {
		<-p.doneCh
	}
	return nil
}

// notify calls the activity callback if one has been registered. The snapshot
// load is taken under lifecycleMu so a concurrent OnActivity cannot race with
// the loop goroutine.
func (p *Pipeline) notify(state VoiceState) {
	p.lifecycleMu.Lock()
	cb := p.activityCb
	p.lifecycleMu.Unlock()
	if cb != nil {
		cb(state)
	}
}

// loop is the main processing goroutine.
func (p *Pipeline) loop(ctx context.Context, audioCh <-chan []float32) {
	defer close(p.doneCh)
	defer p.notify(VoiceStateIdle)

	p.notify(VoiceStateIdle)

	var (
		buffer     []float32      // accumulates audio while PTT is held
		pttTimer   *time.Timer    // fires when PTT max duration is exceeded
		pttTimerCh <-chan time.Time // nil when no timer is active
		listening  bool
	)

	startListening := func() {
		if listening {
			return
		}
		listening = true
		buffer = buffer[:0]
		pttTimer = time.NewTimer(PTTMaxDuration)
		pttTimerCh = pttTimer.C
		p.notify(VoiceStateListening)
	}

	stopListening := func() {
		if !listening {
			return
		}
		listening = false
		if pttTimer != nil {
			pttTimer.Stop()
			pttTimerCh = nil
		}
	}

	transcribeAndDispatch := func(captured []float32) {
		// Work on a copy so the buffer can be reset immediately.
		audio := make([]float32, len(captured))
		copy(audio, captured)

		p.notify(VoiceStateTranscribing)
		text, err := p.cfg.Transcriber.Transcribe(ctx, audio)
		if err != nil {
			p.notify(VoiceStateError)
			return
		}

		intent := ParseIntent(text)
		if intent.Confidence < p.cfg.MinConfidence {
			// Below threshold — treat as error (unrecognised speech).
			p.notify(VoiceStateError)
			return
		}

		p.notify(VoiceStateDispatching)
		if err := p.cfg.DispatchFunc(ctx, intent, p.cfg.DaemonURL); err != nil {
			p.notify(VoiceStateError)
			return
		}

		p.notify(VoiceStateIdle)
	}

	for {
		select {
		case <-ctx.Done():
			return

		case active, ok := <-p.pttCh:
			if !ok {
				return
			}
			if active && !listening {
				startListening()
			} else if !active && listening {
				stopListening()
				captured := append([]float32(nil), buffer...)
				buffer = buffer[:0]
				transcribeAndDispatch(captured)
			}

		case <-pttTimerCh:
			// PTT held too long — auto-release.
			stopListening()
			captured := append([]float32(nil), buffer...)
			buffer = buffer[:0]
			transcribeAndDispatch(captured)
			// Re-enter idle; if the user is still holding PTT physically,
			// SetPTT(true) will be called again by the hotkey handler.

		case chunk, ok := <-audioCh:
			if !ok {
				// Audio source closed — stop the pipeline.
				return
			}
			if listening {
				buffer = append(buffer, chunk...)
			}
			// When idle, chunks are silently discarded.
		}
	}
}
