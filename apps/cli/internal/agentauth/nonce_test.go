package agentauth

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// NonceCache.CheckAndRecord — unit tests
// ---------------------------------------------------------------------------

// TestNonceCache_FreshNonce verifies that the first call with a new tuple
// returns true (fresh).
func TestNonceCache_FreshNonce(t *testing.T) {
	nc := NewNonceCache()
	if !nc.CheckAndRecord("key1", "2026-04-15T12:00:00Z", "abc123") {
		t.Error("expected first call to return fresh=true")
	}
}

// TestNonceCache_ReplayWithinTTL verifies that a second call with the same
// tuple within the TTL window returns false (replay).
func TestNonceCache_ReplayWithinTTL(t *testing.T) {
	nc := NewNonceCache()
	nc.CheckAndRecord("key1", "2026-04-15T12:00:00Z", "abc123")
	if nc.CheckAndRecord("key1", "2026-04-15T12:00:00Z", "abc123") {
		t.Error("expected second call with same tuple to return fresh=false (replay)")
	}
}

// TestNonceCache_DifferentKeyID verifies that the same timestamp+bodyHash
// under a different key ID is treated as a distinct, fresh nonce.
func TestNonceCache_DifferentKeyID(t *testing.T) {
	nc := NewNonceCache()
	nc.CheckAndRecord("key1", "2026-04-15T12:00:00Z", "abc123")
	if !nc.CheckAndRecord("key2", "2026-04-15T12:00:00Z", "abc123") {
		t.Error("expected different keyID to be treated as fresh")
	}
}

// TestNonceCache_DifferentTimestamp verifies that the same keyID+bodyHash
// with a different timestamp is treated as a distinct, fresh nonce.
func TestNonceCache_DifferentTimestamp(t *testing.T) {
	nc := NewNonceCache()
	nc.CheckAndRecord("key1", "2026-04-15T12:00:00Z", "abc123")
	if !nc.CheckAndRecord("key1", "2026-04-15T12:01:00Z", "abc123") {
		t.Error("expected different timestamp to be treated as fresh")
	}
}

// TestNonceCache_DifferentBodyHash verifies that the same keyID+timestamp
// with a different body hash is treated as a distinct, fresh nonce.
func TestNonceCache_DifferentBodyHash(t *testing.T) {
	nc := NewNonceCache()
	nc.CheckAndRecord("key1", "2026-04-15T12:00:00Z", "abc123")
	if !nc.CheckAndRecord("key1", "2026-04-15T12:00:00Z", "def456") {
		t.Error("expected different bodyHash to be treated as fresh")
	}
}

// TestNonceCache_ExpiredEntryIsRefresh verifies that a nonce whose TTL has
// elapsed is treated as fresh on re-presentation (simulated via clock injection).
func TestNonceCache_ExpiredEntryIsRefresh(t *testing.T) {
	nc := NewNonceCache()

	// First call: backdated so the entry appears 11 minutes old.
	past := time.Now().Add(-(nonceTTL + time.Minute))
	callCount := 0
	nc.now = func() time.Time {
		callCount++
		if callCount == 1 {
			return past
		}
		return time.Now()
	}

	nc.CheckAndRecord("key1", "2026-04-15T12:00:00Z", "abc123")

	// Second call: clock returns current time; entry seenAt is > nonceTTL ago,
	// so it should be treated as expired and the nonce accepted as fresh.
	if !nc.CheckAndRecord("key1", "2026-04-15T12:00:00Z", "abc123") {
		t.Error("expected expired nonce to be treated as fresh")
	}
}

