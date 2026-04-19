// swift-tools-version: 5.9
//
// Package.swift — VedoxVoice
//
// A macOS command-line helper that:
//   • Captures microphone audio via AVAudioEngine at 16 kHz mono float32
//   • Registers a global push-to-talk hotkey via NSEvent global monitor
//   • Streams PCM frames over a Unix domain socket to the Go daemon
//
// Build requirements:
//   macOS 13+, Xcode 15+, Apple Developer signing (entitlements)
//
// Quick build (unsigned, sandbox disabled — dev only):
//   swift build -c release
//   .build/release/VedoxVoice --socket /tmp/vedox-voice.sock
//
// Signed build (required for mic access in production):
//   xcodebuild -scheme VedoxVoice \
//     CODE_SIGN_IDENTITY="Developer ID Application: ..." \
//     CODE_SIGN_ENTITLEMENTS=VedoxVoice.entitlements
//
// See README.md for full codesign + notarize instructions.

import PackageDescription

let package = Package(
    name: "VedoxVoice",
    platforms: [
        .macOS(.v13),
    ],
    products: [
        .executable(name: "VedoxVoice", targets: ["VedoxVoice"]),
    ],
    targets: [
        .executableTarget(
            name: "VedoxVoice",
            path: "Sources/VedoxVoice",
            swiftSettings: [
                // Treat all warnings as errors to keep the scaffold honest.
                .unsafeFlags(["-warnings-as-errors"]),
            ]
        ),
    ]
)
