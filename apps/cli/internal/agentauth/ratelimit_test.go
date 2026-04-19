package agentauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

// newIsolatedMiddleware returns a fresh middleware instance backed by
// independent NonceCache and keyRateLimiter instances so tests do not
// interfere with each other or with global state.
func newIsolatedMiddleware(ks *KeyStore) func(http.HandlerFunc) http.HandlerFunc {
	nc := NewNonceCache()
	krl := newKeyRateLimiter()
	return requireAgentWithCacheAndLimiter(ks, nc, krl)
}

// driveRequests sends n signed requests through the middleware and returns a
// slice of HTTP status codes in order.
//
// Each request uses a unique body (seeded with the request index) to avoid
// replay-cache conflicts. The nonce cache is shared across all calls because
// we pass a single middleware instance.
func driveRequests(t *testing.T, mw func(http.HandlerFunc) http.HandlerFunc, n int, keyID, secret string) []int {
	t.Helper()
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		body := []byte{byte(i >> 8), byte(i)} // unique 2-byte body per request
		req := signedRequest(t, "POST", "/api/projects/x/docs/y.md", keyID, secret, body)
		rec := httptest.NewRecorder()
		mw(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})(rec, req)
		codes[i] = rec.Code
	}
	return codes
}

// countCode counts occurrences of the target status in codes.
func countCode(codes []int, target int) int {
	n := 0
	for _, c := range codes {
		if c == target {
			n++
		}
	}
	return n
}

// --- Test: burst exhaustion → 429 ---
//
// Send rateLimitBurst+1 requests as fast as possible. The first rateLimitBurst
// must succeed; at least one additional request must be rejected with 429.
func TestRateLimit_BurstExhaustion(t *testing.T) {
	keyring.MockInit()
	ks, err := LoadKeyStore(t.TempDir())
	if err != nil {
		t.Fatalf("LoadKeyStore: %v", err)
	}
	id, secret, err := ks.IssueKey("burst-agent", "", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}

	mw := newIsolatedMiddleware(ks)
	// Send burst+50 requests to ensure we exceed the bucket.
	total := rateLimitBurst + 50
	codes := driveRequests(t, mw, total, id, secret)

	got429 := countCode(codes, http.StatusTooManyRequests)
	got200 := countCode(codes, http.StatusOK)

	if got200 < rateLimitBurst {
		t.Errorf("want at least %d 200s, got %d", rateLimitBurst, got200)
	}
	if got429 == 0 {
		t.Errorf("want at least one 429 after burst exhaustion, got none (200=%d)", got200)
	}
}

// --- Test: sustained 100 req/s → 200 ---
//
// Send exactly 100 requests spread over one second. Every single request
// should succeed because 100/s is exactly the sustained rate limit.
func TestRateLimit_Sustained100RPS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing-sensitive test in -short mode")
	}

	keyring.MockInit()
	ks, err := LoadKeyStore(t.TempDir())
	if err != nil {
		t.Fatalf("LoadKeyStore: %v", err)
	}
	id, secret, err := ks.IssueKey("sustained-agent", "", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}

	mw := newIsolatedMiddleware(ks)

	// Spread 100 requests evenly over 1 second (one per 10 ms).
	const n = 100
	interval := time.Second / n
	codes := make([]int, n)
	for i := 0; i < n; i++ {
		if i > 0 {
			time.Sleep(interval)
		}
		body := []byte{byte(i >> 8), byte(i)}
		req := signedRequest(t, "POST", "/api/projects/x/docs/y.md", id, secret, body)
		rec := httptest.NewRecorder()
		mw(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})(rec, req)
		codes[i] = rec.Code
	}

	got429 := countCode(codes, http.StatusTooManyRequests)
	if got429 > 0 {
		t.Errorf("sustained 100 req/s: want 0 rejections, got %d 429s", got429)
	}
}

