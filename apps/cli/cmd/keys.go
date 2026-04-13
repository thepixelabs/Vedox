package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/config"
)

// keysCmd is the parent of `vedox keys add|list|revoke`. It does nothing on
// its own — operators always invoke a subcommand.
var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage agent API keys",
	Long: `Manage HMAC-SHA256 API keys used by autonomous AI agents to write
into your Vedox workspace.

Secrets are stored in the OS keychain (macOS Keychain, Linux Secret Service,
Windows Credential Manager). Public metadata lives in .vedox/agent-keys.json.
A secret is shown exactly once — at issuance time. Lost secrets cannot be
recovered; revoke the key and issue a new one.`,
}

// --- keys add -----------------------------------------------------------

var (
	keysAddProject    string
	keysAddPathPrefix string
)

var keysAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Issue a new agent API key",
	Long: `Issue a new HMAC-SHA256 API key for an AI agent.

The plaintext secret is printed exactly once — store it securely. It cannot
be retrieved again. Use --project and --path-prefix to scope the key to a
subset of your workspace; an unscoped key can write anywhere.`,
	Args: cobra.ExactArgs(1),
	RunE: runKeysAdd,
}

func runKeysAdd(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	id, secret, err := ks.IssueKey(args[0], keysAddProject, keysAddPathPrefix)
	if err != nil {
		return err
	}
	fmt.Printf("Key ID: %s\n", id)
	fmt.Printf("Secret (save this — shown once): %s\n", secret)
	return nil
}

// --- keys list ----------------------------------------------------------

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agent API keys",
	RunE:  runKeysList,
}

func runKeysList(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	keys := ks.ListKeys()
	if len(keys) == 0 {
		fmt.Println("No agent API keys. Create one with: vedox keys add <name>")
		return nil
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tPROJECT\tPATH PREFIX\tCREATED\tSTATUS")
	for _, k := range keys {
		status := "active"
		if k.Revoked {
			status = "revoked"
		}
		project := k.Project
		if project == "" {
			project = "(any)"
		}
		prefix := k.PathPrefix
		if prefix == "" {
			prefix = "(any)"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			k.ID, k.Name, project, prefix,
			k.CreatedAt.Format("2006-01-02 15:04"),
			status,
		)
	}
	return tw.Flush()
}

// --- keys revoke --------------------------------------------------------

var keysRevokeCmd = &cobra.Command{
	Use:   "revoke <id>",
	Short: "Revoke an agent API key",
	Long: `Revoke an agent API key by ID.

The secret is deleted from the OS keychain immediately. The metadata entry
is retained with Revoked=true so your audit trail stays intact.`,
	Args: cobra.ExactArgs(1),
	RunE: runKeysRevoke,
}

func runKeysRevoke(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	if err := ks.RevokeKey(args[0]); err != nil {
		return err
	}
	fmt.Printf("Key %s revoked.\n", args[0])
	return nil
}

func init() {
	keysAddCmd.Flags().StringVar(&keysAddProject, "project", "",
		"restrict the key to a single project (empty = any project)")
	keysAddCmd.Flags().StringVar(&keysAddPathPrefix, "path-prefix", "",
		"restrict the key to URL paths starting with this prefix")

	keysCmd.AddCommand(keysAddCmd)
	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysRevokeCmd)

	rootCmd.AddCommand(keysCmd)
}
