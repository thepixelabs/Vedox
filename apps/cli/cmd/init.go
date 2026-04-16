// Package cmd — `vedox init` command.
//
// Two modes:
//
//  1. System init (no path argument)
//     Creates ~/.vedox/, ~/.vedox/repos.json (empty registry), and
//     ~/.vedox/global.db (runs migrations). Idempotent.
//
//  2. Project init (path argument)
//     Creates <path>/.vedox/, checks whether the directory is a Git repo,
//     registers the project in ~/.vedox/repos.json, counts existing Markdown
//     files, and prints a short summary.
//
// Both modes are idempotent: running twice is safe and prints
// "already initialized" instead of returning an error.
package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/vedox/vedox/internal/daemon"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/registry"
)

// initFlags holds the flag values for the init command.
var initFlags struct {
	force bool
}

// initCmd implements `vedox init [path]`.
var initCmd = &cobra.Command{
	Use:   "init [path]",
	Short: "Initialize Vedox for a project or the system",
	Long: `Initialize Vedox.

Without a path argument, performs system-level initialization:
  - Creates ~/.vedox/ (mode 0700)
  - Creates ~/.vedox/repos.json with an empty registry
  - Creates ~/.vedox/global.db and runs schema migrations

With a path argument, initializes a single project:
  - Creates <path>/.vedox/ (mode 0755)
  - Detects whether the path is a Git repository
  - Registers the project in ~/.vedox/repos.json
  - Counts existing Markdown files and reports the total

Both modes are idempotent: running twice is safe and exits 0.
Use --force to re-run initialization even when already initialized.`,

	Args: cobra.MaximumNArgs(1),

	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return runProjectInit(cmd, args[0])
		}
		return runSystemInit(cmd)
	},
}

// runSystemInit performs the system-level (~/.vedox/) initialization.
func runSystemInit(cmd *cobra.Command) error {
	// DefaultVedoxHome creates ~/.vedox/ with mode 0700 if it does not exist.
	vedoxHome, err := daemon.DefaultVedoxHome()
	if err != nil {
		return fmt.Errorf("system init: %w", err)
	}

	// --- repos.json ---
	reposPath := filepath.Join(vedoxHome, "repos.json")
	reposAlreadyExisted := false
	if _, statErr := os.Stat(reposPath); statErr == nil {
		reposAlreadyExisted = true
	}

	if !reposAlreadyExisted || initFlags.force {
		// NewFileRegistry creates the file with an empty manifest if absent.
		// On --force we re-open which is effectively a no-op for an already valid
		// manifest, but it re-validates the JSON and refreshes the in-memory cache.
		if _, regErr := registry.NewFileRegistry(reposPath, nil); regErr != nil {
			return fmt.Errorf("system init: create repos.json: %w", regErr)
		}
	}

	// --- global.db ---
	dbPath := filepath.Join(vedoxHome, db.GlobalDBPath)
	// GlobalDBPath is ".vedox/global.db" — strip the prefix because vedoxHome
	// already ends with ".vedox".
	dbPath = filepath.Join(vedoxHome, "global.db")
	dbAlreadyExisted := false
	if _, statErr := os.Stat(dbPath); statErr == nil {
		dbAlreadyExisted = true
	}

	if !dbAlreadyExisted || initFlags.force {
		globalDB, openErr := db.OpenGlobalDB(dbPath)
		if openErr != nil {
			return fmt.Errorf("system init: open global.db: %w", openErr)
		}
		_ = globalDB.Close()
	}

	// --- user feedback ---
	if reposAlreadyExisted && dbAlreadyExisted && !initFlags.force {
		fmt.Fprintf(cmd.OutOrStdout(),
			"vedox already initialized at %s\n"+
				"  run `vedox init --force` to reinitialize, or `vedox init <project-path>` to add a project.\n",
			vedoxHome,
		)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(),
		"vedox initialized at %s\n"+
			"  run `vedox init <project-path>` to add a project.\n",
		vedoxHome,
	)
	return nil
}

// runProjectInit performs per-project initialization for the given path.
func runProjectInit(cmd *cobra.Command, rawPath string) error {
	// Resolve to an absolute path before any filesystem operations.
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return fmt.Errorf("project init: resolve path %q: %w", rawPath, err)
	}

	// Verify the path exists and is a directory.
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("project init: path does not exist: %s", absPath)
		}
		return fmt.Errorf("project init: stat %s: %w", absPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("project init: path is not a directory: %s", absPath)
	}

	// --- .vedox/ directory inside the project ---
	dotVedox := filepath.Join(absPath, ".vedox")
	dotVedoxAlreadyExisted := false
	if _, statErr := os.Stat(dotVedox); statErr == nil {
		dotVedoxAlreadyExisted = true
	}

	if dotVedoxAlreadyExisted && !initFlags.force {
		fmt.Fprintf(cmd.OutOrStdout(),
			"project already initialized at %s\n"+
				"  run `vedox init --force %s` to reinitialize.\n",
			absPath, rawPath,
		)
		return nil
	}

	// Create .vedox/ — 0755 so tools running inside the project can read it.
	if err := os.MkdirAll(dotVedox, 0o755); err != nil {
		return fmt.Errorf("project init: create .vedox dir: %w", err)
	}

	// --- Git repo detection ---
	isGit := isGitRepo(absPath)

	// --- Register in the global registry ---
	reg, regErr := openRegistry()
	if regErr != nil {
		return fmt.Errorf("project init: open registry: %w", regErr)
	}

	projectName := filepath.Base(absPath)
	repoType := registry.RepoTypeBareLocal
	if isGit {
		// A project-scoped Git repo. Treat it as project-public unless the
		// user reclassifies it later via `vedox repos set-default`.
		repoType = registry.RepoTypeProjectPublic
	}

	repo := registry.Repo{
		Name:     projectName,
		Type:     repoType,
		RootPath: absPath,
		Status:   registry.StatusActive,
	}

	addErr := reg.Add(repo)
	alreadyRegistered := false
	if addErr != nil {
		if errors.Is(addErr, registry.ErrNameConflict) {
			// Idempotent: project was previously registered — that is fine.
			alreadyRegistered = true
		} else {
			return fmt.Errorf("project init: register project: %w", addErr)
		}
	}

	// --- Markdown file count ---
	mdCount := countMDFiles(absPath)

	// --- User feedback ---
	gitTag := ""
	if isGit {
		gitTag = " (git repo detected)"
	}
	regTag := ""
	if alreadyRegistered {
		regTag = " (already registered)"
	}

	fmt.Fprintf(cmd.OutOrStdout(),
		"project initialized%s\n"+
			"  path:       %s\n"+
			"  registered: %s%s\n"+
			"  docs found: %d markdown file(s)\n"+
			"  run `vedox server start` to begin.\n",
		gitTag,
		absPath,
		projectName,
		regTag,
		mdCount,
	)
	return nil
}

// countMDFiles returns the count of .md files under root, skipping hidden
// directories, node_modules, and vendor — consistent with the scanner's policy.
func countMDFiles(root string) int {
	count := 0
	_ = filepath.WalkDir(root, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == "vendor" {
				return fs.SkipDir
			}
			if len(name) > 0 && name[0] == '.' {
				return fs.SkipDir
			}
		}
		if !d.IsDir() && filepath.Ext(d.Name()) == ".md" {
			count++
		}
		return nil
	})
	return count
}

func init() {
	initCmd.Flags().BoolVar(
		&initFlags.force,
		"force",
		false,
		"reinitialize even if already initialized",
	)

	rootCmd.AddCommand(initCmd)
}
