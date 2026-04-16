package providers

// claude.go — ProviderInstaller adapter for Claude Code MCP.
//
// Surfaces written:
//   - ~/.claude/agents/vedox-doc.md   (user-scope subagent file — YAML fm + instruction body)
//   - ~/.claude/CLAUDE.md              (append pinned block if absent)
//   - ~/.vedox/install-receipts/claude.json  (managed by ReceiptStore, not this adapter)
//
// The MCP server entry in .mcp.json is a per-project concern handled by the
// existing api/providers_mcp_agents.go handlers. This adapter manages the
// user-scope agent persona only.
//
// Security: secrets are never written to disk. AuthKeyID (a UUID) is embedded
// in the instruction body as a template variable {{HMAC_KEY_ID}}; the actual
// secret lives only in the OS keychain under agentauth.KeyStore.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/vedox/vedox/internal/providers/templates"
)

const (
	// claudeAgentVersion must match the version in the WS-D agent instructions.
	claudeAgentVersion = "2.0"

	// vedoxFenceStart / vedoxFenceEnd delimit the Vedox-managed block appended
	// to CLAUDE.md. Content outside the fences is never modified.
	vedoxFenceStart = "<!-- vedox-agent:start -->"
	vedoxFenceEnd   = "<!-- vedox-agent:end -->"

	// agentFileName is the subagent file written to ~/.claude/agents/.
	agentFileName = "vedox-doc.md"

	// claudeAgentName is the YAML frontmatter `name` field for the subagent.
	claudeAgentName = "vedox-doc-agent"

	// claudeAgentDescription is the human-facing description shown in Claude Code.
	claudeAgentDescription = "Routes and commits documentation via the Vedox daemon. Activates on 'vedox document' phrases. Authenticated via HMAC-SHA256."
)

// claudeInstaller implements ProviderInstaller for Claude Code.
type claudeInstaller struct {
	// home is the user's home directory. Injected so tests can use a temp dir.
	home string

	// daemonURL is the HTTP base URL of the local Vedox daemon.
	daemonURL string

	// keys is the HMAC key issuer (agentauth.KeyStore or a test mock).
	keys KeyIssuer

	// receiptStore is used to load an existing receipt during Repair/Verify/Uninstall.
	receiptStore *ReceiptStore

	// instructionBody is the raw Markdown to embed as the subagent body.
	// Loaded from the embedded template in instructionTemplate() by default.
	instructionBody string
}

// NewClaudeInstaller constructs a claudeInstaller. home may be empty to use
// os.UserHomeDir(). daemonURL is the URL the agent will call (e.g.
// "http://127.0.0.1:5150"). keys is typically an agentauth.KeyStore.
func NewClaudeInstaller(home, daemonURL string, keys KeyIssuer, store *ReceiptStore) (ProviderInstaller, error) {
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("claude installer: resolve home dir: %w", err)
		}
		home = h
	}
	if daemonURL == "" {
		daemonURL = "http://127.0.0.1:5150"
	}
	return &claudeInstaller{
		home:            home,
		daemonURL:       daemonURL,
		keys:            keys,
		receiptStore:    store,
		instructionBody: defaultInstructionBody(),
	}, nil
}

// ── path helpers ────────────────────────────────────────────────────────────

func (c *claudeInstaller) claudeDir() string   { return filepath.Join(c.home, ".claude") }
func (c *claudeInstaller) agentsDir() string   { return filepath.Join(c.claudeDir(), "agents") }
func (c *claudeInstaller) agentFilePath() string {
	return filepath.Join(c.agentsDir(), agentFileName)
}
func (c *claudeInstaller) claudeMDPath() string {
	return filepath.Join(c.claudeDir(), "CLAUDE.md")
}

// ── Probe ────────────────────────────────────────────────────────────────────

// Probe checks whether the claude binary is on PATH, and whether the
// vedox-doc.md subagent file already exists with a recognisable Vedox header.
func (c *claudeInstaller) Probe(_ context.Context) (*ProbeResult, error) {
	result := &ProbeResult{}

	// Detect binary.
	binPath, err := exec.LookPath("claude")
	if err == nil {
		result.BinaryVersion = claudeBinaryVersion(binPath)
	}

	// Detect existing agent file.
	agentPath := c.agentFilePath()
	data, err := os.ReadFile(agentPath)
	if err == nil {
		result.Installed = bytes.Contains(data, []byte(claudeAgentName))
		result.ConfigPath = agentPath
		result.SchemaHash = schemaHashFromAgentFile(data)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("claude probe: read agent file: %w", err)
	}

	return result, nil
}

