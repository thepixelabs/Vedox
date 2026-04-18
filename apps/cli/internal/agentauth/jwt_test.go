package agentauth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// MintJWT / VerifyJWT — happy path roundtrip
// ---------------------------------------------------------------------------

// TestJWT_RoundtripValid mints a token with a fresh secret and key ID, then
// verifies it with the same secret and confirms the parsed claims match what
// was put in. This is the smoke test that guarantees the basic protocol works.
func TestJWT_RoundtripValid(t *testing.T) {
	secret := []byte("test-secret-32-bytes-of-randomness")
	keyID := "test-key-id-abc123"

	token, err := MintJWT(secret, keyID, 5*time.Minute)
	if err != nil {
		t.Fatalf("MintJWT: %v", err)
	}
	if strings.Count(token, ".") != 2 {
		t.Fatalf("token missing two dots, got: %q", token)
	}

	claims, err := VerifyJWT(secret, token)
	if err != nil {
		t.Fatalf("VerifyJWT: %v", err)
	}
	if claims.KeyID != keyID {
		t.Errorf("kid: got %q want %q", claims.KeyID, keyID)
	}
	if claims.IssuedAt == 0 {
		t.Error("iat should be non-zero")
	}
	if claims.ExpiresAt <= claims.IssuedAt {
		t.Errorf("exp (%d) should be > iat (%d)", claims.ExpiresAt, claims.IssuedAt)
	}
}

// TestJWT_DefaultLifetime sanity-checks that the public JWTLifetime constant
// is the documented 15 minutes, since callers depend on this exact value.
func TestJWT_DefaultLifetime(t *testing.T) {
	if JWTLifetime != 15*time.Minute {
		t.Errorf("JWTLifetime = %v, want 15m (documented contract)", JWTLifetime)
	}
}

// ---------------------------------------------------------------------------
// MintJWT — input validation
// ---------------------------------------------------------------------------

