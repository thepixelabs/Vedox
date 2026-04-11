package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vedox/vedox/internal/config"
	"github.com/vedox/vedox/internal/frontmatter"
)

var lintFlags struct {
	format string
	strict bool // TODO(phase-3): harden to exit 1 after VDX-P2-M clears exemption list
}

var lintCmd = &cobra.Command{
	Use:   "lint [path...]",
	Short: "Validate Markdown files against the WRITING_FRAMEWORK frontmatter contract",
	Long: `Lint checks one or more Markdown files or directories for WRITING_FRAMEWORK
compliance (LINT-001 through LINT-016).

Phase 2: warn-first mode. All issues are reported but exit code is always 0.
--strict is reserved for Phase 3 and currently also exits 0.

Paths can be files or directories (searched recursively). If no paths are
given, the workspace docs/ directory is used.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targets, err := resolveLintTargets(args)
		if err != nil {
			return err
		}

		var issues []frontmatter.LintIssue
		fileCount := 0

		for _, target := range targets {
			info, err := os.Stat(target)
			if err != nil {
				return fmt.Errorf("cannot access %s: %w", target, err)
			}
			if info.IsDir() {
				dirIssues, err := frontmatter.LintDir(target)
				if err != nil {
					return err
				}
				issues = append(issues, dirIssues...)
				fileCount++ // approximate; LintDir could expose a count in future
			} else {
				fileIssues, err := frontmatter.LintFile(target)
				if err != nil {
					return err
				}
				issues = append(issues, fileIssues...)
				fileCount++
			}
		}

		switch lintFlags.format {
		case "json":
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(issues); err != nil {
				return err
			}
		default:
			for _, issue := range issues {
				fmt.Fprintln(os.Stdout, issue.String())
			}
			errCount, warnCount := 0, 0
			for _, i := range issues {
				if i.Severity == frontmatter.SeverityError {
					errCount++
				} else {
					warnCount++
				}
			}
			fmt.Fprintf(os.Stdout, "\n%d error(s), %d warning(s) in %d file(s)\n", errCount, warnCount, fileCount)
		}

		// Phase 2: always exit 0 regardless of --strict.
		// TODO(phase-3): if lintFlags.strict && errCount > 0 { os.Exit(1) }
		return nil
	},
}

// resolveLintTargets returns the list of paths to lint. If no args are given,
// defaults to the workspace docs/ directory.
func resolveLintTargets(args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}
	// Default: find workspace from config and use docs/ inside it.
	cfgPath := globalFlags.configPath
	if cfgPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		cfgPath = filepath.Join(cwd, "vedox.config.toml")
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		// Fall back to ./docs relative to cwd if config missing.
		cwd, _ := os.Getwd()
		return []string{filepath.Join(cwd, "docs")}, nil
	}
	return []string{filepath.Join(cfg.Workspace, "docs")}, nil
}

func init() {
	lintCmd.Flags().StringVar(&lintFlags.format, "format", "text", "output format: text or json")
	lintCmd.Flags().BoolVar(&lintFlags.strict, "strict", false, "exit 1 on errors (Phase 3 only — currently a no-op)")
	rootCmd.AddCommand(lintCmd)
}
