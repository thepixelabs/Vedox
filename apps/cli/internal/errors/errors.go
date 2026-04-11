// Package errors defines the Vedox CLI error taxonomy.
//
// All user-facing errors carry a numbered VDX code so users can look them up
// in the docs and so support conversations have a shared vocabulary.
//
// Format: [VDX-XXX] Human-readable message. See: https://vedox.dev/errors/VDX-XXX
//
// Error code ranges:
//
//	VDX-001 – VDX-099  Phase 1 CLI / runtime errors
//	VDX-100 – VDX-199  (reserved for Phase 2 workspace intelligence)
//	VDX-200 – VDX-299  (reserved for Phase 3 agentic API — ingestion / staging)
//	VDX-300 – VDX-399  Phase 3 agent authentication errors
package errors

import (
	"fmt"
	"strings"
)

// Code is a typed string for VDX error codes so callers cannot accidentally
// pass an arbitrary string where a code is expected.
type Code string

const (
	// ErrPortInUse is returned when the requested TCP port is already bound.
	// Instructs the user to identify the holding process or pick a different port.
	ErrPortInUse Code = "VDX-001"

	// ErrConfigNotFound is returned when vedox.config.toml cannot be located
	// in the working directory or at the path given by --config.
	ErrConfigNotFound Code = "VDX-002"

	// ErrGitIdentityUnset is returned when git config user.email or user.name
	// is not set. Vedox requires both to author Git commits on Publish.
	ErrGitIdentityUnset Code = "VDX-003"

	// ErrWorkspaceRootNotFound is returned when Vedox cannot locate a workspace
	// root (no vedox.config.toml anywhere in the directory hierarchy).
	ErrWorkspaceRootNotFound Code = "VDX-004"

	// ErrPathTraversal is returned when a resolved file path escapes the
	// workspace root boundary. This is a hard security rejection.
	ErrPathTraversal Code = "VDX-005"

	// ErrSecretFileBlocked is returned when an operation targets a file that
	// matches the secret file blocklist (.env, *.pem, *.key, id_rsa, *.p12,
	// credentials.json). The file's path is logged at WARN; its contents are
	// never read or returned.
	ErrSecretFileBlocked Code = "VDX-006"

	// ErrPayloadTooLarge is returned when a request body exceeds the configured
	// maximum size (currently 1 MB for doc write operations).
	ErrPayloadTooLarge Code = "VDX-010"

	// ErrReadOnly is returned when a write or delete is attempted on a document
	// that is served by a read-only adapter (e.g. SymlinkAdapter). Users must
	// Import & Migrate the document into Vedox before they can edit it.
	ErrReadOnly Code = "VDX-011"

	// ── Phase 3 — Agentic API (VDX-300+) ────────────────────────────────────

	// ErrAgentAuthFailed is returned when an agent request fails HMAC
	// authentication. The response never reveals which specific check failed
	// (unknown key, bad signature, stale timestamp, revoked key) — all map to
	// this single code to prevent oracle attacks.
	ErrAgentAuthFailed Code = "VDX-300"

	// ErrAgentScopeViolation is returned when a successfully-authenticated agent
	// key attempts to access a project or path outside its registered scope.
	// This is a 403 (not 401) — the agent is known but not permitted.
	ErrAgentScopeViolation Code = "VDX-301"

	// ErrKeychainUnavailable is returned when the OS keychain cannot be reached
	// to retrieve or store an HMAC secret. This is a 500 — the system is
	// misconfigured, not the caller. Vedox never falls back to plaintext storage.
	ErrKeychainUnavailable Code = "VDX-302"

	// ErrAgentIdentityMismatch is returned when the agent identifier in the
	// request body does not match the API key's registered name. Prevents one
	// agent from submitting docs under another agent's identity.
	ErrAgentIdentityMismatch Code = "VDX-303"

	// ErrRawHTMLInSubmission is returned when an agent-submitted document
	// contains raw HTML tags or event handlers. Agents are never permitted to
	// include raw HTML regardless of content type.
	ErrRawHTMLInSubmission Code = "VDX-304"

	// ErrOptimisticLockConflict is returned when the etag provided by the agent
	// does not match the current content hash of the document. The agent must
	// re-fetch the document, merge, and resubmit.
	ErrOptimisticLockConflict Code = "VDX-305"

	// ErrRateLimitExceeded is returned when a key exceeds 60 writes per minute
	// or when a key has been suspended by the circuit breaker after too many
	// consecutive errors.
	ErrRateLimitExceeded Code = "VDX-306"
)

