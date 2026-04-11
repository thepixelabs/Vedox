package ai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// DefaultTimeout is the maximum wall-clock time allowed for a single AI CLI call.
const DefaultTimeout = 45 * time.Second

// GenerationRequest is the full specification for a name generation run.
type GenerationRequest struct {
	Provider    ProviderID
	AccountName string // empty = use system CLI directly (no AlterGo account override)
	Params      GenerationParams
	Count       int
	Refinement  *RefinementInput
	Timeout     time.Duration // 0 = use DefaultTimeout
}

// GenerationResult carries the output of a successful generation run.
type GenerationResult struct {
	Names        []string `json:"names"`
	ProviderUsed string   `json:"providerUsed"`
	AccountUsed  string   `json:"accountUsed,omitempty"`
	DurationMs   int64    `json:"durationMs"`
}

// RunGeneration executes the AI CLI for req and returns parsed name suggestions.
// The caller-supplied ctx is respected; an additional per-request deadline is
// layered on top via req.Timeout (defaulting to DefaultTimeout).
func RunGeneration(ctx context.Context, req GenerationRequest) (*GenerationResult, error) {
	timeout := req.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()

	binary := BinaryForProvider(req.Provider)
	if binary == "" {
		return nil, fmt.Errorf("unknown provider: %s", req.Provider)
	}

	// Fail early with a clear message if the binary isn't installed.
	binaryPath, err := exec.LookPath(binary)
	if err != nil {
		return nil, fmt.Errorf("AI CLI %q not found on PATH: %w", binary, err)
	}

	// Cap count at 20 per invocation for reliable output format.
	count := req.Count
	if count > 20 {
		count = 20
	}
	if count < 1 {
		count = 10
	}

	prompt := BuildPrompt(req.Params, count, req.Refinement)
	args, useStdin := InvocationForProvider(req.Provider, prompt)
	if args == nil {
		return nil, fmt.Errorf("unknown provider: %s", req.Provider)
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	if useStdin {
		cmd.Stdin = strings.NewReader(prompt)
	}

	// Set a new process group so we can kill the entire child tree on timeout,
	// preventing orphaned subprocesses from accumulating.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// Build environment: start from the current process env, then override HOME
	// when an AlterGo account name is provided so the AI CLI picks up the right
	// credentials from ~/.altergo/accounts/<name>/.
	env := os.Environ()
	accountUsed := req.AccountName
	if req.AccountName != "" {
		accountHome := AccountHome(req.AccountName)
		if accountHome != "" {
			env = overrideEnv(env, "HOME", accountHome)
		}
	}
	cmd.Env = env

	out, err := cmd.Output()
	if err != nil {
		// Check timeout before anything else — the exit error and the context
		// error coexist; we want the more user-friendly message.
		if ctx.Err() == context.DeadlineExceeded {
			// Kill the entire process group to avoid leaving orphans.
			if cmd.Process != nil {
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
			return nil, fmt.Errorf("AI CLI timed out after %s", timeout)
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr == "" {
				stderr = fmt.Sprintf("exited with code %d", exitErr.ExitCode())
			}
			return nil, fmt.Errorf("AI CLI error: %s", stderr)
		}
		return nil, fmt.Errorf("failed to run AI CLI: %w", err)
	}

	names := ParseNames(string(out))

	return &GenerationResult{
		Names:        names,
		ProviderUsed: string(req.Provider),
		AccountUsed:  accountUsed,
		DurationMs:   time.Since(start).Milliseconds(),
	}, nil
}

// overrideEnv returns a copy of env with the given key set to value.
// If the key already exists it is replaced in-place; otherwise it is appended.
func overrideEnv(env []string, key, value string) []string {
	prefix := key + "="
	result := make([]string, 0, len(env))
	replaced := false
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			result = append(result, prefix+value)
			replaced = true
		} else {
			result = append(result, e)
		}
	}
	if !replaced {
		result = append(result, prefix+value)
	}
	return result
}
