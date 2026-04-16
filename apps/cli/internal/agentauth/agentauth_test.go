package agentauth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zalando/go-keyring"
)

// newTestStore spins up a fresh KeyStore backed by a temp workspace and the
// go-keyring in-memory mock. Every test gets its own isolated keychain so
// the order of table cases is irrelevant.
func newTestStore(t *testing.T) *KeyStore {
	t.Helper()
	keyring.MockInit()
	ks, err := LoadKeyStore(t.TempDir())
	if err != nil {
		t.Fatalf("LoadKeyStore: %v", err)
	}
	return ks
}

// signedRequest builds an HTTP request with valid X-Vedox-* headers for the
// given key + secret. Returns the request and the raw body bytes so tests
// can mutate the body post-signing to simulate tampering.
func signedRequest(t *testing.T, method, path, keyID, secret string, body []byte) *http.Request {
	t.Helper()
	ts := time.Now().UTC().Format(time.RFC3339)
	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	sig := ComputeHMAC(secret, BuildSignedString(method, path, ts, hash))

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set(HeaderKeyID, keyID)
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderSignature, sig)
	return req
}

// runMiddleware invokes RequireAgent against a no-op handler and returns the
// recorded response. The handler captures the body that reached it so tests
// can assert the middleware rebuilt r.Body correctly.
//
// Each call gets its own isolated NonceCache so that repeated calls with the
// same signed request within a single test do not trigger replay rejection.
// Tests that explicitly want to verify replay behaviour should call
// runMiddlewareWithCache directly.
func runMiddleware(t *testing.T, ks *KeyStore, req *http.Request) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	return runMiddlewareWithCache(t, ks, NewNonceCache(), req)
}

