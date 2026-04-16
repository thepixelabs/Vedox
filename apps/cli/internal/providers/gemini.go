package providers

// gemini.go — ProviderInstaller adapter for Google Gemini CLI.
//
// Surfaces written:
//   - ~/.gemini/extensions/vedox/vedox-agent.json
//     (Gemini CLI extension manifest — name, description, version,
//      commands, daemon URL, key ID, and the instruction body)
//   - ~/.gemini/config.yaml
//     (append/update a Vedox-managed fenced block that registers the
//      "vedox" extension in the extensions list — idempotent)
//   - ~/.vedox/install-receipts/gemini.json  (managed by ReceiptStore)
//
// TODO(validate-gemini-paths): the extension directory (~/.gemini/extensions/)
// and config.yaml schema are based on the Gemini CLI extension spec documented
// in .tasks/vedox-v2/10-brainstorm/arch-agent-integration-and-voice.md §3.2.4.
// Before shipping, verify against the real Gemini CLI:
//   1. Run `gemini --help` and `gemini extensions --help`.
//   2. Install any existing extension and inspect ~/.gemini/ structure.
//   3. Confirm: (a) extensions dir is ~/.gemini/extensions/, (b) config file
//      is config.yaml, (c) extensions list key is "extensions", (d) manifest
//      schema fields (schemaVersion, commands, etc.) match the real spec.
// If the real path is e.g. ~/.config/gemini/, update geminiDir() below.
//
// Security: secrets are never written to disk. AuthKeyID (a UUID) is embedded
// in the manifest JSON as "keyId"; the actual HMAC secret lives only in the OS
// keychain under agentauth.KeyStore.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// geminiAgentVersion must be bumped when the manifest format or instruction
	// body changes. Semver; matches the WS-D instruction pack version.
	geminiAgentVersion = "2.0"

	// geminiExtensionName is the stable identifier for the Vedox extension.
	geminiExtensionName = "vedox"

	// geminiManifestFile is the filename of the extension manifest.
	geminiManifestFile = "vedox-agent.json"

	// geminiConfigFile is the primary Gemini CLI user-config file.
	//
	// TODO(validate-gemini-paths): confirm this is the actual filename.
	geminiConfigFile = "config.yaml"

	// geminiVedoxConfigStart / geminiVedoxConfigEnd delimit the Vedox-managed
	// block inside config.yaml. Content outside these markers is never touched.
	geminiVedoxConfigStart = "# vedox-gemini:start"
	geminiVedoxConfigEnd   = "# vedox-gemini:end"
)

