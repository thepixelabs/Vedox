package ai

// Re-audit tests for AccountHome path traversal guard.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAccountHome_RejectsTraversal confirms that account names containing
// ".." or path separators are rejected (returns ""). Without this guard an
// HTTP-layer attacker could set HOME= anywhere on disk when we exec the AI
// CLI.
func TestAccountHome_RejectsTraversal(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}

	cases := []string{
		"../../../etc",
		"..",
		".",
		"",
		"/etc/passwd",
		"a/b",
		`a\b`,
		"good/../../bad",
		strings.Repeat("a", 129),       // too long
		"weird char \x00 null",          // control char
		"colon:is:special",              // excluded char
	}
	for _, name := range cases {
		got := AccountHome(name)
		if got != "" {
			t.Errorf("AccountHome(%q) = %q, want empty string", name, got)
		}
	}

	// Sanity: a valid account name still works and yields a path inside
	// ~/.altergo/accounts.
	if got := AccountHome("alice"); got == "" {
		t.Errorf("AccountHome(\"alice\") returned empty for a valid name")
	} else if !strings.HasPrefix(got, filepath.Join(home, ".altergo", "accounts")) {
		t.Errorf("AccountHome(\"alice\") = %q, expected under ~/.altergo/accounts", got)
	}
}

// TestAccountHome_AcceptsDashedUnderscored confirms the allowlist includes
// normal identifier chars real users actually use.
func TestAccountHome_AcceptsDashedUnderscored(t *testing.T) {
	for _, name := range []string{"alice", "bob-test", "user_1", "net.altergo", "A1"} {
		if AccountHome(name) == "" {
			t.Errorf("valid name %q rejected", name)
		}
	}
}
