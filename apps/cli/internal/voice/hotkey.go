// Package voice — hotkey.go
//
// HotkeyListener provides an abstraction over global keyboard shortcut
// detection.  Because registering global hotkeys without CGO or a helper
// binary is not possible on macOS, two implementations are provided:
//
//   - StubHotkeyListener — fully functional in-process implementation for
//     tests and CI.  Exposes SimulatePress / SimulateRelease so tests can
//     drive the PTT path deterministically.
//
//   - NativeHotkeyListener — stub placeholder.  The real macOS implementation
//     requires a Swift helper that calls NSEvent.addGlobalMonitorForEvents:
//     (see design note below).  Until that helper lands, this type returns
//     ErrNativeHotkeyUnavailable on Start, causing the daemon to fall back to
//     stub mode with an informative log message.
//
// # Default hotkey
//
// Ctrl+Shift+V is the default PTT hotkey.  It is configurable via
// ~/.vedox/user-prefs.json (key: "voice.hotkey").  The HotkeyConfig type
// captures the resolved preference.
//
// # macOS native implementation (deferred to v2.0.1 — VedoxVoice.app)
//
// Global hotkeys on macOS require either:
//  1. CGO bindings into the Objective-C AppKit / Carbon event APIs, or
//  2. A thin out-of-process helper (swift or obj-c) that holds the
//     NSEvent.addGlobalMonitorForEventsMatchingMask:handler: subscription
//     and forwards press/release over a Unix socket or named pipe.
//
// Approach 2 is preferred because it keeps the Go binary CGO-free and
// supports sandboxed App Store distribution.  The helper binary is named
// VedoxVoiceHelper and ships as a macOS bundle inside the Vedox.app
// package.  Communication protocol: line-delimited JSON over a Unix socket
// at ~/.vedox/run/voice-hotkey.sock.
//
//	{"event":"press","hotkey":"ctrl+shift+v"}
//	{"event":"release","hotkey":"ctrl+shift+v"}
//
// The NativeHotkeyListener (when fully implemented) will:
//  1. Launch VedoxVoiceHelper if it is not already running.
//  2. Connect to the Unix socket.
//  3. Decode line-delimited JSON events and invoke onPress / onRelease.
//
// Accessibility permission (AXIsProcessTrusted) must be granted; if not,
// the helper surfaces a system dialog.  The Go daemon detects the
// ErrAccessibilityPermission error from NativeHotkeyListener.Start and
// prints actionable guidance.
package voice

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ---------------------------------------------------------------------------
// HotkeyConfig
// ---------------------------------------------------------------------------

// HotkeyConfig holds the resolved push-to-talk hotkey preference.
// All fields have sensible defaults and can be overridden by the user via
// ~/.vedox/user-prefs.json.
type HotkeyConfig struct {
	// Hotkey is the human-readable shortcut string (e.g. "ctrl+shift+v").
	// The native listener parses this into platform-specific key codes.
	// Defaults to DefaultHotkey.
	Hotkey string
}

// DefaultHotkey is the out-of-box PTT hotkey.
const DefaultHotkey = "ctrl+shift+v"

// DefaultHotkeyConfig returns a HotkeyConfig with DefaultHotkey applied.
func DefaultHotkeyConfig() HotkeyConfig {
	return HotkeyConfig{Hotkey: DefaultHotkey}
}

// ---------------------------------------------------------------------------
// HotkeyListener interface
// ---------------------------------------------------------------------------

// HotkeyListener monitors a global keyboard shortcut and invokes callbacks
// when the configured key combination is pressed or released.
//
// Implementations must be goroutine-safe: Start, Stop, and the callbacks may
// be called from different goroutines.
//
// Lifecycle:
//
//	listener := voice.NewStubHotkeyListener(cfg)
//	if err := listener.Start(ctx, onPress, onRelease); err != nil { ... }
//	// listener fires onPress / onRelease in the background
//	listener.Stop()
type HotkeyListener interface {
	// Start begins monitoring the global hotkey.  onPress is called when the
	// hotkey is first detected as pressed; onRelease is called when it is
	// released.  Both callbacks must not block.
	//
	// Start returns after the listener has been initialised and is ready to
	// receive events.  An error is returned if the listener cannot be started
	// (e.g. missing accessibility permission on macOS, hotkey string parse
	// failure, or the helper binary not found).
	//
	// The provided context controls the listener lifetime.  When the context
	// is cancelled, the listener stops automatically (equivalent to Stop).
	Start(ctx context.Context, onPress, onRelease func()) error

	// Stop halts the listener and releases any OS resources.  Safe to call
	// multiple times and before Start has been called.
	Stop() error
}

// ---------------------------------------------------------------------------
// Sentinel errors
// ---------------------------------------------------------------------------

// ErrNativeHotkeyUnavailable is returned by NativeHotkeyListener.Start when
// the native helper binary is not present or the platform does not support
// global hotkeys without a helper.  Callers should fall back to StubHotkeyListener.
var ErrNativeHotkeyUnavailable = errors.New(
	"native global hotkey monitoring is not available in this build; " +
		"the VedoxVoiceHelper binary (macOS) has not been installed — " +
		"use the stub listener for testing or build the full Vedox.app bundle",
)

// ErrAccessibilityPermission is returned when the macOS accessibility
// permission (AXIsProcessTrusted) has not been granted.  The user must open
// System Settings → Privacy & Security → Accessibility and enable Vedox.
var ErrAccessibilityPermission = errors.New(
	"accessibility permission required: open System Settings → Privacy & Security → Accessibility " +
		"and enable Vedox, then restart the daemon",
)

