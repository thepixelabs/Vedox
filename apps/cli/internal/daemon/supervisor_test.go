package daemon

import (
	"encoding/xml"
	"os"
	"runtime"
	"strings"
	"testing"
)

// ── plist template tests ──────────────────────────────────────────────────────

// xmlPlist is a minimal Go struct that mirrors just the keys we validate.
// We don't need a complete plist model — only what the tests assert.
type xmlPlist struct {
	XMLName xml.Name `xml:"plist"`
	Version string   `xml:"version,attr"`
}

// TestRenderPlist_ValidXML verifies that the rendered plist is well-formed XML.
func TestRenderPlist_ValidXML(t *testing.T) {
	content, err := RenderPlist("/usr/local/bin/vedox", "/home/user", false)
	if err != nil {
		t.Fatalf("RenderPlist returned error: %v", err)
	}

	// xml.Unmarshal validates structural well-formedness.
	var p xmlPlist
	if err := xml.Unmarshal(content, &p); err != nil {
		t.Fatalf("rendered plist is not valid XML: %v\nContent:\n%s", err, content)
	}
	if p.Version != "1.0" {
		t.Errorf("expected plist version 1.0, got %q", p.Version)
	}
}

// TestRenderPlist_ContainsBinaryPath verifies that the binary path is embedded.
func TestRenderPlist_ContainsBinaryPath(t *testing.T) {
	const wantBin = "/opt/myapp/bin/vedox"
	content, err := RenderPlist(wantBin, "/home/netzer", false)
	if err != nil {
		t.Fatalf("RenderPlist: %v", err)
	}
	if !strings.Contains(string(content), wantBin) {
		t.Errorf("binary path %q not found in plist:\n%s", wantBin, content)
	}
}

// TestRenderPlist_ContainsHome verifies that HOME substitution works.
func TestRenderPlist_ContainsHome(t *testing.T) {
	const wantHome = "/Users/pixelabs"
	content, err := RenderPlist("/usr/local/bin/vedox", wantHome, false)
	if err != nil {
		t.Fatalf("RenderPlist: %v", err)
	}
	if !strings.Contains(string(content), wantHome) {
		t.Errorf("home path %q not found in plist:\n%s", wantHome, content)
	}
}

// TestRenderPlist_RunAtLoadFalse verifies <false/> immediately follows the
// RunAtLoad key when autoStart is false.
func TestRenderPlist_RunAtLoadFalse(t *testing.T) {
	content, err := RenderPlist("/usr/local/bin/vedox", "/home/user", false)
	if err != nil {
		t.Fatalf("RenderPlist: %v", err)
	}
	plistStr := string(content)

	// Locate the RunAtLoad key then grab just the next line (the boolean value).
	keyStr := "<key>RunAtLoad</key>"
	idx := strings.Index(plistStr, keyStr)
	if idx < 0 {
		t.Fatal("RunAtLoad key not found in plist")
	}
	// The value is on the immediately following line (after trimming whitespace).
	afterKey := strings.TrimSpace(plistStr[idx+len(keyStr):])
	// afterKey starts with either <false/> or <true/> (with possible leading whitespace).
	if !strings.HasPrefix(afterKey, "<false/>") {
		t.Errorf("expected <false/> immediately after RunAtLoad key, got: %.80s", afterKey)
	}
}

// TestRenderPlist_RunAtLoadTrue verifies <true/> immediately follows the
// RunAtLoad key when autoStart is true.
func TestRenderPlist_RunAtLoadTrue(t *testing.T) {
	content, err := RenderPlist("/usr/local/bin/vedox", "/home/user", true)
	if err != nil {
		t.Fatalf("RenderPlist: %v", err)
	}
	plistStr := string(content)

	keyStr := "<key>RunAtLoad</key>"
	idx := strings.Index(plistStr, keyStr)
	if idx < 0 {
		t.Fatal("RunAtLoad key not found in plist")
	}
	afterKey := strings.TrimSpace(plistStr[idx+len(keyStr):])
	if !strings.HasPrefix(afterKey, "<true/>") {
		t.Errorf("expected <true/> immediately after RunAtLoad key, got: %.80s", afterKey)
	}
}

// TestRenderPlist_ContainsLabel verifies the launchd label is present.
func TestRenderPlist_ContainsLabel(t *testing.T) {
	content, err := RenderPlist("/usr/local/bin/vedox", "/home/user", false)
	if err != nil {
		t.Fatalf("RenderPlist: %v", err)
	}
	if !strings.Contains(string(content), LaunchdLabel) {
		t.Errorf("LaunchdLabel %q not found in plist", LaunchdLabel)
	}
}

