// Package voice — transcriber.go
//
// Transcriber abstracts the Whisper.cpp speech-to-text layer.  At compile
// time, exactly one implementation is active:
//
//   - Default (no build tag): StubTranscriber is used.  Safe to run in CI,
//     tests, and environments without Whisper.cpp present.
//   - //go:build whisper: WhisperTranscriber wraps the real whisper.cpp CGO
//     bindings.  Requires the C library and the model file on disk.
//
// Audio format contract (all implementations must honour this):
//
//	Sample rate : 16 000 Hz
//	Channels    : 1 (mono)
//	Sample type : float32, range [-1.0, 1.0]
//	Encoding    : PCM, interleaved (trivial for mono)
package voice

import (
	"context"
	"fmt"
	"sync/atomic"
)

// Transcriber converts a slice of PCM float32 audio samples into a text
// string.  The caller is responsible for ensuring the audio conforms to the
// 16 kHz mono float32 format described in the package doc.
//
// Implementations must be safe for concurrent use from a single goroutine
// (the pipeline loop).  The context must be honoured: if it is cancelled
// before transcription completes, Transcribe should return ctx.Err() wrapped
// in a meaningful message.
type Transcriber interface {
	// Transcribe converts PCM audio samples to text.
	//
	// audio must be at 16 kHz, mono, float32.  An empty slice is valid and
	// should return ("", nil).
	Transcribe(ctx context.Context, audio []float32) (string, error)

	// Close releases any resources held by the transcriber (model memory,
	// CGO handles, etc.).  After Close returns the transcriber must not be
	// used again.
	Close() error
}

// NewTranscriber returns the build-appropriate Transcriber for the given model
// path.  When the "whisper" build tag is present, it returns a
// WhisperTranscriber backed by the C library.  Otherwise it returns a
// StubTranscriber that is safe to use in tests and CI.
//
// modelPath is the path to the ggml model file (e.g.
// "~/.vedox/models/ggml-base.en.bin").  For StubTranscriber the path is
// ignored; for WhisperTranscriber a non-existent path causes an error.
func NewTranscriber(modelPath string) (Transcriber, error) {
	return newTranscriber(modelPath)
}

// ---------------------------------------------------------------------------
// StubTranscriber
// ---------------------------------------------------------------------------

// StubTranscriber is a test/CI implementation of Transcriber.  It returns
// canned responses from a channel or, if the channel is nil or drained, a
// configurable fixed response.
//
// Usage in tests:
//
//	responses := make(chan string, 3)
//	responses <- "vedox document everything"
//	responses <- "vedox stop"
//	st := NewStubTranscriber(responses)
//
// StubTranscriber is safe for concurrent Transcribe / Close calls — the
// closed flag is kept in an atomic.Bool so tests that run the pipeline
// goroutine and invoke Close from t.Cleanup (from the test goroutine)
// do not race under -race.
type StubTranscriber struct {
	// Responses is a channel of strings that Transcribe drains in order.
	// When the channel is nil or has no buffered value, FallbackText is
	// returned instead.
	Responses chan string

	// FallbackText is returned when Responses is nil or drained.
	// Defaults to "" (empty string → CommandUnknown from the parser).
	FallbackText string

	// closed tracks whether Close has been called. Atomic so Transcribe
	// (pipeline goroutine) and Close (test goroutine) never race on the
	// flag bit.
	closed atomic.Bool
}

// NewStubTranscriber constructs a StubTranscriber.  Pass nil for responses to
// always return FallbackText (which defaults to "").
func NewStubTranscriber(responses chan string) *StubTranscriber {
	return &StubTranscriber{Responses: responses}
}

// Transcribe implements Transcriber.  It respects ctx cancellation, drains
// one item from Responses if available, and otherwise returns FallbackText.
func (s *StubTranscriber) Transcribe(ctx context.Context, audio []float32) (string, error) {
	if s.closed.Load() {
		return "", fmt.Errorf("stub transcriber: already closed")
	}

	// Honour context cancellation before doing any work.
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("stub transcriber: context cancelled: %w", ctx.Err())
	default:
	}

	// Empty audio is valid and returns empty text.
	if len(audio) == 0 {
		return "", nil
	}

	// Drain from the response channel if something is ready.
	if s.Responses != nil {
		select {
		case text, ok := <-s.Responses:
			if ok {
				return text, nil
			}
		default:
		}
	}

	return s.FallbackText, nil
}

// Close implements Transcriber.
func (s *StubTranscriber) Close() error {
	s.closed.Store(true)
	return nil
}
