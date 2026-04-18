package agentauth

// jwt.go — Minimal JWT (RFC 7519) mint / verify helpers for the Copilot
// no-MCP authentication path.
//
// Why no external library?
//
//   The classic Go JWT libraries (github.com/golang-jwt/jwt, square/go-jose,
//   lestrrat-go/jwx) each add 5–15 transitive dependencies and a steady
//   stream of CVEs to chase. Vedox uses exactly one signing algorithm
//   (HS256) and one claim shape (kid, iat, exp). A 100-line in-tree
//   implementation is faster to audit and impossible to misconfigure.
//
// Security guarantees enforced here:
//
//   - The header `alg` field is pinned to "HS256" at verify time. Tokens
//     presenting `alg: none`, `alg: HS384`, `alg: RS256`, or anything else
//     are rejected before any signature work — this kills the entire
//     CVE-2015-9235 / "alg confusion" class of bugs.
//   - The signature is compared with hmac.Equal (constant time).
//   - The expiry claim is mandatory. Tokens without `exp`, with a
//     non-numeric `exp`, or with an `exp` in the past are rejected.
//   - The `typ` field, if present, must equal "JWT".
//   - Encoding is base64url *without padding* per RFC 7515 §2.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// JWTLifetime is the default validity window for an issued Copilot JWT.
// Fifteen minutes is short enough to limit replay impact if the token leaks
// into shell history or a screen-share, and long enough to comfortably cover
// a multi-step Copilot session without forcing a re-mint.
const JWTLifetime = 15 * time.Minute

// jwtAlg is the only signing algorithm Vedox accepts. Pinned both at mint
// and verify time. Changing this value is a breaking protocol change — bump
// a separate header field (e.g. add `vdxv: 2`) rather than silently
// switching algorithms.
const jwtAlg = "HS256"

// jwtTyp is the standard JWT type. Some clients and proxies inspect this;
// we set it for interoperability and reject any other value at verify.
const jwtTyp = "JWT"

// JWTClaims is the minimal claim set Vedox tokens carry. The keyID (kid) is
// the bridge back to the agentauth.KeyStore: the verifier looks up the HMAC
// secret for this kid and recomputes the signature.
//
// We deliberately do not include `iss`, `aud`, `sub`, `nbf`, or `jti` — they
// add validation surface for a single-issuer / single-audience system that
// gains nothing from them.
type JWTClaims struct {
	// KeyID is the agent API key ID this token authenticates as. The verifier
	// uses this to look up the shared HMAC secret. Mandatory.
	KeyID string `json:"kid"`

	// IssuedAt is the unix-seconds timestamp at which the token was minted.
	// Used only for human debugging; not enforced (we trust the issuer).
	IssuedAt int64 `json:"iat"`

	// ExpiresAt is the unix-seconds timestamp after which the token is
	// invalid. Mandatory. Tokens without exp are rejected.
	ExpiresAt int64 `json:"exp"`
}

// jwtHeader is the fixed header structure Vedox emits and validates.
// We deliberately allow unknown fields (some libraries add `kid` in the
// header) and validate only `alg` and `typ`.
type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ,omitempty"`
}

