// VedoxVoice — Hotkey.swift
//
// Global push-to-talk hotkey listener using NSEvent.addGlobalMonitorForEvents.
//
// Design:
//   NSEvent global monitors fire for key events in *other* applications.
//   For events in our own process, a local monitor is also installed.
//   Both monitors are required so PTT works whether Vedox is frontmost or not.
//
// Accessibility permission:
//   Global event monitors require the "Input Monitoring" permission
//   (Privacy & Security → Input Monitoring in System Settings) OR the
//   Accessibility permission, depending on macOS version.  If the permission
//   is missing, addGlobalMonitorForEvents returns nil and the listener falls
//   back to local-only mode with a warning.  The binary must be signed with
//   the com.apple.security.automation.apple-events entitlement for
//   accessibility APIs.
//
//   In sandbox-free command-line tool builds (SPM default), the system dialog
//   is shown automatically.  Sandboxed App Store builds require the
//   com.apple.security.temporary-exception.apple-events.* entitlement (which
//   App Store review rarely approves) — ship as a Developer ID binary instead.
//
// Hotkey parsing:
//   The hotkey string "ctrl+shift+v" is parsed into NSEventModifierFlags and
//   a key character.  Supported modifiers: ctrl, shift, alt/option, cmd.
//   The key character is the last '+'-separated token.
//
// Threading:
//   NSEvent monitors fire on the main thread (or a private AppKit thread when
//   installed with the global monitor).  onPress / onRelease are called on
//   whatever thread NSEvent uses — callers must be thread-safe.

import AppKit
import Foundation

// ---------------------------------------------------------------------------
// HotkeyError
// ---------------------------------------------------------------------------

enum HotkeyError: LocalizedError {
    case parseFailure(hotkey: String)
    case globalMonitorUnavailable

    var errorDescription: String? {
        switch self {
        case .parseFailure(let hk):
            return "Cannot parse hotkey \"\(hk)\". Format: \"ctrl+shift+v\" (modifiers then key)."
        case .globalMonitorUnavailable:
            return
                "Global event monitor could not be registered. Grant Input Monitoring permission in System Settings."
        }
    }
}

// ---------------------------------------------------------------------------
// HotkeyListener
// ---------------------------------------------------------------------------

/// Registers a global + local push-to-talk hotkey and fires press/release callbacks.
final class HotkeyListener {
    // -----------------------------------------------------------------------
    // Public interface
    // -----------------------------------------------------------------------

    /// Called when the hotkey is pressed.  Must not block.
    var onPress: (() -> Void)?

    /// Called when the hotkey is released.  Must not block.
    var onRelease: (() -> Void)?

    // -----------------------------------------------------------------------
    // Private state
    // -----------------------------------------------------------------------

    private let hotkeyString: String
    private var modifiers: NSEvent.ModifierFlags = []
    private var keyCharacter: String = ""

    private var globalPressMonitor: Any?
    private var globalReleaseMonitor: Any?
    private var localPressMonitor: Any?
    private var localReleaseMonitor: Any?

    private var isPressed = false
    private let lock = NSLock()

    // -----------------------------------------------------------------------
    // Init / deinit
    // -----------------------------------------------------------------------

    init(hotkey: String) {
        self.hotkeyString = hotkey
    }

    deinit {
        stop()
    }

    // -----------------------------------------------------------------------
    // Lifecycle
    // -----------------------------------------------------------------------

    /// Parse the hotkey string and install NSEvent monitors.
    ///
    /// Throws ``HotkeyError`` if the string cannot be parsed.
    /// Logs a warning (but does not throw) if the global monitor is unavailable
    /// due to missing permissions — local-only monitoring continues.
    func start() throws {
        try parseHotkey(hotkeyString)
        installMonitors()
        print("[HotkeyListener] registered hotkey: \(hotkeyString)")
    }

    /// Remove all installed NSEvent monitors.
    func stop() {
        removeMonitors()
        print("[HotkeyListener] unregistered hotkey: \(hotkeyString)")
    }

