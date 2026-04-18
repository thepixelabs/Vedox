// Package api implements the HTTP API server that bridges the DocStore and
// SQLite search to the SvelteKit frontend.
//
// Security invariants enforced here (see EPIC-001 Security Architecture):
//   - CSP header on every response
//   - CORS restricted to localhost:5151 and 127.0.0.1:5151 only (Vite dev)
//   - Request/response bodies are never logged
//   - All path parameters are validated before use (see docs.go)
package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// corsAllowedOrigins is the explicit allowlist. Anything not in this list
// receives no CORS headers, which causes browsers to block the request.
// We do not use a wildcard — this is a localhost-only server and we want
// to be explicit about which origin the SvelteKit dev server uses.
var corsAllowedOrigins = map[string]bool{
	"http://localhost:5151":  true,
	"http://127.0.0.1:5151":  true,
}

// CSPHeaderValue is the v2.0 Content-Security-Policy mandated by binding
// ruling E9 (vedox-v2 MASTER_PLAN). It is exported so tests in the api
// package — and any future renderer that needs to emit a matching
// <meta http-equiv> in static build output — share a single source of truth.
//
// Policy breakdown:
//
//	default-src 'self'              fall-through: same-origin only
//	script-src  'self'              no inline JS, no eval, no remote scripts
//	style-src   'self' 'unsafe-inline'  REQUIRED by Shiki for syntax highlighting:
//	                                Shiki's tokenizer emits inline `style="..."`
//	                                attributes on every <span>. There is no
//	                                nonce/hash path that works with Shiki's
//	                                output in a static build (E1 forbids
//	                                per-request templating in v2.0). The XSS
//	                                blast radius is bounded because all
//	                                rendered Markdown is DOMPurify-sanitised
//	                                upstream and `script-src 'self'` blocks
//	                                the only meaningful exfiltration path.
//	                                The v2.1 plan lifts this to nonce-based
//	                                CSP once the daemon serves templated HTML.
//	img-src     'self' data:        data: URIs needed for inline diagrams
//	object-src  'none'              no <object>/<embed>/<applet>
//	frame-ancestors 'none'          un-iframable; equivalent to X-Frame-Options: DENY
const CSPHeaderValue = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; object-src 'none'; frame-ancestors 'none'"

// securityHeaders applies the security headers mandated by EPIC-001 to every
// response. This is called from corsMiddleware so the two concerns travel
// together and neither can be applied without the other.
func applySecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", CSPHeaderValue)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
}

// corsMiddleware adds CORS headers for allowed origins, sets the security
// headers on every response, and enforces a server-side Origin check on
// every state-mutating request (POST/PUT/PATCH/DELETE).
//
// CSRF defense: browser CORS is enforced on the RESPONSE side. A malicious
// page can still SEND a "simple" cross-origin request (e.g. POST with
// text/plain) and the browser will deliver it to the server even though it
// then blocks the response from being read. For a localhost API on a dev
// machine, that's a CSRF surface — a drive-by tab could write to provider
// config files. We close it by rejecting any mutating request whose Origin
// header is missing OR not in our allowlist. GET/HEAD/OPTIONS pass through.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		applySecurityHeaders(w)

		if corsAllowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
			w.Header().Set("Access-Control-Max-Age", "3600")
			// Vary tells caches that the response may differ by Origin.
			w.Header().Add("Vary", "Origin")
		}

		// Short-circuit preflight requests. The browser sends OPTIONS before
		// the real request to check CORS permissions; we answer and stop here.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// CSRF defense: reject mutating requests whose Origin is not in the
		// allowlist. The browser always sends Origin on cross-origin fetch
		// requests, so an empty Origin on a mutating verb is either a
		// command-line client (curl/Postman) — which we still want to require
		// the explicit header from in dev — or a drive-by attack we want to block.
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if !corsAllowedOrigins[origin] {
				slog.Warn("api: rejecting mutating request with disallowed origin",
					"method", r.Method,
					"path", r.URL.Path,
					"origin", origin,
				)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"VDX-403","message":"origin not allowed for mutating request"}`))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// requireBootstrapToken is a chi-compatible middleware (func(http.Handler)
// http.Handler) that enforces the daemon bootstrap token on the request.
//
// The token must be supplied as a Bearer credential in the Authorization header:
//
//	Authorization: Bearer <64-hex-char token>
//
// Failure modes (fail-closed):
//   - No token configured on the server   → 401 VDX-401
//   - Missing or malformed Authorization  → 401 VDX-401
//   - Token present but wrong value       → 401 VDX-401
//
// The comparison is constant-time to prevent timing attacks.
func (s *Server) requireBootstrapToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fail-closed: if no token has been configured, every request is
		// rejected. This prevents accidental open access when the daemon
		// starts without having called SetBootstrapToken.
		if s.bootstrapToken == "" {
			slog.Warn("api: browse auth: no bootstrap token configured — rejecting request",
				"method", r.Method, "path", r.URL.Path)
			writeError(w, http.StatusUnauthorized, "VDX-401", "authentication required")
			return
		}

		authHeader := r.Header.Get("Authorization")
		provided := strings.TrimPrefix(authHeader, "Bearer ")
		if authHeader == "" || provided == authHeader {
			// Header absent or not a Bearer scheme.
			slog.Warn("api: browse auth: missing or malformed Authorization header",
				"method", r.Method, "path", r.URL.Path)
			writeError(w, http.StatusUnauthorized, "VDX-401", "authentication required")
			return
		}

		// Constant-time comparison to prevent timing oracle.
		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.bootstrapToken)) != 1 {
			slog.Warn("api: browse auth: token mismatch",
				"method", r.Method, "path", r.URL.Path)
			writeError(w, http.StatusUnauthorized, "VDX-401", "authentication required")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware records every request with method, path, status code, and
// duration. Bodies are deliberately never logged (EPIC-001 logging invariant).
// Status capture uses a thin responseWriter wrapper so we can read the code
// after the handler returns.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		slog.Info("api.request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rw.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// statusRecorder wraps http.ResponseWriter to capture the HTTP status code
// written by the handler. It implements http.ResponseWriter only — no
// Hijacker/Flusher delegation is needed for this API server.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
