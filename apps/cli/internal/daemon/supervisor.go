// Package daemon — supervisor.go
//
// Install and uninstall the Vedox daemon as a supervised OS service.
//
// Supported supervisors:
//   - launchd  (macOS) — LaunchAgent plist at ~/Library/LaunchAgents/sh.pixelabs.vedoxd.plist
//   - systemd  (Linux) — user unit at ~/.config/systemd/user/vedoxd.service
//
// Templates follow the authoritative specifications from the WS-A daemon
// lifecycle spec (§2.1 plist, §3.1 unit file). Both use os.Executable() to
// embed the current binary path at install time — no shell expansion at
// runtime.
//
// Public API:
//
//	DetectSupervisor() string
//	InstallLaunchd(binaryPath string, autoStart bool, force bool) error
//	UninstallLaunchd() error
//	InstallSystemd(binaryPath string, autoStart bool, force bool) error
//	UninstallSystemd() error

package daemon

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
)

// ── Label / service-name constants ───────────────────────────────────────────

const (
	// LaunchdLabel is the CFBundleIdentifier-style label used in the plist and
	// in all launchctl commands. Matches §2.1.
	LaunchdLabel = "sh.pixelabs.vedoxd"

	// SystemdUnit is the .service file name. Matches §3.1.
	SystemdUnit = "vedoxd.service"
)

// ── Path helpers ──────────────────────────────────────────────────────────────

// LaunchdPlistPath returns the canonical plist install path for the current
// user. launchd does not expand "~" — we use the absolute path.
func LaunchdPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, "Library", "LaunchAgents", LaunchdLabel+".plist"), nil
}

// SystemdUnitPath returns the canonical unit file install path for the current
// user. Follows XDG_CONFIG_HOME or falls back to ~/.config.
func SystemdUnitPath() (string, error) {
	cfgDir := os.Getenv("XDG_CONFIG_HOME")
	if cfgDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		cfgDir = filepath.Join(home, ".config")
	}
	return filepath.Join(cfgDir, "systemd", "user", SystemdUnit), nil
}

// ── Supervisor detection ──────────────────────────────────────────────────────

// DetectSupervisor returns "launchd", "systemd", or "none".
//
// Detection is OS-only — it does not check whether the service is actually
// installed. Callers use this to decide which install path to execute.
func DetectSupervisor() string {
	switch runtime.GOOS {
	case "darwin":
		return "launchd"
	case "linux":
		// Check for a running systemd user session.
		if _, err := exec.LookPath("systemctl"); err == nil {
			return "systemd"
		}
		return "none"
	default:
		return "none"
	}
}

// ── plist template (§2.1) ─────────────────────────────────────────────────────

// launchdData is the template context for the launchd plist.
type launchdData struct {
	BinaryPath string // absolute path to the vedox binary
	Home       string // $HOME (absolute, no trailing slash)
	RunAtLoad  bool   // true when --auto-start / --enable is passed
}

// plistTmpl is the authoritative launchd plist from spec §2.1. Template
// variables: {{.BinaryPath}}, {{.Home}}, {{.RunAtLoad}}.
//
// Note: text/template uses {{if .RunAtLoad}}<true/>{{else}}<false/>{{end}}
// to emit the correct XML boolean. We deliberately do NOT use html/template
// because plist files are XML but we need no HTML escaping of the binary path
// (angle brackets / ampersands in paths are already invalid on macOS).
var plistTmpl = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>sh.pixelabs.vedoxd</string>

    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryPath}}</string>
        <string>server</string>
        <string>start</string>
        <string>--foreground</string>
    </array>

    <!-- Lifecycle -->
    <key>RunAtLoad</key>
    {{if .RunAtLoad}}<true/>{{else}}<false/>{{end}}
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key><false/>
        <key>Crashed</key><true/>
    </dict>
    <key>ThrottleInterval</key>
    <integer>10</integer>
    <key>ExitTimeOut</key>
    <integer>60</integer>
    <key>ProcessType</key>
    <string>Interactive</string>

    <!-- Working directory + environment -->
    <key>WorkingDirectory</key>
    <string>{{.Home}}/.vedox</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>VEDOX_HOME</key><string>{{.Home}}/.vedox</string>
        <key>VEDOX_SUPERVISED</key><string>1</string>
        <key>PATH</key><string>/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin</string>
    </dict>

    <!-- Stdout / stderr: captured by launchd, rotated by lumberjack inside daemon -->
    <key>StandardOutPath</key>
    <string>{{.Home}}/.vedox/logs/vedoxd.out</string>
    <key>StandardErrorPath</key>
    <string>{{.Home}}/.vedox/logs/vedoxd.err</string>

    <!-- Resource hygiene -->
    <key>SoftResourceLimits</key>
    <dict>
        <key>NumberOfFiles</key><integer>4096</integer>
    </dict>
    <key>HardResourceLimits</key>
    <dict>
        <key>NumberOfFiles</key><integer>8192</integer>
    </dict>
