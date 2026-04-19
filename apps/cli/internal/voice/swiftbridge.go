// Package voice — swiftbridge.go
//
// SwiftAudioSource is an AudioSource implementation that receives PCM frames
// from the VedoxVoice macOS helper binary over a Unix domain socket.
//
// Architecture:
//
//	VedoxVoice (Swift) ──unix socket──▶ SwiftAudioSource (Go)
//	                                          │
//	                                          ▼
//	                                   Pipeline.loop goroutine
//
// Wire protocol (tag-byte multiplexed, no CGO):
//
//	0x01  Audio frame    — tag(1) + length(4 LE uint32) + float32 samples (LE)
//	0x02  Control event  — tag(1) + JSON bytes + newline '\n'
//
// SwiftAudioSource listens on a Unix socket path (default /tmp/vedox-voice.sock,
// configurable via SocketPath field).  It accepts exactly one connection at a
// time from the Swift binary, reads tag-framed messages, converts audio bytes
// to []float32, and delivers them to the channel returned by Start.
//
// Control events (press/release) are decoded and used to synthesise PTT
// transitions on a registered HotkeyListener.  The pipeline itself still
// calls SetPTT — SwiftAudioSource just bridges the signal.
//
// No CGO is required.  The binary protocol between Swift and Go uses only
// encoding/binary and net package primitives.
//
// Usage:
//
//	src := voice.NewSwiftAudioSource("/tmp/vedox-voice.sock")
//	src.OnHotkeyEvent = func(event string) {
//	    // "press" or "release"
//	    pipeline.SetPTT(event == "press")
//	}
//	ch, err := src.Start(ctx)
//	// read frames from ch
//	src.Stop()
package voice

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
)

// ---------------------------------------------------------------------------
// Tag byte constants (must match SocketBridge.swift)
// ---------------------------------------------------------------------------

const (
	tagAudioFrame    byte = 0x01
	tagControlEvent  byte = 0x02
)

// ---------------------------------------------------------------------------
// SwiftAudioSource
// ---------------------------------------------------------------------------

// SwiftAudioSource implements AudioSource by listening on a Unix domain socket
// for frames from the VedoxVoice Swift helper binary.
//
// Lifecycle:
//
//	src := NewSwiftAudioSource("/tmp/vedox-voice.sock")
//	ch, err := src.Start(ctx)    // starts listening; blocks until Swift connects
//	// pipeline reads from ch
//	src.Stop()
//
// If the Swift binary disconnects the channel is closed and a reconnect window
// opens: the next call to Start will listen again.
//
// SwiftAudioSource is safe for concurrent Stop/Start calls.
type SwiftAudioSource struct {
	// SocketPath is the Unix domain socket path.
	// Defaults to DefaultVoiceSocketPath if empty.
	SocketPath string

	// OnHotkeyEvent, if non-nil, is called on every control event decoded from
	// the Swift binary.  "event" is "press" or "release".
	// Called from the read goroutine — must not block.
	OnHotkeyEvent func(event string)

	// mu guards all mutable fields.
	mu sync.Mutex

	// listener is the net.Listener created by Start.
	listener net.Listener

	// conn is the current accepted Swift connection (nil if not connected).
	conn net.Conn

	// stopCh is closed by Stop to signal the read loop.
	stopCh chan struct{}

	// doneCh is closed by the read goroutine when it exits.
	doneCh chan struct{}
}

// DefaultVoiceSocketPath is the Unix socket path used when SocketPath is empty.
const DefaultVoiceSocketPath = "/tmp/vedox-voice.sock"

// NewSwiftAudioSource constructs a SwiftAudioSource.
// socketPath is the Unix socket path; pass "" to use DefaultVoiceSocketPath.
func NewSwiftAudioSource(socketPath string) *SwiftAudioSource {
	if socketPath == "" {
		socketPath = DefaultVoiceSocketPath
	}
	return &SwiftAudioSource{SocketPath: socketPath}
}

// Start implements AudioSource.
//
// It creates a Unix socket listener, waits for the Swift binary to connect,
// then reads tag-framed messages and forwards audio chunks on the returned
// channel.
//
// The channel is closed when the context is cancelled, Stop is called, or the
// Swift binary closes the connection.
func (s *SwiftAudioSource) Start(ctx context.Context) (<-chan []float32, error) {
	ln, err := net.Listen("unix", s.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("swiftbridge: listen on %s: %w", s.SocketPath, err)
	}

	out := make(chan []float32, 16)
	stopCh := make(chan struct{})
	doneCh := make(chan struct{})

	s.mu.Lock()
	s.listener = ln
	s.stopCh = stopCh
	s.doneCh = doneCh
	s.mu.Unlock()

	go s.run(ctx, ln, out, stopCh, doneCh)
	return out, nil
}

