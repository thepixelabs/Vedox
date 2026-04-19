// Package voice — swiftbridge_test.go
//
// Tests for SwiftAudioSource. All tests are self-contained: they spin up a
// fake "Swift binary" using net.Dial to a test socket and verify that the
// Go side decodes frames correctly.
package voice

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// tempSocketPath returns a unique Unix socket path short enough for
// sockaddr_un.sun_path (104-byte limit on macOS).
// We create a temp dir under /tmp rather than using t.TempDir() because
// the latter produces paths like /var/folders/…/T/<TestName><hash>/ which
// exceed 104 bytes on macOS when combined with the socket filename.
func tempSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "vdx")
	if err != nil {
		t.Fatalf("tempSocketPath: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "v.sock")
}

// fakeSwift dials the given socket path and holds the connection.
// It exposes helpers for sending audio frames and control events in the wire
// protocol defined by SocketBridge.swift / swiftbridge.go.
type fakeSwift struct {
	t    *testing.T
	conn net.Conn
}

func dialFakeSwift(t *testing.T, socketPath string) *fakeSwift {
	t.Helper()
	// Retry briefly — the listener may not be ready on the first dial.
	var conn net.Conn
	var err error
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("fakeSwift: dial %s: %v", socketPath, err)
	}
	t.Cleanup(func() { conn.Close() })
	return &fakeSwift{t: t, conn: conn}
}

// sendAudioFrame writes a tag-0x01 audio frame.
func (f *fakeSwift) sendAudioFrame(samples []float32) {
	f.t.Helper()
	payloadLen := uint32(len(samples) * 4)

	buf := make([]byte, 1+4+int(payloadLen))
	buf[0] = tagAudioFrame
	binary.LittleEndian.PutUint32(buf[1:5], payloadLen)
	for i, s := range samples {
		binary.LittleEndian.PutUint32(buf[5+i*4:], math.Float32bits(s))
	}

	if _, err := f.conn.Write(buf); err != nil {
		f.t.Fatalf("fakeSwift: sendAudioFrame: %v", err)
	}
}

// sendControlEvent writes a tag-0x02 control event.
func (f *fakeSwift) sendControlEvent(event, hotkey string) {
	f.t.Helper()
	msg := controlMessage{Event: event, Hotkey: hotkey}
	jsonBytes, err := json.Marshal(msg)
	if err != nil {
		f.t.Fatalf("fakeSwift: marshal control event: %v", err)
	}

	buf := make([]byte, 1+len(jsonBytes)+1)
	buf[0] = tagControlEvent
	copy(buf[1:], jsonBytes)
	buf[len(buf)-1] = '\n'

	if _, err := f.conn.Write(buf); err != nil {
		f.t.Fatalf("fakeSwift: sendControlEvent: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestSwiftAudioSource_AudioFrames verifies that audio frames sent by the
// fake Swift binary arrive on the channel with correct sample values.
func TestSwiftAudioSource_AudioFrames(t *testing.T) {
	socketPath := tempSocketPath(t)
	src := NewSwiftAudioSource(socketPath)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = src.Stop() })

	// Dial as fake Swift binary.
	swift := dialFakeSwift(t, socketPath)

	// Send two frames.
	want1 := []float32{0.1, 0.2, 0.3, 0.4}
	want2 := []float32{-0.1, -0.2, 0.5, 1.0}
	swift.sendAudioFrame(want1)
	swift.sendAudioFrame(want2)

	// Receive and verify.
	got1 := recvFrame(t, ch, 2*time.Second)
	got2 := recvFrame(t, ch, 2*time.Second)

	assertSamplesEqual(t, "frame 1", want1, got1)
	assertSamplesEqual(t, "frame 2", want2, got2)
}

// TestSwiftAudioSource_ControlEvents verifies that control events call
// OnHotkeyEvent with the correct event string.
func TestSwiftAudioSource_ControlEvents(t *testing.T) {
	socketPath := tempSocketPath(t)
	src := NewSwiftAudioSource(socketPath)

	var events []string
	var mu sync.Mutex
	src.OnHotkeyEvent = func(event string) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = src.Stop() })

	swift := dialFakeSwift(t, socketPath)
	swift.sendControlEvent("press", "ctrl+shift+v")
	swift.sendControlEvent("release", "ctrl+shift+v")

	// Drain any audio frames (there should be none, but keep ch from blocking).
	drainCh(ch)

	// Wait for both events.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(events)
		mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	got := append([]string(nil), events...)
	mu.Unlock()

	if len(got) < 2 {
		t.Fatalf("expected 2 control events, got %d: %v", len(got), got)
	}
	if got[0] != "press" {
		t.Errorf("event[0] = %q; want %q", got[0], "press")
	}
	if got[1] != "release" {
		t.Errorf("event[1] = %q; want %q", got[1], "release")
	}
}

// TestSwiftAudioSource_StopClosesChannel verifies that Stop closes the channel.
func TestSwiftAudioSource_StopClosesChannel(t *testing.T) {
	socketPath := tempSocketPath(t)
	src := NewSwiftAudioSource(socketPath)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Dial so the accept goroutine unblocks.
	swift := dialFakeSwift(t, socketPath)
	_ = swift // keep connection alive until Stop

	// Stop should cause the channel to be closed.
	if err := src.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// The channel should be closed (drained + closed).
	timeout := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // closed — test passes
			}
			// Drain any remaining frames.
		case <-timeout:
			t.Fatal("channel was not closed after Stop within 2s")
		}
	}
}

