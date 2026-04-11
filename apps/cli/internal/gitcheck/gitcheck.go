// Package gitcheck validates the local Git identity before Vedox operations
// that require authoring commits (i.e., Publish).
//
// We shell out to `git config` rather than parsing .gitconfig ourselves.
// This respects the full Git config resolution order (system → global → local)
// and means we behave identically to what `git commit` would see.
//
// On first `vedox dev`, if either user.email or user.name is unset, we fail
// with VDX-003 and print instructions. This is intentional: a silent fallback
// (e.g., using the OS username) would produce commits that can't be pushed to
// remotes with identity policies, causing a confusing failure later.
package gitcheck

import (
	"log/slog"
	"os/exec"
	"strings"

	vdxerr "github.com/vedox/vedox/internal/errors"
)

// Identity holds the Git user identity read from `git config`.
type Identity struct {
	Name  string
	Email string
}

// Check reads user.name and user.email from git config and returns VDX-003 if
// either is missing or empty. Returns nil on success.
//
// This must be called before any operation that creates a Git commit.
func Check() (*Identity, error) {
	name, err := gitConfigValue("user.name")
	if err != nil {
		slog.Debug("gitcheck: failed to read user.name", "error", err)
	}

	email, err := gitConfigValue("user.email")
	if err != nil {
		slog.Debug("gitcheck: failed to read user.email", "error", err)
	}

	var missing []string
	if strings.TrimSpace(name) == "" {
		missing = append(missing, "user.name")
	}
	if strings.TrimSpace(email) == "" {
		missing = append(missing, "user.email")
	}

	if len(missing) > 0 {
		return nil, vdxerr.GitIdentityUnset(missing)
	}

	return &Identity{
		Name:  strings.TrimSpace(name),
		Email: strings.TrimSpace(email),
	}, nil
}

// gitConfigValue runs `git config <key>` and returns the trimmed output.
// Returns an empty string (and no error) when the key is not set —
// git exits 1 with no output when a key is missing, which we treat as "unset".
func gitConfigValue(key string) (string, error) {
	return GetConfigValue(key)
}

// GetConfigValue runs `git config <key>` and returns the trimmed output.
// Returns an empty string (and no error) when the key is not set —
// git exits 1 with no output when a key is missing, which we treat as "unset".
// This is the exported form of gitConfigValue, for use by other packages.
func GetConfigValue(key string) (string, error) {
	// #nosec G204 — key is a hardcoded constant, never user-supplied.
	out, err := exec.Command("git", "config", key).Output()
	if err != nil {
		// Exit code 1 from git config means "key not set". Any other error
		// (e.g., git not installed) we propagate so callers can distinguish.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
