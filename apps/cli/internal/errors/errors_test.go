package errors_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	vdxerr "github.com/vedox/vedox/internal/errors"
)

func TestErrorCodes_Format(t *testing.T) {
	tests := []struct {
		name     string
		err      *vdxerr.VedoxError
		wantCode vdxerr.Code
		wantURL  string
	}{
		{
			name:     "VDX-001 port in use",
			err:      vdxerr.PortInUse(3001),
			wantCode: vdxerr.ErrPortInUse,
			wantURL:  "https://vedox.dev/errors/VDX-001",
		},
		{
			name:     "VDX-002 config not found",
			err:      vdxerr.ConfigNotFound("/some/path/vedox.config.toml"),
			wantCode: vdxerr.ErrConfigNotFound,
			wantURL:  "https://vedox.dev/errors/VDX-002",
		},
		{
			name:     "VDX-003 git identity unset (both)",
			err:      vdxerr.GitIdentityUnset([]string{"user.name", "user.email"}),
			wantCode: vdxerr.ErrGitIdentityUnset,
			wantURL:  "https://vedox.dev/errors/VDX-003",
		},
		{
			name:     "VDX-004 workspace root not found",
			err:      vdxerr.WorkspaceRootNotFound("/home/user/projects"),
			wantCode: vdxerr.ErrWorkspaceRootNotFound,
			wantURL:  "https://vedox.dev/errors/VDX-004",
		},
		{
			name:     "VDX-005 path traversal",
			err:      vdxerr.PathTraversal("../../../etc/passwd"),
			wantCode: vdxerr.ErrPathTraversal,
			wantURL:  "https://vedox.dev/errors/VDX-005",
		},
		{
			name:     "VDX-006 secret file blocked",
			err:      vdxerr.SecretFileBlocked("/workspace/.env"),
			wantCode: vdxerr.ErrSecretFileBlocked,
			wantURL:  "https://vedox.dev/errors/VDX-006",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", tt.err.Code, tt.wantCode)
			}
			msg := tt.err.Error()
			if !strings.Contains(msg, string(tt.wantCode)) {
				t.Errorf("Error() %q does not contain code %q", msg, tt.wantCode)
			}
			if !strings.Contains(msg, tt.wantURL) {
				t.Errorf("Error() %q does not contain docs URL %q", msg, tt.wantURL)
			}
			// Verify the bracket format: [VDX-XXX]
			wantPrefix := fmt.Sprintf("[%s]", tt.wantCode)
			if !strings.HasPrefix(msg, wantPrefix) {
				t.Errorf("Error() %q does not start with %q", msg, wantPrefix)
			}
		})
	}
}

func TestVedoxError_NoCauseInUserMessage(t *testing.T) {
	// The user-facing message must never expose the underlying cause.
	cause := fmt.Errorf("internal: dial tcp 127.0.0.1:3001: connect: connection refused")
	err := vdxerr.Wrap(vdxerr.ErrPortInUse, "Port 3001 is already in use.", cause)

	userMsg := err.UserMessage()
	if strings.Contains(userMsg, "dial tcp") {
		t.Errorf("UserMessage() leaks internal cause: %q", userMsg)
	}
	if strings.Contains(userMsg, "connection refused") {
		t.Errorf("UserMessage() leaks connection details: %q", userMsg)
	}
}

func TestVedoxError_DebugMessageContainsCause(t *testing.T) {
	cause := fmt.Errorf("underlying OS error")
	err := vdxerr.Wrap(vdxerr.ErrPortInUse, "Port 3001 is already in use.", cause)

	debugMsg := err.DebugMessage()
	if !strings.Contains(debugMsg, "underlying OS error") {
		t.Errorf("DebugMessage() should contain cause, got: %q", debugMsg)
	}
}

func TestVedoxError_Unwrap(t *testing.T) {
	cause := fmt.Errorf("sentinel error")
	wrapped := vdxerr.Wrap(vdxerr.ErrPortInUse, "port in use", cause)

	if !errors.Is(wrapped, cause) {
		t.Error("errors.Is should find the cause through Unwrap")
	}
}