// TestRenderPlist_ContainsForegroundFlag verifies --foreground is in ProgramArguments.
func TestRenderPlist_ContainsForegroundFlag(t *testing.T) {
	content, err := RenderPlist("/usr/local/bin/vedox", "/home/user", false)
	if err != nil {
		t.Fatalf("RenderPlist: %v", err)
	}
	if !strings.Contains(string(content), "--foreground") {
		t.Error("--foreground flag not found in plist ProgramArguments")
	}
}

// TestRenderPlist_ContainsSupervisedEnv verifies VEDOX_SUPERVISED=1 is set.
func TestRenderPlist_ContainsSupervisedEnv(t *testing.T) {
	content, err := RenderPlist("/usr/local/bin/vedox", "/home/user", false)
	if err != nil {
		t.Fatalf("RenderPlist: %v", err)
	}
	if !strings.Contains(string(content), "VEDOX_SUPERVISED") {
		t.Error("VEDOX_SUPERVISED env var not found in plist")
	}
}

// ── systemd unit template tests ───────────────────────────────────────────────

// TestRenderUnit_ContainsBinaryPath verifies the binary path is embedded.
func TestRenderUnit_ContainsBinaryPath(t *testing.T) {
	const wantBin = "/usr/local/bin/vedox"
	content, err := RenderUnit(wantBin, "/home/netzer")
	if err != nil {
		t.Fatalf("RenderUnit: %v", err)
	}
	if !strings.Contains(string(content), wantBin) {
		t.Errorf("binary path %q not found in unit:\n%s", wantBin, content)
	}
}

// TestRenderUnit_ContainsForegroundFlag verifies --foreground in ExecStart.
func TestRenderUnit_ContainsForegroundFlag(t *testing.T) {
	content, err := RenderUnit("/usr/local/bin/vedox", "/home/user")
	if err != nil {
		t.Fatalf("RenderUnit: %v", err)
	}
	if !strings.Contains(string(content), "--foreground") {
		t.Error("--foreground not found in unit ExecStart line")
	}
}

// TestRenderUnit_ContainsExecStartLine validates the ExecStart key appears.
func TestRenderUnit_ContainsExecStartLine(t *testing.T) {
	const bin = "/opt/vedox/bin/vedox"
	content, err := RenderUnit(bin, "/home/user")
	if err != nil {
		t.Fatalf("RenderUnit: %v", err)
	}
	if !strings.Contains(string(content), "ExecStart="+bin) {
		t.Errorf("ExecStart line not found in unit:\n%s", content)
	}
}

// TestRenderUnit_ContainsRestartOnFailure verifies crash-recovery policy.
func TestRenderUnit_ContainsRestartOnFailure(t *testing.T) {
	content, err := RenderUnit("/usr/local/bin/vedox", "/home/user")
	if err != nil {
		t.Fatalf("RenderUnit: %v", err)
	}
	if !strings.Contains(string(content), "Restart=on-failure") {
		t.Error("Restart=on-failure not found in unit file")
	}
}

// TestRenderUnit_ContainsInstallSection verifies [Install] section is present.
func TestRenderUnit_ContainsInstallSection(t *testing.T) {
	content, err := RenderUnit("/usr/local/bin/vedox", "/home/user")
	if err != nil {
		t.Fatalf("RenderUnit: %v", err)
	}
	if !strings.Contains(string(content), "[Install]") {
		t.Error("[Install] section not found in unit file")
	}
	if !strings.Contains(string(content), "WantedBy=default.target") {
		t.Error("WantedBy=default.target not found in unit file")
	}
}

// TestRenderUnit_ContainsHome verifies HOME substitution in the unit.
func TestRenderUnit_ContainsHome(t *testing.T) {
	const wantHome = "/home/pixelabs"
	content, err := RenderUnit("/usr/local/bin/vedox", wantHome)
	if err != nil {
		t.Fatalf("RenderUnit: %v", err)
	}
	if !strings.Contains(string(content), wantHome) {
		t.Errorf("home path %q not found in unit file:\n%s", wantHome, content)
	}
}

// TestRenderUnit_ContainsSupervisedEnv verifies VEDOX_SUPERVISED env is set.
func TestRenderUnit_ContainsSupervisedEnv(t *testing.T) {
	content, err := RenderUnit("/usr/local/bin/vedox", "/home/user")
	if err != nil {
		t.Fatalf("RenderUnit: %v", err)
	}
	if !strings.Contains(string(content), "VEDOX_SUPERVISED") {
		t.Error("VEDOX_SUPERVISED env var not found in unit file")
	}
}

