// VedoxVoice — main.swift
//
// Entry point for the VedoxVoice macOS helper binary.
//
// Responsibilities:
//   1. Parse --socket <path> from argv (defaults to /tmp/vedox-voice.sock)
//   2. Initialise the SocketBridge (connects to the Go daemon)
//   3. Initialise AudioCapture (AVAudioEngine, 16 kHz mono float32)
//   4. Initialise the global hotkey listener (NSEvent global monitor)
//   5. Run the main RunLoop until SIGTERM / SIGINT
//
// Protocol over the Unix socket (binary, no framing header):
//
//   While PTT is active the binary streams raw little-endian float32 samples
//   at 16 000 Hz mono.  The Go daemon reads until the connection closes or a
//   framing error occurs.  No length-prefix is used — the daemon drives timing
//   via the PTT start/stop control messages on a companion JSON channel.
//
//   Control channel (same socket, line-delimited JSON written before audio):
//     {"event":"press","hotkey":"ctrl+shift+v"}\n
//     {"event":"release","hotkey":"ctrl+shift+v"}\n
//
//   Audio is interleaved with control messages on the same connection using a
//   simple tag byte prefix:
//     0x01 <4-byte little-endian length> <float32 samples>  — audio frame
//     0x02 <newline-terminated JSON>                        — control event
//
// This design is intentionally simple: the Go side can dispatch on the first
// byte of each message without buffering ambiguity.
//
// IMPORTANT: mic access requires the com.apple.security.device.audio-input
// entitlement AND a valid NSMicrophoneUsageDescription in Info.plist.  The
// binary must be signed and notarized before shipping; see README.md.

import AppKit
import Foundation

// ---------------------------------------------------------------------------
// Argument parsing
// ---------------------------------------------------------------------------

/// Parse --socket <path> from the command line.  Returns the resolved path.
func parseSocketPath() -> String {
    let args = CommandLine.arguments
    if let idx = args.firstIndex(of: "--socket"), idx + 1 < args.count {
        return args[idx + 1]
    }
    return "/tmp/vedox-voice.sock"
}

// ---------------------------------------------------------------------------
// Signal handling
// ---------------------------------------------------------------------------

/// A simple wrapper that catches SIGTERM and SIGINT and tears down gracefully.
final class SignalHandler {
    private let source: DispatchSourceSignal
    private let onSignal: () -> Void

    init(signal sig: Int32, handler: @escaping () -> Void) {
        // Ignore the signal at the POSIX level so DispatchSource can handle it.
        Darwin.signal(sig, SIG_IGN)
        source = DispatchSource.makeSignalSource(signal: sig, queue: .main)
        onSignal = handler
        source.setEventHandler { [weak self] in self?.onSignal() }
        source.resume()
    }

    deinit { source.cancel() }
}

// ---------------------------------------------------------------------------
// Application lifecycle
// ---------------------------------------------------------------------------

final class VedoxVoiceApp {
    private let socketPath: String
    private var bridge: SocketBridge?
    private var audio: AudioCapture?
    private var hotkey: HotkeyListener?

    init(socketPath: String) {
        self.socketPath = socketPath
    }

    func start() throws {
        print("[VedoxVoice] starting — socket: \(socketPath)")

        // 1. Connect to the Go daemon socket.
        let bridge = SocketBridge(socketPath: socketPath)
        try bridge.connect()
        self.bridge = bridge

        // 2. Set up audio capture.
        let audio = AudioCapture()
        audio.onFrame = { [weak bridge] samples in
            // Called from the AVAudioEngine tap thread — must not block.
            bridge?.sendAudioFrame(samples)
        }
        self.audio = audio

        // 3. Register the global PTT hotkey (requires Accessibility permission).
        let hotkey = HotkeyListener(hotkey: "ctrl+shift+v")
        hotkey.onPress = { [weak self] in
            print("[VedoxVoice] PTT press")
            self?.bridge?.sendControlEvent("press", hotkey: "ctrl+shift+v")
            self?.audio?.startCapture()
        }
        hotkey.onRelease = { [weak self] in
            print("[VedoxVoice] PTT release")
            self?.audio?.stopCapture()
            self?.bridge?.sendControlEvent("release", hotkey: "ctrl+shift+v")
        }
        try hotkey.start()
        self.hotkey = hotkey

        print("[VedoxVoice] ready — waiting for PTT (ctrl+shift+v)")
    }

    func stop() {
        print("[VedoxVoice] shutting down")
        audio?.stopCapture()
        hotkey?.stop()
        bridge?.disconnect()
    }
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

let socketPath = parseSocketPath()
let app = VedoxVoiceApp(socketPath: socketPath)

// Graceful shutdown on SIGTERM / SIGINT.
var sigterm: SignalHandler?
var sigint: SignalHandler?
sigterm = SignalHandler(signal: SIGTERM) {
    app.stop()
    exit(0)
}
sigint = SignalHandler(signal: SIGINT) {
    app.stop()
    exit(0)
}

do {
    try app.start()
} catch {
    fputs("[VedoxVoice] fatal: \(error)\n", stderr)
    exit(1)
}

// Hand control to the AppKit run loop so NSEvent global monitors fire.
// This call does not return under normal operation.
NSApplication.shared.run()