func TestMintJWT_RejectsEmptyInputs(t *testing.T) {
	cases := []struct {
		name     string
		secret   []byte
		keyID    string
		lifetime time.Duration
	}{
		{"empty secret", nil, "abc", time.Minute},
		{"empty keyID", []byte("s"), "", time.Minute},
		{"zero lifetime", []byte("s"), "abc", 0},
		{"negative lifetime", []byte("s"), "abc", -time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := MintJWT(tc.secret, tc.keyID, tc.lifetime); err == nil {
				t.Errorf("expected error for %s", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// VerifyJWT — failure modes
// ---------------------------------------------------------------------------

// TestVerifyJWT_Expired confirms a token whose exp is in the past is rejected.
// This is the most important freshness check — it bounds the impact of a
// leaked token to 15 minutes.
func TestVerifyJWT_Expired(t *testing.T) {
	secret := []byte("test-secret")
	// Mint a token that has already expired by hand-rolling the claims.
	token := mintWithCustomClaims(t, secret, JWTClaims{
		KeyID:     "kid",
		IssuedAt:  time.Now().Add(-30 * time.Minute).Unix(),
		ExpiresAt: time.Now().Add(-15 * time.Minute).Unix(),
	})

	if _, err := VerifyJWT(secret, token); err == nil {
		t.Fatal("expected expired token to be rejected")
	} else if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected 'expired' in error, got %v", err)
	}
}

// TestVerifyJWT_TamperedSignature confirms that flipping a single bit in the
// signature segment causes the constant-time comparison to fail.
func TestVerifyJWT_TamperedSignature(t *testing.T) {
	secret := []byte("test-secret")
	token, err := MintJWT(secret, "kid", time.Minute)
	if err != nil {
		t.Fatalf("MintJWT: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	// Decode, mutate, re-encode the signature so the result is still valid base64url.
	rawSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode sig: %v", err)
	}
	rawSig[0] ^= 0x01
	tampered := parts[0] + "." + parts[1] + "." + base64.RawURLEncoding.EncodeToString(rawSig)

	if _, err := VerifyJWT(secret, tampered); err == nil {
		t.Fatal("expected tampered signature to be rejected")
	}
}

// TestVerifyJWT_TamperedPayload confirms that changing the payload (and
// thereby the kid claim) without re-signing is rejected — this is the core
// integrity guarantee of the JWT.
func TestVerifyJWT_TamperedPayload(t *testing.T) {
	secret := []byte("test-secret")
	token, err := MintJWT(secret, "kid-original", time.Minute)
	if err != nil {
		t.Fatalf("MintJWT: %v", err)
	}
	parts := strings.Split(token, ".")
	// Replace payload with a different kid but keep original signature.
	evilClaims, _ := json.Marshal(JWTClaims{
		KeyID:     "kid-attacker",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	tampered := parts[0] + "." + base64.RawURLEncoding.EncodeToString(evilClaims) + "." + parts[2]

	if _, err := VerifyJWT(secret, tampered); err == nil {
		t.Fatal("expected payload tamper to be rejected")
	}
}

// TestVerifyJWT_AlgNoneRejected is the CVE-2015-9235 smoke test: a token
// whose header claims `alg: none` and ships an empty signature must be
// rejected before any signature work happens. This is the single most
// important check in the file.
func TestVerifyJWT_AlgNoneRejected(t *testing.T) {
	header, _ := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	claims, _ := json.Marshal(JWTClaims{
		KeyID:     "kid",
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Minute).Unix(),
	})
	// Empty signature segment, as the alg=none attack expects.
	token := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(claims) + "."

	if _, err := VerifyJWT([]byte("any-secret"), token); err == nil {
		t.Fatal("CVE-2015-9235: alg=none token MUST be rejected")
	} else if !strings.Contains(err.Error(), "alg") {
		t.Errorf("expected alg-related error, got %v", err)
	}
}

// TestVerifyJWT_WrongAlgRejected covers the rest of the alg-confusion class:
// a token signed with HS512 (or claiming RS256) must not be accepted by an
// HS256-only verifier. We spot-check several values.
func TestVerifyJWT_WrongAlgRejected(t *testing.T) {
	cases := []string{"HS384", "HS512", "RS256", "ES256", "PS256", ""}
	for _, alg := range cases {
		t.Run("alg="+alg, func(t *testing.T) {
			header, _ := json.Marshal(map[string]string{"alg": alg, "typ": "JWT"})
			claims, _ := json.Marshal(JWTClaims{
				KeyID:     "kid",
				IssuedAt:  time.Now().Unix(),
				ExpiresAt: time.Now().Add(time.Minute).Unix(),
			})
			signingInput := base64.RawURLEncoding.EncodeToString(header) + "." +
				base64.RawURLEncoding.EncodeToString(claims)
			// Sign with HMAC-SHA256 using the same secret so an unsafe verifier
			// would accept it on signature alone — the alg pin is what saves us.
			sig := jwtHMAC([]byte("secret"), signingInput)
			token := signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)

			if _, err := VerifyJWT([]byte("secret"), token); err == nil {
				t.Errorf("alg=%q must be rejected", alg)
			}
		})
	}
}

// TestVerifyJWT_MalformedToken checks that obviously-broken inputs do not
// panic and do not pass verification.
func TestVerifyJWT_MalformedToken(t *testing.T) {
	cases := []string{
		"",
		"notajwt",
		"only.two",
		"too.many.dots.here.really",
		"!!!.!!!.!!!", // invalid base64
	}
	for _, tok := range cases {
		t.Run(tok, func(t *testing.T) {
			if _, err := VerifyJWT([]byte("s"), tok); err == nil {
				t.Errorf("expected error for token %q", tok)
			}
		})
	}
}

// TestVerifyJWT_MissingExpRejected covers the case where a malicious or
// malformed token omits the exp claim — without enforcement this would be a
// permanent token.
func TestVerifyJWT_MissingExpRejected(t *testing.T) {
	secret := []byte("test-secret")
	// Hand-craft a claim with no exp.
	claimsJSON, _ := json.Marshal(map[string]any{
		"kid": "kid",
		"iat": time.Now().Unix(),
	})
	headerJSON, _ := json.Marshal(jwtHeader{Alg: jwtAlg, Typ: jwtTyp})
	signingInput := jwtB64(headerJSON) + "." + jwtB64(claimsJSON)
	sig := jwtHMAC(secret, signingInput)
	token := signingInput + "." + jwtB64(sig)

	if _, err := VerifyJWT(secret, token); err == nil {
		t.Fatal("expected token without exp to be rejected")
	}
}

// TestVerifyJWT_WrongSecret confirms that a token minted with one secret
// fails verification under a different secret — sanity check that the secret
// is actually load-bearing.
func TestVerifyJWT_WrongSecret(t *testing.T) {
	tok, err := MintJWT([]byte("right-secret"), "kid", time.Minute)
	if err != nil {
		t.Fatalf("MintJWT: %v", err)
	}
	if _, err := VerifyJWT([]byte("wrong-secret"), tok); err == nil {
		t.Fatal("expected verification under wrong secret to fail")
	}
}

// TestVerifyJWT_EmptySecret defends against a footgun where a caller passes
// an empty secret slice (e.g. forgot to load it).
func TestVerifyJWT_EmptySecret(t *testing.T) {
	tok, _ := MintJWT([]byte("s"), "kid", time.Minute)
	if _, err := VerifyJWT(nil, tok); err == nil {
		t.Error("expected empty-secret verify to fail")
	}
}

// ---------------------------------------------------------------------------
// SignJWTForKey (KeyStore integration)
// ---------------------------------------------------------------------------

// TestSignJWTForKey_UnknownKey confirms the keystore-bound helper rejects
// kids that do not exist in the metadata file.
func TestSignJWTForKey_UnknownKey(t *testing.T) {
	ks := NewKeyStore(t.TempDir())
	if _, err := ks.SignJWTForKey("nonexistent", time.Minute); err == nil {
		t.Fatal("expected unknown key to be rejected")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// mintWithCustomClaims builds a fully-formed signed token with caller-supplied
// claims, used by tests that need to inject expired/malformed claim shapes
// the public MintJWT API would refuse to produce.
func mintWithCustomClaims(t *testing.T, secret []byte, claims JWTClaims) string {
	t.Helper()
	headerJSON, err := json.Marshal(jwtHeader{Alg: jwtAlg, Typ: jwtTyp})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	signingInput := jwtB64(headerJSON) + "." + jwtB64(claimsJSON)
	sig := jwtHMAC(secret, signingInput)
	return signingInput + "." + jwtB64(sig)
}
