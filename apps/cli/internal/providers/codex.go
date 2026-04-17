package providers

// codex.go — ProviderInstaller adapter for OpenAI Codex CLI.
//
// Surfaces written:
//   - ~/.codex/config.toml      (add [mcp_servers.vedox] entry via typed TOML write)
//   - ~/.codex/AGENTS.md        (append Vedox fenced block — Codex honours home AGENTS.md)
//   - ~/.vedox/install-receipts/codex.json  (managed by ReceiptStore, not this adapter)
//
// Config path selection:
//   Primary:  ~/.codex/config.toml   (Codex CLI standard)
//   Fallback: ~/.config/codex/config.toml  (XDG-compliant path used by some distros)
//   If neither exists, the adapter creates ~/.codex/config.toml.
//
// Security: secrets are never written to disk. AuthKeyID (a UUID) is embedded
// in the AGENTS.md fenced block as a comment; the actual secret lives only in
// the OS keychain under agentauth.KeyStore. The TOML mcp_servers entry contains
// only the daemon URL and the key ID — not the HMAC secret.
//
// TOML format written into mcp_servers.vedox:
//   [mcp_servers.vedox]
//   url       = "http://127.0.0.1:<PORT>"
//   key_id    = "<HMAC_KEY_ID>"
//   transport = "http"

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/vedox/vedox/internal/providers/templates"
)

const (
	// codexAgentVersion must match the version in the WS-D agent instructions.
	codexAgentVersion = "2.0"

	// codexMCPServerKey is the key used under [mcp_servers] in config.toml.
	codexMCPServerKey = "vedox"

	// codexAgentsMDFileName is the Codex per-user agent instructions file.
	codexAgentsMDFileName = "AGENTS.md"

	// codexVedoxFenceStart / codexVedoxFenceEnd delimit the Vedox block in AGENTS.md.
	codexVedoxFenceStart = "<!-- vedox-agent:start -->"
	codexVedoxFenceEnd   = "<!-- vedox-agent:end -->"
)

// codexInstaller implements ProviderInstaller for OpenAI Codex CLI.
type codexInstaller struct {
	// home is the user's home directory. Injected so tests can use a temp dir.
	home string

	// daemonURL is the HTTP base URL of the local Vedox daemon.
	daemonURL string

	// keys is the HMAC key issuer (agentauth.KeyStore or a test mock).
	keys KeyIssuer

	// receiptStore is used to load an existing receipt during Repair/Verify/Uninstall.
	receiptStore *ReceiptStore
}

// NewCodexInstaller constructs a codexInstaller. home may be empty to use
// os.UserHomeDir(). daemonURL is the URL the agent will call (e.g.
// "http://127.0.0.1:5150"). keys is typically an agentauth.KeyStore.
func NewCodexInstaller(home, daemonURL string, keys KeyIssuer, store *ReceiptStore) (ProviderInstaller, error) {
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("codex installer: resolve home dir: %w", err)
		}
		home = h
	}
	if daemonURL == "" {
		daemonURL = "http://127.0.0.1:5150"
	}
	return &codexInstaller{
		home:         home,
		daemonURL:    daemonURL,
		keys:         keys,
		receiptStore: store,
	}, nil
}

// ── path helpers ────────────────────────────────────────────────────────────

// codexDir returns the primary ~/.codex directory.
func (c *codexInstaller) codexDir() string { return filepath.Join(c.home, ".codex") }

// xdgCodexDir returns the XDG fallback path (~/.config/codex).
func (c *codexInstaller) xdgCodexDir() string {
	return filepath.Join(c.home, ".config", "codex")
}