// geminiExtensionManifest is the JSON structure written to
// ~/.gemini/extensions/vedox/vedox-agent.json.
//
// TODO(validate-gemini-paths): align field names with the real Gemini CLI
// extension JSON schema once the path validation above is done. Current
// structure mirrors the brainstorm spec (§3.2.4).
type geminiExtensionManifest struct {
	SchemaVersion string          `json:"schemaVersion"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Version       string          `json:"version"`
	Provider      string          `json:"provider"`
	DaemonURL     string          `json:"daemonUrl"`
	KeyID         string          `json:"keyId"`
	Commands      []geminiCommand `json:"commands"`
	Instructions  string          `json:"instructions"`
}

// geminiCommand is one slash-command entry in the extension manifest.
//
// TODO(validate-gemini-paths): confirm "commands" key and field names.
type geminiCommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// geminiInstaller implements ProviderInstaller for Google Gemini CLI.
type geminiInstaller struct {
	// home is the user's home directory. Injected so tests can use a temp dir.
	home string

	// daemonURL is the HTTP base URL of the local Vedox daemon.
	daemonURL string

	// keys is the HMAC key issuer (agentauth.KeyStore or a test mock).
	keys KeyIssuer

	// receiptStore is used to load an existing receipt during Repair/Verify/Uninstall.
	receiptStore *ReceiptStore
}

// NewGeminiInstaller constructs a geminiInstaller.
//
//   - home may be empty to use os.UserHomeDir().
//   - daemonURL is the URL of the Vedox daemon (e.g. "http://127.0.0.1:5150").
//   - keys is typically an agentauth.KeyStore.
//   - store is the ReceiptStore for persist/load of the install receipt.
func NewGeminiInstaller(home, daemonURL string, keys KeyIssuer, store *ReceiptStore) (ProviderInstaller, error) {
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("gemini installer: resolve home dir: %w", err)
		}
		home = h
	}
	if daemonURL == "" {
		daemonURL = "http://127.0.0.1:5150"
	}
	return &geminiInstaller{
		home:         home,
		daemonURL:    daemonURL,
		keys:         keys,
		receiptStore: store,
	}, nil
}

// ── path helpers ─────────────────────────────────────────────────────────────

// geminiDir returns ~/.gemini — the root of Gemini CLI user config.
// TODO(validate-gemini-paths): may need to be ~/.config/gemini/ on some systems.
func (g *geminiInstaller) geminiDir() string {
	return filepath.Join(g.home, ".gemini")
}

func (g *geminiInstaller) geminiExtensionsDir() string {
	return filepath.Join(g.geminiDir(), "extensions")
}

func (g *geminiInstaller) vedoxExtensionDir() string {
	return filepath.Join(g.geminiExtensionsDir(), geminiExtensionName)
}

func (g *geminiInstaller) manifestPath() string {
	return filepath.Join(g.vedoxExtensionDir(), geminiManifestFile)
}

func (g *geminiInstaller) geminiConfigPath() string {
	return filepath.Join(g.geminiDir(), geminiConfigFile)
}

// ── Probe ────────────────────────────────────────────────────────────────────

// Probe checks whether the gemini binary is on PATH and whether the Vedox
// extension manifest already exists in ~/.gemini/extensions/vedox/.
func (g *geminiInstaller) Probe(_ context.Context) (*ProbeResult, error) {
	result := &ProbeResult{}

	// Detect binary.
	if binPath, err := exec.LookPath("gemini"); err == nil {
		result.BinaryVersion = geminiBinaryVersion(binPath)
	}

	// Detect existing manifest.
	manifestPath := g.manifestPath()
	data, err := os.ReadFile(manifestPath)
	if err == nil {
		result.Installed = isVedoxGeminiManifest(data)
		result.ConfigPath = manifestPath
		result.SchemaHash = geminiSchemaHashFromManifest(data)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("gemini probe: read manifest: %w", err)
	}

	return result, nil
}

// ── Plan ─────────────────────────────────────────────────────────────────────

// Plan generates FileOps to write the extension manifest and update
// ~/.gemini/config.yaml with the extension registration block. Does NOT issue
// an HMAC key — that happens in Install. Content embeds "{{HMAC_KEY_ID}}" as
// a placeholder; Install substitutes the real key ID before executing.
func (g *geminiInstaller) Plan(_ context.Context) (*InstallPlan, error) {
	ops := []FileOp{}

	// Op 1: write ~/.gemini/extensions/vedox/vedox-agent.json
	manifestContent, err := g.buildManifestContent("{{HMAC_KEY_ID}}")
	if err != nil {
		return nil, fmt.Errorf("gemini plan: build manifest: %w", err)
	}
	ops = append(ops, FileOp{
		Path:     g.manifestPath(),
		Action:   OpUpdate,
		Content:  manifestContent,
		Boundary: g.home,
	})

	// Op 2: update ~/.gemini/config.yaml to register the extension (idempotent).
	configContent, err := g.buildConfigContent()
	if err != nil {
		return nil, fmt.Errorf("gemini plan: build config: %w", err)
	}
	if configContent != nil {
		ops = append(ops, FileOp{
			Path:     g.geminiConfigPath(),
			Action:   OpUpdate,
			Content:  configContent,
			Boundary: g.home,
		})
	}

	return &InstallPlan{
		Provider: ProviderGemini,
		FileOps:  ops,
		PlanHash: planHashOf(ops),
	}, nil
}

// ── Install ──────────────────────────────────────────────────────────────────

// Install issues an HMAC key, substitutes it into the plan's FileOps, then
// executes the ops atomically. Returns a receipt for persistence.
func (g *geminiInstaller) Install(_ context.Context, plan *InstallPlan) (*InstallReceipt, error) {
	keyID, _, err := g.keys.IssueKey(
		"vedox-doc-agent-gemini",
		"", // project: user-scoped
		"", // pathPrefix: unrestricted
	)
	if err != nil {
		return nil, fmt.Errorf("gemini install: issue key: %w", err)
	}

	// Substitute real key ID into every FileOp content.
	ops := substituteKeyID(plan.FileOps, keyID)

	// Execute file ops.
	hashes, err := executeFileOps(ops)
	if err != nil {
		_ = g.keys.RevokeKey(keyID)
		return nil, fmt.Errorf("gemini install: execute file ops: %w", err)
	}

	// Use the manifest hash as the schema hash for drift detection.
	var schemaHash string
	for path, hash := range hashes {
		if strings.HasSuffix(path, geminiManifestFile) {
			schemaHash = hash
			break
		}
	}

	return &InstallReceipt{
		Provider:    ProviderGemini,
		Version:     geminiAgentVersion,
		SchemaHash:  schemaHash,
		AuthKeyID:   keyID,
		DaemonURL:   g.daemonURL,
		FileHashes:  hashes,
		InstalledAt: time.Now().UTC(),
	}, nil
}

// ── Verify ───────────────────────────────────────────────────────────────────

// Verify re-reads every file in the receipt, compares its hash, and reports
// drift. Used by Repair and `vedox agent status`.
func (g *geminiInstaller) Verify(_ context.Context, receipt *InstallReceipt) (*VerifyResult, error) {
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
			return nil, fmt.Errorf("gemini verify: read %s: %w", path, err)
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
// Install if drift is detected. The old HMAC key is revoked; a new one is
// issued automatically. Idempotent.
func (g *geminiInstaller) Repair(ctx context.Context) error {
	if g.receiptStore == nil {
		return fmt.Errorf("gemini repair: no receipt store configured")
	}
	receipt, err := g.receiptStore.Load(ProviderGemini)
	if err != nil {
		return fmt.Errorf("gemini repair: load receipt: %w", err)
	}
	if receipt != nil {
		v, err := g.Verify(ctx, receipt)
		if err != nil {
			return fmt.Errorf("gemini repair: verify: %w", err)
		}
		if v.Healthy {
			return nil // nothing to repair
		}
	}

	// Revoke old key before issuing a fresh one.
	if receipt != nil && receipt.AuthKeyID != "" {
		_ = g.keys.RevokeKey(receipt.AuthKeyID) // best-effort; proceed regardless
	}

	plan, err := g.Plan(ctx)
	if err != nil {
		return fmt.Errorf("gemini repair: plan: %w", err)
	}
	newReceipt, err := g.Install(ctx, plan)
	if err != nil {
		return fmt.Errorf("gemini repair: install: %w", err)
	}
	return g.receiptStore.Save(newReceipt)
}

// ── Uninstall ────────────────────────────────────────────────────────────────

// Uninstall revokes the HMAC key, removes the extension directory entirely
// (it contains only Vedox-managed files), and strips the Vedox fenced block
// from ~/.gemini/config.yaml. User content outside the fenced block is
// preserved.
func (g *geminiInstaller) Uninstall(_ context.Context) error {
	if g.receiptStore == nil {
		return fmt.Errorf("gemini uninstall: no receipt store configured")
	}
	receipt, err := g.receiptStore.Load(ProviderGemini)
	if err != nil {
		return fmt.Errorf("gemini uninstall: load receipt: %w", err)
	}

	// Revoke key first — a live key with no install is a smaller problem than
	// a removed install with a live key.
	if receipt != nil && receipt.AuthKeyID != "" {
		if revokeErr := g.keys.RevokeKey(receipt.AuthKeyID); revokeErr != nil {
			return fmt.Errorf("gemini uninstall: revoke key: %w", revokeErr)
		}
	}

	// Remove the extension directory (Vedox-owned, safe to delete wholesale).
	extDir := g.vedoxExtensionDir()
	if err := os.RemoveAll(extDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("gemini uninstall: remove extension dir: %w", err)
	}

	// Strip Vedox block from config.yaml.
	if err := g.stripVedoxConfigBlock(); err != nil {
		return fmt.Errorf("gemini uninstall: strip config.yaml block: %w", err)
	}

	_ = g.receiptStore.Delete(ProviderGemini)
	return nil
}

// ── content builders ─────────────────────────────────────────────────────────

// buildManifestContent produces the JSON bytes for the extension manifest.
// keyIDPlaceholder is embedded verbatim; substituteKeyID replaces it with the
// real key ID at install time.
func (g *geminiInstaller) buildManifestContent(keyIDPlaceholder string) ([]byte, error) {
	manifest := geminiExtensionManifest{
		// TODO(validate-gemini-paths): confirm schemaVersion value.
		SchemaVersion: "1",
		Name:          "Vedox Doc Agent",
		Description:   "Routes and commits documentation via the Vedox daemon. Activates on 'vedox document' phrases. Authenticated via HMAC-SHA256.",
		Version:       geminiAgentVersion,
		Provider:      string(ProviderGemini),
		DaemonURL:     g.daemonURL,
		KeyID:         keyIDPlaceholder,
		Commands: []geminiCommand{
			{
				// TODO(validate-gemini-paths): confirm slash-command convention for gemini CLI.
				Name:        "/vedox-doc",
				Description: "Activate the Vedox documentation agent. Usage: /vedox-doc document <target>",
			},
		},
		Instructions: geminiInstructionBody(g.daemonURL, keyIDPlaceholder),
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal manifest: %w", err)
	}
	// Ensure trailing newline for POSIX-compliant file.
	return append(data, '\n'), nil
}

// buildConfigContent reads the existing ~/.gemini/config.yaml and returns the
// full updated content with the Vedox extension registration block present
// (idempotent). Returns nil if no update is needed.
func (g *geminiInstaller) buildConfigContent() ([]byte, error) {
	path := g.geminiConfigPath()
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	block := geminiVedoxConfigStart + "\n" +
		"# This block is managed by Vedox. Do not edit manually.\n" +
		"# Use 'vedox agent uninstall --provider gemini' to remove it.\n" +
		"extensions:\n" +
		"  - " + geminiExtensionName + "\n" +
		geminiVedoxConfigEnd + "\n"

	if bytes.Contains(existing, []byte(geminiVedoxConfigStart)) {
		// Block already present — replace it to pick up any changes.
		return replaceGeminiConfigBlock(existing, block), nil
	}

	// Append to existing content.
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

// stripVedoxConfigBlock removes the Vedox fenced block from
// ~/.gemini/config.yaml. No-op if the file is absent or contains no block.
func (g *geminiInstaller) stripVedoxConfigBlock() error {
	path := g.geminiConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !bytes.Contains(data, []byte(geminiVedoxConfigStart)) {
		return nil
	}
	stripped := removeGeminiConfigBlock(data)
	stripped = bytes.TrimRight(stripped, "\n")
	if len(stripped) == 0 {
		return os.Remove(path)
	}
	stripped = append(stripped, '\n')
	return atomicFileWrite(g.home, path, stripped, 0o755, 0o644)
}

// ── block manipulation helpers ────────────────────────────────────────────────

// replaceGeminiConfigBlock replaces the Vedox fenced block (inclusive of
// markers) with newBlock in data.
func replaceGeminiConfigBlock(data []byte, newBlock string) []byte {
	s := string(data)
	start := strings.Index(s, geminiVedoxConfigStart)
	if start < 0 {
		return data
	}
	end := strings.Index(s[start:], geminiVedoxConfigEnd)
	if end < 0 {
		return data
	}
	endAbs := start + end + len(geminiVedoxConfigEnd)
	if endAbs < len(s) && s[endAbs] == '\n' {
		endAbs++
	}
	var buf bytes.Buffer
	buf.WriteString(s[:start])
	buf.WriteString(newBlock)
	buf.WriteString(s[endAbs:])
	return buf.Bytes()
}

// removeGeminiConfigBlock strips the Vedox fenced block and its leading blank
// line from data.
func removeGeminiConfigBlock(data []byte) []byte {
	s := string(data)
	start := strings.Index(s, geminiVedoxConfigStart)
	if start < 0 {
		return data
	}
	end := strings.Index(s[start:], geminiVedoxConfigEnd)
	if end < 0 {
		return data
	}
	endAbs := start + end + len(geminiVedoxConfigEnd)
	if endAbs < len(s) && s[endAbs] == '\n' {
		endAbs++
	}
	before := strings.TrimRight(s[:start], "\n")
	if len(before) > 0 {
		before += "\n"
	}
	return []byte(before + s[endAbs:])
}

// ── probe helpers ─────────────────────────────────────────────────────────────

// isVedoxGeminiManifest returns true if data contains our provider marker,
// i.e. the manifest was written by this adapter.
func isVedoxGeminiManifest(data []byte) bool {
	return bytes.Contains(data, []byte(`"provider"`)) &&
		bytes.Contains(data, []byte(`"`+string(ProviderGemini)+`"`))
}

// geminiSchemaHashFromManifest computes a fingerprint of the JSON key names
// (not values) in the manifest. Used as the schema-drift signal in Verify.
func geminiSchemaHashFromManifest(data []byte) string {
	// Unmarshal into a map to extract top-level keys only.
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &m); err != nil {
		// Fall back to full-content hash if the file is not valid JSON.
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
	return fmt.Sprintf("%x", sum)
}

// geminiBinaryVersion runs `gemini --version` and returns the first line of
// output (trimmed). Returns an empty string on any error.
func geminiBinaryVersion(binPath string) string {
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

// ── instruction body ──────────────────────────────────────────────────────────

// geminiInstructionBody returns the prose instructions for the extension
// manifest, with {{DAEMON_PORT}} and {{HMAC_KEY_ID}} substituted.
func geminiInstructionBody(daemonURL, keyIDPlaceholder string) string {
	port := daemonPort(daemonURL)
	body := strings.ReplaceAll(geminiInstructionTemplate, "{{DAEMON_PORT}}", port)
	body = strings.ReplaceAll(body, "{{HMAC_KEY_ID}}", keyIDPlaceholder)
	return body
}

// geminiInstructionTemplate is the Markdown body embedded in the extension
// manifest JSON. Template variables {{DAEMON_PORT}} and {{HMAC_KEY_ID}} are
// substituted by geminiInstructionBody before the manifest is written.
const geminiInstructionTemplate = `# Vedox Doc Agent — Gemini CLI Extension Instructions

you are the vedox documentation agent, installed into the Gemini CLI as an extension.

your only job is to write, classify, route, and commit markdown documentation
to the correct registered repo through the Vedox daemon API running at
127.0.0.1:{{DAEMON_PORT}}.

you do not:
- modify source code, test files, configuration files, or any file outside a
  registered documentation repo's root or a project's docs/ subtree.
- answer general coding questions, generate tests, or refactor code.
- make outbound network requests. every API call goes to 127.0.0.1 only.
- write speculative content ("Vedox will support X"). document the system as it
  exists at the date you are writing.
- use emoji anywhere — not in documents, frontmatter, commit messages, or
  responses to the user.
- invent frontmatter fields not in the WRITING_FRAMEWORK schema.
- commit directly to main, master, or any branch the user has marked
  protected in ~/.vedox/user-prefs.json.

if the user asks you to do anything outside documentation, respond:
"i only handle documentation. use your main agent for that."

## Activation

you activate on the /vedox-doc command or any of these trigger phrases (exact or paraphrased):

- vedox document everything
- vedox document this folder
- vedox document these changes
- vedox document this conversation
- vedox, document <anything>

you do not activate on any other phrase. do not start a documentation run as a
side effect inside another task.

## HMAC-SHA256 authentication

every daemon request must be signed. unsigned requests are rejected with HTTP 401.

required headers on every request:

  X-Vedox-Agent-Key: {{HMAC_KEY_ID}}
  X-Vedox-Timestamp: <current RFC3339 timestamp>
  X-Vedox-Signature: <lowercase hex-encoded HMAC-SHA256>
  Content-Type: application/json

signed string construction:
  METHOD + "\n" + PATH + "\n" + TIMESTAMP_RFC3339 + "\n" + SHA256_HEX_OF_BODY

clock skew tolerance is 5 minutes.

## Daemon endpoints

- GET /v1/repos — list registered doc repos
- GET /v1/repos/:id/routing-rules — get routing overrides
- POST /v1/scan/secrets — pre-commit secret scan (call before any commit)
- POST /v1/docs/commit — commit docs to a branch
- POST /v1/review-queue — queue unresolved items for Vedox editor review

## Safety rails

- never commit to main, master, or any protected branch
- always call POST /v1/scan/secrets before any write
- always show a diff preview and wait for user confirmation before committing
- daemon unreachable: say "the vedox daemon is not running. start it with 'vedox server' then retry."
- secret detected (critical/high): stop immediately, report, wait for user to fix

## Style

- pixelabs brand voice for public docs: lowercase marketing, ./unix CTAs, no fluff
- neutral professional prose for private docs
- no emoji anywhere
- commit message format: docs(<scope>): <summary> [vedox-agent]
- audit trailer in every commit: [vedox-agent] key-id={{HMAC_KEY_ID}} provider=gemini
`
