package cmd

// vedox completion — shell completion script generator (WS-M)
//
// Outputs a completion script for the requested shell to stdout.
// Homebrew's generate_completions_from_executable calls:
//
//	vedox completion bash
//	vedox completion zsh
//	vedox completion fish
//
// All three subcommands write only to stdout and make zero network calls.

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion <shell>",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for vedox.

Supported shells: bash, zsh, fish

Bash — add to your current session:

  source <(vedox completion bash)

Bash — install system-wide (Linux):

  vedox completion bash > /etc/bash_completion.d/vedox

Bash — install for your user (macOS with bash-completion@2):

  vedox completion bash > $(brew --prefix)/etc/bash_completion.d/vedox

Zsh — add to your current session:

  source <(vedox completion zsh)

Zsh — install (place in a directory on your $fpath):

  vedox completion zsh > "${fpath[1]}/_vedox"

Fish — install:

  vedox completion fish > ~/.config/fish/completions/vedox.fish`,
}

// completionBashCmd outputs a bash completion script using cobra's V2 generator,
// which is compatible with bash-completion >=2.x (the version distributed by
// Homebrew). The V2 script does not source /etc/bash_completion; it is
// self-contained and safe to redirect directly into a completions directory.
var completionBashCmd = &cobra.Command{
	Use:                   "bash",
	Short:                 "Generate bash completion script",
	DisableFlagsInUseLine: true,
	Args:                  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// includeDesc=true embeds flag descriptions as completion hints;
		// harmless if the user's bash-completion does not display them.
		if err := rootCmd.GenBashCompletionV2(os.Stdout, true); err != nil {
			return fmt.Errorf("generating bash completion: %w", err)
		}
		return nil
	},
}

// completionZshCmd outputs a zsh completion script. The script uses the #compdef
// shebang so zsh knows to invoke the _vedox completion function automatically
// when the file is placed on $fpath.
var completionZshCmd = &cobra.Command{
	Use:                   "zsh",
	Short:                 "Generate zsh completion script",
	DisableFlagsInUseLine: true,
	Args:                  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := rootCmd.GenZshCompletion(os.Stdout); err != nil {
			return fmt.Errorf("generating zsh completion: %w", err)
		}
		return nil
	},
}

// completionFishCmd outputs a fish completion script. Fish completions are
// function-based; cobra generates a script that calls complete(1) for every
// flag and subcommand.
var completionFishCmd = &cobra.Command{
	Use:                   "fish",
	Short:                 "Generate fish completion script",
	DisableFlagsInUseLine: true,
	Args:                  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// includeDesc=true embeds short-flag descriptions in the fish completions.
		if err := rootCmd.GenFishCompletion(os.Stdout, true); err != nil {
			return fmt.Errorf("generating fish completion: %w", err)
		}
		return nil
	},
}

func init() {
	completionCmd.AddCommand(completionBashCmd)
	completionCmd.AddCommand(completionZshCmd)
	completionCmd.AddCommand(completionFishCmd)

	rootCmd.AddCommand(completionCmd)
}
