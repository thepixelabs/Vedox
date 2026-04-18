package secrets_test

// autodetect_test.go — FIX-QA-06: branch coverage for AutoDetect.
//
// Three tests cover the three rungs of the AutoDetect selection ladder that
// were previously dark:
//
//  1. macOS → *KeyringStore  (the darwin short-circuit path).
//  2. Linux, no D-Bus, HOME set → *AgeStore  (age-file fallback rung).
//  3. Linux, no D-Bus, HOME unset → *EnvStore  (env-var rung, lowest tier).
//
// Each test asserts the concrete type returned, not just non-nil, which is what
// the existing integration tests check. AutoDetect is deliberately not called
// via Open() here — the task of detecting the right tier is separate from the
// task of opening it.

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/vedox/vedox/internal/secrets"
)

// TestAutoDetect_OnMacOS_ReturnsKeyringStore verifies that AutoDetect always
// selects *KeyringStore on macOS without consulting environment variables. The
// darwin short-circuit in AutoDetect must fire before any env or home-dir probe.
//
// Skipped on non-macOS: the darwin code path is unreachable on other platforms.
func TestAutoDetect_OnMacOS_ReturnsKeyringStore(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS-only: KeyringStore path is behind a runtime.GOOS == darwin guard")
	}

	// Isolate env so the test does not depend on whatever the developer has set.
	// AutoDetect returns before reading any of these on darwin, but clearing them
	// prevents accidental pass-through if the code ever changes.
	t.Setenv("VEDOX_AGE_PASSPHRASE_FILE", "")
	t.Setenv("VEDOX_AGE_PASSPHRASE", "")
	t.Setenv("VEDOX_HMAC_KEY_FILE", "")
	t.Setenv("VEDOX_HMAC_KEY", "")

	store, err := secrets.AutoDetect()
	if err != nil {
		t.Fatalf("AutoDetect: unexpected error on macOS: %v", err)
	}

	if _, ok := store.(*secrets.KeyringStore); !ok {
		t.Fatalf("AutoDetect on macOS: got %T, want *secrets.KeyringStore", store)
	}
}

// TestAutoDetect_WithPassphraseFile_ReturnsAgeStore verifies that AutoDetect
// selects the *AgeStore tier when:
//   - We are not on macOS (darwin always returns KeyringStore).
//   - The D-Bus Secret Service is unreachable (headless Linux / WSL2 / CI).
//   - The home directory is available (vedoxConfigDir succeeds).
//   - VEDOX_AGE_PASSPHRASE_FILE points to a file in a temp dir.
//
// Skipped on macOS because the darwin short-circuit fires before the age rung.
// Skipped on Linux when a D-Bus Secret Service IS available — in that case
// AutoDetect correctly chooses *KeyringStore and this branch is not exercised.
//
// AutoDetect does not call Open, so no scrypt work is performed and no
// passphrase file content is validated here — type selection is all we test.
func TestAutoDetect_WithPassphraseFile_ReturnsAgeStore(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("darwin always returns *KeyringStore; age rung is never reached")
	}

	// Write a passphrase file into a temp dir.  AutoDetect does not read the
	// file — it selects AgeStore because vedoxConfigDir() succeeds.  The file
	// must exist so that a caller who opens the store later can succeed, but its
	// contents are irrelevant to the type-selection logic under test.
	dir := t.TempDir()
	ppFile := dir + "/passphrase.txt"
	if err := writeFile(t, ppFile, "autodetect-age-branch-passphrase\n"); err != nil {
		t.Fatalf("write passphrase file: %v", err)
	}
	t.Setenv("VEDOX_AGE_PASSPHRASE_FILE", ppFile)
	t.Setenv("VEDOX_AGE_PASSPHRASE", "")
	t.Setenv("VEDOX_HMAC_KEY_FILE", "")
	t.Setenv("VEDOX_HMAC_KEY", "")

	store, err := secrets.AutoDetect()
	if err != nil {
		t.Fatalf("AutoDetect: %v", err)
	}

	switch store.(type) {
	case *secrets.AgeStore:
		// Correct — D-Bus was unavailable; age-file tier was selected.
	case *secrets.KeyringStore:
		// D-Bus Secret Service responded — this machine has a live session bus.
		// The AgeStore branch cannot be exercised here; skip rather than fail.
		t.Skip("D-Bus Secret Service is available on this host; *KeyringStore was selected — age branch not reachable in this environment")
	default:
		t.Fatalf("AutoDetect: got %T, want *secrets.AgeStore (or *secrets.KeyringStore when D-Bus is present)", store)
	}
}