// Stop implements AudioSource. Closes the listener and any active connection,
// then waits for the read goroutine to exit. Idempotent.
func (s *SwiftAudioSource) Stop() error {
	s.mu.Lock()
	ln := s.listener
	conn := s.conn
	stopCh := s.stopCh
	doneCh := s.doneCh
	s.listener = nil
	s.conn = nil
	s.mu.Unlock()

	if stopCh == nil {
		return nil // Start was never called
	}

	// Signal the run goroutine.
	select {
	case <-stopCh:
		// already stopped
	default:
		close(stopCh)
	}

	// Unblock Accept and any blocking Read.
	if ln != nil {
		_ = ln.Close()
	}
	if conn != nil {
		_ = conn.Close()
	}

	if doneCh != nil {
		<-doneCh
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal read loop
// ---------------------------------------------------------------------------

// run is the goroutine that accepts a Swift connection and reads messages.
func (s *SwiftAudioSource) run(
	ctx context.Context,
	ln net.Listener,
	out chan<- []float32,
	stopCh chan struct{},
	doneCh chan struct{},
) {
	defer close(out)
	defer close(doneCh)

	// Accept loop: re-accept if the Swift binary reconnects.
	for {
		// Check for shutdown before blocking on Accept.
		select {
		case <-stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		conn, err := ln.Accept()
		if err != nil {
			// Any error here (including "use of closed network connection") means
			// we should stop — the listener was closed by Stop or ctx cancel.
			return
		}

		s.mu.Lock()
		s.conn = conn
		s.mu.Unlock()

		s.readMessages(ctx, conn, out, stopCh)

		s.mu.Lock()
		s.conn = nil
		s.mu.Unlock()

		_ = conn.Close()

		// After disconnect check whether we should keep listening or shut down.
		select {
		case <-stopCh:
			return
		case <-ctx.Done():
			return
		default:
			// Swift binary disconnected — keep listening for a reconnect.
		}
	}
}

// readMessages reads tag-framed messages from conn until an error or shutdown.
func (s *SwiftAudioSource) readMessages(
	ctx context.Context,
	conn net.Conn,
	out chan<- []float32,
	stopCh chan struct{},
) {
	// Close conn when context is cancelled so blocking reads unblock.
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stopCh:
			_ = conn.Close()
		}
	}()

	for {
		// Read one tag byte.
		var tag [1]byte
		if _, err := io.ReadFull(conn, tag[:]); err != nil {
			return // connection closed or error
		}

		switch tag[0] {
		case tagAudioFrame:
			samples, err := readAudioFrame(conn)
			if err != nil {
				return
			}
			if len(samples) == 0 {
				continue
			}
			select {
			case out <- samples:
			case <-stopCh:
				return
			case <-ctx.Done():
				return
			}

		case tagControlEvent:
			event, err := readControlEvent(conn)
			if err != nil {
				return
			}
			if cb := s.OnHotkeyEvent; cb != nil && event != "" {
				cb(event)
			}

		default:
			// Unknown tag — protocol mismatch.  Log and return so we close the
			// connection; the Swift binary will reconnect.
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Frame decoders
// ---------------------------------------------------------------------------

// readAudioFrame reads a length-prefixed float32 frame from conn.
//
// Wire format: 4-byte little-endian uint32 payload length (in bytes) followed
// by N float32 samples encoded as little-endian IEEE-754.
func readAudioFrame(conn net.Conn) ([]float32, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(conn, lenBuf[:]); err != nil {
		return nil, fmt.Errorf("swiftbridge: read audio length: %w", err)
	}
	payloadLen := binary.LittleEndian.Uint32(lenBuf[:])

	// Sanity check: 10 seconds of audio at 16 kHz float32 = 640 KB.
	// Anything larger is almost certainly a protocol error.
	const maxPayloadBytes = 640_000
	if payloadLen == 0 || payloadLen > maxPayloadBytes {
		return nil, fmt.Errorf("swiftbridge: audio frame length %d out of range [1, %d]",
			payloadLen, maxPayloadBytes)
	}
	if payloadLen%4 != 0 {
		return nil, fmt.Errorf("swiftbridge: audio frame length %d is not a multiple of 4 (float32 size)",
			payloadLen)
	}

	raw := make([]byte, payloadLen)
	if _, err := io.ReadFull(conn, raw); err != nil {
		return nil, fmt.Errorf("swiftbridge: read audio payload: %w", err)
	}

	nSamples := int(payloadLen / 4)
	samples := make([]float32, nSamples)
	for i := range samples {
		bits := binary.LittleEndian.Uint32(raw[i*4 : i*4+4])
		samples[i] = math.Float32frombits(bits)
	}
	return samples, nil
}

// controlMessage is the JSON payload for tag 0x02 control events.
type controlMessage struct {
	Event  string `json:"event"`
	Hotkey string `json:"hotkey"`
}

// readControlEvent reads a newline-terminated JSON control message from conn
// and returns the "event" field value ("press" or "release").
func readControlEvent(conn net.Conn) (string, error) {
	// Read until newline (messages are small so a byte-at-a-time read is fine).
	var buf []byte
	oneByte := make([]byte, 1)
	for {
		n, err := conn.Read(oneByte)
		if n > 0 {
			b := oneByte[0]
			if b == '\n' {
				break
			}
			buf = append(buf, b)
			// Guard against a runaway control message.
			if len(buf) > 4096 {
				return "", fmt.Errorf("swiftbridge: control event exceeds 4096 bytes")
			}
		}
		if err != nil {
			if len(buf) == 0 {
				return "", fmt.Errorf("swiftbridge: read control event: %w", err)
			}
			break // parse what we have
		}
	}

	var msg controlMessage
	if err := json.Unmarshal(buf, &msg); err != nil {
		return "", fmt.Errorf("swiftbridge: decode control event %q: %w", buf, err)
	}
	return msg.Event, nil
}
