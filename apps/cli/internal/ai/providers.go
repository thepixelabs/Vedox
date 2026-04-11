package ai

import (
	"os/exec"
	"strings"
)

// ProviderID is the stable identifier for an AI CLI provider.
type ProviderID string

const (
	ProviderClaude  ProviderID = "claude"
	ProviderGemini  ProviderID = "gemini"
	ProviderCodex   ProviderID = "codex"
	ProviderCopilot ProviderID = "copilot"
)

// ProviderInfo describes an AI CLI provider and its availability on the host.
type ProviderInfo struct {
	ID        ProviderID `json:"id"`
	Name      string     `json:"name"`
	Available bool       `json:"available"`
	Version   string     `json:"version,omitempty"`
}

var providerDefs = []struct {
	id     ProviderID
	name   string
	binary string
}{
	{ProviderClaude, "Claude Code", "claude"},
	{ProviderGemini, "Gemini CLI", "gemini"},
	{ProviderCodex, "Codex CLI", "codex"},
	{ProviderCopilot, "GitHub Copilot", "copilot"},
}

// DetectAvailableProviders checks which AI CLI binaries are installed on PATH
// and returns a ProviderInfo slice for every known provider (available or not).
func DetectAvailableProviders() []ProviderInfo {
	results := make([]ProviderInfo, 0, len(providerDefs))
	for _, def := range providerDefs {
		info := ProviderInfo{
			ID:   def.id,
			Name: def.name,
		}
		if path, err := exec.LookPath(def.binary); err == nil {
			info.Available = true
			// Best-effort version detection; ignore errors so a CLI that does
			// not support --version (or exits non-zero) doesn't block the list.
			if out, err := exec.Command(path, "--version").Output(); err == nil {
				info.Version = strings.TrimSpace(string(out))
			}
		}
		results = append(results, info)
	}
	return results
}

// providerInvocation describes how a provider CLI is invoked for a one-shot prompt.
// Some CLIs accept the prompt as a positional argument; others read from stdin.
type providerInvocation struct {
	// args are the flags/arguments passed on the command line.
	// When useStdin is true, the prompt is NOT included here.
	// When useStdin is false, the prompt is appended as the last element.
	args     func(prompt string) []string
	useStdin bool // true = write prompt to cmd.Stdin; false = pass as CLI arg
}

var providerInvocations = map[ProviderID]providerInvocation{
	// Claude Code: `claude --print` reads the prompt from stdin.
	// Passing the prompt as a positional arg causes exit 1 ("no such file").
	ProviderClaude: {
		args:     func(_ string) []string { return []string{"--print"} },
		useStdin: true,
	},
	// Gemini CLI: accepts prompt via the -p flag.
	ProviderGemini: {
		args:     func(p string) []string { return []string{"-p", p} },
		useStdin: false,
	},
	// Codex CLI: first positional arg is the prompt.
	ProviderCodex: {
		args:     func(p string) []string { return []string{p, "--full-auto"} },
		useStdin: false,
	},
	// GitHub Copilot: suggest sub-command with prompt as first arg.
	ProviderCopilot: {
		args:     func(p string) []string { return []string{"suggest", p} },
		useStdin: false,
	},
}

// InvocationForProvider returns the CLI args and stdin flag for a provider+prompt.
// useStdin=true means the caller must write prompt to cmd.Stdin instead of argv.
// Returns nil args and false if the provider ID is unknown.
func InvocationForProvider(id ProviderID, prompt string) (args []string, useStdin bool) {
	inv, ok := providerInvocations[id]
	if !ok {
		return nil, false
	}
	return inv.args(prompt), inv.useStdin
}

// BinaryForProvider returns the binary name for the given provider.
// Returns an empty string if the provider ID is unknown.
func BinaryForProvider(id ProviderID) string {
	for _, def := range providerDefs {
		if def.id == id {
			return def.binary
		}
	}
	return ""
}