// --- Test: distinct keys are isolated ---
//
// Exhaust key A's burst bucket, then confirm key B is unaffected.
func TestRateLimit_KeyIsolation(t *testing.T) {
	keyring.MockInit()
	ks, err := LoadKeyStore(t.TempDir())
	if err != nil {
		t.Fatalf("LoadKeyStore: %v", err)
	}
	idA, secA, err := ks.IssueKey("agent-a", "", "")
	if err != nil {
		t.Fatalf("IssueKey A: %v", err)
	}
	idB, secB, err := ks.IssueKey("agent-b", "", "")
	if err != nil {
		t.Fatalf("IssueKey B: %v", err)
	}

	// All requests share one middleware instance so the same keyRateLimiter is
	// used for both keys.
	nc := NewNonceCache()
	krl := newKeyRateLimiter()
	mwFn := requireAgentWithCacheAndLimiter(ks, nc, krl)

	// Exhaust key A.
	codesA := driveRequests(t, mwFn, rateLimitBurst+50, idA, secA)
	if countCode(codesA, http.StatusTooManyRequests) == 0 {
		t.Fatal("key A should have been rate-limited but was not")
	}

	// Key B gets its own limiter; one request should pass.
	body := []byte{0xde, 0xad}
	reqB := signedRequest(t, "POST", "/api/projects/x/docs/y.md", idB, secB, body)
	rec := httptest.NewRecorder()
	mwFn(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(rec, reqB)
	if rec.Code != http.StatusOK {
		t.Errorf("key B isolated: want 200, got %d", rec.Code)
	}
}

// --- Test: LRU eviction ---
//
// Fill the limiter cache to rateLimitMaxKeys entries, add one more, and verify
// the cache size stays bounded. Then confirm the evicted key still works (a
// fresh limiter is created on demand).
func TestRateLimit_LRUEviction(t *testing.T) {
	krl := newKeyRateLimiter()

	// Fill the cache exactly to capacity.
	for i := 0; i < rateLimitMaxKeys; i++ {
		keyID := string([]byte{byte(i >> 8), byte(i), 'k'})
		krl.Allow(keyID)
	}
	if got := krl.Len(); got != rateLimitMaxKeys {
		t.Fatalf("after fill: want %d entries, got %d", rateLimitMaxKeys, got)
	}

	// Adding one more entry must evict the LRU entry.
	krl.Allow("overflow-key")
	if got := krl.Len(); got != rateLimitMaxKeys {
		t.Fatalf("after overflow: want %d entries (LRU evicted), got %d", rateLimitMaxKeys, got)
	}

	// The evicted key can still be used — a new limiter is created on demand
	// with a full bucket, so Allow must return true.
	evictedKey := string([]byte{0x00, 0x00, 'k'}) // first key added, LRU candidate
	if !krl.Allow(evictedKey) {
		t.Error("re-adding evicted key: Allow should return true (fresh full bucket)")
	}
}

// --- Test: 429 response carries Retry-After header and VDX-306 code ---
func TestRateLimit_ResponseShape(t *testing.T) {
	keyring.MockInit()
	ks, err := LoadKeyStore(t.TempDir())
	if err != nil {
		t.Fatalf("LoadKeyStore: %v", err)
	}
	id, secret, err := ks.IssueKey("shape-agent", "", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}

	// Use a limiter pre-exhausted to zero tokens so the very first request is
	// rejected. We can do this by allowing rateLimitBurst+1 tokens first on a
	// fresh krl, then using the same krl for the middleware.
	nc := NewNonceCache()
	krl := newKeyRateLimiter()
	// Drain the burst bucket silently.
	for i := 0; i < rateLimitBurst+1; i++ {
		krl.Allow(id)
	}

	mw := requireAgentWithCacheAndLimiter(ks, nc, krl)
	body := []byte(`{}`)
	req := signedRequest(t, "POST", "/api/projects/x/docs/y.md", id, secret, body)
	rec := httptest.NewRecorder()
	mw(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("want 429, got %d", rec.Code)
	}
	if ra := rec.Header().Get("Retry-After"); ra == "" {
		t.Error("want Retry-After header, got none")
	}
	if code := decodeErr(t, rec.Body); code != "VDX-306" {
		t.Errorf("want VDX-306, got %s", code)
	}
}
