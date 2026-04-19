package agentauth

import (
	"log/slog"
	"net/http"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
	vdxerr "github.com/vedox/vedox/internal/errors"
	"golang.org/x/time/rate"
)

// rateLimitMaxKeys is the maximum number of per-key rate limiters the cache
// holds before evicting the least-recently-used entry. 1000 active keys is
// well beyond any realistic single-daemon deployment; at ~200 bytes per limiter
// the total footprint stays under 200 kB.
const rateLimitMaxKeys = 1000

// rateLimitRate is the sustained request rate permitted per API key (tokens/s).
const rateLimitRate rate.Limit = 100

// rateLimitBurst is the maximum instantaneous burst size per API key.
const rateLimitBurst = 200

// keyRateLimiter maintains one token-bucket rate limiter per API key ID.
// It is thread-safe and LRU-evicts at rateLimitMaxKeys entries so memory is
// bounded even if an attacker cycles through many key IDs.
//
// The zero value is not usable; construct with newKeyRateLimiter.
type keyRateLimiter struct {
	mu    sync.Mutex
	cache *lru.Cache[string, *rate.Limiter]
}

// newKeyRateLimiter constructs a ready-to-use keyRateLimiter.
func newKeyRateLimiter() *keyRateLimiter {
	c, err := lru.New[string, *rate.Limiter](rateLimitMaxKeys)
	if err != nil {
		// lru.New only errors when size <= 0, which cannot happen here.
		panic("agentauth: rate limiter cache size must be positive")
	}
	return &keyRateLimiter{cache: c}
}

// Allow reports whether keyID is permitted to make a request right now,
// consuming one token from its bucket. If the key has no existing limiter, one
// is created on first use. Returns false when the bucket is exhausted.
func (krl *keyRateLimiter) Allow(keyID string) bool {
	krl.mu.Lock()
	defer krl.mu.Unlock()

	l, ok := krl.cache.Get(keyID)
	if !ok {
		l = rate.NewLimiter(rateLimitRate, rateLimitBurst)
		krl.cache.Add(keyID, l)
	}
	return l.Allow()
}

// Len returns the number of tracked key limiters. Used by tests to verify LRU
// eviction behaviour.
func (krl *keyRateLimiter) Len() int {
	krl.mu.Lock()
	defer krl.mu.Unlock()
	return krl.cache.Len()
}

// globalKeyRateLimiter is the process-wide per-key rate limiter singleton.
// Middleware wires to this by default; tests that need isolation should call
// requireAgentWithCacheAndLimiter directly.
var globalKeyRateLimiter = newKeyRateLimiter()

// rejectRateLimit writes the VDX-306 / HTTP 429 response and logs the event at
// WARN level. The keyID is included in the log so operators can correlate
// bursts to a specific agent; it is never exposed in the HTTP response body.
func rejectRateLimit(w http.ResponseWriter, r *http.Request, keyID string) {
	slog.Warn("agentauth: rate limit exceeded",
		"method", r.Method,
		"path", r.URL.Path,
		"keyId", keyID,
	)
	w.Header().Set("Retry-After", "1")
	writeVDXError(w, http.StatusTooManyRequests, vdxerr.RateLimitExceeded("request rate exceeds 100 req/s sustained or 200 burst"))
}