// TestNonceCache_ConcurrentSafety runs concurrent CheckAndRecord calls and
// verifies exactly one caller sees fresh=true for a given tuple. This is a
// race-condition smoke test — run with -race to confirm.
func TestNonceCache_ConcurrentSafety(t *testing.T) {
	nc := NewNonceCache()
	const goroutines = 50
	var freshCount int
	var mu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if nc.CheckAndRecord("key1", "2026-04-15T12:00:00Z", "hash1") {
				mu.Lock()
				freshCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if freshCount != 1 {
		t.Errorf("expected exactly 1 goroutine to see fresh=true, got %d", freshCount)
	}
}

// ---------------------------------------------------------------------------
// Middleware replay integration — HTTP-level replay rejection
// ---------------------------------------------------------------------------

// TestRequireAgent_ReplayRejected verifies that a valid signed request is
// accepted on first presentation but rejected with HTTP 409 and VDX-307 on
// replay (same keyID + timestamp + bodyHash).
func TestRequireAgent_ReplayRejected(t *testing.T) {
	ks := newTestStore(t)
	id, secret, err := ks.IssueKey("agent", "", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}

	body := []byte(`{"action":"write"}`)
	nc := NewNonceCache()

	// First request — must succeed.
	req1 := signedRequest(t, "POST", "/api/projects/foo/docs/bar.md", id, secret, body)
	rec1, _ := runMiddlewareWithCache(t, ks, nc, req1)
	if rec1.Code != 200 {
		t.Fatalf("first request: want 200, got %d body=%s", rec1.Code, rec1.Body.String())
	}

	// Replay — identical keyID, timestamp, signature, and body. The second
	// signedRequest call would generate a new timestamp, so we copy the exact
	// headers from req1 to guarantee the tuple is identical.
	req2 := httptest.NewRequest("POST", "/api/projects/foo/docs/bar.md", bytes.NewReader(body))
	req2.Header.Set(HeaderKeyID, req1.Header.Get(HeaderKeyID))
	req2.Header.Set(HeaderTimestamp, req1.Header.Get(HeaderTimestamp))
	req2.Header.Set(HeaderSignature, req1.Header.Get(HeaderSignature))

	rec2, _ := runMiddlewareWithCache(t, ks, nc, req2)
	if rec2.Code != 409 {
		t.Fatalf("replay: want 409, got %d body=%s", rec2.Code, rec2.Body.String())
	}
	if code := decodeErr(t, rec2.Body); code != "VDX-307" {
		t.Fatalf("replay: want VDX-307, got %s", code)
	}
}

// TestRequireAgent_FreshRequestAfterDifferentBody verifies that two requests
// from the same key with different bodies (different bodyHash) are both
// accepted — they are not replays.
func TestRequireAgent_FreshRequestAfterDifferentBody(t *testing.T) {
	ks := newTestStore(t)
	id, secret, err := ks.IssueKey("agent", "", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}

	nc := NewNonceCache()

	req1 := signedRequest(t, "POST", "/api/projects/foo/docs/a.md", id, secret, []byte(`{"n":1}`))
	rec1, _ := runMiddlewareWithCache(t, ks, nc, req1)
	if rec1.Code != 200 {
		t.Fatalf("first request: want 200, got %d", rec1.Code)
	}

	// Different body — different bodyHash — must not be treated as a replay.
	req2 := signedRequest(t, "POST", "/api/projects/foo/docs/a.md", id, secret, []byte(`{"n":2}`))
	rec2, _ := runMiddlewareWithCache(t, ks, nc, req2)
	if rec2.Code != 200 {
		t.Fatalf("second request (different body): want 200, got %d body=%s", rec2.Code, rec2.Body.String())
	}
}

// TestRequireAgent_ReplayResponseBody verifies that the 409 replay response
// body contains the VDX-307 code and the expected non-empty message.
func TestRequireAgent_ReplayResponseBody(t *testing.T) {
	ks := newTestStore(t)
	id, secret, err := ks.IssueKey("agent", "", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}

	body := []byte(`{}`)
	nc := NewNonceCache()

	// First request consumed by the cache.
	req1 := signedRequest(t, "POST", "/api/projects/foo/docs/bar.md", id, secret, body)
	runMiddlewareWithCache(t, ks, nc, req1) //nolint:errcheck

	// Build exact replay using req1's headers.
	req2 := httptest.NewRequest("POST", "/api/projects/foo/docs/bar.md", bytes.NewReader(body))
	req2.Header.Set(HeaderKeyID, req1.Header.Get(HeaderKeyID))
	req2.Header.Set(HeaderTimestamp, req1.Header.Get(HeaderTimestamp))
	req2.Header.Set(HeaderSignature, req1.Header.Get(HeaderSignature))

	rec2, _ := runMiddlewareWithCache(t, ks, nc, req2)

	if rec2.Code != 409 {
		t.Fatalf("want 409, got %d", rec2.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec2.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response body: %v (raw=%q)", err, rec2.Body.String())
	}
	if payload["code"] != "VDX-307" {
		t.Errorf("code = %q, want VDX-307", payload["code"])
	}
	if payload["message"] == "" {
		t.Error("expected non-empty message in replay rejection response")
	}
}
