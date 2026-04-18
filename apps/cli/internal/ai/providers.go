package ai

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// probeTimeout is the per-binary deadline for `--version` probes inside
// DetectAvailableProviders. Two seconds is long enough for any well-behaved
// CLI on a loaded machine and short enough that a hung binary (e.g. one that
// prompts for auth) does not block the caller indefinitely.
const probeTimeout = 2 * time.Second

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
			// Best-effort version probe with a hard wall-clock deadline.
			//
			// We run the binary in its own process group (Setpgid) so we can
			// kill all its descendants — not just the top-level process — when
			// the timeout fires. Without this, a shell-script stub that spawns
			// `sleep` keeps the stdout pipe alive after the shell is killed,
			// and Output() blocks until the grandchild finally exits.
			//
			// The probe is run in a goroutine so the outer timer can fire a
			// SIGKILL to the process group and then we just discard the result.
			// On timeout or any error we leave Version empty; the provider is
			// still marked Available=true because the binary is on PATH.
			info.Version = probeVersion(path)
		}
		results = append(results, info)
	}
	return results
}

// probeVersion runs `binary --version` with a hard probeTimeout deadline and
// returns the trimmed stdout output, or an empty string on any error or timeout.
//
// The child process is placed in its own process group (SysProcAttr.Setpgid).
// When the context deadline fires, we send SIGKILL to the entire process group
// via syscall.Kill(-pgid, SIGKILL). This ensures that shell-script launchers
// (which spawn child processes such as `sleep`) cannot hold stdout pipes open
// after the parent has been killed, which would cause Output() to block for the
// duration of the longest grandchild.
func probeVersion(binary string) string {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "--version")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Start(); err != nil {
		return ""
	}

	// Watch for context cancellation in a goroutine. When the deadline fires,
	// kill the entire process group so all descendants are reaped immediately
	// and the stdout pipe is released.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			if cmd.Process != nil {
				// Negative PID targets the process group.
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		case <-done:
		}
	}()

	if err := cmd.Wait(); err != nil {
		return ""
	}
	return strings.TrimSpace(stdout.String())
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