func TestPathTraversal_DoesNotReflectInput(t *testing.T) {
	// The path traversal error must not reflect attacker-controlled input
	// in the user-facing message (prevents XSS-style output injection in
	// terminal emulators that interpret escape sequences).
	maliciousPath := "../../../etc/passwd\x1b[31m INJECTED \x1b[0m"
	err := vdxerr.PathTraversal(maliciousPath)

	msg := err.UserMessage()
	if strings.Contains(msg, maliciousPath) {
		t.Errorf("PathTraversal user message reflects input (unsafe): %q", msg)
	}
	if strings.Contains(msg, "INJECTED") {
		t.Errorf("PathTraversal user message reflects injected content: %q", msg)
	}
}

func TestPortInUse_ContainsPortNumber(t *testing.T) {
	err := vdxerr.PortInUse(8080)
	msg := err.UserMessage()
	if !strings.Contains(msg, "8080") {
		t.Errorf("PortInUse message should contain port number, got: %q", msg)
	}
}

func TestGitIdentityUnset_ListsMissingFields(t *testing.T) {
	err := vdxerr.GitIdentityUnset([]string{"user.email"})
	msg := err.UserMessage()
	if !strings.Contains(msg, "user.email") {
		t.Errorf("GitIdentityUnset should mention missing field, got: %q", msg)
	}
	// Should include instructions
	if !strings.Contains(msg, "git config") {
		t.Errorf("GitIdentityUnset should include fix instructions, got: %q", msg)
	}
}

// ── New / DebugMessage without cause ─────────────────────────────────────────

func TestNew_NoCause_UnwrapReturnsNil(t *testing.T) {
	err := vdxerr.New(vdxerr.ErrConfigNotFound, "config missing")
	if errors.Unwrap(err) != nil {
		t.Error("New should produce an error with nil cause")
	}
}

func TestVedoxError_DebugMessage_NoCause_EqualToUserMessage(t *testing.T) {
	// When Cause is nil, DebugMessage should equal Error().
	err := vdxerr.New(vdxerr.ErrConfigNotFound, "config missing")
	if err.DebugMessage() != err.Error() {
		t.Errorf("DebugMessage() with nil cause should equal Error()\ngot:  %q\nwant: %q",
			err.DebugMessage(), err.Error())
	}
}

// ── WorkspaceRootNotFound message content ────────────────────────────────────

func TestWorkspaceRootNotFound_ContainsSearchPath(t *testing.T) {
	err := vdxerr.WorkspaceRootNotFound("/home/user/myproject")
	msg := err.UserMessage()
	if !strings.Contains(msg, "/home/user/myproject") {
		t.Errorf("WorkspaceRootNotFound message should contain search path, got: %q", msg)
	}
}

// ── PayloadTooLarge ───────────────────────────────────────────────────────────

func TestPayloadTooLarge_HasCorrectCode(t *testing.T) {
	err := vdxerr.PayloadTooLarge()
	if err.Code != vdxerr.ErrPayloadTooLarge {
		t.Errorf("Code = %q, want %q", err.Code, vdxerr.ErrPayloadTooLarge)
	}
}

func TestPayloadTooLarge_MessageMentionsSizeLimit(t *testing.T) {
	err := vdxerr.PayloadTooLarge()
	msg := err.UserMessage()
	if !strings.Contains(msg, "1MB") {
		t.Errorf("PayloadTooLarge message should mention 1MB limit, got: %q", msg)
	}
}

// ── ReadOnly ──────────────────────────────────────────────────────────────────

func TestReadOnly_HasCorrectCode(t *testing.T) {
	err := vdxerr.ReadOnly()
	if err.Code != vdxerr.ErrReadOnly {
		t.Errorf("Code = %q, want %q", err.Code, vdxerr.ErrReadOnly)
	}
}

func TestReadOnly_MessageMentionsMigrateFlow(t *testing.T) {
	err := vdxerr.ReadOnly()
	msg := err.UserMessage()
	if !strings.Contains(msg, "Import") {
		t.Errorf("ReadOnly message should mention Import & Migrate, got: %q", msg)
	}
}

// ── AgentAuthFailed ───────────────────────────────────────────────────────────