</dict>
</plist>
`))

// RenderPlist renders the launchd plist template into a byte slice.
// This is exported so tests can validate the output without touching the
// filesystem or running launchctl.
func RenderPlist(binaryPath, home string, runAtLoad bool) ([]byte, error) {
	data := launchdData{
		BinaryPath: binaryPath,
		Home:       home,
		RunAtLoad:  runAtLoad,
	}
	var buf bytes.Buffer
	if err := plistTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("rendering plist template: %w", err)
	}
	return buf.Bytes(), nil
}

// ── systemd unit template (§3.1) ──────────────────────────────────────────────

// systemdData is the template context for the systemd unit file.
type systemdData struct {
	BinaryPath string // absolute path to the vedox binary
	Home       string // $HOME (absolute, no trailing slash)
}

// unitTmpl is the authoritative systemd unit from spec §3.1.
// We substitute the actual binary path (from os.Executable) instead of the
// %h/.local/bin/vedox placeholder, so users who install to /usr/local/bin get
// the correct path automatically.
var unitTmpl = template.Must(template.New("unit").Parse(`[Unit]
Description=Vedox documentation daemon
Documentation=https://vedox.pixelabs.sh/docs
After=network.target

[Service]
Type=simple

ExecStart={{.BinaryPath}} server start --foreground

Restart=on-failure
RestartSec=5
StartLimitInterval=60
StartLimitBurst=5

StandardOutput=journal
StandardError=journal
SyslogIdentifier=vedoxd

RuntimeDirectory=vedox
RuntimeDirectoryMode=0700

Environment="VEDOX_HOME={{.Home}}/.vedox"
Environment="VEDOX_SUPERVISED=1"

LimitNOFILE=4096

NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=read-write
ReadWritePaths={{.Home}}

