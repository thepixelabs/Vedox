package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/vedox/vedox/internal/config"
)

// reindexCmd rebuilds the SQLite metadata store from the Markdown tree.
//
// This is the disaster-recovery path: if .vedox/index.db is corrupted or
// deleted, `vedox reindex` walks the workspace Markdown files, re-parses
// frontmatter, and rebuilds the index. No data loss is possible because
// Markdown on disk is the source of truth.
//
// Phase 1 implementation: validates config and prints a status message.
// Full reindex logic (SQLite write, FTS5 rebuild) lands in VDX-P1-005.
//
// DR test (from the epic DoD):
//
//	rm .vedox/index.db && vedox reindex
//
// Must restore a fully searchable workspace with zero data loss.
var reindexCmd = &cobra.Command{
	Use:   "reindex",
	Short: "Rebuild the Vedox metadata index from the Markdown workspace",
	Long: `Rebuild the Vedox metadata index from the Markdown workspace.

Use this after recovering from a corrupted or deleted .vedox/index.db.
All data is re-derived from Markdown frontmatter — no data loss is possible.

Full implementation lands in VDX-P1-005.`,
	RunE: runReindex,
}

func runReindex(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}

	slog.Info("reindex started", "workspace", cfg.Workspace)

	// Phase 1 placeholder. The full reindex walker lands in VDX-P1-005.
	fmt.Println("Reindexing workspace...")
	slog.Info("reindex complete (placeholder)")

	return nil
}
