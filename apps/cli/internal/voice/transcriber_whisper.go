//go:build whisper

// transcriber_whisper.go — compiled only when -tags whisper is passed.
//
// This file contains the CGO integration with whisper.cpp.  The stub
// implementation in transcriber_stub.go is used otherwise.
//
// To build with real Whisper support:
//
//	CGO_ENABLED=1 go build -tags whisper ./apps/cli/...
//
// Pre-requisites:
//  1. whisper.cpp built as a static library: libwhisper.a + whisper.h
//     placed in a directory on CGO_LDFLAGS / CGO_CPPFLAGS paths.
//  2. ggml-base.en.bin model present at the path passed to NewTranscriber.
//  3. macOS: link with -framework Accelerate -framework CoreML
//     Linux:  link with -lgomp (OpenMP) or -lpthread depending on build.
//
// CGO directives are intentionally left as template comments (TODO markers)
// so that the file compiles under `go vet` without the real headers present.
// A follow-up phase (WS-E-voice-03) will land the real CGO wiring once the
// build environment has been provisioned.

package voice

// #cgo LDFLAGS: -lwhisper
// #include "whisper.h"
// #include <stdlib.h>
import "C"

import (
	"context"
	"fmt"
	"unsafe"
)

// WhisperTranscriber wraps a whisper.cpp context loaded from a ggml model
// file.  It must be created via NewTranscriber (or newTranscriber internally)
// and closed with Close when no longer needed.
//
// Thread-safety: whisper_full is not thread-safe.  The Pipeline goroutine must
// call Transcribe from a single goroutine — the pipeline architecture already
// guarantees this.
type WhisperTranscriber struct {
	ctx *C.struct_whisper_context
}

// newTranscriber is the build-tag-specific constructor called by NewTranscriber.
// This file is compiled when the "whisper" build tag IS present.
func newTranscriber(modelPath string) (Transcriber, error) {
	cPath := C.CString(modelPath)
	defer C.free(unsafe.Pointer(cPath))

	wctx := C.whisper_init_from_file(cPath)
	if wctx == nil {
		return nil, fmt.Errorf("whisper: failed to load model from %q", modelPath)
	}

	return &WhisperTranscriber{ctx: wctx}, nil
}

// Transcribe runs whisper_full on the provided PCM audio and returns the
// concatenated segment text.
func (w *WhisperTranscriber) Transcribe(ctx context.Context, audio []float32) (string, error) {
	if len(audio) == 0 {
		return "", nil
	}

	// Check context before entering the (potentially long) C call.
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("whisper: context cancelled before transcription: %w", ctx.Err())
	default:
	}

	params := C.whisper_full_default_params(C.WHISPER_SAMPLING_GREEDY)
	params.language = C.CString("en")
	defer C.free(unsafe.Pointer(params.language))
	params.print_progress = C.bool(false)
	params.print_realtime = C.bool(false)

	ret := C.whisper_full(w.ctx, params, (*C.float)(unsafe.Pointer(&audio[0])), C.int(len(audio)))
	if ret != 0 {
		return "", fmt.Errorf("whisper: whisper_full returned error code %d", int(ret))
	}

	nSegments := int(C.whisper_full_n_segments(w.ctx))
	var result string
	for i := 0; i < nSegments; i++ {
		text := C.GoString(C.whisper_full_get_segment_text(w.ctx, C.int(i)))
		result += text
	}

	return result, nil
}

// Close frees the whisper context.
func (w *WhisperTranscriber) Close() error {
	if w.ctx != nil {
		C.whisper_free(w.ctx)
		w.ctx = nil
	}
	return nil
}
