package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// TestVersionDefault confirms the default version string is the expected
// alpha release identifier rather than the dev placeholder. This test
// catches accidental resets of the version constant before a tag is cut.
func TestVersionDefault(t *testing.T) {
	const want = "v0.1.0-alpha.1"
	if version != want {
		t.Errorf("version = %q, want %q", version, want)
	}
}

// TestVersionCmd_Output confirms vedox version prints all three fields
// (version, commit, built) in the expected format on stdout.
func TestVersionCmd_Output(t *testing.T) {
	buf := &bytes.Buffer{}
	versionCmd.SetOut(buf)
	versionCmd.SetErr(buf)
	versionCmd.Run(versionCmd, nil)

	got := buf.String()
	if !strings.Contains(got, "v0.1.0-alpha.1") {
		t.Errorf("versionCmd output missing version string; got: %q", got)
	}
	if !strings.Contains(got, "commit") {
		t.Errorf("versionCmd output missing 'commit' label; got: %q", got)
	}
	if !strings.Contains(got, "built") {
		t.Errorf("versionCmd output missing 'built' label; got: %q", got)
	}
}
