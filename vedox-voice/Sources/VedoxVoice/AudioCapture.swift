// VedoxVoice — AudioCapture.swift
//
// AVAudioEngine wrapper that captures microphone input at 16 kHz mono float32
// and forwards chunks to a caller-supplied closure.
//
// Audio format contract (must match the Go daemon's AudioSource interface):
//   Sample rate : 16 000 Hz
//   Channels    : 1 (mono)
//   Sample type : IEEE-754 float32, range [-1.0, 1.0]
//   Chunk size  : DefaultChunkSamples (512 samples = 32 ms)
//
// TCC (Transparency, Consent, and Control) note:
//   On macOS, AVAudioEngine microphone capture requires:
//     1. com.apple.security.device.audio-input entitlement in the signed binary.
//     2. NSMicrophoneUsageDescription key in Info.plist.
//     3. The user must have granted mic permission (prompted automatically on
//        first use when the binary is properly signed and notarized).
//   AVAudioSession-style explicit permission requests are iOS-only; on macOS
//   the system dialog appears when AVAudioEngine.start() is called without
//   prior permission.  If permission is denied, start() throws an error —
//   AudioCapture surfaces this as AudioCaptureError.permissionDenied.
//
// Threading:
//   startCapture / stopCapture are safe to call from any thread.
//   onFrame is invoked on an AVAudioEngine internal tap thread; it must not
//   block and must copy any data it needs beyond its own invocation.

import AVFoundation
import Foundation

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

enum AudioCaptureError: LocalizedError {
    /// The user denied microphone access or the entitlement is missing.
    case permissionDenied
    /// AVAudioEngine could not be configured or started.
    case engineFailure(underlying: Error)
    /// The input node's native format cannot be converted to 16 kHz mono.
    case unsupportedFormat(description: String)

    var errorDescription: String? {
        switch self {
        case .permissionDenied:
            return "Microphone access denied. Grant permission in System Settings → Privacy & Security → Microphone."
        case .engineFailure(let err):
            return "AVAudioEngine failure: \(err.localizedDescription)"
        case .unsupportedFormat(let desc):
            return "Unsupported input format: \(desc)"
        }
    }
}

// ---------------------------------------------------------------------------
// AudioCapture
// ---------------------------------------------------------------------------

/// DefaultChunkSamples is the number of float32 samples per emitted frame.
/// 512 samples at 16 kHz = 32 ms — matches the Go daemon's DefaultChunkSamples.
let DefaultChunkSamples: AVAudioFrameCount = 512

/// AudioCapture wraps AVAudioEngine to deliver 16 kHz mono float32 chunks.
final class AudioCapture {
    // -----------------------------------------------------------------------
    // Public interface
    // -----------------------------------------------------------------------

    /// Called on every captured audio frame.  Must not block.
    /// The [Float] slice is a copy — the closure may retain it safely.
    var onFrame: (([Float]) -> Void)?

    // -----------------------------------------------------------------------
    // Private state
    // -----------------------------------------------------------------------

    private let engine = AVAudioEngine()
    private let targetFormat: AVAudioFormat

    /// Converter from the hardware input format to 16 kHz mono float32.
    private var converter: AVAudioConverter?

    private let lock = NSLock()
    private var capturing = false

    // -----------------------------------------------------------------------
    // Init
    // -----------------------------------------------------------------------

    init() {
        // Force-unwrap is safe: these are constant, valid parameters.
        targetFormat = AVAudioFormat(
            commonFormat: .pcmFormatFloat32,
            sampleRate: 16_000,
            channels: 1,
            interleaved: false
        )!
    }

    // -----------------------------------------------------------------------
    // Capture control
    // -----------------------------------------------------------------------

    /// Begin capturing audio from the default microphone.
    ///
    /// Throws ``AudioCaptureError`` if the engine cannot be started or the
    /// microphone permission is not granted.
    func startCapture() {
        lock.lock()
        defer { lock.unlock() }

        guard !capturing else { return }

        let inputNode = engine.inputNode
        let hwFormat = inputNode.outputFormat(forBus: 0)

        // Build a converter from whatever the hardware delivers to our target.
        guard let conv = AVAudioConverter(from: hwFormat, to: targetFormat) else {
            fputs(
                "[AudioCapture] cannot build converter from \(hwFormat) to \(targetFormat)\n",
                stderr
            )
            return
        }
        converter = conv

        // Install a tap on the input node's bus 0.
        // bufferSize is a hint; Core Audio may deliver larger or smaller buffers.
        inputNode.installTap(
            onBus: 0,
            bufferSize: DefaultChunkSamples,
            format: hwFormat
        ) { [weak self] buffer, _ in
            self?.handleTap(buffer: buffer)
        }

        do {
            try engine.start()
        } catch {
            fputs("[AudioCapture] engine start failed: \(error)\n", stderr)
            inputNode.removeTap(onBus: 0)
            converter = nil
            return
        }

        capturing = true
        print("[AudioCapture] capturing at 16 kHz mono float32")
    }

    /// Stop capturing audio and release the tap.
    func stopCapture() {
        lock.lock()
        defer { lock.unlock() }

        guard capturing else { return }

        engine.inputNode.removeTap(onBus: 0)
        engine.stop()
        converter = nil
        capturing = false
        print("[AudioCapture] stopped")
    }

    // -----------------------------------------------------------------------
    // Tap callback
    // -----------------------------------------------------------------------

    /// Called by AVAudioEngine on an internal tap thread for every captured buffer.
    private func handleTap(buffer: AVAudioPCMBuffer) {
        guard let conv = converter else { return }
        guard let cb = onFrame else { return }

        // Compute how many output frames this input buffer will produce.
        let inputFrameCount = buffer.frameLength
        let sampleRateRatio = targetFormat.sampleRate / buffer.format.sampleRate
        let outputFrameCapacity = AVAudioFrameCount(
            ceil(Double(inputFrameCount) * sampleRateRatio)
        )

        guard outputFrameCapacity > 0 else { return }

        guard let outBuf = AVAudioPCMBuffer(
            pcmFormat: targetFormat,
            frameCapacity: outputFrameCapacity
        ) else { return }

        var error: NSError?
        var inputConsumed = false

        // AVAudioConverter uses a callback-based input model.
        conv.convert(to: outBuf, error: &error) { _, outStatus in
            if inputConsumed {
                outStatus.pointee = .noDataNow
                return nil
            }
            inputConsumed = true
            outStatus.pointee = .haveData
            return buffer
        }

        if let err = error {
            fputs("[AudioCapture] conversion error: \(err)\n", stderr)
            return
        }

        let frameCount = Int(outBuf.frameLength)
        guard frameCount > 0 else { return }

        // Copy samples out of the buffer before releasing it.
        guard let channelData = outBuf.floatChannelData?[0] else { return }
        let samples = Array(UnsafeBufferPointer(start: channelData, count: frameCount))

        // Deliver in DefaultChunkSamples-sized chunks so the Go side sees a
        // predictable frame size.  If conversion produces a partial final chunk,
        // emit it as-is.
        var offset = 0
        let chunkSize = Int(DefaultChunkSamples)
        while offset < samples.count {
            let end = min(offset + chunkSize, samples.count)
            cb(Array(samples[offset..<end]))
            offset = end
        }
    }
}