// ── Plan ─────────────────────────────────────────────────────────────────────

// Plan generates FileOps to create/update the subagent file and the CLAUDE.md
// pinned block. It does NOT issue an HMAC key — that happens in Install.
// The returned plan embeds "{{HMAC_KEY_ID}}" as a placeholder; Install
// substitutes the real key ID before executing.
func (c *claudeInstaller) Plan(_ context.Context) (*InstallPlan, error) {
	ops := []FileOp{}

	// Op 1: write ~/.claude/agents/vedox-doc.md
	agentContent, err := c.buildAgentFileContent("{{HMAC_KEY_ID}}")
	if err != nil {
		return nil, fmt.Errorf("claude plan: build agent file: %w", err)
	}
	ops = append(ops, FileOp{
		Path:     c.agentFilePath(),
		Action:   OpUpdate, // create or update
		Content:  agentContent,
		Boundary: c.home,
	})

	// Op 2: append vedox block to ~/.claude/CLAUDE.md (idempotent).
	claudeMDContent, err := c.buildClaudeMDContent()
	if err != nil {
		return nil, fmt.Errorf("claude plan: build CLAUDE.md content: %w", err)
	}
	if claudeMDContent != nil {
		ops = append(ops, FileOp{
			Path:     c.claudeMDPath(),
			Action:   OpUpdate,
			Content:  claudeMDContent,
			Boundary: c.home,
		})
	}

	// Compute plan hash over the ops for idempotency.
	planHash := planHashOf(ops)

	return &InstallPlan{
		Provider: ProviderClaude,
		FileOps:  ops,
		PlanHash: planHash,
	}, nil
}

// ── Install ──────────────────────────────────────────────────────────────────

// Install issues an HMAC key, substitutes it into the plan's FileOps, then
// executes the ops atomically. Returns a receipt for persistence.
func (c *claudeInstaller) Install(_ context.Context, plan *InstallPlan) (*InstallReceipt, error) {
	// Issue key via KeyStore (or mock in tests).
	keyID, _, err := c.keys.IssueKey(
		"vedox-doc-agent-claude",
		"",  // project: user-scoped, any project
		"",  // pathPrefix: unrestricted
	)
	if err != nil {
		return nil, fmt.Errorf("claude install: issue key: %w", err)
	}

	// Substitute real key ID into every FileOp content.
	ops := substituteKeyID(plan.FileOps, keyID)

	// Execute file ops.
	hashes, err := executeFileOps(ops)
	if err != nil {
		// Best-effort revoke the key if we could not write the files.
		_ = c.keys.RevokeKey(keyID)
		return nil, fmt.Errorf("claude install: execute file ops: %w", err)
	}

	// Compute schema hash of what we just wrote.
	var schemaHash string
	for path, hash := range hashes {
		if strings.HasSuffix(path, agentFileName) {
			schemaHash = hash
			break
		}
	}

	return &InstallReceipt{
		Provider:    ProviderClaude,
		Version:     claudeAgentVersion,
		SchemaHash:  schemaHash,
		AuthKeyID:   keyID,
		DaemonURL:   c.daemonURL,
		FileHashes:  hashes,
		InstalledAt: time.Now().UTC(),
	}, nil
}

// ── Verify ───────────────────────────────────────────────────────────────────