func TestAgentAuthFailed_HasCorrectCode(t *testing.T) {
	err := vdxerr.AgentAuthFailed()
	if err.Code != vdxerr.ErrAgentAuthFailed {
		t.Errorf("Code = %q, want %q", err.Code, vdxerr.ErrAgentAuthFailed)
	}
}

func TestAgentAuthFailed_MessageIsGeneric(t *testing.T) {
	// Must not reveal which specific check failed (oracle attack prevention).
	err := vdxerr.AgentAuthFailed()
	msg := err.UserMessage()
	if strings.Contains(strings.ToLower(msg), "invalid key") ||
		strings.Contains(strings.ToLower(msg), "bad signature") ||
		strings.Contains(strings.ToLower(msg), "stale") {
		t.Errorf("AgentAuthFailed message reveals specific failure detail: %q", msg)
	}
	if !strings.Contains(msg, "X-Vedox-Key-Id") {
		t.Errorf("AgentAuthFailed message should list required headers, got: %q", msg)
	}
}

// ── AgentScopeViolation ───────────────────────────────────────────────────────

func TestAgentScopeViolation_HasCorrectCode(t *testing.T) {
	err := vdxerr.AgentScopeViolation("key-abc123")
	if err.Code != vdxerr.ErrAgentScopeViolation {
		t.Errorf("Code = %q, want %q", err.Code, vdxerr.ErrAgentScopeViolation)
	}
}

func TestAgentScopeViolation_ContainsKeyID(t *testing.T) {
	err := vdxerr.AgentScopeViolation("key-abc123")
	msg := err.UserMessage()
	if !strings.Contains(msg, "key-abc123") {
		t.Errorf("AgentScopeViolation message should contain key ID, got: %q", msg)
	}
}

// ── KeychainUnavailable ───────────────────────────────────────────────────────

func TestKeychainUnavailable_HasCorrectCode(t *testing.T) {
	err := vdxerr.KeychainUnavailable()
	if err.Code != vdxerr.ErrKeychainUnavailable {
		t.Errorf("Code = %q, want %q", err.Code, vdxerr.ErrKeychainUnavailable)
	}
}

func TestKeychainUnavailable_MessageMentionsKeychain(t *testing.T) {
	err := vdxerr.KeychainUnavailable()
	msg := err.UserMessage()
	if !strings.Contains(strings.ToLower(msg), "keychain") {
		t.Errorf("KeychainUnavailable message should mention keychain, got: %q", msg)
	}
}

// ── AgentIdentityMismatch ─────────────────────────────────────────────────────

func TestAgentIdentityMismatch_HasCorrectCode(t *testing.T) {
	err := vdxerr.AgentIdentityMismatch()
	if err.Code != vdxerr.ErrAgentIdentityMismatch {
		t.Errorf("Code = %q, want %q", err.Code, vdxerr.ErrAgentIdentityMismatch)
	}
}

func TestAgentIdentityMismatch_MessageDescribesMismatch(t *testing.T) {
	err := vdxerr.AgentIdentityMismatch()
	msg := err.UserMessage()
	if !strings.Contains(strings.ToLower(msg), "mismatch") {
		t.Errorf("AgentIdentityMismatch message should describe the mismatch, got: %q", msg)
	}
}

// ── RawHTMLInSubmission ───────────────────────────────────────────────────────

func TestRawHTMLInSubmission_HasCorrectCode(t *testing.T) {
	err := vdxerr.RawHTMLInSubmission()
	if err.Code != vdxerr.ErrRawHTMLInSubmission {
		t.Errorf("Code = %q, want %q", err.Code, vdxerr.ErrRawHTMLInSubmission)
	}
}

func TestRawHTMLInSubmission_MessageMentionsHTML(t *testing.T) {
	err := vdxerr.RawHTMLInSubmission()
	msg := err.UserMessage()
	if !strings.Contains(strings.ToLower(msg), "html") {
		t.Errorf("RawHTMLInSubmission message should mention HTML, got: %q", msg)
	}
}

// ── OptimisticLockConflict ────────────────────────────────────────────────────

