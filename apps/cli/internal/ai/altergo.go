package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// AccountInfo represents an AlterGo account discovered on disk.
type AccountInfo struct {
	Name      string   `json:"name"`
	Providers []string `json:"providers"`
}

// AltergoInfo contains the overall AlterGo availability status.
type AltergoInfo struct {
	Available bool          `json:"available"`
	Accounts  []AccountInfo `json:"accounts"`
}

// accountMeta mirrors the schema of ~/.altergo/accounts/<name>/account.json.
type accountMeta struct {
	Version   int      `json:"version"`
	Providers []string `json:"providers"`
}

// DiscoverAltergo reads ~/.altergo/accounts/ and returns account metadata.
// Returns AltergoInfo with Available=false if the directory does not exist —
// that is the normal state on machines that don't have AlterGo installed.
// Never returns an error; callers can always safely use the result.
func DiscoverAltergo() AltergoInfo {
	home, err := os.UserHomeDir()
	if err != nil {
		return AltergoInfo{Available: false}
	}

	base := filepath.Join(home, ".altergo", "accounts")
	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return AltergoInfo{Available: false}
	}
	if err != nil {
		return AltergoInfo{Available: false}
	}

	var accounts []AccountInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := filepath.Join(base, e.Name(), "account.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			// No metadata file — include the account with unknown providers.
			accounts = append(accounts, AccountInfo{Name: e.Name()})
			continue
		}
		var meta accountMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			accounts = append(accounts, AccountInfo{Name: e.Name()})
			continue
		}
		accounts = append(accounts, AccountInfo{
			Name:      e.Name(),
			Providers: meta.Providers,
		})
	}

	return AltergoInfo{
		Available: true,
		Accounts:  accounts,
	}
}

// AccountHome returns the home directory for the given AlterGo account.
// Returns an empty string if the user's home directory cannot be determined
// or if accountName is not a safe single-segment identifier.
//
// Security: accountName reaches this function from the HTTP layer (see
// POST /api/ai/generate-names). If we let filepath.Join clean ".." out, an
// attacker could set HOME= anywhere on disk when we later exec the AI CLI.
// Reject anything that is not a single plain path segment.
func AccountHome(accountName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if !validAccountName(accountName) {
		return ""
	}
	return filepath.Join(home, ".altergo", "accounts", accountName)
}

// validAccountName enforces a conservative character policy: letters, digits,
// dashes, underscores, and dots (but not '.' or '..' standalone). This is the
// intersection of what the real AlterGo directory layout allows and what is
// safe to interpolate into a filesystem path.
func validAccountName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if len(name) > 128 {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return false
		}
	}
	// Explicitly reject path separators and any ".." segments even though
	// the char policy already excludes '/' and '\\'.
	if strings.ContainsAny(name, `/\`) {
		return false
	}
	return true
}