// resolveConfigPath returns the path to use for config.toml.
// It prefers ~/.codex/config.toml if either ~/.codex/ exists or neither
// candidate directory exists (creation path). Falls back to
// ~/.config/codex/config.toml if that directory already exists and
// ~/.codex/ does not.
func (c *codexInstaller) resolveConfigPath() string {
	primary := filepath.Join(c.codexDir(), "config.toml")
	xdg := filepath.Join(c.xdgCodexDir(), "config.toml")

	if _, err := os.Stat(c.codexDir()); err == nil {
		return primary
	}
	if _, err := os.Stat(c.xdgCodexDir()); err == nil {
		return xdg
	}
	// Neither exists — default to primary; Install will create it.
	return primary
}

// agentsMDPath returns the path to the Codex AGENTS.md file, co-located with
// config.toml in the resolved codex config directory.
func (c *codexInstaller) agentsMDPath() string {
	return filepath.Join(filepath.Dir(c.resolveConfigPath()), codexAgentsMDFileName)
}

// ── Probe ────────────────────────────────────────────────────────────────────

// Probe checks whether the codex binary is on PATH, detects the config path,
// and checks whether a Vedox mcp_servers entry already exists.
func (c *codexInstaller) Probe(_ context.Context) (*ProbeResult, error) {
	result := &ProbeResult{}

	// Detect binary.
	binPath, err := exec.LookPath("codex")
	if err == nil {
		result.BinaryVersion = codexBinaryVersion(binPath)
	}

	// Detect existing config and Vedox entry.
	configPath := c.resolveConfigPath()
	data, err := os.ReadFile(configPath)
	if err == nil {
		result.ConfigPath = configPath
		result.SchemaHash = codexSchemaHash(data)

		// Parse TOML to check for mcp_servers.vedox.
		m := map[string]any{}
		if _, tomlErr := toml.Decode(string(data), &m); tomlErr == nil {
			if servers, ok := m["mcp_servers"].(map[string]any); ok {
				if _, hasVedox := servers[codexMCPServerKey]; hasVedox {
					result.Installed = true
				}
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("codex probe: read config: %w", err)
	}

	// Also check AGENTS.md for Vedox fence marker (belt-and-suspenders).
	agentsPath := c.agentsMDPath()
	agentsData, agentsErr := os.ReadFile(agentsPath)
	if agentsErr == nil && bytes.Contains(agentsData, []byte(codexVedoxFenceStart)) {
		if !result.Installed {
			result.Installed = true
		}
		if result.ConfigPath == "" {
			result.ConfigPath = agentsPath
		}
	}

	return result, nil
}

// ── Plan ─────────────────────────────────────────────────────────────────────

// Plan generates FileOps to write the mcp_servers.vedox entry into
// config.toml and append the Vedox fenced block into AGENTS.md.
// The returned plan embeds "{{HMAC_KEY_ID}}" as a placeholder; Install
// substitutes the real key ID before executing.
func (c *codexInstaller) Plan(_ context.Context) (*InstallPlan, error) {
	ops := []FileOp{}

	// Op 1: write mcp_servers.vedox into config.toml.
	configPath := c.resolveConfigPath()
	configContent, err := c.buildConfigTOML("{{HMAC_KEY_ID}}")
	if err != nil {
		return nil, fmt.Errorf("codex plan: build config.toml: %w", err)
	}
	ops = append(ops, FileOp{
		Path:     configPath,
		Action:   OpUpdate,
		Content:  configContent,
		Boundary: c.home,
	})

	// Op 2: append/update Vedox block in AGENTS.md.
	agentsMDContent, err := c.buildAgentsMDContent("{{HMAC_KEY_ID}}")
	if err != nil {
		return nil, fmt.Errorf("codex plan: build AGENTS.md: %w", err)
	}
	ops = append(ops, FileOp{
		Path:     c.agentsMDPath(),
		Action:   OpUpdate,
		Content:  agentsMDContent,
		Boundary: c.home,
	})

	return &InstallPlan{
		Provider: ProviderCodex,
		FileOps:  ops,
		PlanHash: planHashOf(ops),
	}, nil
}

// ── Install ──────────────────────────────────────────────────────────────────

// Install issues an HMAC key, substitutes it into the plan's FileOps, then
// executes the ops atomically. Returns a receipt for persistence.
func (c *codexInstaller) Install(_ context.Context, plan *InstallPlan) (*InstallReceipt, error) {
	// Issue key via KeyStore (or mock in tests).
	keyID, _, err := c.keys.IssueKey(
		"vedox-doc-agent-codex",
		"",  // project: user-scoped
		"",  // pathPrefix: unrestricted
	)
	if err != nil {
		return nil, fmt.Errorf("codex install: issue key: %w", err)
	}

	// Substitute real key ID into every FileOp content.
	ops := substituteKeyID(plan.FileOps, keyID)

	// Execute file ops.
	hashes, err := executeFileOps(ops)
	if err != nil {
		// Best-effort revoke the key if file writes failed.
		_ = c.keys.RevokeKey(keyID)
		return nil, fmt.Errorf("codex install: execute file ops: %w", err)
	}

	// Use the config.toml hash as the canonical schema hash.
	configPath := c.resolveConfigPath()
	schemaHash := hashes[configPath]

	return &InstallReceipt{
		Provider:    ProviderCodex,
		Version:     codexAgentVersion,
		SchemaHash:  schemaHash,
		AuthKeyID:   keyID,
		DaemonURL:   c.daemonURL,
		FileHashes:  hashes,
		InstalledAt: time.Now().UTC(),
	}, nil
}

// ── Verify ───────────────────────────────────────────────────────────────────

// Verify re-reads every file in the receipt, compares its hash, and reports drift.
func (c *codexInstaller) Verify(_ context.Context, receipt *InstallReceipt) (*VerifyResult, error) {
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
			return nil, fmt.Errorf("codex verify: read %s: %w", path, err)
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
// Install if drift is detected. The old HMAC key is revoked; a new one is issued.
func (c *codexInstaller) Repair(ctx context.Context) error {
	if c.receiptStore == nil {
		return fmt.Errorf("codex repair: no receipt store configured")
	}
	receipt, err := c.receiptStore.Load(ProviderCodex)
	if err != nil {
		return fmt.Errorf("codex repair: load receipt: %w", err)
	}
	if receipt != nil {
		v, err := c.Verify(ctx, receipt)
		if err != nil {
			return fmt.Errorf("codex repair: verify: %w", err)
		}
		if v.Healthy {
			return nil // nothing to repair
		}
	}

	// Revoke old key before issuing a new one.
	if receipt != nil && receipt.AuthKeyID != "" {
		_ = c.keys.RevokeKey(receipt.AuthKeyID) // best-effort
	}

	plan, err := c.Plan(ctx)
	if err != nil {
		return fmt.Errorf("codex repair: plan: %w", err)
	}
	newReceipt, err := c.Install(ctx, plan)
	if err != nil {
		return fmt.Errorf("codex repair: install: %w", err)
	}
	return c.receiptStore.Save(newReceipt)
}

// ── Uninstall ────────────────────────────────────────────────────────────────

// Uninstall revokes the HMAC key, removes the mcp_servers.vedox entry from
// config.toml, and strips the Vedox fenced block from AGENTS.md.
func (c *codexInstaller) Uninstall(_ context.Context) error {
	if c.receiptStore == nil {
		return fmt.Errorf("codex uninstall: no receipt store configured")
	}
	receipt, err := c.receiptStore.Load(ProviderCodex)
	if err != nil {
		return fmt.Errorf("codex uninstall: load receipt: %w", err)
	}

	// Revoke key first — a live key with no install is less harmful than the reverse.
	if receipt != nil && receipt.AuthKeyID != "" {
		if revokeErr := c.keys.RevokeKey(receipt.AuthKeyID); revokeErr != nil {
			return fmt.Errorf("codex uninstall: revoke key: %w", revokeErr)
		}
	}

	// Strip mcp_servers.vedox from config.toml.
	if err := c.stripCodexMCPEntry(); err != nil {
		return fmt.Errorf("codex uninstall: strip config.toml: %w", err)
	}

	// Strip Vedox block from AGENTS.md.
	if err := c.stripAgentsMDBlock(); err != nil {
		return fmt.Errorf("codex uninstall: strip AGENTS.md: %w", err)
	}

	// Remove receipt.
	_ = c.receiptStore.Delete(ProviderCodex)

	return nil
}

// ── content builders ─────────────────────────────────────────────────────────

// buildConfigTOML reads the existing config.toml (if any), upserts the
// mcp_servers.vedox subtree, and returns the full updated TOML bytes.
// keyIDPlaceholder is substituted verbatim; Install replaces it with the real key ID.
func (c *codexInstaller) buildConfigTOML(keyIDPlaceholder string) ([]byte, error) {
	configPath := c.resolveConfigPath()

	m := map[string]any{}
	raw, readErr := os.ReadFile(configPath)
	if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
		return nil, fmt.Errorf("read config.toml: %w", readErr)
	}
	if len(raw) > 0 {
		if _, err := toml.Decode(string(raw), &m); err != nil {
			return nil, fmt.Errorf("parse existing config.toml: %w", err)
		}
	}

	// Ensure mcp_servers map exists.
	servers, _ := m["mcp_servers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}

	// Upsert the vedox entry with only typed, safe fields.
	servers[codexMCPServerKey] = map[string]any{
		"url":       c.daemonURL,
		"key_id":    keyIDPlaceholder,
		"transport": "http",
	}
	m["mcp_servers"] = servers

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(m); err != nil {
		return nil, fmt.Errorf("encode config.toml: %w", err)
	}
	return buf.Bytes(), nil
}

// buildAgentsMDContent reads the existing AGENTS.md (if any) and returns the
// full updated content with the Vedox fenced block appended (idempotent).
// If the block is already present it is replaced in-place.
func (c *codexInstaller) buildAgentsMDContent(keyIDPlaceholder string) ([]byte, error) {
	path := c.agentsMDPath()
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	// Build the instruction body from the embedded template, substituting
	// runtime values for {{DAEMON_PORT}} and {{HMAC_KEY_ID}}.
	body := strings.ReplaceAll(templates.Codex, "{{DAEMON_PORT}}", daemonPort(c.daemonURL))
	body = strings.ReplaceAll(body, "{{HMAC_KEY_ID}}", keyIDPlaceholder)

	block := codexVedoxFenceStart + "\n" +
		"<!-- This block is managed by Vedox. Do not edit manually. -->\n" +
		"<!-- Use 'vedox agent uninstall --provider codex' to remove it. -->\n" +
		"<!-- vedox key-id: " + keyIDPlaceholder + " -->\n" +
		"\n" +
		body +
		"\n" +
		codexVedoxFenceEnd + "\n"

	if bytes.Contains(existing, []byte(codexVedoxFenceStart)) {
		// Block already present — replace it to pick up any changes.
		return replaceCodexFencedBlock(existing, block), nil
	}

	// Append the block.
	var buf bytes.Buffer
	buf.Write(existing)
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		buf.WriteByte('\n')
	}
	buf.WriteString("\n")
	buf.WriteString(block)
	return buf.Bytes(), nil
}

// stripCodexMCPEntry removes the mcp_servers.vedox key from config.toml.
// If the file does not exist or already has no such key, this is a no-op.
func (c *codexInstaller) stripCodexMCPEntry() error {
	configPath := c.resolveConfigPath()
	raw, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	m := map[string]any{}
	if _, err := toml.Decode(string(raw), &m); err != nil {
		// Unparseable TOML — leave the file intact to avoid data loss.
		return fmt.Errorf("parse config.toml for strip: %w", err)
	}

	servers, ok := m["mcp_servers"].(map[string]any)
	if !ok {
		return nil // no mcp_servers section
	}
	if _, exists := servers[codexMCPServerKey]; !exists {
		return nil // vedox entry not present
	}
	delete(servers, codexMCPServerKey)
	if len(servers) == 0 {
		delete(m, "mcp_servers")
	} else {
		m["mcp_servers"] = servers
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(m); err != nil {
		return fmt.Errorf("re-encode config.toml after strip: %w", err)
	}
	// Use the config directory as boundary so rename is same-filesystem.
	boundary := filepath.Dir(configPath)
	return atomicFileWrite(boundary, configPath, buf.Bytes(), 0o700, 0o600)
}

// stripAgentsMDBlock removes the Vedox fenced block from AGENTS.md. If the
// file does not exist or contains no block, this is a no-op.
func (c *codexInstaller) stripAgentsMDBlock() error {
	path := c.agentsMDPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !bytes.Contains(data, []byte(codexVedoxFenceStart)) {
		return nil
	}
	stripped := removeCodexFencedBlock(data)
	// Use directory as boundary for atomic rename.
	boundary := filepath.Dir(path)
	return atomicFileWrite(boundary, path, stripped, 0o755, 0o644)
}

// ── helpers ──────────────────────────────────────────────────────────────────

// replaceCodexFencedBlock replaces the content between codexVedoxFenceStart
// and codexVedoxFenceEnd (inclusive) with newBlock.
func replaceCodexFencedBlock(data []byte, newBlock string) []byte {
	s := string(data)
	start := strings.Index(s, codexVedoxFenceStart)
	if start < 0 {
		return data
	}
	end := strings.Index(s[start:], codexVedoxFenceEnd)
	if end < 0 {
		return data
	}
	endAbs := start + end + len(codexVedoxFenceEnd)
	if endAbs < len(s) && s[endAbs] == '\n' {
		endAbs++
	}
	var buf bytes.Buffer
	buf.WriteString(s[:start])
	buf.WriteString(newBlock)
	buf.WriteString(s[endAbs:])
	return buf.Bytes()
}

// removeCodexFencedBlock strips the Vedox fenced block and its surrounding
// blank lines from data.
func removeCodexFencedBlock(data []byte) []byte {
	s := string(data)
	start := strings.Index(s, codexVedoxFenceStart)
	if start < 0 {
		return data
	}
	end := strings.Index(s[start:], codexVedoxFenceEnd)
	if end < 0 {
		return data
	}
	endAbs := start + end + len(codexVedoxFenceEnd)
	if endAbs < len(s) && s[endAbs] == '\n' {
		endAbs++
	}
	before := s[:start]
	before = strings.TrimRight(before, "\n")
	if len(before) > 0 {
		before += "\n"
	}
	return []byte(before + s[endAbs:])
}

// codexSchemaHash computes a deterministic fingerprint of the top-level key
// set in a TOML config file. This is the schema-drift signal stored in
// ProbeResult.SchemaHash and InstallReceipt.SchemaHash.
func codexSchemaHash(data []byte) string {
	if len(data) == 0 {
		return sha256Hex(data)
	}
	m := map[string]any{}
	if _, err := toml.Decode(string(data), &m); err != nil {
		return sha256Hex(data)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		h.Write([]byte(k))
	}
	sum := h.Sum(nil)
	// Reuse sha256Hex from atomic.go by hashing the sum itself would be
	// circular; instead encode directly.
	const hextable = "0123456789abcdef"
	dst := make([]byte, len(sum)*2)
	for i, v := range sum {
		dst[i*2] = hextable[v>>4]
		dst[i*2+1] = hextable[v&0x0f]
	}
	return string(dst)
}

// codexBinaryVersion runs `codex --version` and returns the first line of
// output (trimmed). Returns an empty string on any error or on timeout.
func codexBinaryVersion(binPath string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, binPath, "--version").Output()
	if err != nil {
		return ""
	}
	lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}