// MintJWT produces a signed compact-form JWT for the given keyID, valid for
// `lifetime` from now. The secret must be the HMAC secret associated with
// keyID in the KeyStore (or any non-empty byte slice for tests).
//
// Returns the standard three-part `header.payload.signature` string.
//
// A non-nil error is only returned for json.Marshal failures, which in
// practice cannot happen for the fixed header / claim shapes used here.
func MintJWT(secret []byte, keyID string, lifetime time.Duration) (string, error) {
	if len(secret) == 0 {
		return "", errors.New("jwt: empty secret")
	}
	if keyID == "" {
		return "", errors.New("jwt: empty keyID")
	}
	if lifetime <= 0 {
		return "", errors.New("jwt: non-positive lifetime")
	}

	now := time.Now().UTC()
	headerJSON, err := json.Marshal(jwtHeader{Alg: jwtAlg, Typ: jwtTyp})
	if err != nil {
		return "", fmt.Errorf("jwt: marshal header: %w", err)
	}
	claimsJSON, err := json.Marshal(JWTClaims{
		KeyID:     keyID,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(lifetime).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("jwt: marshal claims: %w", err)
	}

	signingInput := jwtB64(headerJSON) + "." + jwtB64(claimsJSON)
	sig := jwtHMAC(secret, signingInput)
	return signingInput + "." + jwtB64(sig), nil
}

// VerifyJWT parses and validates a compact-form JWT.
//
// On success returns the parsed claims. On any failure returns a non-nil
// error and a nil claims pointer.
//
// Validation order (fail-closed at every step):
//  1. Token is exactly three base64url segments separated by dots.
//  2. Header decodes as JSON with alg="HS256" and (if present) typ="JWT".
//     Any other alg — including "none" — is rejected before signature work.
//  3. Signature is recomputed over `header.payload` with the supplied
//     secret and compared with hmac.Equal (constant time).
//  4. Claims decode as JSON. exp is present and a finite unix-seconds value
//     in the future.
//
// VerifyJWT does NOT consult any KeyStore — the caller passes in the secret
// it has already resolved from the kid. This keeps the JWT primitive pure
// and testable, and keeps the KeyStore lookup at the call site where a 401
// can be returned cleanly.
func VerifyJWT(secret []byte, token string) (*JWTClaims, error) {
	if len(secret) == 0 {
		return nil, errors.New("jwt: empty secret")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("jwt: malformed token (expected 3 segments)")
	}
	headerB64, payloadB64, sigB64 := parts[0], parts[1], parts[2]

	// Step 1 — decode header.
	headerJSON, err := jwtB64Decode(headerB64)
	if err != nil {
		return nil, fmt.Errorf("jwt: decode header: %w", err)
	}
	var hdr jwtHeader
	if err := json.Unmarshal(headerJSON, &hdr); err != nil {
		return nil, fmt.Errorf("jwt: parse header: %w", err)
	}

	// Step 2 — pin algorithm. This single check defeats the entire alg-confusion
	// CVE class (CVE-2015-9235 et al). NEVER trust hdr.Alg to select the
	// verifier; we only accept HS256.
	if hdr.Alg != jwtAlg {
		return nil, fmt.Errorf("jwt: unsupported alg %q (want %q)", hdr.Alg, jwtAlg)
	}
	if hdr.Typ != "" && hdr.Typ != jwtTyp {
		return nil, fmt.Errorf("jwt: unsupported typ %q", hdr.Typ)
	}

	// Step 3 — recompute and compare signature in constant time.
	providedSig, err := jwtB64Decode(sigB64)
	if err != nil {
		return nil, fmt.Errorf("jwt: decode signature: %w", err)
	}
	expectedSig := jwtHMAC(secret, headerB64+"."+payloadB64)
	if !hmac.Equal(expectedSig, providedSig) {
		return nil, errors.New("jwt: signature mismatch")
	}

	// Step 4 — decode claims and enforce expiry.
	claimsJSON, err := jwtB64Decode(payloadB64)
	if err != nil {
		return nil, fmt.Errorf("jwt: decode claims: %w", err)
	}
	var claims JWTClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("jwt: parse claims: %w", err)
	}
	if claims.KeyID == "" {
		return nil, errors.New("jwt: missing kid claim")
	}
	if claims.ExpiresAt == 0 {
		return nil, errors.New("jwt: missing exp claim")
	}
	if time.Now().UTC().Unix() >= claims.ExpiresAt {
		return nil, errors.New("jwt: token expired")
	}

	return &claims, nil
}

// SignJWTForKey looks up the HMAC secret for the given keyID in this
// KeyStore and mints a JWT bound to it. Returns the compact-form JWT.
//
// The plaintext secret never leaves the KeyStore — it is loaded into a
// local byte slice, used for the single HMAC call, and dropped on return.
//
// Errors:
//   - VDX-302 (keychain unavailable) propagates from getSecret.
//   - "no entry for <keyID>" if the kid is unknown.
//   - "key is revoked" if the kid is tombstoned.
func (ks *KeyStore) SignJWTForKey(keyID string, lifetime time.Duration) (string, error) {
	key, ok := ks.Lookup(keyID)
	if !ok {
		return "", fmt.Errorf("jwt: no key entry for %s", keyID)
	}
	if key.Revoked {
		return "", fmt.Errorf("jwt: key %s is revoked", keyID)
	}
	secret, err := ks.getSecret(keyID)
	if err != nil {
		return "", err
	}
	return MintJWT([]byte(secret), keyID, lifetime)
}

// jwtB64 base64url-encodes b without padding (RFC 7515 §2).
func jwtB64(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// jwtB64Decode base64url-decodes s, accepting both padded and unpadded inputs
// for interop tolerance even though we never emit padded forms.
func jwtB64Decode(s string) ([]byte, error) {
	// RawURLEncoding rejects padding; tolerate it by stripping for interop.
	s = strings.TrimRight(s, "=")
	return base64.RawURLEncoding.DecodeString(s)
}

// jwtHMAC computes HMAC-SHA256(secret, signingInput) and returns the raw
// digest bytes (not hex/base64). Callers base64url-encode for transport.
func jwtHMAC(secret []byte, signingInput string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	return mac.Sum(nil)
}
