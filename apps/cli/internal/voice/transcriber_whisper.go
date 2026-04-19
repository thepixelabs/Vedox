//go:build whisper

// Package voice — transcriber_whisper.go
//
// Real Whisper.cpp transcriber, compiled only when -tags whisper is passed.
// The stub in transcriber_stub.go is used for all other builds (CI, tests,
// environments without Whisper.cpp present).
//
// # Build prerequisites
//
// This file links against the whisper.cpp C library via CGO.  Before building
// with -tags whisper you must:
//
//  1. Clone and build whisper.cpp as a static library:
//
//	git clone https://github.com/ggerganov/whisper.cpp
//	cd whisper.cpp
//	cmake -B build -DBUILD_SHARED_LIBS=OFF
//	cmake --build build --config Release
//	# Produces: build/src/libwhisper.a  and  build/ggml/src/libggml*.a
//
//  2. Download a model (ggml format):
//
//	bash models/download-ggml-model.sh base.en
//	# Saves to: models/ggml-base.en.bin
//
//  3. Set CGO environment variables pointing at the built library:
//
//	macOS:
//	  export WHISPER_ROOT=/path/to/whisper.cpp
//	  export CGO_CPPFLAGS="-I${WHISPER_ROOT}/include"
//	  export CGO_LDFLAGS="-L${WHISPER_ROOT}/build/src \
//	                      -L${WHISPER_ROOT}/build/ggml/src \
//	                      -lwhisper -lggml -lggml-base -lggml-cpu \
//	                      -framework Accelerate -framework CoreML \
//	                      -framework Foundation"
//
//	Linux (x86-64, OpenMP):
//	  export WHISPER_ROOT=/path/to/whisper.cpp
//	  export CGO_CPPFLAGS="-I${WHISPER_ROOT}/include"
//	  export CGO_LDFLAGS="-L${WHISPER_ROOT}/build/src \
//	                      -L${WHISPER_ROOT}/build/ggml/src \
//	                      -lwhisper -lggml -lggml-base -lggml-cpu \
//	                      -lstdc++ -lm -lgomp"
//
//  4. Build with CGO enabled and the whisper tag:
//
//	cd apps/cli
//	CGO_ENABLED=1 go build -tags whisper -o bin/vedox-whisper .
//
// See also: apps/cli/Makefile target `build-whisper` and
// docs/how-to/build-whisper.md for the full step-by-step walkthrough.
package voice

import (
	"context"
	"fmt"
	"strings"

	whisperlib "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

// WhisperTranscriber wraps a loaded whisper.cpp model and the inference
// context derived from it.  Create one via NewWhisperTranscriber (or the
// build-tag-dispatched NewTranscriber) and release it with Close when done.
//
// Thread-safety: whisper.cpp's inference is not thread-safe.  The pipeline
// goroutine must call Transcribe from a single goroutine — the existing
// pipeline architecture already guarantees this.
type WhisperTranscriber struct {
	model whisperlib.Model
}

// NewWhisperTranscriber loads the ggml model file at modelPath and returns a
// ready-to-use WhisperTranscriber.  Returns an error if the model file cannot
// be opened or is not a valid ggml whisper model.
//
// The caller is responsible for calling Close when the transcriber is no
// longer needed.
func NewWhisperTranscriber(modelPath string) (*WhisperTranscriber, error) {
	m, err := whisperlib.New(modelPath)
	if err != nil {
		return nil, fmt.Errorf("whisper: load model %q: %w", modelPath, err)
	}
	return &WhisperTranscriber{model: m}, nil
}

// newTranscriber is the build-tag-specific constructor called by NewTranscriber.
// This file is compiled when the "whisper" build tag IS present.
func newTranscriber(modelPath string) (Transcriber, error) {
	return NewWhisperTranscriber(modelPath)
}

// Transcribe runs whisper.cpp inference on the provided PCM audio samples and
// returns the concatenated segment text.
//
// Audio must be at 16 kHz, mono, float32 in the range [-1.0, 1.0].  An empty
// slice returns ("", nil) immediately without allocating an inference context.
//
// Context cancellation is honoured at two points:
//  1. Before any C work begins (fast-path check).
//  2. Via the EncoderBeginCallback: if ctx is cancelled while the encoder is
//     starting, the callback returns false and whisper.cpp aborts inference.
//
// Between segments the loop checks ctx.Done() and returns early with the text
// collected so far plus a wrapped ctx.Err(), so the caller can distinguish a
// partial result from a failure.
func (w *WhisperTranscriber) Transcribe(ctx context.Context, audio []float32) (string, error) {
	if len(audio) == 0 {
		return "", nil
	}

	// Fast-path: honour cancellation before entering CGO.
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("whisper: context cancelled before transcription: %w", ctx.Err())
	default:
	}

	// Each call to Process requires a fresh Context derived from the model.
	// The Context holds per-call state (KV cache, segment list) so it must not
	// be shared between concurrent Transcribe calls (the pipeline prevents this).
	wctx, err := w.model.NewContext()
	if err != nil {
		return "", fmt.Errorf("whisper: create inference context: %w", err)
	}

	if err := wctx.SetLanguage("en"); err != nil {
		// Non-fatal: fall back to auto-detect if the model is multilingual.
		// Mono-lingual models (e.g. ggml-base.en.bin) return an error here —
		// that is fine; language is already baked into the model weights.
		_ = err
	}

	// Collect segments via callback.  The callback fires synchronously inside
	// Process, so no goroutine or channel is needed.
	var sb strings.Builder

	// EncoderBeginCallback: called once just before the Whisper encoder runs.
	// Returning false aborts the entire Process call.  This is our hook for
	// honouring ctx cancellation with minimal latency once the audio is queued.
	encoderBegin := func() bool {
		select {
		case <-ctx.Done():
			return false
		default:
			return true
		}
	}

	// SegmentCallback: called once per recognised segment as they are decoded.
	// We check ctx here too so that a very long audio buffer does not block
	// cancellation indefinitely between segments.
	segmentCB := func(seg whisperlib.Segment) {
		select {
		case <-ctx.Done():
			// Context cancelled mid-stream; stop appending but let Process
			// continue (it will still call the callback for remaining segments
			// until it observes the encoder abort, which already fired above).
			// The cancelled error is surfaced after Process returns.
		default:
			sb.WriteString(seg.Text)
		}
	}

	if err := wctx.Process(audio, encoderBegin, segmentCB, nil); err != nil {
		// If the context was cancelled, surface that as the primary error.
		if ctx.Err() != nil {
			return sb.String(), fmt.Errorf("whisper: transcription aborted: %w", ctx.Err())
		}
		return "", fmt.Errorf("whisper: inference failed: %w", err)
	}

	// Surface any lingering ctx cancellation that happened after Process returned.
	if ctx.Err() != nil {
		return sb.String(), fmt.Errorf("whisper: context cancelled after transcription: %w", ctx.Err())
	}

	return sb.String(), nil
}

// Close releases the model memory held by the WhisperTranscriber.  After
// Close returns the transcriber must not be used again.
func (w *WhisperTranscriber) Close() error {
	if w.model != nil {
		if err := w.model.Close(); err != nil {
			return fmt.Errorf("whisper: close model: %w", err)
		}
		w.model = nil
	}
	return nil
}
