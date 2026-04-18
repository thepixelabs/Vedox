package secrets

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// dBusProbeTimeout is the maximum time AutoDetect waits for the D-Bus Secret
// Service to respond on Linux. go-keyring can block for 30+ seconds on a
// headless machine; we short-circuit with a tight deadline.
const dBusProbeTimeout = 500 * time.Millisecond

// AutoDetect selects the highest available SecretStore tier for the current
// platform and deployment environment, following the ladder defined in §2 of
// the design doc:
//
//  1. OS keychain (macOS Keychain / Linux D-Bus Secret Service) via go-keyring.
//  2. age-encrypted ~/.vedox/secrets.age (headless Linux / WSL2).
//  3. Env-file / bare env var (VEDOX_HMAC_KEY_FILE / VEDOX_HMAC_KEY).
//
// AutoDetect does NOT call AgeStore.Open or EnvStore.Open — the caller is
// responsible for calling Open on the returned store before use. This keeps
// passphrase prompting under the caller's control (e.g., the daemon startup
// sequence).
//
// Returns (nil, error) with the VDX-D04 message when no backend is available.
func AutoDetect() (SecretStore, error) {
	// macOS — always prefer the OS Keychain. The go-keyring probe is fast
	// (synchronous IPC to securityd).
	if runtime.GOOS == "darwin" {
		ks := NewKeyringStore()
		slog.Debug("secrets: using OS Keychain (macOS)")
		return ks, nil
	}

	// Linux — probe D-Bus Secret Service first. If reachable, use go-keyring.
	// If not (headless server, WSL2, container without dbus), fall through.
	if runtime.GOOS == "linux" {
		if probeDBusSecretService() {
			ks := NewKeyringStore()
			slog.Debug("secrets: using D-Bus Secret Service (libsecret)")
			return ks, nil
		}
		slog.Debug("secrets: D-Bus Secret Service unavailable; trying age-file fallback")
	}

	// age-file fallback — available whenever the config dir is writable and
	// a passphrase can be resolved (the Open call handles that).
	configDir, err := vedoxConfigDir()
	if err == nil {
		as := NewAgeStore(configDir)
		slog.Debug("secrets: using age-encrypted file", "path", filepath.Join(configDir, ageSecretsFileName))
		return as, nil
	}

	// Env-store fallback — only when at least one of the env vars is set.
	if os.Getenv("VEDOX_HMAC_KEY_FILE") != "" || os.Getenv("VEDOX_HMAC_KEY") != "" {
		es := NewEnvStore()
		slog.Debug("secrets: using env-var store (dev/container fallback)")
		return es, nil
	}

	// Nothing works.
	return nil, fmt.Errorf(
		"VDX-D04: no secret storage backend available. " +
			"On macOS / desktop Linux: ensure the keychain service is running. " +
			"On headless Linux: set VEDOX_AGE_PASSPHRASE_FILE or VEDOX_AGE_PASSPHRASE. " +
			"For dev/container: set VEDOX_HMAC_KEY_FILE or VEDOX_HMAC_KEY. " +
			"See https://vedox.pixelabs.sh/docs/deploy/secret-storage",
	)
}

// probeDBusSecretService checks whether org.freedesktop.secrets is reachable
// on the current D-Bus session bus within dBusProbeTimeout. Returns true only
// when the service responds successfully.
//
// We use dbus-send rather than the godbus library to keep the probe outside
// the go-keyring hot path — go-keyring will be used for actual secret I/O.
// If dbus-send is not installed the probe returns false, triggering the age
// fallback (correct behaviour on a stripped container image).
func probeDBusSecretService() bool {
	ctx, cancel := context.WithTimeout(context.Background(), dBusProbeTimeout)
	defer cancel()

	// dbus-send exits 0 when the service responds, non-zero otherwise.
	cmd := exec.CommandContext(ctx,
		"dbus-send",
		"--session",
		"--print-reply",
		"--dest=org.freedesktop.secrets",
		"/org/freedesktop/secrets",
		"org.freedesktop.DBus.Introspectable.Introspect",
	)
	// Suppress all output — we care only about the exit code.
	cmd.Stdout = nil
	cmd.Stderr = nil

	err := cmd.Run()
	return err == nil
}

// vedoxConfigDir returns the absolute path of the ~/.vedox/ directory.
// Returns an error only when the home directory cannot be determined — on
// Linux this is an extremely rare condition (no HOME and no /etc/passwd entry).
func vedoxConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".vedox"), nil
}
