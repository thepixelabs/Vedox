package agentauth

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// RejectAllAuth returns a Middleware that rejects every request with HTTP 503
// "key store unavailable". Use this as the fail-closed replacement for
// PassthroughAuth when the daemon could not load its HMAC key store at
// startup — an agent-protected endpoint cannot safely admit a request if
// there is no key material to validate against.
//
// Contract:
//   - Every request returns 503 with a JSON body {"code":"VDX-302","message":...}.
//   - The response body does NOT leak details about why the key store failed
//     to load; that information lives only in the daemon log.
//   - The WWW-Authenticate header is intentionally omitted — 503 signals a
//     server-side unavailability, not a client credential problem.
//
// This is a security-critical default. The previous implementation used
// PassthroughAuth on keystore failure, which is fail-OPEN: a partially
// initialised daemon would have served agent routes with no authentication
// at all. RejectAllAuth is fail-CLOSED: an unhealthy keystore means no
// agent routes can be reached, which is the correct behaviour under the
// assume-breach principle.
func RejectAllAuth() Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			slog.Warn("agentauth: request rejected — key store unavailable",
				"method", r.Method,
				"path", r.URL.Path,
			)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			payload := map[string]string{
				"code":    "VDX-302",
				"message": "key store unavailable",
			}
			_ = json.NewEncoder(w).Encode(payload)
		}
	}
}
