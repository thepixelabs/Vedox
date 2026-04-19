// VedoxVoice — SocketBridge.swift
//
// Unix domain socket sender that streams audio frames and control events to
// the Go daemon.
//
// Wire protocol (one connection, full-duplex):
//
//   Each message starts with a single tag byte:
//     0x01  Audio frame    — tag(1) + length(4, LE uint32) + float32 samples
//     0x02  Control event  — tag(1) + JSON bytes + newline '\n'
//
//   The Go daemon reads tag bytes to dispatch; this allows audio and control
//   messages to be multiplexed on a single Unix socket connection without a
//   separate control channel.
//
//   Audio encoding: each float32 sample is written as 4 bytes, little-endian
//   IEEE-754 (matching binary.LittleEndian.Uint32 / math.Float32frombits on the
//   Go side).
//
// Threading:
//   connect / disconnect are called from the main thread at startup/shutdown.
//   sendAudioFrame is called from the AVAudioEngine tap thread.
//   sendControlEvent is called from the NSEvent monitor thread.
//   All writes are serialised through a DispatchQueue to prevent interleaving.
//
// Error handling:
//   Write errors are logged to stderr.  The bridge does not attempt automatic
//   reconnection — the Go daemon is expected to restart the helper if the
//   connection drops.  A future version may add a reconnect back-off loop.

import Foundation

// ---------------------------------------------------------------------------
// Message tag bytes
// ---------------------------------------------------------------------------

private let tagAudioFrame: UInt8 = 0x01
private let tagControlEvent: UInt8 = 0x02

// ---------------------------------------------------------------------------
// SocketBridge
// ---------------------------------------------------------------------------

/// Maintains a Unix domain socket connection to the Go daemon and provides
/// thread-safe methods for sending audio frames and control events.
final class SocketBridge {
    // -----------------------------------------------------------------------
    // Public interface
    // -----------------------------------------------------------------------

    let socketPath: String

    // -----------------------------------------------------------------------
    // Private state
    // -----------------------------------------------------------------------

    private var fileHandle: FileHandle?

    /// Serial queue that serialises all socket writes.
    private let writeQueue = DispatchQueue(label: "net.pixelabs.vedox.voice.socket-bridge")

    // -----------------------------------------------------------------------
    // Init
    // -----------------------------------------------------------------------

    init(socketPath: String) {
        self.socketPath = socketPath
    }

    // -----------------------------------------------------------------------
    // Connection management
    // -----------------------------------------------------------------------

    /// Open a connection to the Unix domain socket at `socketPath`.
    ///
    /// The Go daemon must already be listening before this is called.
    /// Throws if the socket cannot be opened.
    func connect() throws {
        let fd = socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else {
            throw SocketBridgeError.socketCreateFailed(errno: errno)
        }

        var addr = sockaddr_un()
        addr.sun_family = sa_family_t(AF_UNIX)

        // Copy the path into the fixed-size sun_path C array.
        let pathBytes = socketPath.utf8CString
        guard pathBytes.count <= MemoryLayout.size(ofValue: addr.sun_path) else {
            Darwin.close(fd)
            throw SocketBridgeError.pathTooLong(socketPath)
        }
        withUnsafeMutableBytes(of: &addr.sun_path) { dst in
            pathBytes.withUnsafeBytes { src in
                dst.copyMemory(from: UnsafeRawBufferPointer(start: src.baseAddress, count: pathBytes.count))
            }
        }

        let addrLen = socklen_t(MemoryLayout<sockaddr_un>.size)
        let result = withUnsafePointer(to: &addr) { ptr in
            ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) { sptr in
                Darwin.connect(fd, sptr, addrLen)
            }
        }

        guard result == 0 else {
            Darwin.close(fd)
            throw SocketBridgeError.connectFailed(path: socketPath, errno: errno)
        }

        fileHandle = FileHandle(fileDescriptor: fd, closeOnDealloc: true)
        print("[SocketBridge] connected to \(socketPath)")
    }

    /// Close the connection to the Go daemon.
    func disconnect() {
        writeQueue.sync {
            try? fileHandle?.close()
            fileHandle = nil
        }
        print("[SocketBridge] disconnected")
    }

    // -----------------------------------------------------------------------
    // Sending
    // -----------------------------------------------------------------------

    /// Send an audio frame to the daemon.
    ///
    /// Thread-safe.  Called from the AVAudioEngine tap thread.
    /// samples must be float32 values in the range [-1.0, 1.0].
    func sendAudioFrame(_ samples: [Float]) {
        guard !samples.isEmpty else { return }

        // Encode: tag(1) + length(4 LE) + samples (4 bytes each, LE float32)
        let payloadLength = samples.count * MemoryLayout<Float>.size
        var buf = Data(capacity: 1 + 4 + payloadLength)

        buf.append(tagAudioFrame)

        // 4-byte little-endian length
        var len = UInt32(payloadLength).littleEndian
        withUnsafeBytes(of: &len) { buf.append(contentsOf: $0) }

        // float32 samples as little-endian bytes
        samples.withUnsafeBytes { raw in
            // On Apple silicon and x86 both use little-endian — no swap needed.
            buf.append(contentsOf: raw)
        }

        write(buf)
    }

    /// Send a PTT control event (press or release) to the daemon.
    ///
    /// Thread-safe.  Called from the NSEvent monitor thread.
    func sendControlEvent(_ event: String, hotkey: String) {
        // Build compact JSON: {"event":"press","hotkey":"ctrl+shift+v"}
        let json: [String: String] = ["event": event, "hotkey": hotkey]
        guard let jsonData = try? JSONSerialization.data(withJSONObject: json) else { return }

        var buf = Data(capacity: 1 + jsonData.count + 1)
        buf.append(tagControlEvent)
        buf.append(contentsOf: jsonData)
        buf.append(0x0A)  // newline terminator

        write(buf)
    }

    // -----------------------------------------------------------------------
    // Internal write helper
    // -----------------------------------------------------------------------

    /// Write `data` to the socket on the serial write queue.
    private func write(_ data: Data) {
        writeQueue.async { [weak self] in
            guard let self, let fh = self.fileHandle else { return }
            do {
                try fh.write(contentsOf: data)
            } catch {
                fputs("[SocketBridge] write error: \(error)\n", stderr)
                // Close the handle so subsequent calls fail fast rather than
                // repeatedly printing errors.
                try? fh.close()
                self.fileHandle = nil
            }
        }
    }
}

// ---------------------------------------------------------------------------
// SocketBridgeError
// ---------------------------------------------------------------------------

enum SocketBridgeError: LocalizedError {
    case socketCreateFailed(errno: Int32)
    case pathTooLong(String)
    case connectFailed(path: String, errno: Int32)

    var errorDescription: String? {
        switch self {
        case .socketCreateFailed(let e):
            return "socket() failed: \(String(cString: strerror(e)))"
        case .pathTooLong(let p):
            return "Socket path too long for sockaddr_un.sun_path: \(p)"
        case .connectFailed(let path, let e):
            return "connect() to \(path) failed: \(String(cString: strerror(e)))"
        }
    }
}
