package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vedox/vedox/internal/doctor"
)

var doctorJSONFlag bool

// doctorCmd runs the Vedox environment diagnostic suite.
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run Vedox environment diagnostics",
	Long: `Run a full diagnostic check of your Vedox environment.

Checks performed:
  - git installed and identity configured
  - gh CLI installed (>= 2.20.0) and authenticated
  - Vedox daemon running, healthy, and version-matched
  - Daemon port available (when daemon is not running)
  - Registry valid (repos.json, orphan detection)
  - Disk space >= 500 MB on the ~/.vedox partition
  - OS keychain accessible (macOS Keychain / Linux Secret Service)
  - Log directory writable
  - SQLite WAL size within limits
  - inotify watch limit (Linux only)

Exit code is 0 if all checks pass or only warnings exist.
Exit code is 1 if any check is a hard failure.

Use --json to emit machine-readable JSON.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := doctor.DefaultConfig(version)
		if err != nil {
			return fmt.Errorf("doctor: cannot determine environment: %w", err)
		}

		results := doctor.RunAll(cfg)

		if doctorJSONFlag {
			b, err := json.MarshalIndent(results, "", "  ")
			if err != nil {
				return fmt.Errorf("doctor: JSON encoding failed: %w", err)
			}
			fmt.Println(string(b))
		} else {
			fmt.Println("vedox doctor — environment diagnostics")
			fmt.Println()
			fmt.Print(doctor.FormatText(results))
			fmt.Println()
			fmt.Println(doctor.Summary(results))
		}

		if doctor.AnyFailed(results) {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorJSONFlag, "json", false, "emit results as JSON")
	rootCmd.AddCommand(doctorCmd)
}