[Install]
WantedBy=default.target
`))

// RenderUnit renders the systemd unit template into a byte slice.
// Exported for testing.
func RenderUnit(binaryPath, home string) ([]byte, error) {
	data := systemdData{
		BinaryPath: binaryPath,
		Home:       home,
	}
	var buf bytes.Buffer
	if err := unitTmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("rendering unit template: %w", err)
	}
	return buf.Bytes(), nil
}

// ── launchd install / uninstall ───────────────────────────────────────────────

// InstallLaunchd writes the launchd plist and bootstraps the LaunchAgent.
//
//   - binaryPath: absolute path to the vedox binary (from os.Executable).
//   - autoStart:  if true, sets RunAtLoad=true and immediately bootstraps with
//     launchctl bootstrap + kickstart so the daemon starts now and on every
//     subsequent login.
//   - force:      overwrite an existing plist without prompting.
//
// On macOS ≥ 10.10 the preferred commands are `launchctl bootstrap` /
// `launchctl bootout`; the deprecated `launchctl load` / `unload` are NOT used
// per spec §2.3.
func InstallLaunchd(binaryPath string, autoStart bool, force bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	plistPath, err := LaunchdPlistPath()
	if err != nil {
		return err
	}

	// Guard: plist already exists and --force not set.
	if _, statErr := os.Stat(plistPath); statErr == nil && !force {
		return fmt.Errorf(
			"plist already exists at %s — use --force to overwrite, or run 'vedox server uninstall' first",
			plistPath,
		)
	}

	// Render the plist.
	plistContent, err := RenderPlist(binaryPath, home, autoStart)
	if err != nil {
		return err
	}

	// Ensure the LaunchAgents directory exists (it always should, but be safe).
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return fmt.Errorf("cannot create LaunchAgents directory: %w", err)
	}

	// Write the plist atomically (temp → rename so launchd never sees a partial file).
	if err := writeFileAtomic(plistPath, plistContent, 0o644); err != nil {
		return fmt.Errorf("cannot write plist: %w", err)
	}

	fmt.Printf("plist written to %s\n", plistPath)

	if autoStart {
		// Bootstrap the agent so it starts now and on every login.
		uid := fmt.Sprintf("%d", os.Getuid())
		domain := "gui/" + uid

		if err := runCmd("launchctl", "bootstrap", domain, plistPath); err != nil {
			return fmt.Errorf("launchctl bootstrap failed: %w", err)
		}

		// Kickstart (-k = kill-and-restart if already running) immediately.
		serviceTarget := domain + "/" + LaunchdLabel
		if err := runCmd("launchctl", "kickstart", "-k", serviceTarget); err != nil {
			// Non-fatal: the agent will start on next login even if kickstart fails.
			fmt.Fprintf(os.Stderr, "warning: launchctl kickstart failed (daemon will start on next login): %v\n", err)
		}

		fmt.Println("vedox daemon registered with launchd and started.")
		fmt.Printf("  stop:      vedox server stop\n")
		fmt.Printf("  uninstall: vedox server uninstall\n")
	} else {
		fmt.Println("vedox daemon registered with launchd (RunAtLoad=false).")
		fmt.Printf("  start now: vedox server start\n")
		fmt.Printf("  auto-start on login: vedox server install --auto-start\n")
		fmt.Printf("  uninstall: vedox server uninstall\n")
	}

	return nil
}

// UninstallLaunchd unloads and removes the launchd LaunchAgent.
//
// It uses `launchctl bootout` (the modern API). If the agent is not loaded,
// bootout returns an error which we treat as a soft warning — the plist
// removal still proceeds.
func UninstallLaunchd() error {
	plistPath, err := LaunchdPlistPath()
	if err != nil {
		return err
	}

	uid := fmt.Sprintf("%d", os.Getuid())
	serviceTarget := "gui/" + uid + "/" + LaunchdLabel

	// Attempt to unload. Non-fatal if not currently loaded.
	if err := runCmd("launchctl", "bootout", serviceTarget); err != nil {
		fmt.Fprintf(os.Stderr,
			"warning: launchctl bootout failed (agent may not be loaded): %v\n", err)
	}

	// Remove the plist file.
	if err := os.Remove(plistPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "warning: plist not found at %s (already removed?)\n", plistPath)
		} else {
			return fmt.Errorf("cannot remove plist: %w", err)
		}
	} else {
		fmt.Printf("removed %s\n", plistPath)
	}

	// Clean up the PID file if it exists.
	if err := cleanupPIDIfExists(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not remove PID file: %v\n", err)
	}

	fmt.Println("vedox daemon uninstalled from launchd.")
	return nil
}

// ── systemd install / uninstall ───────────────────────────────────────────────

// InstallSystemd writes the systemd user unit and enables it.
//
//   - binaryPath: absolute path to the vedox binary (from os.Executable).
//   - autoStart:  if true, `systemctl --user enable --now vedoxd.service` is
//     run so the daemon starts immediately and on every future login.
//   - force:      overwrite an existing unit file without prompting.
func InstallSystemd(binaryPath string, autoStart bool, force bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	unitPath, err := SystemdUnitPath()
	if err != nil {
		return err
	}

	// Guard: unit already exists and --force not set.
	if _, statErr := os.Stat(unitPath); statErr == nil && !force {
		return fmt.Errorf(
			"unit file already exists at %s — use --force to overwrite, or run 'vedox server uninstall' first",
			unitPath,
		)
	}

	// Render the unit.
	unitContent, err := RenderUnit(binaryPath, home)
	if err != nil {
		return err
	}

	// Ensure the systemd user unit directory exists.
	if err := os.MkdirAll(filepath.Dir(unitPath), 0o755); err != nil {
		return fmt.Errorf("cannot create systemd user unit directory: %w", err)
	}

	// Write atomically.
	if err := writeFileAtomic(unitPath, unitContent, 0o644); err != nil {
		return fmt.Errorf("cannot write unit file: %w", err)
	}

	fmt.Printf("unit file written to %s\n", unitPath)

	// Always daemon-reload so systemd picks up the new unit.
	if err := runCmd("systemctl", "--user", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl --user daemon-reload failed: %w", err)
	}

	if autoStart {
		// Enable AND start immediately.
		if err := runCmd("systemctl", "--user", "enable", "--now", SystemdUnit); err != nil {
			return fmt.Errorf("systemctl --user enable --now failed: %w", err)
		}
		fmt.Println("vedox daemon registered with systemd and started.")
	} else {
		// Enable for login-start only (don't start now).
		if err := runCmd("systemctl", "--user", "enable", SystemdUnit); err != nil {
			return fmt.Errorf("systemctl --user enable failed: %w", err)
		}
		fmt.Println("vedox daemon registered with systemd (enabled for login-start).")
		fmt.Printf("  start now: vedox server start\n")
	}

	fmt.Printf("  stop:      vedox server stop\n")
	fmt.Printf("  uninstall: vedox server uninstall\n")
	fmt.Printf("\ntip: run 'loginctl enable-linger $USER' to keep the daemon running without an active login session\n")

	return nil
}

// UninstallSystemd disables and removes the systemd user unit.
func UninstallSystemd() error {
	unitPath, err := SystemdUnitPath()
	if err != nil {
		return err
	}

	// Stop the service (non-fatal if not running).
	if err := runCmd("systemctl", "--user", "stop", SystemdUnit); err != nil {
		fmt.Fprintf(os.Stderr, "warning: systemctl --user stop failed (may not be running): %v\n", err)
	}

	// Disable so it won't start on next login.
	if err := runCmd("systemctl", "--user", "disable", SystemdUnit); err != nil {
		fmt.Fprintf(os.Stderr, "warning: systemctl --user disable failed: %v\n", err)
	}

	// Remove the unit file.
	if err := os.Remove(unitPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "warning: unit file not found at %s (already removed?)\n", unitPath)
		} else {
			return fmt.Errorf("cannot remove unit file: %w", err)
		}
	} else {
		fmt.Printf("removed %s\n", unitPath)
	}

	// Final daemon-reload so systemd forgets the unit.
	if err := runCmd("systemctl", "--user", "daemon-reload"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: systemctl --user daemon-reload failed: %v\n", err)
	}

	// Clean up the PID file if it exists.
	if err := cleanupPIDIfExists(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not remove PID file: %v\n", err)
	}

	fmt.Println("vedox daemon uninstalled from systemd.")
	return nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

// writeFileAtomic writes content to path using a temp-file + rename pattern.
// This ensures launchd/systemd never reads a partially-written file.
func writeFileAtomic(path string, content []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".vedox-supervisor-*")
	if err != nil {
		return fmt.Errorf("cannot create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("cannot write to temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("cannot close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("cannot chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("cannot rename temp file to %s: %w", path, err)
	}
	return nil
}

// runCmd runs an external command and returns a combined-output error on failure.
// We capture stderr in the error message so callers get actionable context.
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, msg)
		}
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

// cleanupPIDIfExists removes the PID file from the default VedoxHome if it
// exists. Errors are non-fatal during uninstall — we do best-effort cleanup.
func cleanupPIDIfExists() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	vedoxHome := filepath.Join(home, ".vedox")
	p := NewPaths(vedoxHome)
	if err := os.Remove(p.PIDFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