const docsBaseURL = "https://vedox.dev/errors"

// VedoxError is the structured error type for all user-facing CLI failures.
// It wraps an optional underlying cause that is only displayed in --debug mode.
type VedoxError struct {
	Code    Code
	Message string
	// Cause is the underlying Go error. Never shown to users unless --debug is
	// active. Log it at DEBUG level only.
	Cause error
}

func (e *VedoxError) Error() string {
	return fmt.Sprintf("[%s] %s See: %s/%s", e.Code, e.Message, docsBaseURL, e.Code)
}

// Unwrap implements the errors.Unwrap interface so callers can use errors.Is /
// errors.As against the underlying cause.
func (e *VedoxError) Unwrap() error {
	return e.Cause
}

// UserMessage returns the full user-facing string, always without the Go cause.
func (e *VedoxError) UserMessage() string {
	return e.Error()
}

// DebugMessage returns the user message plus the underlying cause chain, for
// use only when --debug is active.
func (e *VedoxError) DebugMessage() string {
	if e.Cause == nil {
		return e.Error()
	}
	return fmt.Sprintf("%s\n  caused by: %v", e.Error(), e.Cause)
}

// New creates a VedoxError with no underlying cause.
func New(code Code, message string) *VedoxError {
	return &VedoxError{Code: code, Message: message}
}

// Wrap creates a VedoxError that records an underlying cause. The cause is
// never surfaced in the user-facing message.
func Wrap(code Code, message string, cause error) *VedoxError {
	return &VedoxError{Code: code, Message: message, Cause: cause}
}

// --- Convenience constructors for each defined error code ---

// PortInUse returns VDX-001 for the given port number.
func PortInUse(port int) *VedoxError {
	return New(
		ErrPortInUse,
		fmt.Sprintf(
			"Port %d is already in use. Run 'lsof -i :%d' to find the process, or set a different port in vedox.config.toml.",
			port, port,
		),
	)
}

// ConfigNotFound returns VDX-002 for the given config path.
func ConfigNotFound(path string) *VedoxError {
	return New(
		ErrConfigNotFound,
		fmt.Sprintf(
			"Config file not found at '%s'. Create a vedox.config.toml in your workspace root or use --config to specify a path.",
			path,
		),
	)
}

// GitIdentityUnset returns VDX-003 listing which fields are missing.
func GitIdentityUnset(missingFields []string) *VedoxError {
	fields := strings.Join(missingFields, " and ")
	return New(
		ErrGitIdentityUnset,
		fmt.Sprintf(
			"Git %s not set. Vedox uses your Git identity to author commits. "+
				"Fix with: git config --global user.name \"Your Name\" && git config --global user.email \"you@example.com\"",
			fields,
		),
	)
}

// WorkspaceRootNotFound returns VDX-004.
func WorkspaceRootNotFound(searchedFrom string) *VedoxError {
	return New(
		ErrWorkspaceRootNotFound,
		fmt.Sprintf(
			"Could not locate a Vedox workspace root from '%s'. "+
				"Ensure vedox.config.toml exists in your project root.",
			searchedFrom,
		),
	)
}

// PathTraversal returns VDX-005 for the offending path.
func PathTraversal(attempted string) *VedoxError {
	// Deliberately omit the attempted path from the user message to avoid
	// reflecting attacker-controlled input. Log the path at DEBUG level in
	// the caller instead.
	_ = attempted
	return New(
		ErrPathTraversal,
		"A file path resolved outside the workspace boundary and was rejected. "+
			"Ensure all paths are within your workspace root.",
	)
}

