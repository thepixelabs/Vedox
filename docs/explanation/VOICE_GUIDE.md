---
title: "voice input"
type: explanation
status: published
date: 2026-04-17
project: "vedox"
tags: ["voice", "whisper", "push-to-talk", "intent", "privacy"]
audience: "developer"
applies-to: "vedox v2.0 / v2.0.1"
---

# voice input

vedox supports hands-free documentation commands through a local, offline
speech-to-text pipeline. no audio ever leaves your machine.

## how it works

### v2.0 — push-to-talk

hold the push-to-talk hotkey (default: `ctrl+shift+v`), say a command phrase,
release the key. the daemon captures the audio, transcribes it locally with
whisper.cpp, parses the intent, and dispatches the command.

```
hold ctrl+shift+v
 → mic opens
 → "vedox document this folder"
 → mic closes
 → whisper transcribes (local, no network)
 → intent parser: CommandDocumentFolder, confidence 1.0
 → daemon executes
```

### v2.0.1 — wake-word (planned)

v2.0.1 will add an always-on wake-word listener so you do not need to hold a
key. the daemon stays in a low-power detection loop; audio processing only
starts after the wake word fires. the push-to-talk path remains available for
environments where always-on is not appropriate.

## intent grammar

the parser is deterministic and table-driven — no LLM is in the loop. it strips
the wake word "vedox" (or a known mishearing) from the front of the transcript,
then matches the remainder against a priority-ordered rule table.

| what you say | command | notes |
|---|---|---|
| `vedox document everything` | `document_everything` | all files in the current project |
| `vedox document this folder` | `document_folder` | files in the current working directory |
| `vedox document these changes` | `document_changes` | files changed in git diff |
| `vedox document <path>` | `document_file` | specific file; path captured as target |
| `vedox status` | `status` | queries daemon for running state |
| `vedox stop` | `stop` | cancels the current agent job |

accepted variants — the parser tolerates natural speech:

- "doc everything", "document all" → `document_everything`
- "doc folder", "document the folder", "document this directory" → `document_folder`
- "doc changes", "document the changes", "document my changes" → `document_changes`
- "cancel", "halt", "abort" → `stop`

**confidence scoring**

| match type | confidence |
|---|---|
| exact wake word + exact phrase | 1.0 |
| fuzzy wake word (Levenshtein ≤ 2: "vedocks", "vdox", etc.) + exact phrase | 0.7 |
| phrase matched without wake word | 0.5 |
| no match | 0.0 (command: `unknown`) |

the default minimum confidence threshold is 0.5. commands below that threshold
are silently dropped. check the threshold with `vedox voice status`.

## privacy

- **local-only transcription.** whisper.cpp runs entirely on your machine. the
  go daemon calls the model directly through the go-whisper binding — no http
  request is made.
- **no audio persistence.** pcm frames flow from the microphone into an
  in-memory channel and are discarded after transcription. nothing is written
  to disk.
- **no cloud dependency.** the voice pipeline works with the network interface
  completely offline. the daemon only makes outbound connections to your
  registered doc repos.

## platform support

### macos — VedoxVoice.app helper

global hotkey capture on macos requires a small swift helper
(`VedoxVoiceHelper`) that holds the `NSEvent.addGlobalMonitorForEvents`
subscription. the helper ships inside the `Vedox.app` bundle and communicates
with the go daemon over a unix socket at `~/.vedox/run/voice-hotkey.sock` using
line-delimited json:

```
{"event":"press","hotkey":"ctrl+shift+v"}
{"event":"release","hotkey":"ctrl+shift+v"}
```

audio frames follow on a separate socket at `/tmp/vedox-voice.sock` using a
tag-byte framed binary protocol (no cgo required on the go side).

the helper requires **accessibility permission**: system settings →
privacy & security → accessibility → vedox. if the permission is missing, the
daemon logs `ErrAccessibilityPermission` and prints actionable guidance.

**status: ships in v2.0.1.**

### linux — whisper.cpp direct

on linux, audio is captured via the evdev subsystem (no cgo). the go daemon
reads `/dev/input/event*` directly from a goroutine for hotkey detection and
captures audio through the configured alsa or pipewire device.

build with the whisper tag to enable real transcription:

```sh
./build-whisper.sh   # see docs/how-to/build-whisper.md
```

**status: available in v2.0 with the `-tags whisper` build.**

### windows — deferred

windows support is not scheduled. the daemon falls back to stub mode on windows
and logs `ErrNativeHotkeyUnavailable`.

## enable voice

```sh
# 1. start the daemon with voice enabled
vedox server start --voice

# 2. on macos: install the VedoxVoice.app helper (ships with the .app bundle)
open /Applications/Vedox.app   # launches and registers VedoxVoiceHelper

# 3. on linux: build with whisper support first (see above)
```

the `--voice` flag is required; without it the `/api/voice/status` endpoint
still responds but reports `enabled: false`.

## test without a microphone

`vedox voice test` runs the full pipeline with a stub audio source. no daemon
is required.

```sh
# simulate a transcription result without real audio
vedox voice test --text "vedox document this folder"
```

expected output:

```
voice test: starting stub pipeline (hotkey: ctrl+shift+v)
voice test: PTT active for 3s...
voice test: success
  transcript: "vedox document this folder"
  command:    document_folder
  confidence: 1.00
  dispatch:   intercepted (stub mode — no daemon call made)
```

other useful invocations:

```sh
# check what a fuzzy wake word resolves to
vedox voice test --text "vedocks document everything"

# test the document_file path
vedox voice test --text "vedox document src/main.go"

# shorten the hold duration
vedox voice test --text "vedox status" --duration 1
```

## troubleshooting

**mic permission denied (macos)**

```
accessibility permission required: open System Settings → Privacy & Security →
Accessibility and enable Vedox, then restart the daemon
```

open system settings → privacy & security → accessibility and toggle vedox on.
then `vedox server stop && vedox server start --voice`.

---

**helper not running (macos)**

```
native global hotkey monitoring is not available in this build; the VedoxVoiceHelper
binary (macOS) has not been installed
```

the daemon fell back to stub mode. the `VedoxVoiceHelper` binary was not found
at `~/.vedox/bin/VedoxVoiceHelper`. launch `Vedox.app` once to let the bundle
register the helper, or copy it manually:

```sh
cp /Applications/Vedox.app/Contents/Helpers/VedoxVoiceHelper ~/.vedox/bin/
chmod +x ~/.vedox/bin/VedoxVoiceHelper
vedox server stop && vedox server start --voice
```

---

**intent confidence too low**

```sh
vedox voice test --text "<your transcript here>"
```

if confidence prints as `0.50` or `0.00`, the phrase did not match a known rule.
use one of the canonical phrases from the intent grammar table above. the
minimum confidence threshold (default 0.5) can be lowered with
`--min-confidence 0.4` on `vedox server start --voice`, but values below 0.5
risk false dispatches.

---

**daemon not running**

```
vedox daemon is not running — start it with 'vedox server start'
```

run `vedox server start --voice` before calling `vedox voice status` or any
voice command.

## see also

- [build vedox with real whisper transcription](../how-to/build-whisper.md)
- [how-to: use command palette](../how-to/use-command-palette.md)