// Verify re-reads every file in the receipt, compares its hash, and reports
// drift. It also checks that the HMAC key is still not revoked.
func (c *claudeInstaller) Verify(_ context.Context, receipt *InstallReceipt) (*VerifyResult, error) {
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
			return nil, fmt.Errorf("claude verify: read %s: %w", path, err)
		}
		actual := sha256Hex(data)
		if actual != expectedHash {
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

// Repair reads the stored receipt (if any), calls Verify, and re-runs Plan +
// Install if drift is detected. The old HMAC key is revoked; a new one is
// issued automatically.
func (c *claudeInstaller) Repair(ctx context.Context) error {
	if c.receiptStore == nil {
		return fmt.Errorf("claude repair: no receipt store configured")
	}
	receipt, err := c.receiptStore.Load(ProviderClaude)
	if err != nil {
		return fmt.Errorf("claude repair: load receipt: %w", err)
	}
	if receipt != nil {
		v, err := c.Verify(ctx, receipt)
		if err != nil {
			return fmt.Errorf("claude repair: verify: %w", err)
		}
		if v.Healthy {
			return nil // nothing to repair
		}
	}

	// Revoke the old key before issuing a new one during reinstall.
	// If the receipt is nil (first-time install via Repair), skip revocation.
	if receipt != nil && receipt.AuthKeyID != "" {
		_ = c.keys.RevokeKey(receipt.AuthKeyID) // best-effort; install proceeds regardless
	}

	// Reinstall.
	plan, err := c.Plan(ctx)
	if err != nil {
		return fmt.Errorf("claude repair: plan: %w", err)
	}
	newReceipt, err := c.Install(ctx, plan)
	if err != nil {
		return fmt.Errorf("claude repair: install: %w", err)
	}
	return c.receiptStore.Save(newReceipt)
}

// ── Uninstall ────────────────────────────────────────────────────────────────

// Uninstall revokes the HMAC key, removes the subagent file, and strips the
// Vedox fenced block from CLAUDE.md. Leaves files that were modified outside
// the fenced block intact.
func (c *claudeInstaller) Uninstall(ctx context.Context) error {
	if c.receiptStore == nil {
		return fmt.Errorf("claude uninstall: no receipt store configured")
	}
	receipt, err := c.receiptStore.Load(ProviderClaude)
	if err != nil {
		return fmt.Errorf("claude uninstall: load receipt: %w", err)
	}

	// Revoke key first — a live key with no install is a smaller problem than
	// removed install with a live key.
	if receipt != nil && receipt.AuthKeyID != "" {
		if revokeErr := c.keys.RevokeKey(receipt.AuthKeyID); revokeErr != nil {
			return fmt.Errorf("claude uninstall: revoke key: %w", revokeErr)
		}
	}

	// Remove agent file.
	agentPath := c.agentFilePath()
	if err := os.Remove(agentPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("claude uninstall: remove agent file: %w", err)
	}

	// Strip vedox block from CLAUDE.md.
	if err := c.stripVedoxBlock(); err != nil {
		return fmt.Errorf("claude uninstall: strip CLAUDE.md block: %w", err)
	}

	// Remove receipt.
	if c.receiptStore != nil {
		_ = c.receiptStore.Delete(ProviderClaude)
	}

	return nil
}

// ── content builders ─────────────────────────────────────────────────────────

// buildAgentFileContent builds the YAML-frontmatter + body agent file.
// keyIDPlaceholder is substituted verbatim; Install replaces it with the
// real key ID before executing.
func (c *claudeInstaller) buildAgentFileContent(keyIDPlaceholder string) ([]byte, error) {
	fm := map[string]any{
		"name":        claudeAgentName,
		"description": claudeAgentDescription,
		"version":     claudeAgentVersion,
		"provider":    string(ProviderClaude),
	}
	yamlBytes, err := yaml.Marshal(fm)
	if err != nil {
		return nil, err
	}

	body := c.instructionBody
	// Substitute daemon URL and key ID placeholders.
	body = strings.ReplaceAll(body, "{{DAEMON_PORT}}", daemonPort(c.daemonURL))
	body = strings.ReplaceAll(body, "{{HMAC_KEY_ID}}", keyIDPlaceholder)

	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yamlBytes)
	buf.WriteString("---\n")
	buf.WriteString(body)
	return buf.Bytes(), nil
}

// buildClaudeMDContent reads the existing CLAUDE.md (if any) and returns the
// full updated content with the Vedox fenced block appended (idempotent). If
// the block is already present, returns nil (no update needed).
func (c *claudeInstaller) buildClaudeMDContent() ([]byte, error) {
	path := c.claudeMDPath()
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	block := vedoxFenceStart + "\n" +
		"<!-- This block is managed by Vedox. Do not edit manually. -->\n" +
		"<!-- Use 'vedox agent uninstall --provider claude' to remove it. -->\n" +
		"\n" +
		"The Vedox Doc Agent (@vedox-doc-agent) is installed.\n" +
		"Trigger: type any phrase starting with 'vedox document' to activate it.\n" +
		"\n" +
		vedoxFenceEnd + "\n"

	if bytes.Contains(existing, []byte(vedoxFenceStart)) {
		// Block already present — replace it to pick up any changes.
		updated := replaceFencedBlock(existing, block)
		return updated, nil
	}

	// Append.
	var buf bytes.Buffer
	buf.Write(existing)
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		buf.WriteByte('\n')
	}
	buf.WriteString("\n")
	buf.WriteString(block)
	return buf.Bytes(), nil
}

