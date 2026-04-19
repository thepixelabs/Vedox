// Package cmd wires together all cobra commands for the vedox binary.
//
// All user-facing errors must use VDX error codes from internal/errors.
// Go stack traces are never shown to users; use --debug to surface causes.
//
// NETWORK POLICY: This package and all commands it registers make ZERO
// outbound network calls. No version checks, no telemetry, no DNS lookups
// outside of serving localhost. See main.go for the authoritative policy
// comment.
package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	vdxerr "github.com/vedox/vedox/internal/errors"
	"github.com/vedox/vedox/internal/logging"
)

// Build-time variables injected via -ldflags. Example:
//
//	go build -ldflags "-X github.com/vedox/vedox/cmd.version=0.1.0 \
//	                   -X github.com/vedox/vedox/cmd.commit=$(git rev-parse --short HEAD) \
//	                   -X github.com/vedox/vedox/cmd.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var (
	version   = "v0.1.0-alpha.1"
	commit    = "none"
	buildDate = "unknown"
)

// globalFlags holds the values of flags defined on the root command.
// All subcommands inherit these via PersistentFlags.
var globalFlags struct {
	// configPath overrides the default ./vedox.config.toml location.
	configPath string
	// debug enables DEBUG-level logging and prints full error cause chains.
	debug bool
}

// logCleanup is the function returned by logging.Setup. It is called by
// Execute after the command finishes to flush and close the log file.
var logCleanup func()

// rootCmd is the base cobra command. It does not run anything itself; it
// exists to host global flags and provide usage/help output.
var rootCmd = &cobra.Command{
	Use:   "vedox",
	Short: "Vedox — local-first, Git-native documentation CMS",
	Long: `Vedox is a local-first documentation CMS.

It stores documents as Markdown on disk, indexes them in SQLite for fast
search, and serves a WYSIWYG editor on localhost.

Zero outbound network calls are made by default.`,

	// PersistentPreRunE runs before every subcommand. We initialise logging
	// here so all subcommands share the same configured logger instance.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		level := slog.LevelInfo
		if globalFlags.debug {
			level = slog.LevelDebug
		}

		cleanup, setupErr := logging.Setup(level)
		logCleanup = cleanup
		if setupErr != nil {
			// Non-fatal: logging.Setup already fell back to stderr.
			// Print a warning so the user knows logs may not be persisted.
			fmt.Fprintf(os.Stderr, "warning: %v\n", setupErr)
		}

		slog.Debug("vedox starting",
			"version", version,
			"commit", commit,
			"build_date", buildDate,
			"debug", globalFlags.debug,
		)
		return nil
	},

	// SilenceUsage prevents cobra printing the full usage block on runtime
	// errors — that is noise when the error is not a usage mistake.
	SilenceUsage: true,

	// SilenceErrors prevents cobra printing errors — Execute handles that
	// itself to enforce the VDX error taxonomy format.
	SilenceErrors: true,
}

// Execute is the entry point called from main. It runs the selected
// subcommand and formats errors per the VDX error taxonomy.
func Execute() error {
	defer func() {
		if logCleanup != nil {
			logCleanup()
		}
	}()

	if err := rootCmd.Execute(); err != nil {
		printError(err)
		return err
	}
	return nil
}

// printError formats and prints an error to stderr. VedoxErrors are shown
// with their code and docs URL. --debug surfaces the full cause chain.
// Raw Go stack traces are never shown.
func printError(err error) {
	var vdxErr *vdxerr.VedoxError
	if errors.As(err, &vdxErr) {
		if globalFlags.debug {
			fmt.Fprintln(os.Stderr, vdxErr.DebugMessage())
		} else {
			fmt.Fprintln(os.Stderr, vdxErr.UserMessage())
		}
		slog.Error("command failed",
			"code", string(vdxErr.Code),
			"message", vdxErr.Message,
		)
		return
	}

	// Untyped internal error. Show the raw message in debug mode only.
	if globalFlags.debug {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	} else {
		fmt.Fprintln(os.Stderr, "An unexpected error occurred. Run with --debug for details.")
	}
	slog.Error("unexpected error", "error", err.Error())
}

func init() {
	rootCmd.PersistentFlags().StringVar(
		&globalFlags.configPath,
		"config",
		"",
		"path to vedox.config.toml (default: ./vedox.config.toml)",
	)
	rootCmd.PersistentFlags().BoolVar(
		&globalFlags.debug,
		"debug",
		false,
		"enable debug logging and verbose error output (never used in production)",
	)

	// Register subcommands.
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(reindexCmd)
	rootCmd.AddCommand(versionCmd)
	// completionCmd is registered by its own init() in completion.go.
}
