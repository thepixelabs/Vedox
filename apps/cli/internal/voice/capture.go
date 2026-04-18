// Package voice — capture.go
//
// AudioSource abstracts the hardware or file-based audio capture layer.
//
// Audio format contract (all implementations):
//
//	Sample rate : 16 000 Hz
//	Channels    : 1 (mono)
//	Sample type : float32, range [-1.0, 1.0]
//	Chunk size  : implementation-defined; pipeline buffers until PTT release
//
// Build tags used in this file:
//   - No build tag on this file — it contains only the interface and stub.
//   - capture_coreaudio.go (//go:build darwin && cgo) holds the CoreAudio impl.
//   - capture_alsa.go      (//go:build linux && cgo)  holds the ALSA impl.
package voice

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
)

const (
	// AudioSampleRate is the required sample rate for all AudioSource
	// implementations and for Transcriber.Transcribe.
	AudioSampleRate = 16_000

	// AudioChannels is always 1 (mono).
	AudioChannels = 1

	// DefaultChunkSamples is the number of float32 samples per chunk emitted
	// by AudioSource implementations when no other size is specified.
	// 512 samples = 32 ms at 16 kHz — short enough for responsive PTT.
	DefaultChunkSamples = 512
)

// AudioSource is the interface implemented by all audio capture backends.
//
// Lifecycle:
//
//	src, err := NewAudioSource(...)
//	ch, err := src.Start(ctx)
//	// read from ch until it is closed
//	src.Stop()
//
// When the context is cancelled, the implementation must close the channel
// and return from any internal goroutine.  Stop is idempotent.
type AudioSource interface {
	// Start begins capturing audio and returns a channel of PCM chunks.
	// The channel is closed when the context is cancelled or Stop is called.
	// Each chunk is a slice of float32 samples at AudioSampleRate, mono.
	// The channel must not be nil on success.
	Start(ctx context.Context) (<-chan []float32, error)

	// Stop halts audio capture and releases hardware resources.  The channel
	// returned by Start will be closed before Stop returns.  Calling Stop
	// before Start is a no-op.  Stop is idempotent.
	Stop() error
}

// ---------------------------------------------------------------------------
// StubAudioSource
// ---------------------------------------------------------------------------

// StubAudioSource is a test/CI AudioSource implementation.  It can operate in
// two modes:
//
//  1. File mode (FilePath != ""): reads raw little-endian float32 samples from
//     the file and emits them in chunks of ChunkSamples.  When the file is
//     exhausted the channel is closed.
//
//  2. Silence mode (FilePath == ""): emits chunks of zero-valued samples
//     (silence) until the context is cancelled or Stop is called.  Useful for
//     PTT timeout tests.
//
// ChunkSamples defaults to DefaultChunkSamples if zero.
//
// StubAudioSource is safe for concurrent Start/Stop calls — the internal
// channels are guarded by mu so Start and Stop may race without triggering
// the race detector.
type StubAudioSource struct {
	// FilePath, if non-empty, is the path to a raw PCM float32 file to read.
	// The file must contain IEEE-754 little-endian float32 samples at 16 kHz mono.
	FilePath string

	// ChunkSamples is the number of samples per emitted chunk.
	// Defaults to DefaultChunkSamples when zero.
	ChunkSamples int

	mu     sync.Mutex
	stopCh chan struct{}
	doneCh chan struct{}
}

// NewStubAudioSource constructs a StubAudioSource.  filePath may be "" for
// silence mode.
func NewStubAudioSource(filePath string) *StubAudioSource {
	return &StubAudioSource{FilePath: filePath}
}

// Start implements AudioSource.
func (s *StubAudioSource) Start(ctx context.Context) (<-chan []float32, error) {
	chunkSize := s.ChunkSamples
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSamples
	}

	// Buffered so the goroutine can queue a couple of chunks without blocking.
	out := make(chan []float32, 4)
	s.mu.Lock()
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	stopCh := s.stopCh
	doneCh := s.doneCh
	s.mu.Unlock()

	if s.FilePath != "" {
		go s.runFile(ctx, out, chunkSize, stopCh, doneCh)
	} else {
		go s.runSilence(ctx, out, chunkSize, stopCh, doneCh)
	}

	return out, nil
}

