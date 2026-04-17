package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/config"
	"github.com/vedox/vedox/internal/providers"
)

// agentCmd is the parent of `vedox agent install|uninstall|repair|list`.
// It does nothing on its own — operators always invoke a subcommand.
var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage the Vedox Doc Agent in AI provider environments",
	Long: `Manage the Vedox Doc Agent.

The Doc Agent is installed into AI provider environments (Claude Code via
MCP, GitHub Copilot, OpenAI Codex, Google Gemini). Once installed, the
trigger phrase "vedox document everything" routes documentation to your
registered repos using the HMAC-SHA256 auth layer.

Routing rules:
  - Public-facing docs  → the registered project-scoped repo
  - Private docs        → the default private repo (see: vedox repos set-default)
  - Multiple private repos → the agent asks which repo to use`,
}

// validProviders is the set of provider identifiers the agent commands accept.
// Exposed as a package-level slice so cobra can register ValidArgs for shell
// completion on all subcommands that accept --provider.
var validProviders = []string{"claude", "codex", "copilot", "gemini"}

// --- agent install --------------------------------------------------------

var agentInstallFlags struct {
	provider string
}

var agentInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install the Vedox Doc Agent into an AI provider",
	Long: `Install the Vedox Doc Agent configuration into an AI provider environment.

Supported providers:
  claude    Claude Code (user-scope subagent + CLAUDE.md block)
  codex     OpenAI Codex CLI (config.toml mcp_servers + AGENTS.md block)
  copilot   GitHub Copilot (.github/copilot-instructions.md — degraded/read-only mode)
  gemini    Google Gemini CLI (~/.gemini/extensions/vedox/ + config.yaml)

The install is idempotent — running it twice does not create duplicate
entries.`,
	RunE: runAgentInstall,
}

func runAgentInstall(cmd *cobra.Command, _ []string) error {
	p := agentInstallFlags.provider
	if p == "" {
		return fmt.Errorf("--provider is required (one of: claude, codex, copilot, gemini)")
	}

	switch providers.ProviderID(p) {
	case providers.ProviderClaude:
		return runClaudeInstall(cmd)
	case providers.ProviderCodex:
		return runCodexInstall(cmd)
	case providers.ProviderCopilot:
		return runCopilotInstall(cmd)
	case providers.ProviderGemini:
		return runGeminiInstall(cmd)
	default:
		return fmt.Errorf("provider %q is not supported; valid providers: claude, codex, copilot, gemini", p)
	}
}

func runClaudeInstall(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	store, err := providers.NewReceiptStore("")
	if err != nil {
		return err
	}
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	installer, err := providers.NewClaudeInstaller("", daemonURL, ks, store)
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	probe, err := installer.Probe(ctx)
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}
	if probe.Installed {
		fmt.Println("vedox doc agent is already installed for claude. run 'vedox agent repair --provider claude' to re-apply.")
		return nil
	}

	plan, err := installer.Plan(ctx)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	fmt.Printf("install plan — %d file operation(s):\n", len(plan.FileOps))
	for _, op := range plan.FileOps {
		fmt.Printf("  [%s] %s\n", op.Action, op.Path)
	}
	fmt.Println()

	receipt, err := installer.Install(ctx, plan)
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if err := store.Save(receipt); err != nil {
		return fmt.Errorf("save receipt: %w", err)
	}

	fmt.Printf("vedox doc agent installed for claude\n")
	fmt.Printf("  key id:       %s\n", receipt.AuthKeyID)
	fmt.Printf("  version:      %s\n", receipt.Version)
	fmt.Printf("  daemon url:   %s\n", receipt.DaemonURL)
	fmt.Printf("  installed at: %s\n", receipt.InstalledAt.Format("2006-01-02 15:04 UTC"))
	fmt.Printf("  files written:\n")
	for path := range receipt.FileHashes {
		fmt.Printf("    %s\n", path)
	}
	return nil
}

// --- agent uninstall ------------------------------------------------------

var agentUninstallFlags struct {
	provider string
}

var agentUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the Vedox Doc Agent from an AI provider",
	Long: `Remove the Vedox Doc Agent configuration from an AI provider environment.

The provider's configuration file is updated in place. No other provider
configurations are affected.`,
	RunE: runAgentUninstall,
}

func runAgentUninstall(cmd *cobra.Command, _ []string) error {
	p := agentUninstallFlags.provider
	if p == "" {
		return fmt.Errorf("--provider is required (one of: claude, codex, copilot, gemini)")
	}

	switch providers.ProviderID(p) {
	case providers.ProviderClaude:
		return runClaudeUninstall(cmd)
	case providers.ProviderCodex:
		return runCodexUninstall(cmd)
	case providers.ProviderCopilot:
		return runCopilotUninstall(cmd)
	case providers.ProviderGemini:
		return runGeminiUninstall(cmd)
	default:
		return fmt.Errorf("provider %q is not supported; valid providers: claude, codex, copilot, gemini", p)
	}
}

func runCodexInstall(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	store, err := providers.NewReceiptStore("")
	if err != nil {
		return err
	}
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	installer, err := providers.NewCodexInstaller("", daemonURL, ks, store)
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	probe, err := installer.Probe(ctx)
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}
	if probe.Installed {
		fmt.Println("vedox doc agent is already installed for codex. run 'vedox agent repair --provider codex' to re-apply.")
		return nil
	}

	plan, err := installer.Plan(ctx)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	fmt.Printf("install plan — %d file operation(s):\n", len(plan.FileOps))
	for _, op := range plan.FileOps {
		fmt.Printf("  [%s] %s\n", op.Action, op.Path)
	}
	fmt.Println()

	receipt, err := installer.Install(ctx, plan)
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if err := store.Save(receipt); err != nil {
		return fmt.Errorf("save receipt: %w", err)
	}

	fmt.Printf("vedox doc agent installed for codex\n")
	fmt.Printf("  key id:       %s\n", receipt.AuthKeyID)
	fmt.Printf("  version:      %s\n", receipt.Version)
	fmt.Printf("  daemon url:   %s\n", receipt.DaemonURL)
	fmt.Printf("  installed at: %s\n", receipt.InstalledAt.Format("2006-01-02 15:04 UTC"))
	fmt.Printf("  files written:\n")
	for path := range receipt.FileHashes {
		fmt.Printf("    %s\n", path)
	}
	return nil
}

func runClaudeUninstall(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	store, err := providers.NewReceiptStore("")
	if err != nil {
		return err
	}
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	installer, err := providers.NewClaudeInstaller("", daemonURL, ks, store)
	if err != nil {
		return err
	}

	if err := installer.Uninstall(cmd.Context()); err != nil {
		return fmt.Errorf("uninstall: %w", err)
	}
	fmt.Println("vedox doc agent removed for claude")
	return nil
}

func runCodexUninstall(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	store, err := providers.NewReceiptStore("")
	if err != nil {
		return err
	}
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	installer, err := providers.NewCodexInstaller("", daemonURL, ks, store)
	if err != nil {
		return err
	}

	if err := installer.Uninstall(cmd.Context()); err != nil {
		return fmt.Errorf("uninstall: %w", err)
	}
	fmt.Println("vedox doc agent removed for codex")
	return nil
}

// --- agent repair ---------------------------------------------------------

var agentRepairFlags struct {
	provider string
}

var agentRepairCmd = &cobra.Command{
	Use:   "repair",
	Short: "Re-apply Doc Agent config (idempotent)",
	Long: `Re-apply the Vedox Doc Agent configuration for a provider.

Use this if the agent stops responding after a provider update or if the
config file was edited manually. Repair is equivalent to uninstall followed
by install, but preserves any provider-specific customisations Vedox does
not manage.`,
	RunE: runAgentRepair,
}

