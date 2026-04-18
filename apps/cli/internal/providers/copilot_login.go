package providers

// copilot_login.go — JWT-based "agent login" path for the Copilot adapter.
//
// Copilot has no MCP layer, so it cannot perform the per-request HMAC
// handshake the other adapters use. Instead, the user runs
// `vedox agent login --provider copilot`, which calls Login() below to mint
// a short-lived JWT bearer token bound to the keyID that was issued during
// `vedox agent install --provider copilot`.
//
// The token is signed HS256 with the same HMAC secret stored in the OS
// keychain (or pluggable secret store) under the keyID. The 15-minute
// lifetime is the project-wide JWTLifetime — bound here through a small
// indirection so the package boundary stays clean.
//
// This file lives separately from copilot.go so the Login surface is easy
// to find and so the per-request HMAC pieces of the adapter remain a pure
// translation of the existing prose-installer flow.

import (
	"context"
	"fmt"
	"time"
)

// CopilotJWTLifetime is the validity window for a Login() JWT. Mirroring
// agentauth.JWTLifetime here (instead of importing it) keeps providers
// dependency-free of agentauth at the type level — the cmd layer wires the
// concrete agentauth.KeyStore in as the JWTSigner, which is the only place
// that needs to resolve secrets.
//
// Keep this in sync with agentauth.JWTLifetime; both are 15 minutes.
const CopilotJWTLifetime = 15 * time.Minute

// Login mints a short-lived JWT bound to the Copilot agent's installed key.
// The token is the auth credential a user pastes into Copilot's bearer-token
// field (or any future Copilot tool-call surface) — Copilot has no MCP
// layer and therefore cannot perform the per-request HMAC handshake the
// other adapters use.
//
// The returned string is a compact-form JWT signed HS256 with the key's
// HMAC secret. The lifetime is intentionally short (CopilotJWTLifetime,
// 15 minutes) so a leaked token blasts only a small replay window.
//
// Login is read-only: it does NOT issue or revoke any key. The caller is
// expected to have run `vedox agent install --provider copilot` already so
// a receipt exists with an AuthKeyID. If no receipt is found, Login returns
// an actionable error pointing at the install command.
//
// The JWTSigner argument is decoupled from KeyIssuer on purpose — only this
// path needs JWT minting today, and the existing copilot mock in
// providers_test does not need to learn a new method.
func (c *copilotInstaller) Login(_ context.Context, jwt JWTSigner) (string, error) {
	if jwt == nil {
		return "", fmt.Errorf("copilot login: nil JWTSigner")
	}
	if c.receiptStore == nil {
		return "", fmt.Errorf("copilot login: no receipt store configured")
	}
	receipt, err := c.receiptStore.Load(ProviderCopilot)
	if err != nil {
		return "", fmt.Errorf("copilot login: load receipt: %w", err)
	}
	if receipt == nil || receipt.AuthKeyID == "" {
		return "", fmt.Errorf("copilot login: no install receipt found — run 'vedox agent install --provider copilot' first")
	}
	token, err := jwt.SignJWTForKey(receipt.AuthKeyID, CopilotJWTLifetime)
	if err != nil {
		return "", fmt.Errorf("copilot login: sign jwt: %w", err)
	}
	return token, nil
}

// Compile-time assertion that *copilotInstaller satisfies AgentAuthenticator.
// If this stops compiling, the Login signature has drifted from the
// interface and the cmd layer's type assertion will silently fall through
// to the "unsupported provider" branch.
var _ AgentAuthenticator = (*copilotInstaller)(nil)
