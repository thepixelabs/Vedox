package agentauth

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	vdxerr "github.com/vedox/vedox/internal/errors"
	"github.com/zalando/go-keyring"
)

// ---------------------------------------------------------------------------
// KeyStore — ListKeys
// ---------------------------------------------------------------------------

// TestListKeys_EmptyStore verifies that ListKeys returns an empty (non-nil)
// slice when no keys have been issued.
func TestListKeys_EmptyStore(t *testing.T) {
	ks := newTestStore(t)
	keys := ks.ListKeys()
	if keys == nil {
		t.Error("expected non-nil slice, got nil")
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

// TestListKeys_SortedByCreatedAt verifies that ListKeys returns keys in
// ascending CreatedAt order regardless of insertion order.
func TestListKeys_SortedByCreatedAt(t *testing.T) {
	ks := newTestStore(t)
	for _, name := range []string{"first", "second", "third"} {
		if _, _, err := ks.IssueKey(name, "", ""); err != nil {
			t.Fatalf("IssueKey %s: %v", name, err)
		}
		// Small sleep so each key gets a strictly later CreatedAt.
		time.Sleep(2 * time.Millisecond)
	}
	keys := ks.ListKeys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	for i := 1; i < len(keys); i++ {
		if keys[i].CreatedAt.Before(keys[i-1].CreatedAt) {
			t.Errorf("keys[%d].CreatedAt %v is before keys[%d].CreatedAt %v",
				i, keys[i].CreatedAt, i-1, keys[i-1].CreatedAt)
		}
	}
}

// TestListKeys_IncludesRevokedKeys verifies that revoked keys remain in the
// list (audit trail) but are marked Revoked=true.
func TestListKeys_IncludesRevokedKeys(t *testing.T) {
	ks := newTestStore(t)
	id, _, err := ks.IssueKey("agent", "", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}
	if err := ks.RevokeKey(id); err != nil {
		t.Fatalf("RevokeKey: %v", err)
	}
	keys := ks.ListKeys()
	if len(keys) != 1 {
		t.Fatalf("expected 1 key (tombstoned), got %d", len(keys))
	}
	if !keys[0].Revoked {
		t.Error("expected Revoked=true for the tombstoned key")
	}
}

// ---------------------------------------------------------------------------
// RevokeKey — error paths
// ---------------------------------------------------------------------------

// TestRevokeKey_Nonexistent verifies that revoking an unknown ID returns an error.
func TestRevokeKey_Nonexistent(t *testing.T) {
	ks := newTestStore(t)
	if err := ks.RevokeKey("does-not-exist"); err == nil {
		t.Error("expected error when revoking unknown key, got nil")
	}
}

// TestRevokeKey_Idempotent verifies that revoking an already-revoked key
// returns nil (no-op).
func TestRevokeKey_Idempotent(t *testing.T) {
	ks := newTestStore(t)
	id, _, err := ks.IssueKey("agent", "", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}
	if err := ks.RevokeKey(id); err != nil {
		t.Fatalf("first RevokeKey: %v", err)
	}
	if err := ks.RevokeKey(id); err != nil {
		t.Errorf("second RevokeKey (idempotent) returned error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// LoadKeyStore — branches not exercised by the existing test helper
// ---------------------------------------------------------------------------

// TestLoadKeyStore_EmptyFile verifies that a zero-byte metadata file is
// treated as an empty store (not a parse error).
func TestLoadKeyStore_EmptyFile(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	vedoxDir := dir + "/.vedox"
	if err := os.MkdirAll(vedoxDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(vedoxDir+"/agent-keys.json", []byte{}, 0o600); err != nil {
		t.Fatalf("write empty file: %v", err)
	}
	ks, err := LoadKeyStore(dir)
	if err != nil {
		t.Fatalf("LoadKeyStore with empty file: %v", err)
	}
	if len(ks.ListKeys()) != 0 {
		t.Errorf("expected 0 keys, got %d", len(ks.ListKeys()))
	}
}

// TestLoadKeyStore_PersistsAndReloads verifies the round-trip: issue keys,
// open a new KeyStore from the same workspace, and find the same metadata.
func TestLoadKeyStore_PersistsAndReloads(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()

	ks1, err := LoadKeyStore(dir)
	if err != nil {
		t.Fatalf("LoadKeyStore (first): %v", err)
	}
	id1, _, err := ks1.IssueKey("agent-a", "proj", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}

	// Open a fresh KeyStore backed by the same workspace directory.
	ks2, err := LoadKeyStore(dir)
	if err != nil {
		t.Fatalf("LoadKeyStore (second): %v", err)
	}
	keys := ks2.ListKeys()
	if len(keys) != 1 {
		t.Fatalf("expected 1 key after reload, got %d", len(keys))
	}
	if keys[0].ID != id1 {
		t.Errorf("reloaded key ID = %q, want %q", keys[0].ID, id1)
	}
	if keys[0].Project != "proj" {
		t.Errorf("reloaded key Project = %q, want %q", keys[0].Project, "proj")
	}
}

// TestNewKeyStore_EmptyStore verifies that NewKeyStore returns an empty store
// without performing any disk I/O.
func TestNewKeyStore_EmptyStore(t *testing.T) {
	keyring.MockInit()
	ks := NewKeyStore(t.TempDir())
	if ks == nil {
		t.Fatal("NewKeyStore returned nil")
	}
	if len(ks.ListKeys()) != 0 {
		t.Errorf("expected empty store, got %d keys", len(ks.ListKeys()))
	}
}

// ---------------------------------------------------------------------------
// PassthroughAuth — testing.go
// ---------------------------------------------------------------------------

// TestPassthroughAuth_Passes verifies that PassthroughAuth lets every request
// through to the inner handler unchanged.
func TestPassthroughAuth_Passes(t *testing.T) {
	reached := false
	mw := PassthroughAuth()
	handler := mw(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest("GET", "/api/projects/foo/docs/bar.md", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !reached {
		t.Error("inner handler was not reached")
	}
}

// ---------------------------------------------------------------------------
// ValidateTimestamp — boundary values
// ---------------------------------------------------------------------------

// TestValidateTimestamp_Empty verifies that an empty string is rejected.
func TestValidateTimestamp_Empty(t *testing.T) {
	if err := ValidateTimestamp(""); err == nil {
		t.Error("expected error for empty timestamp, got nil")
	}
}

// TestValidateTimestamp_InvalidFormat verifies that non-RFC3339 input is rejected.
func TestValidateTimestamp_InvalidFormat(t *testing.T) {
	if err := ValidateTimestamp("not-a-timestamp"); err == nil {
		t.Error("expected error for invalid format, got nil")
	}
}

// TestValidateTimestamp_JustInsideTolerance verifies that a timestamp exactly
// at clockSkewTolerance - 1s is accepted.
func TestValidateTimestamp_JustInsideTolerance(t *testing.T) {
	ts := time.Now().UTC().Add(-(clockSkewTolerance - time.Second)).Format(time.RFC3339)
	if err := ValidateTimestamp(ts); err != nil {
		t.Errorf("expected nil for timestamp inside tolerance, got %v", err)
	}
}

// TestValidateTimestamp_JustOutsideTolerance verifies that a timestamp at
// clockSkewTolerance + 1s is rejected.
func TestValidateTimestamp_JustOutsideTolerance(t *testing.T) {
	ts := time.Now().UTC().Add(-(clockSkewTolerance + time.Second)).Format(time.RFC3339)
	if err := ValidateTimestamp(ts); err == nil {
		t.Error("expected error for timestamp outside tolerance, got nil")
	}
}

// TestValidateTimestamp_FutureJustInside verifies that a timestamp slightly in
// the future (within tolerance) is accepted.
func TestValidateTimestamp_FutureJustInside(t *testing.T) {
	ts := time.Now().UTC().Add(clockSkewTolerance - time.Second).Format(time.RFC3339)
	if err := ValidateTimestamp(ts); err != nil {
		t.Errorf("expected nil for near-future timestamp, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// BuildSignedString — content coverage
// ---------------------------------------------------------------------------

// TestBuildSignedString_Format verifies the exact format of the signed string.
func TestBuildSignedString_Format(t *testing.T) {
	got := BuildSignedString("POST", "/api/projects/foo/docs/bar.md", "2026-04-13T12:00:00Z", "abc123")
	want := "POST\n/api/projects/foo/docs/bar.md\n2026-04-13T12:00:00Z\nabc123"
	if got != want {
		t.Errorf("BuildSignedString = %q, want %q", got, want)
	}
}

// TestBuildSignedString_VaryMethod verifies that the method forms the first line.
func TestBuildSignedString_VaryMethod(t *testing.T) {
	for _, method := range []string{"GET", "PUT", "PATCH", "DELETE"} {
		s := BuildSignedString(method, "/p", "t", "h")
		if s[:len(method)] != method {
			t.Errorf("method %q not at start of signed string: %q", method, s)
		}
	}
}

// ---------------------------------------------------------------------------
// ComputeHMAC — determinism
// ---------------------------------------------------------------------------

// TestComputeHMAC_Deterministic verifies that the same inputs produce the same
// output on repeated calls.
func TestComputeHMAC_Deterministic(t *testing.T) {
	secret := "testsecret"
	signed := BuildSignedString("POST", "/path", "ts", "hash")
	a := ComputeHMAC(secret, signed)
	b := ComputeHMAC(secret, signed)
	if a != b {
		t.Errorf("non-deterministic HMAC: %q vs %q", a, b)
	}
}

// TestComputeHMAC_DifferentSecrets verifies that different secrets produce
// different MACs (key separation).
func TestComputeHMAC_DifferentSecrets(t *testing.T) {
	signed := BuildSignedString("POST", "/path", "ts", "hash")
	mac1 := ComputeHMAC("secret1", signed)
	mac2 := ComputeHMAC("secret2", signed)
	if mac1 == mac2 {
		t.Error("different secrets produced the same HMAC")
	}
}

// ---------------------------------------------------------------------------
// SecureEqual
// ---------------------------------------------------------------------------

// TestSecureEqual_EqualStrings verifies that identical strings compare equal.
func TestSecureEqual_EqualStrings(t *testing.T) {
	if !SecureEqual("abc", "abc") {
		t.Error("SecureEqual returned false for identical strings")
	}
}

// TestSecureEqual_DifferentStrings verifies that differing strings compare unequal.
func TestSecureEqual_DifferentStrings(t *testing.T) {
	if SecureEqual("abc", "xyz") {
		t.Error("SecureEqual returned true for different strings")
	}
}

// TestSecureEqual_DifferentLengths verifies that strings of different lengths
// compare unequal without panicking.
func TestSecureEqual_DifferentLengths(t *testing.T) {
	if SecureEqual("short", "a-much-longer-string") {
		t.Error("SecureEqual returned true for strings of different lengths")
	}
}

// ---------------------------------------------------------------------------
// Middleware — individual missing-header paths
// ---------------------------------------------------------------------------

// TestRequireAgent_MissingKeyIDHeader verifies that a request with no
// X-Vedox-Key-Id header is rejected with VDX-300.
func TestRequireAgent_MissingKeyIDHeader(t *testing.T) {
	ks := newTestStore(t)
	body := []byte(`{}`)
	req := httptest.NewRequest("POST", "/api/projects/foo/docs/bar.md", bytes.NewReader(body))
	// Deliberately omit all agent headers.

	rec, _ := runMiddleware(t, ks, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
	if code := decodeErr(t, rec.Body); code != "VDX-300" {
		t.Fatalf("want VDX-300, got %s", code)
	}
}

// TestRequireAgent_MissingTimestampHeader verifies that a request with a valid
// key ID but no X-Vedox-Timestamp header is rejected with VDX-300.
func TestRequireAgent_MissingTimestampHeader(t *testing.T) {
	ks := newTestStore(t)
	id, _, err := ks.IssueKey("agent", "", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}
	body := []byte(`{}`)
	req := httptest.NewRequest("POST", "/api/projects/foo/docs/bar.md", bytes.NewReader(body))
	req.Header.Set(HeaderKeyID, id)
	// No timestamp or signature headers.

	rec, _ := runMiddleware(t, ks, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
	if code := decodeErr(t, rec.Body); code != "VDX-300" {
		t.Fatalf("want VDX-300, got %s", code)
	}
}

// TestRequireAgent_MissingSignatureHeader verifies that an otherwise-valid
// request with no X-Vedox-Signature header is rejected.
func TestRequireAgent_MissingSignatureHeader(t *testing.T) {
	ks := newTestStore(t)
	id, _, err := ks.IssueKey("agent", "", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	body := []byte(`{}`)
	req := httptest.NewRequest("POST", "/api/projects/foo/docs/bar.md", bytes.NewReader(body))
	req.Header.Set(HeaderKeyID, id)
	req.Header.Set(HeaderTimestamp, ts)
	// Deliberately omit X-Vedox-Signature.

	rec, _ := runMiddleware(t, ks, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
	if code := decodeErr(t, rec.Body); code != "VDX-300" {
		t.Fatalf("want VDX-300, got %s", code)
	}
}

// TestRequireAgent_PathPrefixMismatch verifies that a key with a path-prefix
// scope rejects requests on non-matching paths with VDX-301.
func TestRequireAgent_PathPrefixMismatch(t *testing.T) {
	ks := newTestStore(t)
	// Key is scoped to /api/projects/foo/docs/reference/
	id, secret, err := ks.IssueKey("agent", "", "/api/projects/foo/docs/reference/")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}

	// Request targets a path outside the allowed prefix.
	path := "/api/projects/foo/docs/other/thing.md"
	body := []byte(`{}`)
	req := signedRequest(t, "POST", path, id, secret, body)

	rec, _ := runMiddleware(t, ks, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rec.Code)
	}
	if code := decodeErr(t, rec.Body); code != "VDX-301" {
		t.Fatalf("want VDX-301, got %s", code)
	}
}

// TestRequireAgent_PathPrefixMatch verifies that a key with a matching
// path-prefix scope allows the request through.
func TestRequireAgent_PathPrefixMatch(t *testing.T) {
	ks := newTestStore(t)
	prefix := "/api/projects/foo/docs/reference/"
	id, secret, err := ks.IssueKey("agent", "", prefix)
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}

	path := "/api/projects/foo/docs/reference/page.md"
	body := []byte(`{}`)
	req := signedRequest(t, "POST", path, id, secret, body)

	rec, _ := runMiddleware(t, ks, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestRequireAgent_OversizedBody verifies that a body exceeding maxAgentBodyBytes
// is rejected with 413 before any signature work is attempted.
func TestRequireAgent_OversizedBody(t *testing.T) {
	ks := newTestStore(t)
	id, secret, err := ks.IssueKey("agent", "", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}

	// Build a body 1 byte over the limit.
	bigBody := make([]byte, maxAgentBodyBytes+1)
	ts := time.Now().UTC().Format(time.RFC3339)
	sum := sha256.Sum256(bigBody)
	sig := ComputeHMAC(secret, BuildSignedString("POST", "/api/projects/foo/docs/bar.md", ts, hex.EncodeToString(sum[:])))

	req := httptest.NewRequest("POST", "/api/projects/foo/docs/bar.md", bytes.NewReader(bigBody))
	req.Header.Set(HeaderKeyID, id)
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderSignature, sig)

	rec, _ := runMiddleware(t, ks, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("want 413, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// TestRequireAgent_NilBody verifies that requests with a nil r.Body are
// handled gracefully — the body hash is computed over zero bytes.
func TestRequireAgent_NilBody(t *testing.T) {
	ks := newTestStore(t)
	id, secret, err := ks.IssueKey("agent", "", "")
	if err != nil {
		t.Fatalf("IssueKey: %v", err)
	}

	ts := time.Now().UTC().Format(time.RFC3339)
	// The empty body hash.
	sum := sha256.Sum256([]byte{})
	sig := ComputeHMAC(secret, BuildSignedString("GET", "/api/projects/foo/docs/bar.md", ts, hex.EncodeToString(sum[:])))

	req := httptest.NewRequest("GET", "/api/projects/foo/docs/bar.md", nil)
	req.Header.Set(HeaderKeyID, id)
	req.Header.Set(HeaderTimestamp, ts)
	req.Header.Set(HeaderSignature, sig)
	// Set to actual nil (httptest.NewRequest sets to http.NoBody otherwise).
	req.Body = nil

	rec, _ := runMiddleware(t, ks, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 for nil-body request, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// LoadKeyStore — corrupt JSON and generic read error
// ---------------------------------------------------------------------------

// TestLoadKeyStore_CorruptJSON verifies that a syntactically invalid
// agent-keys.json returns an error rather than silently discarding keys.
func TestLoadKeyStore_CorruptJSON(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	vedoxDir := dir + "/.vedox"
	if err := os.MkdirAll(vedoxDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(vedoxDir+"/agent-keys.json", []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := LoadKeyStore(dir)
	if err == nil {
		t.Error("expected error for corrupt agent-keys.json, got nil")
	}
}

// TestLoadKeyStore_UnreadableFile verifies that a non-IsNotExist read error
// (e.g. a directory where a file is expected) propagates as an error.
func TestLoadKeyStore_UnreadableFile(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	vedoxDir := dir + "/.vedox"
	if err := os.MkdirAll(vedoxDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Create a directory with the same name as the metadata file so
	// os.ReadFile returns a "is a directory" error (not IsNotExist).
	if err := os.MkdirAll(vedoxDir+"/agent-keys.json", 0o700); err != nil {
		t.Fatalf("mkdir as file: %v", err)
	}
	_, err := LoadKeyStore(dir)
	if err == nil {
		t.Error("expected error when metadata path is a directory, got nil")
	}
}

// ---------------------------------------------------------------------------
// asVDX — middleware helper
// ---------------------------------------------------------------------------

// TestAsVDX_PlainError verifies asVDX returns false when given a non-VedoxError.
func TestAsVDX_PlainError(t *testing.T) {
	var result *vdxerr.VedoxError
	if asVDX(fmt.Errorf("not a VedoxError"), &result) {
		t.Error("expected asVDX to return false for a plain error")
	}
	if result != nil {
		t.Errorf("expected result to remain nil, got %+v", result)
	}
}

// TestAsVDX_NilError verifies asVDX handles a nil error without panicking.
func TestAsVDX_NilError(t *testing.T) {
	var result *vdxerr.VedoxError
	if asVDX(nil, &result) {
		t.Error("expected asVDX to return false for a nil error")
	}
}

// TestAsVDX_WrappedVedoxError verifies asVDX unwraps through fmt.Errorf
// wrappers to find a *VedoxError at any depth in the chain.
func TestAsVDX_WrappedVedoxError(t *testing.T) {
	inner := vdxerr.Wrap(vdxerr.ErrKeychainUnavailable, "keychain boom", fmt.Errorf("underlying"))
	wrapped := fmt.Errorf("context: %w", fmt.Errorf("deeper: %w", inner))

	var result *vdxerr.VedoxError
	if !asVDX(wrapped, &result) {
		t.Fatal("expected asVDX to unwrap to *VedoxError")
	}
	if result == nil {
		t.Fatal("expected non-nil result pointer")
	}
	if result.Code != vdxerr.ErrKeychainUnavailable {
		t.Errorf("unwrapped wrong error: got code %q, want %q", result.Code, vdxerr.ErrKeychainUnavailable)
	}
}
