package providers

// copilot.go — ProviderInstaller adapter for GitHub Copilot.
//
// Surfaces written:
//   - <projectRoot>/.github/copilot-instructions.md
//     (append/update a "## Vedox Documentation Agent" section)
//   - ~/.vedox/install-receipts/copilot.json  (managed by ReceiptStore)
//
// Degraded-mode rationale:
//   Copilot has no MCP tool surface and cannot call the Vedox daemon HTTP API
//   directly. The adapter installs prose routing rules so Copilot can read and
//   follow them, but it cannot execute HMAC-signed requests. An HMAC key is
//   issued and stored in the OS keychain for future use (when Copilot gains
//   tool support), but no key ID is embedded in the instruction file — there
//   is no tool surface to consume it. The InstallReceipt carries Degraded=true
//   (via Version suffix) to signal this limitation.
//
// Security: secrets are never written to disk. The keychain stores the HMAC
// secret; the receipt stores only the key ID.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vedox/vedox/internal/providers/templates"
)

const (
	// copilotAgentVersion is the instruction-pack version for Copilot.
	// The "+degraded" suffix signals read-only mode to tooling.
	copilotAgentVersion = "2.0+degraded"

	// copilotInstructionsFile is the path relative to the project root where
	// GitHub Copilot reads its custom instructions.
	copilotInstructionsFile = ".github/copilot-instructions.md"

	// copilotSectionStart / copilotSectionEnd delimit the Vedox-managed section
	// inside copilot-instructions.md.  Content outside the markers is never touched.
	copilotSectionStart = "<!-- vedox-copilot:start -->"
	copilotSectionEnd   = "<!-- vedox-copilot:end -->"

	// copilotSectionHeading is the H2 heading that identifies the Vedox block.
	copilotSectionHeading = "## Vedox Documentation Agent"
)

// copilotInstaller implements ProviderInstaller for GitHub Copilot.
type copilotInstaller struct {
	// projectRoot is the root of the project whose .github/ directory will be
	// written. Injected so tests can use a temp dir.
	projectRoot string

	// home is the user's home directory, used as the receipt boundary.
	home string

	// daemonURL is the HTTP base URL of the local Vedox daemon (stored for
	// prose reference; Copilot cannot call it directly in v2).
	daemonURL string

	// keys is the HMAC key issuer (agentauth.KeyStore or a test mock).
	keys KeyIssuer

	// receiptStore is used to load an existing receipt during Repair/Verify/Uninstall.
	receiptStore *ReceiptStore
}

// NewCopilotInstaller constructs a copilotInstaller.
//
//   - projectRoot is the Git project root that contains (or will contain)
//     .github/copilot-instructions.md.  May be empty to use the current
//     working directory.
//   - home may be empty to use os.UserHomeDir() (used as the receipt
//     boundary — copilot-instructions.md lives inside the project, so the
//     boundary is the project root, but the receipt lives under ~/.vedox).
//   - daemonURL is the URL of the Vedox daemon (e.g. "http://127.0.0.1:5150").
//   - keys is typically an agentauth.KeyStore.
func NewCopilotInstaller(projectRoot, home, daemonURL string, keys KeyIssuer, store *ReceiptStore) (ProviderInstaller, error) {
	if projectRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("copilot installer: resolve project root: %w", err)
		}
		projectRoot = cwd
	}
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("copilot installer: resolve home dir: %w", err)
		}
		home = h
	}
	if daemonURL == "" {
		daemonURL = "http://127.0.0.1:5150"
	}
	return &copilotInstaller{
		projectRoot:  projectRoot,
		home:         home,
		daemonURL:    daemonURL,
		keys:         keys,
		receiptStore: store,
	}, nil
}

// ── path helpers ─────────────────────────────────────────────────────────────

func (c *copilotInstaller) githubDir() string {
	return filepath.Join(c.projectRoot, ".github")
}

func (c *copilotInstaller) instructionsPath() string {
	return filepath.Join(c.projectRoot, copilotInstructionsFile)
}

// ── Probe ────────────────────────────────────────────────────────────────────

// Probe checks whether .github/copilot-instructions.md exists in the project
// root and whether it already contains a Vedox-managed section.
// No binary is required — Copilot is a cloud service with no local CLI.
func (c *copilotInstaller) Probe(_ context.Context) (*ProbeResult, error) {
	result := &ProbeResult{}

	path := c.instructionsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// File absent: not installed, no error.
			return result, nil
		}
		return nil, fmt.Errorf("copilot probe: read instructions file: %w", err)
	}

	result.ConfigPath = path
	result.SchemaHash = sha256Hex(data)
	result.Installed = bytes.Contains(data, []byte(copilotSectionStart))

	return result, nil
}

// ── Plan ─────────────────────────────────────────────────────────────────────