// TestRenderUnit_NoTemplateVariablesRemain verifies no unrendered {{.}} markers.
func TestRenderUnit_NoTemplateVariablesRemain(t *testing.T) {
	content, err := RenderUnit("/usr/local/bin/vedox", "/home/user")
	if err != nil {
		t.Fatalf("RenderUnit: %v", err)
	}
	if strings.Contains(string(content), "{{") || strings.Contains(string(content), "}}") {
		t.Errorf("unrendered template markers found in unit file:\n%s", content)
	}
}

// TestRenderPlist_NoTemplateVariablesRemain verifies no unrendered {{.}} markers.
func TestRenderPlist_NoTemplateVariablesRemain(t *testing.T) {
	content, err := RenderPlist("/usr/local/bin/vedox", "/home/user", true)
	if err != nil {
		t.Fatalf("RenderPlist: %v", err)
	}
	if strings.Contains(string(content), "{{") || strings.Contains(string(content), "}}") {
		t.Errorf("unrendered template markers found in plist:\n%s", content)
	}
}

// ── DetectSupervisor tests ────────────────────────────────────────────────────

// TestDetectSupervisor_CurrentOS verifies DetectSupervisor returns a known value.
// We can only assert the current OS; we don't mock runtime.GOOS because it is
// a constant, not a variable. The test documents expected behaviour per platform.
func TestDetectSupervisor_CurrentOS(t *testing.T) {
	got := DetectSupervisor()
	switch runtime.GOOS {
	case "darwin":
		if got != "launchd" {
			t.Errorf("on darwin expected DetectSupervisor()=%q, got %q", "launchd", got)
		}
	case "linux":
		// On Linux we expect either "systemd" (if systemctl is on PATH) or "none".
		if got != "systemd" && got != "none" {
			t.Errorf("on linux expected DetectSupervisor() to be %q or %q, got %q", "systemd", "none", got)
		}
	default:
		if got != "none" {
			t.Errorf("on %s expected DetectSupervisor()=%q, got %q", runtime.GOOS, "none", got)
		}
	}
}

// ── Path helper tests ─────────────────────────────────────────────────────────

// TestLaunchdPlistPath_ContainsLabel verifies the plist path embeds the label.
func TestLaunchdPlistPath_ContainsLabel(t *testing.T) {
	path, err := LaunchdPlistPath()
	if err != nil {
		t.Fatalf("LaunchdPlistPath: %v", err)
	}
	if !strings.Contains(path, LaunchdLabel) {
		t.Errorf("LaunchdPlistPath %q does not contain label %q", path, LaunchdLabel)
	}
	if !strings.HasSuffix(path, ".plist") {
		t.Errorf("LaunchdPlistPath %q does not end with .plist", path)
	}
}

// TestSystemdUnitPath_ContainsUnit verifies the systemd unit path embeds the unit name.
func TestSystemdUnitPath_ContainsUnit(t *testing.T) {
	path, err := SystemdUnitPath()
	if err != nil {
		t.Fatalf("SystemdUnitPath: %v", err)
	}
	if !strings.Contains(path, SystemdUnit) {
		t.Errorf("SystemdUnitPath %q does not contain %q", path, SystemdUnit)
	}
}

// TestSystemdUnitPath_RespectsXDGConfigHome verifies XDG_CONFIG_HOME is honoured.
func TestSystemdUnitPath_RespectsXDGConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	path, err := SystemdUnitPath()
	if err != nil {
		t.Fatalf("SystemdUnitPath: %v", err)
	}
	if !strings.HasPrefix(path, tmp) {
		t.Errorf("SystemdUnitPath %q should be under XDG_CONFIG_HOME %q", path, tmp)
	}
}

// ── writeFileAtomic tests ─────────────────────────────────────────────────────

// TestWriteFileAtomic_CreatesFile verifies the helper writes the expected content.
func TestWriteFileAtomic_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	target := dir + "/output.txt"
	wantContent := []byte("hello supervisor")

	if err := writeFileAtomic(target, wantContent, 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(wantContent) {
		t.Errorf("file content mismatch: got %q, want %q", got, wantContent)
	}
}

// TestWriteFileAtomic_Overwrites verifies that a second write replaces content.
func TestWriteFileAtomic_Overwrites(t *testing.T) {
	dir := t.TempDir()
	target := dir + "/output.txt"

	if err := writeFileAtomic(target, []byte("first"), 0o644); err != nil {
		t.Fatalf("first writeFileAtomic: %v", err)
	}
	if err := writeFileAtomic(target, []byte("second"), 0o644); err != nil {
		t.Fatalf("second writeFileAtomic: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "second" {
		t.Errorf("overwrite failed: got %q", got)
	}
}
