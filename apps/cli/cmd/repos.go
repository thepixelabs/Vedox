package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vedox/vedox/internal/registry"
)

// defaultReposJSONPath returns ~/.vedox/repos.json, creating ~/.vedox if needed.
func defaultReposJSONPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".vedox")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("cannot create %s: %w", dir, err)
	}
	return filepath.Join(dir, "repos.json"), nil
}

// openRegistry is a helper shared by all repos subcommands.
func openRegistry() (*registry.FileRegistry, error) {
	path, err := defaultReposJSONPath()
	if err != nil {
		return nil, err
	}
	return registry.NewFileRegistry(path, nil)
}

// isGitRepo returns true if path contains a .git directory or file.
func isGitRepo(path string) bool {
	gitPath := filepath.Join(path, ".git")
	_, err := os.Stat(gitPath)
	return err == nil
}

// reposCmd is the parent of `vedox repos list|add|remove|set-default|create`.
var reposCmd = &cobra.Command{
	Use:   "repos",
	Short: "Manage registered documentation repositories",
	Long: `Manage the Vedox documentation repository registry.

Vedox stores documentation in dedicated Git repos separate from your
project source repos. You can register existing repos or create new
ones via the gh CLI.

Registry state is persisted to ~/.vedox/repos.json.`,
}

// --- repos list ----------------------------------------------------------

var reposListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered documentation repos",
	Long: `List all repos registered in ~/.vedox/repos.json.

Each entry shows the repo name, type (private | project-public | bare-local),
local path, status, and whether it is the default routing target for the Doc Agent.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		reg, err := openRegistry()
		if err != nil {
			return fmt.Errorf("cannot open registry: %w", err)
		}

		repos, err := reg.List()
		if err != nil {
			return fmt.Errorf("cannot list repos: %w", err)
		}

		if len(repos) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no repos registered — run `vedox repos add <path>` or `vedox repos create --name <name>`")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tPATH\tSTATUS\tDEFAULT")
		fmt.Fprintln(w, "----\t----\t----\t------\t-------")
		for _, r := range repos {
			def := ""
			if r.IsDefault {
				def = "*"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				r.Name, string(r.Type), r.RootPath, string(r.Status), def)
		}
		return w.Flush()
	},
}

// --- repos add -----------------------------------------------------------

var reposAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Register an existing documentation repo",
	Long: `Register an existing Git repository as a Vedox documentation repo.

<path> must be a local filesystem path to a cloned Git repo. The repo
is not copied — Vedox registers its location and begins watching it.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := args[0]

		abs, err := filepath.Abs(repoPath)
		if err != nil {
			return fmt.Errorf("cannot resolve path %q: %w", repoPath, err)
		}

		info, err := os.Stat(abs)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("path does not exist: %s", abs)
			}
			return fmt.Errorf("cannot stat %s: %w", abs, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", abs)
		}
		if !isGitRepo(abs) {
			return fmt.Errorf("path is not a Git repository (no .git found): %s", abs)
		}

		// Derive a name from the directory basename.
		name := filepath.Base(abs)

		reg, err := openRegistry()
		if err != nil {
			return fmt.Errorf("cannot open registry: %w", err)
		}

		repo := registry.Repo{
			Name:     name,
			Type:     registry.RepoTypeBareLocal,
			RootPath: abs,
			Status:   registry.StatusActive,
		}
		if err := reg.Add(repo); err != nil {
			return fmt.Errorf("cannot add repo: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "registered %q at %s\n", name, abs)
		return nil
	},
}

// --- repos remove --------------------------------------------------------

var reposRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Unregister a documentation repo",
	Long: `Remove a repo from the Vedox registry.

This does NOT delete any files. It removes the entry from
~/.vedox/repos.json and stops the file watcher for that repo.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		reg, err := openRegistry()
		if err != nil {
			return fmt.Errorf("cannot open registry: %w", err)
		}

		// Find by name to get the ID.
		repos, err := reg.List()
		if err != nil {
			return fmt.Errorf("cannot list repos: %w", err)
		}

		var id string
		for _, r := range repos {
			if r.Name == name {
				id = r.ID
				break
			}
		}
		if id == "" {
			return fmt.Errorf("no repo named %q — run `vedox repos list` to see registered repos", name)
		}

		if err := reg.Remove(id); err != nil {
			return fmt.Errorf("cannot remove repo %q: %w", name, err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "unregistered %q (files on disk are untouched)\n", name)
		return nil
	},
}

// --- repos set-default ---------------------------------------------------

var reposSetDefaultCmd = &cobra.Command{
	Use:   "set-default <name>",
	Short: "Set the default private repo for the Doc Agent",
	Long: `Mark a registered repo as the default destination for private
