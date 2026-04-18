// Package providers defines the ProviderInstaller interface and shared types
// used by all Doc Agent install adapters (Claude Code, Codex, Copilot, Gemini).
//
// Architecture contract:
//   - Every adapter is a pure value — no unexported state escapes the package.
//   - File writes always go through atomicFileWrite (temp + fsync + rename).
//   - Secrets are NEVER written to FileOp.Content or any on-disk file.
//     AuthKeyID in InstallPlan is a key ID only; the plaintext secret lives in
//     the OS keychain managed by agentauth.KeyStore.
//   - Receipt files live under ~/.vedox/install-receipts/<provider>.json.
package providers

import (
	"context"
	"time"
)

// ProviderID is a stable identifier for a supported AI provider.
type ProviderID string

const (
	ProviderClaude  ProviderID = "claude"
	ProviderCodex   ProviderID = "codex"
	ProviderCopilot ProviderID = "copilot"
	ProviderGemini  ProviderID = "gemini"
)

// OpMode describes the file operation an installer wants to perform.
type OpMode string

const (
	OpCreate OpMode = "create" // write a new file; fail if it already exists
	OpUpdate OpMode = "update" // overwrite or patch an existing file
	OpDelete OpMode = "delete" // remove a file
)

// FileOp describes a single atomic file operation. The installer executes
// these in order as returned by Plan.
type FileOp struct {
	// Path is the absolute path of the target file.
	Path string

	// Action is create, update, or delete.
	Action OpMode

	// Content is the bytes to write for create/update. Ignored for delete.
	// Must NOT contain secrets — key IDs only.
	Content []byte

	// Boundary is the security boundary for symlink-ancestor checks.
	// Typically the user's home directory for user-scoped files.
	Boundary string
}

// ProbeResult is the output of Probe — what the installer found on disk.
type ProbeResult struct {
	// Installed is true when a recognisable Vedox agent entry already exists
	// for this provider (subagent file, MCP entry, instruction block, etc.).
	Installed bool

	// BinaryVersion is the version string reported by the provider binary
	// (empty if the binary was not found on PATH).
	BinaryVersion string

	// ConfigPath is the primary provider config file the adapter will write to.
	// Empty if none was detected.
	ConfigPath string

	// SchemaHash is a SHA-256 fingerprint of the *keys* (not values) in the
	// provider's config. Used to detect upstream schema drift on Verify.
	SchemaHash string
}

// InstallPlan is the dry-run output of Plan — what the installer intends to do.
type InstallPlan struct {
	// Provider identifies the adapter that produced this plan.
	Provider ProviderID

	// FileOps is the ordered list of file operations to execute.
	FileOps []FileOp

	// AuthKeyID is the agentauth key ID that will be embedded into the
	// provider configuration. The caller must issue this key before calling
	// Install.
	AuthKeyID string

	// PlanHash is a SHA-256 of the serialised FileOps for idempotency checks.
	PlanHash string
}

// InstalledFile records one file written by Install.
type InstalledFile struct {
	// Path is the absolute path that was written.
	Path string

	// SHA256 is the hex-encoded hash of the bytes written.
	SHA256 string

	// Deletable marks files that can be fully removed on uninstall
	// (as opposed to config files where we only strip the Vedox block).
	Deletable bool
}

// InstallReceipt is the output of Install — written to disk by ReceiptStore.
type InstallReceipt struct {
	// Provider identifies the adapter.
	Provider ProviderID `json:"provider"`

	// Version is the instruction-pack version installed (semver, e.g. "2.0").
	Version string `json:"version"`

	// SchemaHash is the SchemaHash from the ProbeResult at install time.
	// Verify re-computes and compares this to detect upstream drift.
	SchemaHash string `json:"schemaHash"`

	// AuthKeyID is the agentauth key ID embedded in the provider config.
	AuthKeyID string `json:"authKeyID"`

	// DaemonURL is the HTTP base URL of the Vedox daemon the agent is
	// configured to call (e.g. "http://127.0.0.1:5150").
	DaemonURL string `json:"daemonURL"`

	// FileHashes maps absolute path → sha256 of written content, allowing
	// Verify to detect out-of-band edits.
	FileHashes map[string]string `json:"fileHashes"`

	// InstalledAt is the UTC timestamp of the successful Install call.
	InstalledAt time.Time `json:"installedAt"`
}

// VerifyResult is the output of Verify — a health snapshot of an installed agent.
type VerifyResult struct {
	// Healthy is true when all files match their recorded hashes AND the
	// provider binary is still present.
	Healthy bool

	// Drift is true when at least one managed file has been modified since
	// install (hash mismatch), OR the provider's config schema has changed.
	Drift bool

	// Issues is a human-readable list of problems found. Empty when Healthy.
	Issues []string
}

// KeyIssuer is the minimal interface from agentauth.KeyStore that installers
// need. Wrapping the full KeyStore behind this interface lets tests inject a
// mock without depending on the OS keychain.
type KeyIssuer interface {
	IssueKey(name, project, pathPrefix string) (id, secret string, err error)
	RevokeKey(id string) error
}

// ProviderInstaller is the single abstraction every provider adapter implements.
//
// Call order for a fresh install:
//
//	probe  := installer.Probe(ctx)
//	plan   := installer.Plan(ctx)            // dry-run; no side-effects
//	receipt := installer.Install(ctx, plan)  // execute FileOps + issue key
//
// Call order for status check:
//
//	result := installer.Verify(ctx, storedReceipt)
//
// Call order for repair:
//
//	installer.Repair(ctx)                   // verify + reinstall if drift
//
// Call order for removal:
//
//	installer.Uninstall(ctx)                // revoke key + strip/delete files
type ProviderInstaller interface {
	// Probe checks whether the provider is installed and detects the current
	// state of any existing Vedox agent configuration. Safe to call at any
	// time; makes no writes.
	Probe(ctx context.Context) (*ProbeResult, error)

	// Plan generates the ordered set of FileOps required to install (or
	// reinstall) the Vedox Doc Agent for this provider. Returns a dry-run
	// plan — callers must call Install to execute it.
	Plan(ctx context.Context) (*InstallPlan, error)

	// Install executes the plan returned by Plan. It issues an HMAC key via
	// the KeyIssuer, executes each FileOp atomically, and returns a receipt
	// suitable for persistence by ReceiptStore.
	Install(ctx context.Context, plan *InstallPlan) (*InstallReceipt, error)

	// Repair verifies the current state against stored receipts and re-runs
	// Install for any drift detected. It is idempotent.
	Repair(ctx context.Context) error

	// Uninstall revokes the HMAC key and removes or strips all Vedox-managed
	// content from provider config files.
	Uninstall(ctx context.Context) error

	// Verify re-reads provider config files and compares file hashes to the
	// stored receipt. Returns a VerifyResult describing any drift.
	Verify(ctx context.Context, receipt *InstallReceipt) (*VerifyResult, error)
}