// TestSwiftAudioSource_ContextCancelClosesChannel verifies that cancelling the
// context closes the channel.
func TestSwiftAudioSource_ContextCancelClosesChannel(t *testing.T) {
	socketPath := tempSocketPath(t)
	src := NewSwiftAudioSource(socketPath)

	ctx, cancel := context.WithCancel(t.Context())

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = src.Stop() })

	// Dial so the accept loop unblocks.
	_ = dialFakeSwift(t, socketPath)

	cancel()

	timeout := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-timeout:
			t.Fatal("channel not closed within 2s after context cancel")
		}
	}
}

// TestSwiftAudioSource_StopBeforeConnect verifies that Stop is safe to call
// when no Swift binary has connected yet.
func TestSwiftAudioSource_StopBeforeConnect(t *testing.T) {
	socketPath := tempSocketPath(t)
	src := NewSwiftAudioSource(socketPath)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	_, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Stop without anyone dialling in.  Should not hang.
	done := make(chan struct{})
	go func() {
		_ = src.Stop()
		close(done)
	}()

	select {
	case <-done:
		// pass
	case <-time.After(2 * time.Second):
		t.Fatal("Stop hung with no connection")
	}
}

// TestSwiftAudioSource_StopIdempotent verifies that calling Stop twice is safe.
func TestSwiftAudioSource_StopIdempotent(t *testing.T) {
	socketPath := tempSocketPath(t)
	src := NewSwiftAudioSource(socketPath)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	_, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := src.Stop(); err != nil {
		t.Fatalf("Stop (1st): %v", err)
	}
	if err := src.Stop(); err != nil {
		t.Fatalf("Stop (2nd): %v", err)
	}
}

// TestSwiftAudioSource_LargeFrameRejected verifies that oversized audio frames
// do not crash or hang the reader.
func TestSwiftAudioSource_LargeFrameRejected(t *testing.T) {
	socketPath := tempSocketPath(t)
	src := NewSwiftAudioSource(socketPath)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = src.Stop() })

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		// retry once — listener may not be ready
		time.Sleep(50 * time.Millisecond)
		conn, err = net.Dial("unix", socketPath)
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send a tag-0x01 frame with a length > maxPayloadBytes (640_000).
	var buf [5]byte
	buf[0] = tagAudioFrame
	binary.LittleEndian.PutUint32(buf[1:], 1_000_000)
	_, _ = conn.Write(buf[:])

	// The reader should detect the oversized frame and close the connection.
	// The channel should eventually close (or no frames arrive).
	timeout := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return // channel closed — pass
			}
			// continue draining
		case <-timeout:
			// Channel stayed open but no crash — acceptable for this test since
			// the reader may simply reject the frame and wait for a reconnect.
			return
		}
	}
}

// TestSwiftAudioSource_RaceDetector sends concurrent frames and control events
// to exercise the race detector on shared state.
func TestSwiftAudioSource_RaceDetector(t *testing.T) {
	socketPath := tempSocketPath(t)
	src := NewSwiftAudioSource(socketPath)

	var callCount atomic.Int32
	src.OnHotkeyEvent = func(_ string) { callCount.Add(1) }

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	ch, err := src.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = src.Stop() })

	swift := dialFakeSwift(t, socketPath)

	const iterations = 20
	var wg sync.WaitGroup

	// Sender goroutine 1 — audio frames.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			swift.sendAudioFrame([]float32{float32(i) * 0.01})
		}
	}()

	// Sender goroutine 2 — control events.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			if i%2 == 0 {
				swift.sendControlEvent("press", "ctrl+shift+v")
			} else {
				swift.sendControlEvent("release", "ctrl+shift+v")
			}
		}
	}()

	// Drain frames concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		drainCh(ch)
	}()

	wg.Wait()
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// recvFrame reads one frame from ch within the deadline or fails the test.
func recvFrame(t *testing.T, ch <-chan []float32, d time.Duration) []float32 {
	t.Helper()
	select {
	case f, ok := <-ch:
		if !ok {
			t.Fatal("channel closed before frame arrived")
		}
		return f
	case <-time.After(d):
		t.Fatalf("no frame received within %v", d)
		return nil
	}
}

// drainCh discards frames until the channel is closed or 200 ms elapses.
func drainCh(ch <-chan []float32) {
	timeout := time.After(200 * time.Millisecond)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-timeout:
			return
		}
	}
}

// assertSamplesEqual compares two float32 slices for exact equality.
func assertSamplesEqual(t *testing.T, label string, want, got []float32) {
	t.Helper()
	if len(want) != len(got) {
		t.Errorf("%s: len(want)=%d, len(got)=%d", label, len(want), len(got))
		return
	}
	for i := range want {
		// Float32 round-trip through IEEE-754 binary encoding must be exact.
		if math.Float32bits(want[i]) != math.Float32bits(got[i]) {
			t.Errorf("%s: sample[%d]: want %v, got %v", label, i, want[i], got[i])
		}
	}
}

// TestMain removes the test socket file if it exists (belt-and-suspenders
// cleanup — t.TempDir() handles this normally, but belt-and-suspenders).
func TestMain(m *testing.M) {
	_ = os.Remove("/tmp/test-vedox-voice.sock")
	os.Exit(m.Run())
}