documents written by the Vedox Doc Agent.

When the Doc Agent receives a documentation request without an explicit
repo target, it routes the document to this repo.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		reg, err := openRegistry()
		if err != nil {
			return fmt.Errorf("cannot open registry: %w", err)
		}

		repos, err := reg.List()
		if err != nil {
			return fmt.Errorf("cannot list repos: %w", err)
		}

		var id string
		for _, r := range repos {
			if r.Name == name {
				id = r.ID
				break
			}
		}
		if id == "" {
			return fmt.Errorf("no repo named %q — run `vedox repos list` to see registered repos", name)
		}

		if err := reg.SetDefault(id); err != nil {
			return fmt.Errorf("cannot set default: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "default repo set to %q\n", name)
		return nil
	},
}

// --- repos create --------------------------------------------------------

var reposCreateFlags struct {
	name    string
	private bool
	bare    bool
}

var reposCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new local repo and register it",
	Long: `Create a new local documentation repository and register it with Vedox.

A directory is created at ~/.vedox/repos/<name>, git init is run inside it,
and the repo is added to ~/.vedox/repos.json.

If --private is set, the repo is typed as "private".
If --bare is set, the repo is typed as "bare-local" (default when no --private).

To push to a remote later, use gh repo create or git remote add manually.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.TrimSpace(reposCreateFlags.name)
		if name == "" {
			return fmt.Errorf("--name is required")
		}

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("cannot determine home directory: %w", err)
		}

		reposDir := filepath.Join(home, ".vedox", "repos")
		if err := os.MkdirAll(reposDir, 0o700); err != nil {
			return fmt.Errorf("cannot create repos dir %s: %w", reposDir, err)
		}

		repoPath := filepath.Join(reposDir, name)
		if _, err := os.Stat(repoPath); err == nil {
			return fmt.Errorf("directory already exists: %s", repoPath)
		}

		if err := os.MkdirAll(repoPath, 0o750); err != nil {
			return fmt.Errorf("cannot create repo directory %s: %w", repoPath, err)
		}

		// git init
		gitCmd := exec.Command("git", "init", repoPath)
		gitCmd.Stdout = cmd.OutOrStdout()
		gitCmd.Stderr = cmd.ErrOrStderr()
		if err := gitCmd.Run(); err != nil {
			_ = os.RemoveAll(repoPath) // clean up on failure
			return fmt.Errorf("git init failed: %w", err)
		}

		repoType := registry.RepoTypeBareLocal
		if reposCreateFlags.private {
			repoType = registry.RepoTypePrivate
		}

		reg, err := openRegistry()
		if err != nil {
			return fmt.Errorf("cannot open registry: %w", err)
		}

		repo := registry.Repo{
			Name:     name,
			Type:     repoType,
			RootPath: repoPath,
			Status:   registry.StatusActive,
		}
		if err := reg.Add(repo); err != nil {
			return fmt.Errorf("cannot register repo: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "created and registered %q at %s\n", name, repoPath)
		return nil
	},
}

func init() {
	// repos create flags
	reposCreateCmd.Flags().StringVar(&reposCreateFlags.name, "name", "",
		"repository name (required)")
	reposCreateCmd.Flags().BoolVar(&reposCreateFlags.private, "private", false,
		"set repo type to private (default: bare-local)")
	reposCreateCmd.Flags().BoolVar(&reposCreateFlags.bare, "bare", false,
		"set repo type to bare-local (default when --private is not set)")

	// wire subcommands
	reposCmd.AddCommand(reposListCmd)
	reposCmd.AddCommand(reposAddCmd)
	reposCmd.AddCommand(reposRemoveCmd)
	reposCmd.AddCommand(reposSetDefaultCmd)
	reposCmd.AddCommand(reposCreateCmd)

	rootCmd.AddCommand(reposCmd)
}
