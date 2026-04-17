package ai

// Tests for providers.go pure functions — InvocationForProvider,
// BinaryForProvider, and overrideEnv. Previously 0% covered.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestInvocationForProvider_AllKnownProviders verifies that every declared
// provider ID produces a non-nil argv and that each provider's stdin policy
// matches the provider's documented CLI convention. A regression here would
// silently corrupt generation output (wrong flag, prompt missing, etc.).
func TestInvocationForProvider_AllKnownProviders(t *testing.T) {
	prompt := "invent 3 names"

	// Expected per-provider assertions; grouped in one test to keep the
	// argv matrix in a single readable place. Each row = one CLI contract.
	cases := []struct {
		id               ProviderID
		wantUseStdin     bool
		wantFirstArg     string // first argv element
		promptInArgv     bool   // whether prompt must appear in argv
		mustNotHavePrompt bool  // claude uses stdin; prompt must be absent
	}{
		{ProviderClaude, true, "--print", false, true},
		{ProviderGemini, false, "-p", true, false},
		{ProviderCodex, false, prompt, true, false},
		{ProviderCopilot, false, "suggest", true, false},
	}
	for _, c := range cases {
		t.Run(string(c.id), func(t *testing.T) {
			args, useStdin := InvocationForProvider(c.id, prompt)
			if args == nil {
				t.Fatalf("InvocationForProvider(%q): args=nil, want non-nil", c.id)
			}
			if useStdin != c.wantUseStdin {
				t.Errorf("useStdin = %v, want %v", useStdin, c.wantUseStdin)
			}
			if args[0] != c.wantFirstArg {
				t.Errorf("first arg = %q, want %q; full=%v", args[0], c.wantFirstArg, args)
			}
			joined := strings.Join(args, " ")
			if c.promptInArgv && !strings.Contains(joined, prompt) {
				t.Errorf("prompt %q missing from argv: %v", prompt, args)
			}
			if c.mustNotHavePrompt && strings.Contains(joined, prompt) {
				t.Errorf("argv unexpectedly contained prompt (stdin provider): %v", args)
			}
		})
	}
}

// TestInvocationForProvider_UnknownID_ReturnsNil exercises the unknown-ID
// branch of InvocationForProvider. Callers rely on args==nil to short-circuit.
func TestInvocationForProvider_UnknownID_ReturnsNil(t *testing.T) {
	args, useStdin := InvocationForProvider(ProviderID("not-a-real-provider"), "x")
	if args != nil {
		t.Errorf("args = %v, want nil for unknown provider", args)
	}
	if useStdin {
		t.Errorf("useStdin = true, want false for unknown provider")
	}
}

// TestBinaryForProvider_Known asserts each provider maps to a stable binary
// name. The provider installer (/api/agents/install) uses this to resolve the
// CLI path; a typo here breaks onboarding for that provider.
func TestBinaryForProvider_Known(t *testing.T) {
	cases := map[ProviderID]string{
		ProviderClaude:  "claude",
		ProviderGemini:  "gemini",
		ProviderCodex:   "codex",
		ProviderCopilot: "copilot",
	}
	for id, want := range cases {
		if got := BinaryForProvider(id); got != want {
			t.Errorf("BinaryForProvider(%q) = %q, want %q", id, got, want)
		}
	}
}

// TestBinaryForProvider_Unknown returns "" for an unrecognised provider.
func TestBinaryForProvider_Unknown(t *testing.T) {
	if got := BinaryForProvider(ProviderID("xyzzy")); got != "" {
		t.Errorf("BinaryForProvider(xyzzy) = %q, want empty", got)
	}
}

// ---- overrideEnv ----

// TestOverrideEnv_AppendsWhenMissing verifies the append branch: if the key
// is not present in the input env, it is appended.
func TestOverrideEnv_AppendsWhenMissing(t *testing.T) {
	in := []string{"PATH=/usr/bin", "HOME=/tmp"}
	out := overrideEnv(in, "HF_HOME", "/opt/hf")

	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3; out=%v", len(out), out)
	}
	if out[2] != "HF_HOME=/opt/hf" {
		t.Errorf("out[2] = %q, want HF_HOME=/opt/hf", out[2])
	}
	// Must not mutate the input slice.
	if len(in) != 2 {
		t.Errorf("input slice was mutated: %v", in)
	}
}

