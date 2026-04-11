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