func TestOptimisticLockConflict_HasCorrectCode(t *testing.T) {
	err := vdxerr.OptimisticLockConflict("etag-xyz")
	if err.Code != vdxerr.ErrOptimisticLockConflict {
		t.Errorf("Code = %q, want %q", err.Code, vdxerr.ErrOptimisticLockConflict)
	}
}

func TestOptimisticLockConflict_ContainsEtag(t *testing.T) {
	err := vdxerr.OptimisticLockConflict("etag-xyz-789")
	msg := err.UserMessage()
	if !strings.Contains(msg, "etag-xyz-789") {
		t.Errorf("OptimisticLockConflict message should contain the current etag, got: %q", msg)
	}
}

func TestOptimisticLockConflict_MessageMentionsRefetch(t *testing.T) {
	err := vdxerr.OptimisticLockConflict("any-etag")
	msg := err.UserMessage()
	if !strings.Contains(strings.ToLower(msg), "re-fetch") && !strings.Contains(strings.ToLower(msg), "refetch") {
		t.Errorf("OptimisticLockConflict message should instruct to re-fetch, got: %q", msg)
	}
}

// ── RateLimitExceeded ─────────────────────────────────────────────────────────

func TestRateLimitExceeded_HasCorrectCode(t *testing.T) {
	err := vdxerr.RateLimitExceeded("60 writes/minute exceeded")
	if err.Code != vdxerr.ErrRateLimitExceeded {
		t.Errorf("Code = %q, want %q", err.Code, vdxerr.ErrRateLimitExceeded)
	}
}

func TestRateLimitExceeded_ContainsReason(t *testing.T) {
	err := vdxerr.RateLimitExceeded("circuit breaker open")
	msg := err.UserMessage()
	if !strings.Contains(msg, "circuit breaker open") {
		t.Errorf("RateLimitExceeded message should contain reason, got: %q", msg)
	}
}

// ── SecretFileBlocked: does not reflect input ─────────────────────────────────

func TestSecretFileBlocked_DoesNotReflectInput(t *testing.T) {
	maliciousPath := "/workspace/.env\x1b[31m INJECTED \x1b[0m"
	err := vdxerr.SecretFileBlocked(maliciousPath)
	msg := err.UserMessage()
	if strings.Contains(msg, "INJECTED") {
		t.Errorf("SecretFileBlocked user message reflects injected content: %q", msg)
	}
}

// ── Error interface compliance ────────────────────────────────────────────────

func TestVedoxError_ImplementsErrorInterface(t *testing.T) {
	var _ error = vdxerr.New(vdxerr.ErrPortInUse, "test")
}

// ── Code constant values ──────────────────────────────────────────────────────

func TestCodeConstants_HaveExpectedValues(t *testing.T) {
	tests := []struct {
		code vdxerr.Code
		want string
	}{
		{vdxerr.ErrPortInUse, "VDX-001"},
		{vdxerr.ErrConfigNotFound, "VDX-002"},
		{vdxerr.ErrGitIdentityUnset, "VDX-003"},
		{vdxerr.ErrWorkspaceRootNotFound, "VDX-004"},
		{vdxerr.ErrPathTraversal, "VDX-005"},
		{vdxerr.ErrSecretFileBlocked, "VDX-006"},
		{vdxerr.ErrPayloadTooLarge, "VDX-010"},
		{vdxerr.ErrReadOnly, "VDX-011"},
		{vdxerr.ErrAgentAuthFailed, "VDX-300"},
		{vdxerr.ErrAgentScopeViolation, "VDX-301"},
		{vdxerr.ErrKeychainUnavailable, "VDX-302"},
		{vdxerr.ErrAgentIdentityMismatch, "VDX-303"},
		{vdxerr.ErrRawHTMLInSubmission, "VDX-304"},
		{vdxerr.ErrOptimisticLockConflict, "VDX-305"},
		{vdxerr.ErrRateLimitExceeded, "VDX-306"},
	}
	for _, tt := range tests {
		if string(tt.code) != tt.want {
			t.Errorf("code %q has value %q, want %q", tt.code, string(tt.code), tt.want)
		}
	}
}
