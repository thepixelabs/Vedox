# VedoxVoice — macOS helper binary

Push-to-talk audio capture for the Vedox documentation editor.

Captures microphone input at 16 kHz mono float32 via AVAudioEngine, registers
a global `ctrl+shift+v` hotkey via NSEvent, and streams PCM frames over a Unix
domain socket to the Go daemon.

## Requirements

- macOS 13 Ventura or later
- Xcode 15+ (for `xcodebuild`) or Swift 5.9+ (for `swift build`)
- Apple Developer Program membership (for signing and notarization)

## Quick start (development, unsigned)

Unsigned builds work for local development but mic access will be blocked by
TCC on macOS.  Use a signed build for real hardware testing.

```sh
cd vedox-voice
swift build -c release
.build/release/VedoxVoice --socket /tmp/vedox-voice.sock
```

## Signed build (Developer ID — required for production)

### 1. Build

```sh
cd vedox-voice
swift build -c release
```

### 2. Codesign with entitlements

Replace `TEAM_ID` with your 10-character Apple team identifier.

```sh
codesign \
  --sign "Developer ID Application: Pixelabs (TEAM_ID)" \
  --entitlements VedoxVoice.entitlements \
  --options runtime \
  --timestamp \
  --force \
  .build/release/VedoxVoice
```

Verify:

```sh
codesign --verify --verbose=4 .build/release/VedoxVoice
spctl --assess --type exec .build/release/VedoxVoice
```

### 3. Embed Info.plist

The SPM command-line build does not embed Info.plist automatically.  Do this
before notarization:

```sh
# Embed the plist into the Mach-O binary's __TEXT,__info_plist section.
xcrun lipo -create .build/release/VedoxVoice -output .build/release/VedoxVoice
/usr/libexec/PlistBuddy -c "Merge Info.plist" .build/release/VedoxVoice 2>/dev/null || true

# Re-sign after plist embed.
codesign \
  --sign "Developer ID Application: Pixelabs (TEAM_ID)" \
  --entitlements VedoxVoice.entitlements \
  --options runtime \
  --timestamp \
  --force \
  .build/release/VedoxVoice
```

### 4. Notarize

Package as a zip (notarytool requires a zip or dmg for command-line tools):

```sh
zip -j VedoxVoice.zip .build/release/VedoxVoice

xcrun notarytool submit VedoxVoice.zip \
  --apple-id "pixi@pixelabs.net" \
  --team-id "TEAM_ID" \
  --password "@keychain:AC_PASSWORD" \
  --wait
```

Staple the notarization ticket.  Note: tickets can only be stapled to bundles
or disk images, not bare executables.  For a bare binary, the ticket is held
by the Apple CDN — the system will verify it online on first run.

To check notarization status:

```sh
xcrun notarytool history \
  --apple-id "pixi@pixelabs.net" \
  --team-id "TEAM_ID" \
  --password "@keychain:AC_PASSWORD"
```

## Wire protocol

The binary connects to the Go daemon's Unix socket and sends two message types,
distinguished by a tag byte prefix:

| Tag  | Message type  | Payload                                          |
|------|---------------|--------------------------------------------------|
| 0x01 | Audio frame   | 4-byte LE uint32 length + float32 samples (LE)   |
| 0x02 | Control event | JSON bytes + `\n` newline                         |

Control event JSON schema:

```json
{"event": "press",   "hotkey": "ctrl+shift+v"}
{"event": "release", "hotkey": "ctrl+shift+v"}
```

Audio samples are raw IEEE-754 little-endian float32 at 16 kHz mono.

## Permissions

On first launch the system will prompt for:

1. **Microphone** — required for AVAudioEngine capture.
2. **Input Monitoring** — required for global hotkey detection.

If either is denied, the binary logs to stderr and exits with code 1.

## Runtime flags

| Flag       | Default                  | Description                      |
|------------|--------------------------|----------------------------------|
| `--socket` | `/tmp/vedox-voice.sock`  | Path to the Go daemon Unix socket |

## Integration with Go daemon

The Go daemon exposes an `AudioSource` implementation at
`apps/cli/internal/voice/swiftbridge.go`.  It creates the listening socket,
waits for the VedoxVoice binary to connect, and reads frames into the pipeline.

Start order:
1. Go daemon creates and listens on the Unix socket.
2. VedoxVoice binary is launched (by the daemon or by the user) pointing at the same socket path.
3. VedoxVoice connects and begins streaming on PTT press.