// Plan generates a single FileOp to create or update the Vedox section in
// .github/copilot-instructions.md.  No HMAC key ID is embedded in the file
// content — Copilot cannot use tool-call auth.  The key is issued in Install
// and stored in the keychain for future use only.
func (c *copilotInstaller) Plan(_ context.Context) (*InstallPlan, error) {
	content, err := c.buildInstructionsContent()
	if err != nil {
		return nil, fmt.Errorf("copilot plan: build instructions: %w", err)
	}

	ops := []FileOp{
		{
			Path:     c.instructionsPath(),
			Action:   OpUpdate,
			Content:  content,
			Boundary: c.projectRoot,
		},
	}

	return &InstallPlan{
		Provider: ProviderCopilot,
		FileOps:  ops,
		PlanHash: planHashOf(ops),
	}, nil
}

// ── Install ──────────────────────────────────────────────────────────────────

// Install executes the plan, issues an HMAC key for future use, and returns a
// receipt.  The receipt Version carries the "+degraded" suffix to signal that
// Copilot operates in read-only mode (no tool calls).
func (c *copilotInstaller) Install(_ context.Context, plan *InstallPlan) (*InstallReceipt, error) {
	// Issue key now — Copilot cannot use it today but we want it in the
	// keychain ready for when Copilot gains tool support.
	keyID, _, err := c.keys.IssueKey(
		"vedox-doc-agent-copilot",
		"", // project: user-scoped
		"", // pathPrefix: unrestricted
	)
	if err != nil {
		return nil, fmt.Errorf("copilot install: issue key: %w", err)
	}

	// The plan FileOps do not embed the key ID (no tool surface), so we
	// execute them as-is.
	hashes, err := executeFileOps(plan.FileOps)
	if err != nil {
		// Best-effort revoke if we could not write the file.
		_ = c.keys.RevokeKey(keyID)
		return nil, fmt.Errorf("copilot install: execute file ops: %w", err)
	}

	return &InstallReceipt{
		Provider:    ProviderCopilot,
		Version:     copilotAgentVersion,
		SchemaHash:  hashes[c.instructionsPath()],
		AuthKeyID:   keyID,
		DaemonURL:   c.daemonURL,
		FileHashes:  hashes,
		InstalledAt: time.Now().UTC(),
	}, nil
}

// ── Verify ───────────────────────────────────────────────────────────────────

// Verify re-reads every file in the receipt and reports drift via hash comparison.
func (c *copilotInstaller) Verify(_ context.Context, receipt *InstallReceipt) (*VerifyResult, error) {
	result := &VerifyResult{Healthy: true}

	for path, expectedHash := range receipt.FileHashes {
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				result.Healthy = false
				result.Drift = true
				result.Issues = append(result.Issues,
					fmt.Sprintf("file missing: %s", path))
				continue
			}
			return nil, fmt.Errorf("copilot verify: read %s: %w", path, err)
		}
		if actual := sha256Hex(data); actual != expectedHash {
			result.Drift = true
			result.Issues = append(result.Issues,
				fmt.Sprintf("file modified since install: %s", path))
		}
	}

	if result.Drift {
		result.Healthy = false
	}
	return result, nil
}

// ── Repair ───────────────────────────────────────────────────────────────────

// Repair reads the stored receipt, verifies current state, and re-runs Plan +
// Install if drift is detected.  The old HMAC key is revoked; a new one is issued.
func (c *copilotInstaller) Repair(ctx context.Context) error {
	if c.receiptStore == nil {
		return fmt.Errorf("copilot repair: no receipt store configured")
	}
	receipt, err := c.receiptStore.Load(ProviderCopilot)
	if err != nil {
		return fmt.Errorf("copilot repair: load receipt: %w", err)
	}
	if receipt != nil {
		v, err := c.Verify(ctx, receipt)
		if err != nil {
			return fmt.Errorf("copilot repair: verify: %w", err)
		}
		if v.Healthy {
			return nil // nothing to repair
		}
	}

	// Revoke old key before issuing a fresh one.
	if receipt != nil && receipt.AuthKeyID != "" {
		_ = c.keys.RevokeKey(receipt.AuthKeyID) // best-effort
	}

	plan, err := c.Plan(ctx)
	if err != nil {
		return fmt.Errorf("copilot repair: plan: %w", err)
	}
	newReceipt, err := c.Install(ctx, plan)
	if err != nil {
		return fmt.Errorf("copilot repair: install: %w", err)
	}
	return c.receiptStore.Save(newReceipt)
}

// ── Uninstall ────────────────────────────────────────────────────────────────