// runMiddlewareWithCache invokes requireAgentWithCache with the supplied
// NonceCache. Use this in tests that need to share cache state across
// multiple requests (e.g., replay-detection tests).
func runMiddlewareWithCache(t *testing.T, ks *KeyStore, nc *NonceCache, req *http.Request) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	var reached []byte
	handler := requireAgentWithCache(ks, nc)(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		reached = b
		w.WriteHeader(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec, reached
}

// wrapChi mounts the handler inside a chi router so chi.URLParam("project")
// resolves during scope enforcement tests. Each call gets a fresh NonceCache
// so test isolation is preserved.
func wrapChi(ks *KeyStore, pattern string) http.Handler {
	r := chi.NewRouter()
	nc := NewNonceCache()
	r.Post(pattern, requireAgentWithCache(ks, nc)(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	return r
}

func decodeErr(t *testing.T, body *bytes.Buffer) string {
	t.Helper()
	var payload map[string]string
	if err := json.Unmarshal(body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error body: %v (raw=%q)", err, body.String())
	}
	return payload["code"]
}

// --- Test 1: valid request passes. ---
func TestRequireAgent_ValidRequest(t *testing.T) {
	ks := newTestStore(t)
	id, secret, err := ks.IssueKey("test-agent", "", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}

	body := []byte(`{"hello":"world"}`)
	req := signedRequest(t, "POST", "/api/projects/foo/docs/bar.md", id, secret, body)

	rec, reached := runMiddleware(t, ks, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Equal(reached, body) {
		t.Fatalf("handler saw body %q, want %q", reached, body)
	}
}

// --- Test 2: body tampered after signing. ---
func TestRequireAgent_TamperedBody(t *testing.T) {
	ks := newTestStore(t)
	id, secret, _ := ks.IssueKey("test-agent", "", "")

	signedBody := []byte(`{"hello":"world"}`)
	req := signedRequest(t, "POST", "/api/projects/foo/docs/bar.md", id, secret, signedBody)
	// Swap the body out for a different payload — signature now covers the
	// wrong hash and must be rejected.
	tampered := []byte(`{"hello":"evil"}`)
	req.Body = io.NopCloser(bytes.NewReader(tampered))
	req.ContentLength = int64(len(tampered))

	rec, _ := runMiddleware(t, ks, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
	if code := decodeErr(t, rec.Body); code != "VDX-300" {
		t.Fatalf("want VDX-300, got %s", code)
	}
}

// --- Test 3: timestamp 6 minutes in the past. ---
func TestRequireAgent_StaleTimestamp(t *testing.T) {
	ks := newTestStore(t)
	id, secret, _ := ks.IssueKey("test-agent", "", "")

	body := []byte(`{}`)
	path := "/api/projects/foo/docs/bar.md"
	stale := time.Now().UTC().Add(-6 * time.Minute).Format(time.RFC3339)
	sum := sha256.Sum256(body)
	sig := ComputeHMAC(secret, BuildSignedString("POST", path, stale, hex.EncodeToString(sum[:])))

	req := httptest.NewRequest("POST", path, bytes.NewReader(body))
	req.Header.Set(HeaderKeyID, id)
	req.Header.Set(HeaderTimestamp, stale)
	req.Header.Set(HeaderSignature, sig)

	rec, _ := runMiddleware(t, ks, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
	if code := decodeErr(t, rec.Body); code != "VDX-300" {
		t.Fatalf("want VDX-300, got %s", code)
	}
}

// --- Test 4: timestamp 6 minutes in the future. ---
func TestRequireAgent_FutureTimestamp(t *testing.T) {
	ks := newTestStore(t)
	id, secret, _ := ks.IssueKey("test-agent", "", "")

	body := []byte(`{}`)
	path := "/api/projects/foo/docs/bar.md"
	future := time.Now().UTC().Add(6 * time.Minute).Format(time.RFC3339)
	sum := sha256.Sum256(body)
	sig := ComputeHMAC(secret, BuildSignedString("POST", path, future, hex.EncodeToString(sum[:])))

	req := httptest.NewRequest("POST", path, bytes.NewReader(body))
	req.Header.Set(HeaderKeyID, id)
	req.Header.Set(HeaderTimestamp, future)
	req.Header.Set(HeaderSignature, sig)

	rec, _ := runMiddleware(t, ks, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
	if code := decodeErr(t, rec.Body); code != "VDX-300" {
		t.Fatalf("want VDX-300, got %s", code)
	}
}

// --- Test 5: unknown key ID. ---
func TestRequireAgent_UnknownKeyID(t *testing.T) {
	ks := newTestStore(t)
	// Issue a real key, then hand-craft a request using a bogus ID with the
	// real secret. The middleware should fail at lookup before ever seeing
	// the signature.
	_, secret, _ := ks.IssueKey("test-agent", "", "")
	body := []byte(`{}`)
	req := signedRequest(t, "POST", "/api/projects/foo/docs/bar.md", "not-a-real-id", secret, body)

	rec, _ := runMiddleware(t, ks, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
	if code := decodeErr(t, rec.Body); code != "VDX-300" {
		t.Fatalf("want VDX-300, got %s", code)
	}
}

// --- Test 6: revoked key. ---
func TestRequireAgent_RevokedKey(t *testing.T) {
	ks := newTestStore(t)
	id, secret, _ := ks.IssueKey("test-agent", "", "")
	if err := ks.RevokeKey(id); err != nil {
		t.Fatalf("RevokeKey: %v", err)
	}

	body := []byte(`{}`)
	req := signedRequest(t, "POST", "/api/projects/foo/docs/bar.md", id, secret, body)

	rec, _ := runMiddleware(t, ks, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
	if code := decodeErr(t, rec.Body); code != "VDX-300" {
		t.Fatalf("want VDX-300, got %s", code)
	}
}

// --- Test 7: project scope mismatch. ---
func TestRequireAgent_ProjectScopeMismatch(t *testing.T) {
	ks := newTestStore(t)
	id, secret, _ := ks.IssueKey("test-agent", "alpha", "")

	body := []byte(`{}`)
	path := "/api/projects/beta/docs/bar.md"
	req := signedRequest(t, "POST", path, id, secret, body)

	// Use a chi router so chi.URLParam("project") resolves to "beta".
	handler := wrapChi(ks, "/api/projects/{project}/docs/*")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if code := decodeErr(t, rec.Body); code != "VDX-301" {
		t.Fatalf("want VDX-301, got %s", code)
	}
}