// SecretFileBlocked returns VDX-006 when an operation is rejected because the
// target path matches the secret file blocklist. The path is intentionally
// omitted from the user-facing message; callers must log it at WARN level
// (path only — never contents).
func SecretFileBlocked(attempted string) *VedoxError {
	_ = attempted
	return New(
		ErrSecretFileBlocked,
		"Operation blocked: the target path matches the secret file blocklist "+
			"(.env, *.pem, *.key, id_rsa, *.p12, credentials.json). "+
			"Vedox never reads or writes secret files.",
	)
}

// PayloadTooLarge returns VDX-010 when the request body exceeds the maximum
// allowed size. Callers should respond with HTTP 413.
func PayloadTooLarge() *VedoxError {
	return New(
		ErrPayloadTooLarge,
		"Request body exceeds 1MB limit.",
	)
}

// ReadOnly returns VDX-011 when a write or delete is attempted on a symlinked
// (read-only) document. The user-facing message explains how to gain editing
// access via the Import & Migrate flow.
func ReadOnly() *VedoxError {
	return New(
		ErrReadOnly,
		"This document is read-only. Symlinked docs cannot be edited in Vedox. "+
			"Use Import & Migrate to edit this document.",
	)
}

// AgentAuthFailed returns VDX-300. The message is deliberately generic — it
// never reveals which specific check failed to prevent oracle attacks.
func AgentAuthFailed() *VedoxError {
	return New(
		ErrAgentAuthFailed,
		"Agent authentication failed. Ensure the request includes valid "+
			"X-Vedox-Key-Id, X-Vedox-Signature, and X-Vedox-Timestamp headers.",
	)
}

// AgentScopeViolation returns VDX-301 when a valid key attempts to access a
// resource outside its registered project or path prefix scope.
func AgentScopeViolation(keyID string) *VedoxError {
	// Deliberately omit the target path to avoid reflecting attacker-controlled
	// input. The key ID is safe to include — the agent already knows it.
	return New(
		ErrAgentScopeViolation,
		fmt.Sprintf(
			"API key %s is not permitted to access this resource. "+
				"The request path is outside the key's project or path prefix scope.",
			keyID,
		),
	)
}

// KeychainUnavailable returns VDX-302 when the OS keychain cannot be reached.
// This is a server-side misconfiguration, not a caller error.
func KeychainUnavailable() *VedoxError {
	return New(
		ErrKeychainUnavailable,
		"The OS keychain is unavailable. Vedox requires the OS keychain to store "+
			"agent API key secrets. Ensure the keychain service is accessible "+
			"(macOS Keychain, Linux Secret Service, or Windows Credential Manager).",
	)
}

// AgentIdentityMismatch returns VDX-303 when the agent identifier in the
// request body does not match the authenticated API key's name.
func AgentIdentityMismatch() *VedoxError {
	return New(
		ErrAgentIdentityMismatch,
		"Agent identity mismatch: the 'agent' field in the request body does not "+
			"match the authenticated API key's registered name.",
	)
}

// RawHTMLInSubmission returns VDX-304 when agent-submitted Markdown contains
// raw HTML tags or event handler attributes.
func RawHTMLInSubmission() *VedoxError {
	return New(
		ErrRawHTMLInSubmission,
		"Raw HTML is not permitted in agent submissions. Remove all HTML tags, "+
			"inline event handlers (on*=), and HTML-only elements before resubmitting.",
	)
}

// OptimisticLockConflict returns VDX-305 with the current document etag so
// the agent can re-read, merge, and resubmit with the correct base.
func OptimisticLockConflict(currentEtag string) *VedoxError {
	return &VedoxError{
		Code: ErrOptimisticLockConflict,
		Message: fmt.Sprintf(
			"Document was modified by another writer. Current etag: %s. "+
				"Re-fetch the document, merge your changes, and resubmit with the current etag.",
			currentEtag,
		),
	}
}

// RateLimitExceeded returns VDX-306 when a key has exceeded its write rate
// limit or has been suspended by the circuit breaker.
func RateLimitExceeded(reason string) *VedoxError {
	return New(
		ErrRateLimitExceeded,
		fmt.Sprintf("Rate limit exceeded: %s", reason),
	)
}
