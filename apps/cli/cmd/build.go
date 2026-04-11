package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/vedox/vedox/internal/config"
)

// buildCmd generates a static HTML build of the Vedox documentation workspace.
//
// Phase 1 implementation: validates config and prints a status message.
// Full static rendering (Markdown → HTML, CSP meta tags, asset bundling) is
// implemented in VDX-P1-004.
//
// Per the CTO audit: static HTML only in Phase 1. No "prod" Go HTTP server.
// Static output is hostable on Netlify, Cloudflare Pages, GitHub Pages, or S3.
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build static HTML output from your Vedox workspace",
	Long: `Build a static HTML portal from your Vedox workspace.

Output is written to ./dist/ by default. The generated HTML includes a
Content-Security-Policy <meta> tag matching the dev server CSP:

  default-src 'self'; script-src 'none'; object-src 'none'

Full implementation lands in VDX-P1-004.`,
	RunE: runBuild,
}

func runBuild(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}

	slog.Info("build started", "workspace", cfg.Workspace, "profile", string(cfg.Profile))

	// Phase 1 placeholder. The full static HTML renderer lands in VDX-P1-004.
	fmt.Println("Building static output...")
	slog.Info("build complete (placeholder)")

	return nil
}
