package agentauth

import "net/http"

// PassthroughAuth returns a Middleware that performs NO authentication.
// It exists solely for tests and for the current (pre-VDX-P3-INGEST) wiring
// where the api server expects a Middleware but no routes yet require auth.
//
// NEVER use this in a production code path. The linter-visible name is
// intentionally ugly so nobody reaches for it by accident.
func PassthroughAuth() Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return next
	}
}
