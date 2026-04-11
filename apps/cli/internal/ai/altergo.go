package ai

import (
	"encoding/json"
	"os"
	"path/filepath"
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
// Returns an empty string if the user's home directory cannot be determined.
func AccountHome(accountName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".altergo", "accounts", accountName)
}