// runFile reads float32 samples from a file and emits them in chunks. The
// stopCh/doneCh are passed in (captured at Start time) so Stop can replace
// the fields in s without racing the running goroutine.
func (s *StubAudioSource) runFile(ctx context.Context, out chan<- []float32, chunkSize int, stopCh, doneCh chan struct{}) {
	defer close(out)
	defer close(doneCh)

	f, err := os.Open(s.FilePath)
	if err != nil {
		// Surface the error as a single-element chunk containing NaN as a
		// sentinel; callers must validate samples.  In practice tests
		// should always use valid files.
		return
	}
	defer f.Close()

	buf := make([]float32, chunkSize)
	raw := make([]byte, chunkSize*4) // 4 bytes per float32

	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		default:
		}

		n, err := io.ReadFull(f, raw)
		if n == 0 {
			return // EOF
		}

		// Partial read at end of file — shrink the chunk.
		samples := n / 4
		chunk := buf[:samples]
		for i := 0; i < samples; i++ {
			bits := binary.LittleEndian.Uint32(raw[i*4 : i*4+4])
			chunk[i] = math.Float32frombits(bits)
		}

		select {
		case out <- append([]float32(nil), chunk...):
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		}

		if err != nil {
			return // io.ErrUnexpectedEOF or real error — stop
		}
	}
}

// runSilence emits zero-valued chunks until stopped.
func (s *StubAudioSource) runSilence(ctx context.Context, out chan<- []float32, chunkSize int, stopCh, doneCh chan struct{}) {
	defer close(out)
	defer close(doneCh)

	silence := make([]float32, chunkSize) // zero-valued by Go initialisation

	for {
		select {
		case <-ctx.Done():
			return
		case <-stopCh:
			return
		case out <- append([]float32(nil), silence...):
			// chunk sent; continue
		}
	}
}

// Stop implements AudioSource. Safe to call from any goroutine and
// concurrent with Start — mu guards the channel fields.
func (s *StubAudioSource) Stop() error {
	s.mu.Lock()
	stopCh := s.stopCh
	doneCh := s.doneCh
	s.mu.Unlock()

	if stopCh == nil {
		return nil // Start was never called
	}
	select {
	case <-stopCh:
		// already stopped
	default:
		close(stopCh)
	}
	if doneCh != nil {
		<-doneCh // wait for goroutine to exit
	}
	return nil
}

// ---------------------------------------------------------------------------
// Future implementations (stubs for the darwin/linux CGO builds)
// ---------------------------------------------------------------------------

// CoreAudioSource (macOS, CGO) and ALSASource (Linux, CGO) are declared in
// their respective platform-specific files:
//
//   - capture_coreaudio.go  (//go:build darwin && cgo)
//   - capture_alsa.go       (//go:build linux && cgo)
//
// Both implement AudioSource and return float32 samples at AudioSampleRate.
//
// Design notes for implementers:
//
//   - CoreAudioSource should use AudioQueueNewInput (push-to-talk path does
//     not require a low-latency AUGraph) and request a format of
//     kAudioFormatLinearPCM with float32, 16 kHz, mono.  The queue callback
//     converts to float32 and sends on the channel.  TCC mic permission must
//     be requested before Start; if not granted, Start returns a typed error
//     so the caller can surface actionable guidance to the user.
//
//   - ALSASource should open the default capture device with snd_pcm_open and
//     configure hw_params for S16_LE at 16 kHz mono, then convert signed
//     int16 samples to float32 on the fly (divide by 32768.0).
//
// Neither implementation exists yet; they will land in WS-E-voice-03.

// newAudioSourceForPlatform is a placeholder called by tests that need a
// real (non-stub) source.  It returns an error on any platform until the
// CGO capture files land.
func newAudioSourceForPlatform() (AudioSource, error) {
	return nil, fmt.Errorf(
		"native audio capture not available in this build; " +
			"use NewStubAudioSource for testing or build with -tags coreaudio / alsa",
	)
}
