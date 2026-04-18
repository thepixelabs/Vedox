package providers

// auth.go — Provider-agnostic authentication interfaces used by the
// `vedox agent login` command and the Copilot bearer-JWT path.
//
// These types are kept in a small dedicated file (rather than appended to
// providers.go) so the interface surface is easy to find and so adding new
// auth capabilities later does not balloon the central provider contract
// file.

import (
	"context"
	"time"
)

// JWTSigner is the minimal interface adapters use to mint short-lived JWTs
// for the Copilot no-MCP path (and any future bearer-token-only providers).
//
// It is deliberately decoupled from KeyIssuer so the existing per-provider
// mocks compile unchanged — only the Copilot adapter needs JWT minting today
// and only the Copilot Login() path takes a JWTSigner argument.
//
// Implementations live in agentauth (KeyStore.SignJWTForKey) and in tests
// that need to assert on the issued kid without exercising the keychain.
type JWTSigner interface {
	// SignJWTForKey mints a JWT bound to the given keyID with the requested
	// lifetime. The implementation owns secret resolution — secrets do not
	// flow across this interface.
	SignJWTForKey(keyID string, lifetime time.Duration) (string, error)
}

// AgentAuthenticator is the optional capability a ProviderInstaller may
// implement to expose a `vedox agent login` flow. Today only the Copilot
// adapter implements this — all other adapters use per-request HMAC and do
// not need a separate "issue me a bearer token" call.
//
// Callers do a runtime type assertion on a ProviderInstaller value to test
// for support; an installer that does not implement AgentAuthenticator MUST
// be reported to the operator with a clear "this provider does not support
// `agent login`" message rather than a panic.
type AgentAuthenticator interface {
	// Login mints a short-lived bearer credential bound to the installer's
	// already-installed key. Returns an actionable error if the installer
	// has not been installed yet.
	Login(ctx context.Context, jwt JWTSigner) (string, error)
}