func runAgentRepair(cmd *cobra.Command, _ []string) error {
	p := agentRepairFlags.provider
	if p == "" {
		return fmt.Errorf("--provider is required (one of: claude, codex, copilot, gemini)")
	}

	switch providers.ProviderID(p) {
	case providers.ProviderClaude:
		return runClaudeRepair(cmd)
	case providers.ProviderCodex:
		return runCodexRepair(cmd)
	case providers.ProviderCopilot:
		return runCopilotRepair(cmd)
	case providers.ProviderGemini:
		return runGeminiRepair(cmd)
	default:
		return fmt.Errorf("provider %q is not supported; valid providers: claude, codex, copilot, gemini", p)
	}
}

func runClaudeRepair(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	store, err := providers.NewReceiptStore("")
	if err != nil {
		return err
	}
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	installer, err := providers.NewClaudeInstaller("", daemonURL, ks, store)
	if err != nil {
		return err
	}

	if err := installer.Repair(cmd.Context()); err != nil {
		return fmt.Errorf("repair: %w", err)
	}
	fmt.Println("vedox doc agent repaired for claude")
	return nil
}

func runCodexRepair(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	store, err := providers.NewReceiptStore("")
	if err != nil {
		return err
	}
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	installer, err := providers.NewCodexInstaller("", daemonURL, ks, store)
	if err != nil {
		return err
	}

	if err := installer.Repair(cmd.Context()); err != nil {
		return fmt.Errorf("repair: %w", err)
	}
	fmt.Println("vedox doc agent repaired for codex")
	return nil
}

func runCopilotInstall(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	store, err := providers.NewReceiptStore("")
	if err != nil {
		return err
	}
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	installer, err := providers.NewCopilotInstaller("", "", daemonURL, ks, store)
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	probe, err := installer.Probe(ctx)
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}
	if probe.Installed {
		fmt.Println("vedox doc agent is already installed for copilot. run 'vedox agent repair --provider copilot' to re-apply.")
		return nil
	}

	plan, err := installer.Plan(ctx)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	fmt.Printf("install plan — %d file operation(s):\n", len(plan.FileOps))
	for _, op := range plan.FileOps {
		fmt.Printf("  [%s] %s\n", op.Action, op.Path)
	}
	fmt.Println()

	receipt, err := installer.Install(ctx, plan)
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if err := store.Save(receipt); err != nil {
		return fmt.Errorf("save receipt: %w", err)
	}

	fmt.Printf("vedox doc agent installed for copilot (degraded mode — read-only)\n")
	fmt.Printf("  note:         copilot cannot call the vedox daemon directly (no tool support).\n")
	fmt.Printf("                routing rules are installed as prose in .github/copilot-instructions.md.\n")
	fmt.Printf("                tool-call support will be enabled automatically when copilot adds MCP.\n")
	fmt.Printf("  key id:       %s\n", receipt.AuthKeyID)
	fmt.Printf("  version:      %s\n", receipt.Version)
	fmt.Printf("  daemon url:   %s\n", receipt.DaemonURL)
	fmt.Printf("  installed at: %s\n", receipt.InstalledAt.Format("2006-01-02 15:04 UTC"))
	fmt.Printf("  files written:\n")
	for path := range receipt.FileHashes {
		fmt.Printf("    %s\n", path)
	}
	return nil
}

func runCopilotUninstall(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	store, err := providers.NewReceiptStore("")
	if err != nil {
		return err
	}
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	installer, err := providers.NewCopilotInstaller("", "", daemonURL, ks, store)
	if err != nil {
		return err
	}

	if err := installer.Uninstall(cmd.Context()); err != nil {
		return fmt.Errorf("uninstall: %w", err)
	}
	fmt.Println("vedox doc agent removed for copilot")
	return nil
}

func runCopilotRepair(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	store, err := providers.NewReceiptStore("")
	if err != nil {
		return err
	}
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	installer, err := providers.NewCopilotInstaller("", "", daemonURL, ks, store)
	if err != nil {
		return err
	}

	if err := installer.Repair(cmd.Context()); err != nil {
		return fmt.Errorf("repair: %w", err)
	}
	fmt.Println("vedox doc agent repaired for copilot")
	return nil
}