// TestOverrideEnv_ReplacesExisting verifies the in-place replace branch.
// The key's position in the list must be preserved so callers who rely on
// PATH being first (etc.) still work.
func TestOverrideEnv_ReplacesExisting(t *testing.T) {
	in := []string{"PATH=/usr/bin", "HOME=/old", "OTHER=x"}
	out := overrideEnv(in, "HOME", "/new")

	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3 (replace, not append); out=%v", len(out), out)
	}
	if out[1] != "HOME=/new" {
		t.Errorf("out[1] = %q, want HOME=/new", out[1])
	}
	if out[0] != "PATH=/usr/bin" || out[2] != "OTHER=x" {
		t.Errorf("neighbouring entries changed: out=%v", out)
	}
}

// TestOverrideEnv_ReplacesAllMatchingKeys documents current behaviour: if the
// key appears more than once in the input (a malformed env), every copy is
// overwritten. This is conservative — no stale value survives.
func TestOverrideEnv_ReplacesAllMatchingKeys(t *testing.T) {
	in := []string{"DUP=a", "OTHER=y", "DUP=b"}
	out := overrideEnv(in, "DUP", "z")

	// Both DUP entries must now be "z".
	count := 0
	for _, e := range out {
		if e == "DUP=z" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("want 2× DUP=z, got %d; out=%v", count, out)
	}
}

// TestOverrideEnv_PrefixCollisionIsExact guards against a subtle bug:
// "HOMEBREW_PREFIX" must not be treated as the "HOME" key. The prefix check
// must require the '=' character.
func TestOverrideEnv_PrefixCollisionIsExact(t *testing.T) {
	in := []string{"HOMEBREW_PREFIX=/usr/local", "HOME=/tmp"}
	out := overrideEnv(in, "HOME", "/new")

	// HOMEBREW_PREFIX must be unchanged.
	foundBrew := false
	foundHome := false
	for _, e := range out {
		if e == "HOMEBREW_PREFIX=/usr/local" {
			foundBrew = true
		}
		if e == "HOME=/new" {
			foundHome = true
		}
	}
	if !foundBrew {
		t.Errorf("HOMEBREW_PREFIX was incorrectly modified: %v", out)
	}
	if !foundHome {
		t.Errorf("HOME was not replaced: %v", out)
	}
}

// TestOverrideEnv_EmptyInput appends to an empty env without panicking.
func TestOverrideEnv_EmptyInput(t *testing.T) {
	out := overrideEnv(nil, "FOO", "bar")
	if len(out) != 1 || out[0] != "FOO=bar" {
		t.Errorf("overrideEnv(nil, FOO, bar) = %v, want [FOO=bar]", out)
	}
}

// TestProbeVersion_HangingBinaryTimesOut verifies that probeVersion enforces
// the 2-second probeTimeout even when the binary forks long-lived children.
//
// The stub is a shell script that does nothing but `sleep 10`. After the
// parent shell is killed, its child `sleep` process holds the stdout pipe
// open. Without the process-group SIGKILL in probeVersion, Output() would
// block for the full 10 seconds (or until the OS reaps the orphan).
//
// We assert that probeVersion returns within 3 seconds — 1 second of
// headroom above the 2-second probeTimeout — and that the returned string
// is empty because the process never printed anything before it was killed.
func TestProbeVersion_HangingBinaryTimesOut(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stub shell script not supported on Windows")
	}

	// Write a shell script that sleeps far beyond the probe timeout.
	dir := t.TempDir()
	stub := filepath.Join(dir, "hanging-provider")
	script := fmt.Sprintf("#!/bin/sh\nsleep 10\n")
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile stub: %v", err)
	}

	const deadline = 3 * time.Second
	start := time.Now()
	version := probeVersion(stub)
	elapsed := time.Since(start)

	if elapsed > deadline {
		t.Errorf("probeVersion took %v, want ≤ %v (probe timeout not enforced)", elapsed, deadline)
	}
	if version != "" {
		t.Errorf("probeVersion = %q, want empty (stub never printed before killed)", version)
	}
}