// ---------------------------------------------------------------------------
// StubHotkeyListener
// ---------------------------------------------------------------------------

// StubHotkeyListener is a fully in-process HotkeyListener for tests and CI.
// It never touches OS event APIs; instead, tests drive it via SimulatePress
// and SimulateRelease.
//
// Usage:
//
//	l := voice.NewStubHotkeyListener(voice.DefaultHotkeyConfig())
//	l.Start(ctx, onPress, onRelease)
//	l.SimulatePress()
//	// ... PTT active
//	l.SimulateRelease()
//	l.Stop()
type StubHotkeyListener struct {
	cfg HotkeyConfig

	mu        sync.Mutex
	onPress   func()
	onRelease func()
	started   bool
	stopped   bool
	stopCh    chan struct{}
}

// NewStubHotkeyListener constructs a StubHotkeyListener with the given config.
func NewStubHotkeyListener(cfg HotkeyConfig) *StubHotkeyListener {
	return &StubHotkeyListener{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
}

// Start implements HotkeyListener.  The stub does not register any OS hotkey;
// it simply stores the callbacks and marks itself as started.
func (s *StubHotkeyListener) Start(ctx context.Context, onPress, onRelease func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return fmt.Errorf("stub hotkey listener: already started")
	}

	s.onPress = onPress
	s.onRelease = onRelease
	s.started = true

	// Watch for context cancellation and stop ourselves.
	go func() {
		select {
		case <-ctx.Done():
			s.Stop() //nolint:errcheck
		case <-s.stopCh:
		}
	}()

	return nil
}

// Stop implements HotkeyListener.
func (s *StubHotkeyListener) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return nil
	}
	s.stopped = true

	select {
	case <-s.stopCh:
		// already closed
	default:
		close(s.stopCh)
	}
	return nil
}

// SimulatePress fires the onPress callback as if the user pressed the hotkey.
// Safe to call from any goroutine.  No-op if the listener is not started.
func (s *StubHotkeyListener) SimulatePress() {
	s.mu.Lock()
	cb := s.onPress
	s.mu.Unlock()
	if cb != nil {
		cb()
	}
}

// SimulateRelease fires the onRelease callback as if the user released the
// hotkey.  Safe to call from any goroutine.  No-op if the listener is not started.
func (s *StubHotkeyListener) SimulateRelease() {
	s.mu.Lock()
	cb := s.onRelease
	s.mu.Unlock()
	if cb != nil {
		cb()
	}
}

// ---------------------------------------------------------------------------
// NativeHotkeyListener
// ---------------------------------------------------------------------------

// NativeHotkeyListener is the production implementation.  Currently it
// returns ErrNativeHotkeyUnavailable on all platforms because the
// VedoxVoiceHelper binary has not yet been built.
//
// When VedoxVoiceHelper ships (v2.0.1), this type will:
//   - On darwin: launch ~/.vedox/bin/VedoxVoiceHelper, connect to the Unix
//     socket at ~/.vedox/run/voice-hotkey.sock, and decode line-delimited
//     JSON events.
//   - On linux: use the evdev subsystem (read-only /dev/input/event*)
//     polled from a goroutine — no CGO required.
//   - On other platforms: return ErrNativeHotkeyUnavailable.
//
// Use NewNativeHotkeyListener to construct one; call Start to determine at
// runtime whether it is available and fall back to StubHotkeyListener when
// ErrNativeHotkeyUnavailable is returned.
type NativeHotkeyListener struct {
	cfg HotkeyConfig
}

// NewNativeHotkeyListener constructs a NativeHotkeyListener.
func NewNativeHotkeyListener(cfg HotkeyConfig) *NativeHotkeyListener {
	return &NativeHotkeyListener{cfg: cfg}
}

// Start implements HotkeyListener.  Currently returns ErrNativeHotkeyUnavailable
// on all platforms.  This will be replaced in v2.0.1.
func (n *NativeHotkeyListener) Start(_ context.Context, _, _ func()) error {
	return ErrNativeHotkeyUnavailable
}

// Stop implements HotkeyListener.  No-op until the native implementation lands.
func (n *NativeHotkeyListener) Stop() error {
	return nil
}

// ---------------------------------------------------------------------------
// Constructor helper used by the daemon
// ---------------------------------------------------------------------------

// NewBestHotkeyListener returns the most capable HotkeyListener available on
// the current platform.  It tries NativeHotkeyListener first; if that returns
// ErrNativeHotkeyUnavailable, it falls back to StubHotkeyListener and sets
// *stubFallback to true so the caller can log an informative message.
//
// This is the function the daemon (cmd/server.go) calls — it never needs to
// know which concrete type it received.
func NewBestHotkeyListener(cfg HotkeyConfig) (listener HotkeyListener, stubFallback bool) {
	native := NewNativeHotkeyListener(cfg)
	// Probe: try a no-op Start with a cancelled context to detect availability.
	probeCtx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancelled — no real subscription is established
	if err := native.Start(probeCtx, func() {}, func() {}); err == nil {
		// Native listener is available (shouldn't happen yet, but future-proofs the path).
		_ = native.Stop()
		return NewNativeHotkeyListener(cfg), false
	}
	return NewStubHotkeyListener(cfg), true
}
