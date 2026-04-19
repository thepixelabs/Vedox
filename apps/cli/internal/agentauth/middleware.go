package agentauth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	vdxerr "github.com/vedox/vedox/internal/errors"
)

// Request headers used by the agent auth protocol. Header names are
// case-insensitive per RFC 7230; we use the canonicalised forms for clarity.
const (
	HeaderKeyID     = "X-Vedox-Key-Id"
	HeaderTimestamp = "X-Vedox-Timestamp"
	HeaderSignature = "X-Vedox-Signature"
)

// maxAgentBodyBytes is a hard ceiling on how much body the middleware is
// willing to buffer into memory for hashing. Larger payloads are rejected
// with VDX-010 before any expensive crypto work. 2 MB is double the standard
// doc write limit — ingestion endpoints set a stricter per-route limit on
// top of this.
const maxAgentBodyBytes = 2 * 1024 * 1024

// Middleware is the signature of an auth middleware function. It takes an
// inner handler and returns a wrapping handler that enforces authentication.
// The api package accepts this type directly so it does not need to import
// the agentauth package.
type Middleware func(http.HandlerFunc) http.HandlerFunc

// RequireAgent returns middleware that enforces HMAC-SHA256 authentication
// against the given KeyStore on all requests it wraps. It uses the
// process-wide nonce cache to reject replayed requests and the process-wide
// per-key rate limiter to reject bursts exceeding 100 req/s sustained / 200
// burst per API key.
//
// Validation order (fail-closed at every step):
//  1. Key ID header present and resolves to a known, non-revoked APIKey.
//  2. Timestamp header present, parseable, and within ±5 minutes of now.
//  3. Request body buffered and sha256-hashed; r.Body is rebuilt so the
//     downstream handler can still read it.
//  4. Expected HMAC is computed from (method, path, timestamp, bodyHash).
//  5. Provided signature compared with SecureEqual.
//  5b. Rate limit check: the keyID must not have exceeded its token-bucket
//      quota. Overflow returns HTTP 429 (VDX-306). This check runs AFTER
//      signature validation so unauthenticated requests never consume quota,
//      but BEFORE the nonce check so denial is cheap (no cache write).
//  5c. Nonce cache check: the (keyID, timestamp, bodyHash) tuple must not
//      have been seen within the last 10 minutes. Replays return HTTP 409
//      with VDX-307 — distinct from VDX-300 so legitimate agents can
//      distinguish a replay rejection from a credential error.
//  6. Scope enforcement: key.Project (if set) must match the chi "project"
//     URL param, and key.PathPrefix (if set) must be a prefix of r.URL.Path.
//     Scope violations return VDX-301 (403), not VDX-300 (401) — the agent
//     is authenticated, just not permitted.
//
// Every auth failure is logged at WARN level with the key ID (never the
// signature, timestamp, or body). The user-facing error is the generic
// VDX-300 message — we do not reveal which specific check failed, to avoid
// building an oracle an attacker can use to probe the system.
func RequireAgent(ks *KeyStore) Middleware {
	return requireAgentWithCacheAndLimiter(ks, globalNonceCache, globalKeyRateLimiter)
}

// requireAgentWithCache is retained for backward compatibility with existing
// tests that only need to inject an isolated NonceCache. It wires the
// process-wide rate limiter.
func requireAgentWithCache(ks *KeyStore, nc *NonceCache) Middleware {
	return requireAgentWithCacheAndLimiter(ks, nc, globalKeyRateLimiter)
}

