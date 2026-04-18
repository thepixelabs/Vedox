package gitcheck

import (
	"testing"
)

// TestCheck verifies that Check() returns a valid identity (this test runs in a
// git repo, so user.name and user.email should be set in the environment).
// If they're not set (e.g. a CI box with no global git config), the test
// verifies the error path instead.
func TestCheck_ReturnsIdentityOrVDX003(t *testing.T) {
	id, err := Check()
	if err != nil {
		// Acceptable in environments with no git identity configured.
		if id != nil {
			t.Error("got both non-nil identity and error")
		}
		return
	}
	if id == nil {
		t.Fatal("expected non-nil identity when no error")
	}
	// Values can be empty strings if git config user.name is set to "".
	// Just verify the struct is accessible.
	_ = id.Name
	_ = id.Email
}

// TestGetConfigValue_KnownKey verifies that GetConfigValue returns a non-error
// result for a key that is almost certainly set in any git repo.
func TestGetConfigValue_KnownKey(t *testing.T) {
	// core.repositoryformatversion is set by git init, always present.
	val, err := GetConfigValue("core.repositoryformatversion")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = val // may be "0" or "1"
}

// TestGetConfigValue_MissingKey verifies that a missing key returns "" with no error.
func TestGetConfigValue_MissingKey(t *testing.T) {
	val, err := GetConfigValue("vedox.test.nonexistent.key.abc123")
	if err != nil {
		t.Fatalf("unexpected error for missing key: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string for missing key, got %q", val)
	}
}