// Uninstall revokes the HMAC key and strips the Vedox section from
// copilot-instructions.md.  If the file becomes empty after stripping, it is
// removed entirely to avoid committing an empty instructions file.
func (c *copilotInstaller) Uninstall(_ context.Context) error {
	if c.receiptStore == nil {
		return fmt.Errorf("copilot uninstall: no receipt store configured")
	}
	receipt, err := c.receiptStore.Load(ProviderCopilot)
	if err != nil {
		return fmt.Errorf("copilot uninstall: load receipt: %w", err)
	}

	// Revoke key first.
	if receipt != nil && receipt.AuthKeyID != "" {
		if revokeErr := c.keys.RevokeKey(receipt.AuthKeyID); revokeErr != nil {
			return fmt.Errorf("copilot uninstall: revoke key: %w", revokeErr)
		}
	}

	// Strip Vedox section from copilot-instructions.md.
	if err := c.stripVedoxSection(); err != nil {
		return fmt.Errorf("copilot uninstall: strip instructions: %w", err)
	}

	_ = c.receiptStore.Delete(ProviderCopilot)
	return nil
}

// ── content builder ───────────────────────────────────────────────────────────

// buildInstructionsContent reads any existing copilot-instructions.md and
// returns the full updated content with the Vedox section present (idempotent).
// If the section already exists it is replaced to pick up any template changes.
func (c *copilotInstaller) buildInstructionsContent() ([]byte, error) {
	path := c.instructionsPath()
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	block := c.vedoxSection()

	if bytes.Contains(existing, []byte(copilotSectionStart)) {
		// Block present — replace in-place.
		return replaceCopilotSection(existing, block), nil
	}

	// Append.
	var buf bytes.Buffer
	buf.Write(existing)
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		buf.WriteByte('\n')
	}
	if len(existing) > 0 {
		buf.WriteByte('\n')
	}
	buf.WriteString(block)
	return buf.Bytes(), nil
}

// vedoxSection returns the Vedox-managed prose block, delimited by HTML
// comments so the installer can locate and update it deterministically.
//
// Degraded mode means Copilot reads these rules as plain instructions —
// it cannot call the Vedox daemon HTTP API directly. The routing rules are
// written as prose so Copilot follows them when composing documentation in
// the editor.
//
// The block body is loaded from the embedded templates/copilot.md file.
// {{DAEMON_URL}} is the only template variable substituted here.
func (c *copilotInstaller) vedoxSection() string {
	body := strings.ReplaceAll(templates.Copilot, "{{DAEMON_URL}}", c.daemonURL)

	return copilotSectionStart + "\n" +
		"<!-- This section is managed by Vedox. Do not edit manually. -->\n" +
		"<!-- Use 'vedox agent uninstall --provider copilot' to remove it. -->\n" +
		"\n" +
		body +
		"\n" +
		copilotSectionEnd + "\n"
}

// stripVedoxSection removes the Vedox section from copilot-instructions.md.
// If the file is empty or absent after stripping, the file is removed.
func (c *copilotInstaller) stripVedoxSection() error {
	path := c.instructionsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !bytes.Contains(data, []byte(copilotSectionStart)) {
		return nil // nothing to strip
	}

	stripped := removeCopilotSection(data)
	stripped = bytes.TrimSpace(stripped)

	if len(stripped) == 0 {
		// File would be empty — remove it to avoid committing an empty file.
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("remove empty instructions file: %w", removeErr)
		}
		return nil
	}

	// Restore trailing newline.
	stripped = append(stripped, '\n')
	return atomicFileWrite(c.projectRoot, path, stripped, 0o755, 0o644)
}

// ── block manipulation helpers ────────────────────────────────────────────────

// replaceCopilotSection replaces the Vedox section (inclusive of markers) with
// newBlock in data.
func replaceCopilotSection(data []byte, newBlock string) []byte {
	s := string(data)
	start := strings.Index(s, copilotSectionStart)
	if start < 0 {
		return data
	}
	end := strings.Index(s[start:], copilotSectionEnd)
	if end < 0 {
		return data
	}
	endAbs := start + end + len(copilotSectionEnd)
	if endAbs < len(s) && s[endAbs] == '\n' {
		endAbs++
	}
	var buf bytes.Buffer
	buf.WriteString(s[:start])
	buf.WriteString(newBlock)
	buf.WriteString(s[endAbs:])
	return buf.Bytes()
}

// removeCopilotSection strips the Vedox section and its leading blank line
// from data.
func removeCopilotSection(data []byte) []byte {
	s := string(data)
	start := strings.Index(s, copilotSectionStart)
	if start < 0 {
		return data
	}
	end := strings.Index(s[start:], copilotSectionEnd)
	if end < 0 {
		return data
	}
	endAbs := start + end + len(copilotSectionEnd)
	if endAbs < len(s) && s[endAbs] == '\n' {
		endAbs++
	}
	before := strings.TrimRight(s[:start], "\n")
	if len(before) > 0 {
		before += "\n"
	}
	return []byte(before + s[endAbs:])
}