    // -----------------------------------------------------------------------
    // Parsing
    // -----------------------------------------------------------------------

    private func parseHotkey(_ hotkey: String) throws {
        var parts = hotkey.lowercased().split(separator: "+").map(String.init)
        guard !parts.isEmpty else {
            throw HotkeyError.parseFailure(hotkey: hotkey)
        }

        // Last part is the key character; everything before is a modifier.
        let keyPart = parts.removeLast()
        guard !keyPart.isEmpty else {
            throw HotkeyError.parseFailure(hotkey: hotkey)
        }
        keyCharacter = keyPart

        var flags: NSEvent.ModifierFlags = []
        for mod in parts {
            switch mod {
            case "ctrl", "control":
                flags.insert(.control)
            case "shift":
                flags.insert(.shift)
            case "alt", "option":
                flags.insert(.option)
            case "cmd", "command":
                flags.insert(.command)
            default:
                throw HotkeyError.parseFailure(hotkey: hotkey)
            }
        }
        modifiers = flags
    }

    // -----------------------------------------------------------------------
    // Monitor management
    // -----------------------------------------------------------------------

    private func installMonitors() {
        // Global monitors fire when any other app is frontmost.
        globalPressMonitor = NSEvent.addGlobalMonitorForEvents(
            matching: .keyDown
        ) { [weak self] event in
            self?.handleEvent(event, isPress: true)
        }

        globalReleaseMonitor = NSEvent.addGlobalMonitorForEvents(
            matching: .keyUp
        ) { [weak self] event in
            self?.handleEvent(event, isPress: false)
        }

        if globalPressMonitor == nil || globalReleaseMonitor == nil {
            fputs(
                "[HotkeyListener] WARNING: global monitor unavailable — " +
                    "PTT will only work when VedoxVoice is the frontmost app. " +
                    "Grant Input Monitoring permission in System Settings.\n",
                stderr
            )
        }

        // Local monitors fire when VedoxVoice itself is frontmost.
        localPressMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) {
            [weak self] event in
            self?.handleEvent(event, isPress: true)
            return event  // don't consume — let it propagate
        }

        localReleaseMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyUp) {
            [weak self] event in
            self?.handleEvent(event, isPress: false)
            return event
        }
    }

    private func removeMonitors() {
        if let m = globalPressMonitor { NSEvent.removeMonitor(m); globalPressMonitor = nil }
        if let m = globalReleaseMonitor { NSEvent.removeMonitor(m); globalReleaseMonitor = nil }
        if let m = localPressMonitor { NSEvent.removeMonitor(m); localPressMonitor = nil }
        if let m = localReleaseMonitor { NSEvent.removeMonitor(m); localReleaseMonitor = nil }
    }

    // -----------------------------------------------------------------------
    // Event matching
    // -----------------------------------------------------------------------

    private func handleEvent(_ event: NSEvent, isPress: Bool) {
        guard matchesHotkey(event) else { return }

        lock.lock()
        let wasPressed = isPressed
        isPressed = isPress
        lock.unlock()

        // Suppress auto-repeat key-down events (isPress true when already pressed).
        if isPress && wasPressed { return }
        if !isPress && !wasPressed { return }

        if isPress {
            onPress?()
        } else {
            onRelease?()
        }
    }

    private func matchesHotkey(_ event: NSEvent) -> Bool {
        // Strip modifiers not relevant to matching (caps lock, numpad, etc).
        let relevantFlags: NSEvent.ModifierFlags = [.control, .shift, .option, .command]
        let eventMods = event.modifierFlags.intersection(relevantFlags)
        guard eventMods == modifiers else { return false }

        // characters(byApplyingModifiers:) is only available on 10.15+.
        // characters gives the unshifted character for simple keys.
        // For the PTT use case the hotkey key should be a simple printable char.
        guard let chars = event.charactersIgnoringModifiers?.lowercased() else { return false }
        return chars == keyCharacter
    }
}
