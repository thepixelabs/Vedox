package agentauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// clockSkewTolerance is the maximum allowed difference between the client's
// X-Vedox-Timestamp header and the server's wall clock. Five minutes is the
// industry-standard value (AWS SigV4, Kubernetes service account tokens,
// OAuth 2.0 JWT bearer): long enough to tolerate NTP drift and dev-machine
// clock skew, short enough to limit replay windows.
const clockSkewTolerance = 5 * time.Minute

// BuildSignedString returns the canonical string-to-sign for an agent request.
//
// Format (newline-separated, no trailing newline):
//
//	METHOD
//	PATH
//	TIMESTAMP
//	BODY_SHA256_HEX
//
// The client and server MUST compute this identically. Changing the format
// is a breaking protocol change — bump the X-Vedox-Sig-Version header (when
// introduced) rather than silently mutating this function.
//
// PATH is the raw URL path including the leading slash, without query string.
// Query strings are not part of the signed payload in this initial version;
// mutating endpoints do not accept query params.
func BuildSignedString(method, path, timestamp, bodyHashHex string) string {
	return method + "\n" + path + "\n" + timestamp + "\n" + bodyHashHex
}

// ComputeHMAC returns hex(HMAC-SHA256(secret, signedString)).
// Both inputs are strings because the HMAC secret is a hex-encoded random
// value (see IssueKey) and the signed string is printable ASCII.
func ComputeHMAC(secret, signedString string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedString))
	return hex.EncodeToString(mac.Sum(nil))
}

// SecureEqual performs a constant-time comparison of two strings. Use this
// for ALL signature comparisons. Never use == or strings.EqualFold on a
// signature — the length-dependent short-circuit in == leaks information to
// a timing-sensitive attacker.
func SecureEqual(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

// ValidateTimestamp parses an RFC3339 timestamp string and returns nil if it
// is within clockSkewTolerance of the current wall clock. Otherwise it
// returns an error describing the violation.
//
// Rejecting stale timestamps is the primary defense against replay attacks.
// The companion defense (a nonce cache) is deferred to VDX-P3-INGEST because
// it requires per-project state; for the auth layer, timestamp freshness is
// sufficient to limit the replay window to the tolerance.
func ValidateTimestamp(ts string) error {
	if ts == "" {
		return fmt.Errorf("missing timestamp")
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return fmt.Errorf("invalid timestamp format (expected RFC3339): %w", err)
	}
	delta := time.Since(t)
	if delta < 0 {
		delta = -delta
	}
	if delta > clockSkewTolerance {
		return fmt.Errorf("timestamp outside ±%s tolerance", clockSkewTolerance)
	}
	return nil
}