func runGeminiInstall(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	store, err := providers.NewReceiptStore("")
	if err != nil {
		return err
	}
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	installer, err := providers.NewGeminiInstaller("", daemonURL, ks, store)
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	probe, err := installer.Probe(ctx)
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}
	if probe.Installed {
		fmt.Println("vedox doc agent is already installed for gemini. run 'vedox agent repair --provider gemini' to re-apply.")
		return nil
	}

	plan, err := installer.Plan(ctx)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	fmt.Printf("install plan — %d file operation(s):\n", len(plan.FileOps))
	for _, op := range plan.FileOps {
		fmt.Printf("  [%s] %s\n", op.Action, op.Path)
	}
	fmt.Println()

	receipt, err := installer.Install(ctx, plan)
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}
	if err := store.Save(receipt); err != nil {
		return fmt.Errorf("save receipt: %w", err)
	}

	fmt.Printf("vedox doc agent installed for gemini\n")
	fmt.Printf("  note:         config path is ~/.gemini/extensions/vedox/ — validate against\n")
	fmt.Printf("                your installed gemini CLI version if the agent does not appear.\n")
	fmt.Printf("                run 'vedox agent repair --provider gemini' to re-apply after\n")
	fmt.Printf("                a gemini CLI upgrade.\n")
	fmt.Printf("  key id:       %s\n", receipt.AuthKeyID)
	fmt.Printf("  version:      %s\n", receipt.Version)
	fmt.Printf("  daemon url:   %s\n", receipt.DaemonURL)
	fmt.Printf("  installed at: %s\n", receipt.InstalledAt.Format("2006-01-02 15:04 UTC"))
	fmt.Printf("  files written:\n")
	for path := range receipt.FileHashes {
		fmt.Printf("    %s\n", path)
	}
	return nil
}

func runGeminiUninstall(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	store, err := providers.NewReceiptStore("")
	if err != nil {
		return err
	}
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	installer, err := providers.NewGeminiInstaller("", daemonURL, ks, store)
	if err != nil {
		return err
	}

	if err := installer.Uninstall(cmd.Context()); err != nil {
		return fmt.Errorf("uninstall: %w", err)
	}
	fmt.Println("vedox doc agent removed for gemini")
	return nil
}

func runGeminiRepair(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	store, err := providers.NewReceiptStore("")
	if err != nil {
		return err
	}
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	installer, err := providers.NewGeminiInstaller("", daemonURL, ks, store)
	if err != nil {
		return err
	}

	if err := installer.Repair(cmd.Context()); err != nil {
		return fmt.Errorf("repair: %w", err)
	}
	fmt.Println("vedox doc agent repaired for gemini")
	return nil
}

// --- agent login ----------------------------------------------------------

var agentLoginFlags struct {
	provider string
}

var agentLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Mint a short-lived JWT for a bearer-token-only provider",
	Long: `Mint a short-lived (15 minute) JWT bound to the installed agent key.

Used for providers that cannot perform per-request HMAC signing — currently
only Copilot. The token is printed to stdout so you can pipe it to a
clipboard helper:

    vedox agent login --provider copilot | pbcopy

Other providers (claude, codex, gemini) sign each request with HMAC and do
not need a separate login step; running login for them returns an error.`,
	RunE: runAgentLogin,
}

func runAgentLogin(cmd *cobra.Command, _ []string) error {
	p := agentLoginFlags.provider
	if p == "" {
		return fmt.Errorf("--provider is required (currently only 'copilot' is supported)")
	}
	if providers.ProviderID(p) != providers.ProviderCopilot {
		return fmt.Errorf("provider %q does not support 'agent login'; only 'copilot' uses bearer-token auth", p)
	}

	cfg, err := config.LoadConfig(globalFlags.configPath)
	if err != nil {
		return err
	}
	ks, err := agentauth.LoadKeyStore(cfg.Workspace)
	if err != nil {
		return err
	}
	store, err := providers.NewReceiptStore("")
	if err != nil {
		return err
	}
	daemonURL := fmt.Sprintf("http://127.0.0.1:%d", cfg.Port)
	installer, err := providers.NewCopilotInstaller("", "", daemonURL, ks, store)
	if err != nil {
		return err
	}

	// Test for the optional Login capability. The compile-time assertion in
	// copilot_login.go guarantees Copilot satisfies it; this assertion exists
	// so the cmd layer fails cleanly if a future provider is rerouted here
	// without implementing the interface.
	auth, ok := installer.(providers.AgentAuthenticator)
	if !ok {
		return fmt.Errorf("internal: installer for %q does not implement AgentAuthenticator", p)
	}

	token, err := auth.Login(cmd.Context(), ks)
	if err != nil {
		return err
	}
	// Print only the token on stdout — no surrounding text — so the output
	// is pipe-safe (e.g. `vedox agent login --provider copilot | pbcopy`).
	fmt.Println(token)
	return nil
}

