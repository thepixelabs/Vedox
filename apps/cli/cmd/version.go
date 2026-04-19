package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// versionCmd prints the Vedox CLI version string.
//
// The version, commit, and buildDate variables are injected at build time via
// -ldflags. Local dev builds show "dev / none / unknown".
//
// Example release build:
//
//	go build -ldflags "\
//	  -X github.com/vedox/vedox/cmd.version=0.1.0 \
//	  -X github.com/vedox/vedox/cmd.commit=$(git rev-parse --short HEAD) \
//	  -X github.com/vedox/vedox/cmd.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the Vedox CLI version",
	// Use Run (not RunE) — this command cannot fail.
	Run: func(cmd *cobra.Command, args []string) {
		// Use cobra's writer (cmd.OutOrStdout) so tests can capture output via SetOut.
		fmt.Fprintf(cmd.OutOrStdout(), "vedox %s (commit %s, built %s)\n", version, commit, buildDate)
	},
}