// TestAutoDetect_WithHMACKeyOnly_ReturnsEnvStore verifies that AutoDetect
// selects *EnvStore — the lowest rung — when:
//   - We are not on macOS.
//   - The D-Bus Secret Service is unreachable.
//   - HOME is empty, making os.UserHomeDir() fail so vedoxConfigDir() fails.
//   - VEDOX_HMAC_KEY is set.
//
// Clearing HOME is the only reliable way to make the age rung unreachable
// without mocking unexported internals. os.UserHomeDir on Linux reads $HOME
// first; an empty $HOME causes it to return an error, which propagates through
// vedoxConfigDir and makes AutoDetect skip the age tier.
//
// Skipped on macOS for the same reason as the age test above.
// Skipped on Linux when D-Bus IS available — AutoDetect returns *KeyringStore
// before HOME is even consulted.
func TestAutoDetect_WithHMACKeyOnly_ReturnsEnvStore(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("darwin always returns *KeyringStore; env rung is never reached")
	}

	// Probe D-Bus availability before clearing HOME, so the skip message is
	// accurate. We use the same approach as probeDBusSecretService internally:
	// run dbus-send and observe the exit code.
	if dbusAvailable() {
		t.Skip("D-Bus Secret Service is available on this host; *KeyringStore would be selected — env rung not reachable in this environment")
	}

	// Force HOME to empty so os.UserHomeDir fails → vedoxConfigDir fails → age
	// rung is skipped → AutoDetect falls through to the env tier.
	t.Setenv("HOME", "")
	// Also clear XDG_CONFIG_HOME / USERPROFILE so go's home-dir lookup has no
	// fallback on unusual setups.
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("USERPROFILE", "")

	// Clear age vars so we are definitely at the env tier.
	t.Setenv("VEDOX_AGE_PASSPHRASE_FILE", "")
	t.Setenv("VEDOX_AGE_PASSPHRASE", "")
	t.Setenv("VEDOX_HMAC_KEY_FILE", "")
	t.Setenv("VEDOX_HMAC_KEY", strings.Repeat("de", 32)) // 64 hex chars = 32 bytes

	store, err := secrets.AutoDetect()
	if err != nil {
		t.Fatalf("AutoDetect: %v", err)
	}

	if _, ok := store.(*secrets.EnvStore); !ok {
		t.Fatalf("AutoDetect with HOME='' and VEDOX_HMAC_KEY set: got %T, want *secrets.EnvStore", store)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// writeFile writes content to path. It is a thin helper so test bodies stay
// under 10 lines of setup without depending on os.WriteFile directly.
func writeFile(t *testing.T, path, content string) error {
	t.Helper()
	return os.WriteFile(path, []byte(content), 0o600)
}

// dbusAvailable returns true when the org.freedesktop.secrets D-Bus service
// responds within a short timeout. It mirrors the logic in
// probeDBusSecretService (detect.go) so tests can gate themselves correctly
// without coupling to that unexported symbol.
func dbusAvailable() bool {
	cmd := exec.Command(
		"dbus-send",
		"--session",
		"--print-reply",
		"--dest=org.freedesktop.secrets",
		"/org/freedesktop/secrets",
		"org.freedesktop.DBus.Introspectable.Introspect",
	)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}