// --- agent list -----------------------------------------------------------

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all installed Doc Agent configurations",
	Long: `List every AI provider where the Vedox Doc Agent is currently installed.

Output includes the provider name, config file path, and whether the
agent entry is valid and up to date.`,
	RunE: runAgentList,
}

func runAgentList(_ *cobra.Command, _ []string) error {
	store, err := providers.NewReceiptStore("")
	if err != nil {
		return err
	}
	receipts, err := store.List()
	if err != nil {
		return err
	}
	if len(receipts) == 0 {
		fmt.Println("no doc agent installations found. run 'vedox agent install --provider <name>' to install.")
		return nil
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "PROVIDER\tVERSION\tKEY ID\tINSTALLED AT")
	for _, r := range receipts {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			r.Provider,
			r.Version,
			r.AuthKeyID,
			r.InstalledAt.Format("2006-01-02 15:04 UTC"),
		)
	}
	return tw.Flush()
}

func init() {
	// agent install flags — ValidArgs enables shell tab-completion for the enum
	agentInstallCmd.Flags().StringVar(&agentInstallFlags.provider, "provider", "",
		"AI provider to install into: claude, codex, copilot, gemini")
	if err := agentInstallCmd.RegisterFlagCompletionFunc("provider",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return validProviders, cobra.ShellCompDirectiveNoFileComp
		},
	); err != nil {
		panic(fmt.Sprintf("agent install: RegisterFlagCompletionFunc: %v", err))
	}

	// agent uninstall flags
	agentUninstallCmd.Flags().StringVar(&agentUninstallFlags.provider, "provider", "",
		"AI provider to remove the agent from: claude, codex, copilot, gemini")
	if err := agentUninstallCmd.RegisterFlagCompletionFunc("provider",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return validProviders, cobra.ShellCompDirectiveNoFileComp
		},
	); err != nil {
		panic(fmt.Sprintf("agent uninstall: RegisterFlagCompletionFunc: %v", err))
	}

	// agent repair flags
	agentRepairCmd.Flags().StringVar(&agentRepairFlags.provider, "provider", "",
		"AI provider to repair: claude, codex, copilot, gemini")
	if err := agentRepairCmd.RegisterFlagCompletionFunc("provider",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return validProviders, cobra.ShellCompDirectiveNoFileComp
		},
	); err != nil {
		panic(fmt.Sprintf("agent repair: RegisterFlagCompletionFunc: %v", err))
	}

	// agent login flags — only copilot is valid today; gate completion to that
	// single provider so users do not get tab-completed into a guaranteed-error
	// path for claude/codex/gemini.
	agentLoginCmd.Flags().StringVar(&agentLoginFlags.provider, "provider", "",
		"AI provider to mint a JWT for (currently only: copilot)")
	if err := agentLoginCmd.RegisterFlagCompletionFunc("provider",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return []string{"copilot"}, cobra.ShellCompDirectiveNoFileComp
		},
	); err != nil {
		panic(fmt.Sprintf("agent login: RegisterFlagCompletionFunc: %v", err))
	}

	// wire subcommands
	agentCmd.AddCommand(agentInstallCmd)
	agentCmd.AddCommand(agentUninstallCmd)
	agentCmd.AddCommand(agentRepairCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentLoginCmd)

	rootCmd.AddCommand(agentCmd)
}
