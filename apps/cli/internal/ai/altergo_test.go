package ai

// Tests for altergo.go — AccountHome, validAccountName, and DiscoverAltergo.
// These reach the HTTP layer (POST /api/ai/generate-names) as user-supplied
// account names. Path-traversal defence is a security-critical path.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestValidAccountName_Accepts exercises the positive character-policy cases.
func TestValidAccountName_Accepts(t *testing.T) {
	good := []string{
		"pocus",
		"netz-main",
		"team_alpha",
		"v1.2.3",
		"A1B2",
	}
	for _, name := range good {
		if !validAccountName(name) {
			t.Errorf("validAccountName(%q) = false, want true", name)
		}
	}
}

// TestValidAccountName_Rejects is the path-traversal defence contract. A
// regression here opens up HOME= rewrites to arbitrary locations when the
// daemon spawns an AI CLI.
func TestValidAccountName_Rejects(t *testing.T) {
	bad := []struct {
		name   string
		reason string
	}{
		{"", "empty"},
		{".", "single dot"},
		{"..", "parent dir"},
		{"../etc", "explicit traversal"},
		{"foo/bar", "forward slash"},
		{`foo\bar`, "backslash"},
		{"foo bar", "space"},
		{"foo;rm", "semicolon"},
		{"foo$", "shell metachar"},
		{"foo\x00bar", "null byte"},
		{"hello world", "whitespace"},
	}
	for _, c := range bad {
		if validAccountName(c.name) {
			t.Errorf("validAccountName(%q) = true, want false (%s)", c.name, c.reason)
		}
	}

	// Length cap.
	long := make([]byte, 129)
	for i := range long {
		long[i] = 'a'
	}
	if validAccountName(string(long)) {
		t.Errorf("validAccountName(129-char string) = true, want false")
	}
}

// TestAccountHome_Valid returns a path under ~/.altergo/accounts/<name>.
// The returned path must live under the user's home directory — a regression
// where os.UserHomeDir is bypassed would show up here.
func TestAccountHome_Valid(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no user home available: %v", err)
	}

	got := AccountHome("pocus")
	want := filepath.Join(home, ".altergo", "accounts", "pocus")
	if got != want {
		t.Errorf("AccountHome(pocus) = %q, want %q", got, want)
	}
}

// TestAccountHome_InvalidNameReturnsEmpty is the security contract: if the
// name fails the validator, AccountHome returns "" rather than a traversed
// path. Callers treat "" as "not found" and refuse to spawn the AI CLI.
func TestAccountHome_InvalidNameReturnsEmpty(t *testing.T) {
	for _, name := range []string{"", "..", "../etc", "a/b", "x;y"} {
		if got := AccountHome(name); got != "" {
			t.Errorf("AccountHome(%q) = %q, want empty (invalid name)", name, got)
		}
	}
}

// TestDiscoverAltergo_MissingHome never returns an error and marks the
// available flag false when ~/.altergo/accounts does not exist. Callers rely
// on this function never panicking even in CI environments with no HOME.
func TestDiscoverAltergo_MissingHome(t *testing.T) {
	// Point HOME at a temp dir that doesn't have ~/.altergo/accounts.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	info := DiscoverAltergo()
	if info.Available {
		t.Errorf("Available=true, want false (no ~/.altergo/accounts)")
	}
	if len(info.Accounts) != 0 {
		t.Errorf("Accounts=%v, want empty", info.Accounts)
	}
}

// TestDiscoverAltergo_WithAccounts sets up a fake ~/.altergo layout and
// verifies both branches of the metadata-parse logic:
//   1. Valid account.json -> providers populated
//   2. Missing account.json -> account included with nil providers
//   3. Malformed account.json -> account included with nil providers
func TestDiscoverAltergo_WithAccounts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	base := filepath.Join(home, ".altergo", "accounts")
	// Account 1: valid metadata
	validMeta := accountMeta{Version: 1, Providers: []string{"claude", "codex"}}
	if err := writeAccount(base, "alpha", &validMeta); err != nil {
		t.Fatalf("setup alpha: %v", err)
	}
	// Account 2: directory only, no account.json
	if err := os.MkdirAll(filepath.Join(base, "beta"), 0o755); err != nil {
		t.Fatalf("setup beta: %v", err)
	}
	// Account 3: malformed JSON
	malformedDir := filepath.Join(base, "gamma")
	if err := os.MkdirAll(malformedDir, 0o755); err != nil {
		t.Fatalf("setup gamma dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(malformedDir, "account.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("setup gamma json: %v", err)
	}
	// Non-directory file in the accounts dir — must be ignored.
	if err := os.WriteFile(filepath.Join(base, "scratch.txt"), []byte("x"), 0o600); err != nil {
		t.Fatalf("setup scratch: %v", err)
	}

	info := DiscoverAltergo()
	if !info.Available {
		t.Fatalf("Available=false, want true")
	}
	if len(info.Accounts) != 3 {
		t.Fatalf("got %d accounts, want 3: %+v", len(info.Accounts), info.Accounts)
	}

	byName := map[string]AccountInfo{}
	for _, a := range info.Accounts {
		byName[a.Name] = a
	}
	if alpha, ok := byName["alpha"]; !ok {
		t.Errorf("alpha missing")
	} else if len(alpha.Providers) != 2 || alpha.Providers[0] != "claude" {
		t.Errorf("alpha.Providers = %v, want [claude codex]", alpha.Providers)
	}
	if beta, ok := byName["beta"]; !ok {
		t.Errorf("beta missing")
	} else if beta.Providers != nil {
		t.Errorf("beta.Providers = %v, want nil (no metadata)", beta.Providers)
	}
	if gamma, ok := byName["gamma"]; !ok {
		t.Errorf("gamma missing")
	} else if gamma.Providers != nil {
		t.Errorf("gamma.Providers = %v, want nil (malformed json)", gamma.Providers)
	}
}

// writeAccount creates <base>/<name>/account.json with the given metadata.
func writeAccount(base, name string, meta *accountMeta) error {
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "account.json"), data, 0o600)
}
