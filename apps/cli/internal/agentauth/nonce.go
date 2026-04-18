package agentauth

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
)

// nonceMaxEntries is the maximum number of (keyID, timestamp, bodyHash) tuples
// the cache will hold before it begins evicting the least-recently-used entry.
// At 10 000 entries the in-memory footprint is negligible (~1–2 MB); this is
// the practical upper bound given the 5-minute clock-skew window and a
// generous 100 req/s peak load per daemon instance.
const nonceMaxEntries = 10_000

// nonceTTL is the duration for which a seen nonce is remembered. It is set to
// 10 minutes — double the clockSkewTolerance — so that a request replayed at
// the very edge of the timestamp validity window is still caught by the cache.
const nonceTTL = 10 * time.Minute

// nonceKey is the composite cache key that uniquely identifies a single signed
// request. Two requests are considered identical replays if and only if all
// three fields are equal: the same API key, the same timestamp string, and the
// same body content.
//
// Using the raw timestamp string (not a parsed time.Time) is intentional: it
// preserves the exact bit-for-bit representation the client signed, preventing
// any normalisation bypass (e.g. "2026-04-15T12:00:00Z" vs "2026-04-15T12:00:00+00:00").
type nonceKey struct {
	keyID     string
	timestamp string
	bodyHash  string
}

// nonceEntry records when a nonce was first seen, used for TTL-based eviction.
type nonceEntry struct {
	seenAt time.Time
}

// NonceCache is a thread-safe, TTL-bounded LRU cache that detects replayed
// agent requests. It is keyed by (keyID, timestamp, bodyHash) tuples and
// evicts entries after nonceTTL or when the cache exceeds nonceMaxEntries,
// whichever comes first.
//
// The zero value is not usable; construct with NewNonceCache.
type NonceCache struct {
	mu    sync.Mutex
	cache *lru.Cache[nonceKey, nonceEntry]
	now   func() time.Time // injectable clock for deterministic testing
}

// NewNonceCache constructs a ready-to-use NonceCache.
func NewNonceCache() *NonceCache {
	c, err := lru.New[nonceKey, nonceEntry](nonceMaxEntries)
	if err != nil {
		// lru.New only errors when size <= 0, which cannot happen here.
		panic("agentauth: nonce cache size must be positive")
	}
	return &NonceCache{
		cache: c,
		now:   time.Now,
	}
}

// CheckAndRecord checks whether the tuple (keyID, timestamp, bodyHash) has
// been seen before and, if not, records it.
//
// Returns true if the nonce is FRESH (not seen before in the TTL window),
// false if the nonce is a REPLAY (already recorded).
//
// Thread-safe: multiple goroutines may call CheckAndRecord concurrently.
func (nc *NonceCache) CheckAndRecord(keyID, timestamp, bodyHash string) (fresh bool) {
	key := nonceKey{keyID: keyID, timestamp: timestamp, bodyHash: bodyHash}
	now := nc.now()

	nc.mu.Lock()
	defer nc.mu.Unlock()

	if entry, ok := nc.cache.Peek(key); ok {
		// Entry exists — check whether it is still within the TTL window.
		// If it has expired, treat as fresh (the timestamp validator has already
		// rejected any request outside the clock-skew window, so an expired
		// nonce here means a legitimate new request in a rare clock boundary
		// edge-case; recording it again is safe and conservative).
		if now.Sub(entry.seenAt) < nonceTTL {
			return false // replay detected
		}
		// Expired entry — fall through and overwrite with fresh record.
	}

	nc.cache.Add(key, nonceEntry{seenAt: now})
	return true
}

// globalNonceCache is the process-wide singleton. Middleware wires to this by
// default; tests can construct an isolated NonceCache via NewNonceCache().
var globalNonceCache = NewNonceCache()