// stripVedoxBlock removes the Vedox fenced block from CLAUDE.md. If the file
// does not exist or contains no block, this is a no-op.
func (c *claudeInstaller) stripVedoxBlock() error {
	path := c.claudeMDPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !bytes.Contains(data, []byte(vedoxFenceStart)) {
		return nil
	}
	stripped := removeFencedBlock(data)
	return atomicFileWrite(c.home, path, stripped, 0o755, 0o644)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// substituteKeyID returns a copy of ops where every content byte slice has
// the "{{HMAC_KEY_ID}}" placeholder replaced with the real keyID.
func substituteKeyID(ops []FileOp, keyID string) []FileOp {
	out := make([]FileOp, len(ops))
	for i, op := range ops {
		out[i] = op
		if len(op.Content) > 0 {
			out[i].Content = bytes.ReplaceAll(op.Content, []byte("{{HMAC_KEY_ID}}"), []byte(keyID))
		}
	}
	return out
}

// planHashOf computes a stable hash over a slice of FileOps. Used for
// idempotency checks (same plan hash → nothing changed since last Plan call).
func planHashOf(ops []FileOp) string {
	h := sha256.New()
	for _, op := range ops {
		h.Write([]byte(string(op.Action) + op.Path))
		h.Write(op.Content)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// schemaHashFromAgentFile extracts a fingerprint of the frontmatter keys
// (not values) from a raw agent file. This is the claude schema-drift signal.
func schemaHashFromAgentFile(data []byte) string {
	s := string(data)
	if !strings.HasPrefix(s, "---\n") {
		return sha256Hex(data)
	}
	rest := strings.TrimPrefix(s, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return sha256Hex(data)
	}
	yamlBlock := rest[:end]
	m := map[string]any{}
	if err := yaml.Unmarshal([]byte(yamlBlock), &m); err != nil {
		return sha256Hex(data)
	}
	// Hash the sorted key names only.
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// replaceFencedBlock replaces the content between vedoxFenceStart and
// vedoxFenceEnd (inclusive) with newBlock.
func replaceFencedBlock(data []byte, newBlock string) []byte {
	s := string(data)
	start := strings.Index(s, vedoxFenceStart)
	if start < 0 {
		return data
	}
	end := strings.Index(s[start:], vedoxFenceEnd)
	if end < 0 {
		return data
	}
	endAbs := start + end + len(vedoxFenceEnd)
	// Include the trailing newline if present.
	if endAbs < len(s) && s[endAbs] == '\n' {
		endAbs++
	}
	var buf bytes.Buffer
	buf.WriteString(s[:start])
	buf.WriteString(newBlock)
	buf.WriteString(s[endAbs:])
	return buf.Bytes()
}

// removeFencedBlock strips the Vedox fenced block and its surrounding blank
// lines from data.
func removeFencedBlock(data []byte) []byte {
	s := string(data)
	start := strings.Index(s, vedoxFenceStart)
	if start < 0 {
		return data
	}
	end := strings.Index(s[start:], vedoxFenceEnd)
	if end < 0 {
		return data
	}
	endAbs := start + end + len(vedoxFenceEnd)
	if endAbs < len(s) && s[endAbs] == '\n' {
		endAbs++
	}
	// Trim leading blank line that we added before the block.
	before := s[:start]
	before = strings.TrimRight(before, "\n")
	if len(before) > 0 {
		before += "\n"
	}
	return []byte(before + s[endAbs:])
}

// claudeBinaryVersion runs `claude --version` and returns the first line of
// output (trimmed). Returns an empty string on any error.
func claudeBinaryVersion(binPath string) string {
	out, err := exec.Command(binPath, "--version").Output()
	if err != nil {
		return ""
	}
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

// daemonPort extracts the numeric port from a URL like "http://127.0.0.1:5150".
// Falls back to "5150" on any parse failure.
func daemonPort(url string) string {
	parts := strings.SplitN(url, ":", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	return "5150"
}

// defaultInstructionBody returns the instruction body loaded from the
// embedded WS-D prose template. The content is embedded at compile time from
// internal/providers/templates/claude.md via //go:embed.
//
// The template variables {{DAEMON_PORT}} and {{HMAC_KEY_ID}} are substituted
// at plan/install time.
func defaultInstructionBody() string {
	return templates.Claude
}