// requireAgentWithCacheAndLimiter is the fully-injectable constructor used by
// RequireAgent and by tests that need isolated state for both the nonce cache
// and the per-key rate limiter.
func requireAgentWithCacheAndLimiter(ks *KeyStore, nc *NonceCache, krl *keyRateLimiter) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// 1. Key ID lookup.
			keyID := r.Header.Get(HeaderKeyID)
			if keyID == "" {
				rejectAuth(w, r, "", "missing key id header")
				return
			}
			key, ok := ks.Lookup(keyID)
			if !ok {
				rejectAuth(w, r, keyID, "unknown key id")
				return
			}
			if key.Revoked {
				rejectAuth(w, r, keyID, "key is revoked")
				return
			}

			// 2. Timestamp validation.
			ts := r.Header.Get(HeaderTimestamp)
			if err := ValidateTimestamp(ts); err != nil {
				rejectAuth(w, r, keyID, "timestamp: "+err.Error())
				return
			}

			// 3. Body buffer + hash. We cap the read at maxAgentBodyBytes + 1
			//    so we can detect oversize and reject cleanly.
			var body []byte
			if r.Body != nil {
				limited := io.LimitReader(r.Body, maxAgentBodyBytes+1)
				b, err := io.ReadAll(limited)
				if err != nil {
					rejectAuth(w, r, keyID, "could not read body")
					return
				}
				if len(b) > maxAgentBodyBytes {
					writeVDXError(w, http.StatusRequestEntityTooLarge, vdxerr.PayloadTooLarge())
					return
				}
				body = b
			}
			// Rebuild r.Body so downstream handlers can read it.
			r.Body = io.NopCloser(bytes.NewReader(body))

			sum := sha256.Sum256(body)
			bodyHash := hex.EncodeToString(sum[:])

			// 4. Fetch secret and compute expected signature.
			secret, err := ks.getSecret(keyID)
			if err != nil {
				// If this is a VDX-302 keychain error, surface it faithfully
				// so operators can diagnose misconfiguration. Otherwise treat
				// as a generic auth failure (missing entry = unknown key).
				var vdx *vdxerr.VedoxError
				if asVDX(err, &vdx) && vdx.Code == vdxerr.ErrKeychainUnavailable {
					writeVDXError(w, http.StatusInternalServerError, vdx)
					return
				}
				rejectAuth(w, r, keyID, "no keychain entry")
				return
			}

			signed := BuildSignedString(r.Method, r.URL.Path, ts, bodyHash)
			expected := ComputeHMAC(secret, signed)

			// 5. Constant-time signature compare.
			provided := r.Header.Get(HeaderSignature)
			if provided == "" || !SecureEqual(expected, provided) {
				rejectAuth(w, r, keyID, "signature mismatch")
				return
			}

			// 5b. Rate limit check. The keyID's token bucket is consulted after
			//     the signature has been verified so unauthenticated callers
			//     cannot deplete legitimate keys' quotas. Denial is cheap here:
			//     no cache write occurs, and the 429 is the cheapest possible
			//     rejection path after HMAC verification.
			if !krl.Allow(keyID) {
				rejectRateLimit(w, r, keyID)
				return
			}

			// 5c. Nonce / replay check. The (keyID, timestamp, bodyHash) tuple
			//     is recorded in the nonce cache on the first valid request.
			//     Any identical replay within the 10-minute TTL window is
			//     rejected with 409 Conflict (VDX-307) — a separate code from
			//     VDX-300 so agents can distinguish replay rejection from a
			//     credential error and know not to retry with the same request.
			if !nc.CheckAndRecord(keyID, ts, bodyHash) {
				slog.Warn("agentauth: replay rejected",
					"method", r.Method,
					"path", r.URL.Path,
					"keyId", keyID,
				)
				writeVDXError(w, http.StatusConflict, vdxerr.ReplayedRequest())
				return
			}

			// 6. Scope enforcement.
			if key.Project != "" {
				urlProject := chi.URLParam(r, "project")
				if urlProject == "" || urlProject != key.Project {
					rejectScope(w, r, keyID, "project mismatch")
					return
				}
			}
			if key.PathPrefix != "" && !strings.HasPrefix(r.URL.Path, key.PathPrefix) {
				rejectScope(w, r, keyID, "path prefix mismatch")
				return
			}

			next(w, r)
		}
	}
}

// rejectAuth writes the generic VDX-300 response and logs the underlying
// reason at WARN level. The reason is NEVER included in the HTTP response
// body — it exists only for operator debugging.
func rejectAuth(w http.ResponseWriter, r *http.Request, keyID, reason string) {
	slog.Warn("agentauth: auth rejected",
		"method", r.Method,
		"path", r.URL.Path,
		"keyId", keyID,
		"reason", reason,
	)
	writeVDXError(w, http.StatusUnauthorized, vdxerr.AgentAuthFailed())
}

// rejectScope writes a VDX-301 response (403) — the agent is known but not
// permitted to touch this resource. This is deliberately distinct from
// VDX-300 so legitimate agents get an actionable error when misconfigured.
func rejectScope(w http.ResponseWriter, r *http.Request, keyID, reason string) {
	slog.Warn("agentauth: scope rejected",
		"method", r.Method,
		"path", r.URL.Path,
		"keyId", keyID,
		"reason", reason,
	)
	writeVDXError(w, http.StatusForbidden, vdxerr.AgentScopeViolation(keyID))
}

// writeVDXError serialises a VedoxError as a JSON response with the given
// HTTP status. This duplicates a helper in the api package on purpose: the
// agentauth package must not import api (that would create an import cycle
// because api depends on agentauth for its middleware type).
func writeVDXError(w http.ResponseWriter, status int, e *vdxerr.VedoxError) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	payload := map[string]string{
		"code":    string(e.Code),
		"message": e.Message,
	}
	_ = json.NewEncoder(w).Encode(payload)
}

// asVDX is a tiny helper over errors.As to keep call sites readable. It is
// defined locally (instead of using errors.As directly) so the middleware
// body reads top-to-bottom without type-assertion noise.
func asVDX(err error, out **vdxerr.VedoxError) bool {
	for e := err; e != nil; {
		if v, ok := e.(*vdxerr.VedoxError); ok {
			*out = v
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := e.(unwrapper)
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}
