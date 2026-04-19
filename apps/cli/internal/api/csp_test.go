package api

// Tests for the CSP nonce helpers in csp.go and the editor HTML serve path
// in editor.go.
//
// Coverage:
//
//  1. TestGenerateNonce_Length         — nonce is 22 chars (16 raw bytes base64url-nopad)
//  2. TestGenerateNonce_Uniqueness     — two calls never return the same value
//  3. TestInjectNonce_BareStyleTag     — <style> → <style nonce="…">
//  4. TestInjectNonce_StyleWithAttrs   — <style type=…> → <style nonce="…" type=…>
//  5. TestInjectNonce_MultipleStyles   — all <style> tags get the same nonce
//  6. TestInjectNonce_NoStyle          — HTML without <style> is returned unchanged
//  7. TestInjectNonce_EmptyNonce       — empty nonce returns original html
//  8. TestInjectNonce_MalformedHTML    — truncated / weird bytes survive without panic
//  9. TestCSPWithNonce_NoncePresent    — CSP header contains nonce-<value>; no unsafe-inline in style-src
// 10. TestCSPWithNonce_EmptyNonce      — CSP header falls back to static CSPHeaderValue
// 11. TestServeIndexWithNonce_Match    — CSP header nonce matches nonce in body
// 12. TestServeIndexWithNonce_Unique   — two requests produce different nonces
// 13. TestServeIndexWithNonce_NilHTML  — nil rawHTML yields 503
// 14. TestServeIndexWithNonce_RaceOK   — concurrent requests don't share state

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// ── generateNonce ─────────────────────────────────────────────────────────────

func TestGenerateNonce_Length(t *testing.T) {
	t.Parallel()
	n, err := generateNonce()
	if err != nil {
		t.Fatalf("generateNonce: unexpected error: %v", err)
	}
	// base64.RawURLEncoding of 16 bytes = ceil(16*8/6) = 22 characters.
	if len(n) != 22 {
		t.Errorf("nonce length = %d, want 22; value=%q", len(n), n)
	}
}

func TestGenerateNonce_Uniqueness(t *testing.T) {
	t.Parallel()
	const iters = 100
	seen := make(map[string]bool, iters)
	for i := range iters {
		n, err := generateNonce()
		if err != nil {
			t.Fatalf("generateNonce[%d]: %v", i, err)
		}
		s := string(n)
		if seen[s] {
			t.Fatalf("collision detected at iteration %d: nonce=%q", i, s)
		}
		seen[s] = true
	}
}

// ── injectNonce ───────────────────────────────────────────────────────────────

func TestInjectNonce_BareStyleTag(t *testing.T) {
	t.Parallel()
	nonce := []byte("abc123XYZ")
	html := []byte(`<html><head><style>.a{color:red}</style></head></html>`)
	got := injectNonce(html, nonce)

	want := fmt.Sprintf(`<style nonce="%s"`, nonce)
	if !bytes.Contains(got, []byte(want)) {
		t.Errorf("injectNonce did not add nonce to bare <style>:\ngot:  %s\nwant substring: %s", got, want)
	}
}

func TestInjectNonce_StyleWithAttrs(t *testing.T) {
	t.Parallel()
	nonce := []byte("zz99")
	html := []byte(`<html><head><style type="text/css">.b{}</style></head></html>`)
	got := injectNonce(html, nonce)

	// nonce must appear right after "<style "
	wantPrefix := fmt.Sprintf(`<style nonce="%s"`, nonce)
	if !bytes.Contains(got, []byte(wantPrefix)) {
		t.Errorf("injectNonce missing nonce in <style type=…>: got %s", got)
	}
	// original type attribute must still be present
	if !bytes.Contains(got, []byte(`type="text/css"`)) {
		t.Errorf("injectNonce dropped original type attribute: %s", got)
	}
}

func TestInjectNonce_MultipleStyles(t *testing.T) {
	t.Parallel()
	nonce := []byte("nn11")
	html := []byte(`<head><style>.a{}</style><style>.b{}</style><style>.c{}</style></head>`)
	got := injectNonce(html, nonce)

	want := fmt.Sprintf(`nonce="%s"`, nonce)
	count := bytes.Count(got, []byte(want))
	if count != 3 {
		t.Errorf("expected 3 nonce injections, got %d in: %s", count, got)
	}
}

func TestInjectNonce_NoStyle(t *testing.T) {
	t.Parallel()
	nonce := []byte("ww77")
	html := []byte(`<html><head><title>hi</title></head><body><p>hello</p></body></html>`)
	got := injectNonce(html, nonce)
	if !bytes.Equal(got, html) {
		t.Errorf("expected unchanged html when no <style> present;\ngot: %s", got)
	}
}

func TestInjectNonce_EmptyNonce(t *testing.T) {
	t.Parallel()
	html := []byte(`<html><head><style>.x{}</style></head></html>`)
	got := injectNonce(html, nil)
	if !bytes.Equal(got, html) {
		t.Errorf("empty nonce must return original html; got: %s", got)
	}
	got2 := injectNonce(html, []byte{})
	if !bytes.Equal(got2, html) {
		t.Errorf("zero-length nonce must return original html; got: %s", got2)
	}
}

func TestInjectNonce_MalformedHTML(t *testing.T) {
	t.Parallel()
	nonce := []byte("safe99")
	cases := [][]byte{
		{},
		[]byte("<"),
		[]byte("<style"),
		[]byte("<style nonce"),
		[]byte("<<style>>"),
		[]byte("<style>\x00\xFF</style>"),
		[]byte("<STYLE>.a{}</STYLE>"), // uppercase — not rewritten (by design; SvelteKit emits lowercase)
	}
	for _, html := range cases {
		// Must not panic.
		_ = injectNonce(html, nonce)
	}
}

// ── cspWithNonce ──────────────────────────────────────────────────────────────

func TestCSPWithNonce_NoncePresent(t *testing.T) {
	t.Parallel()
	nonce := []byte("testNonce42")
	csp := cspWithNonce(nonce)

	if !strings.Contains(csp, "'nonce-testNonce42'") {
		t.Errorf("CSP missing nonce directive; got: %s", csp)
	}
	// style-src must NOT contain the blanket 'unsafe-inline' (only the nonce).
	// We check the style-src segment specifically.
	styleSrc := extractDirective(csp, "style-src")
	if strings.Contains(styleSrc, "'unsafe-inline'") {
		t.Errorf("style-src still contains 'unsafe-inline' when nonce is present; style-src=%q", styleSrc)
	}
}

func TestCSPWithNonce_EmptyNonce(t *testing.T) {
	t.Parallel()
	csp := cspWithNonce(nil)
	if csp != CSPHeaderValue {
		t.Errorf("empty nonce must return static CSPHeaderValue;\ngot:  %q\nwant: %q", csp, CSPHeaderValue)
	}
	csp2 := cspWithNonce([]byte{})
	if csp2 != CSPHeaderValue {
		t.Errorf("zero-length nonce must return static CSPHeaderValue;\ngot:  %q\nwant: %q", csp2, CSPHeaderValue)
	}
}

// extractDirective returns the value of a single CSP directive (everything
// between the directive name and the next ';').
func extractDirective(csp, name string) string {
	for _, part := range strings.Split(csp, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, name) {
			return strings.TrimPrefix(part, name)
		}
	}
	return ""
}

// ── serveIndexWithNonce ───────────────────────────────────────────────────────

// sampleHTML is a minimal SvelteKit-like index.html used by serve tests.
const sampleHTML = `<!DOCTYPE html>
<html>
<head>
<style>.root{margin:0}</style>
<style type="text/css">.app{display:flex}</style>
</head>
<body><div id="svelte"></div></body>
</html>`

func TestServeIndexWithNonce_Match(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	serveIndexWithNonce(w, []byte(sampleHTML))

	resp := w.Result()
	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header missing")
	}

	// Extract the nonce value from the CSP header.
	nonceVal := extractNonceFromCSP(t, csp)

	// The same nonce must appear in the HTML body.
	body := w.Body.String()
	wantAttr := fmt.Sprintf(`nonce="%s"`, nonceVal)
	if !strings.Contains(body, wantAttr) {
		t.Errorf("CSP nonce %q not found in HTML body:\n%s", nonceVal, body)
	}
}

func TestServeIndexWithNonce_Unique(t *testing.T) {
	t.Parallel()
	html := []byte(sampleHTML)

	w1 := httptest.NewRecorder()
	serveIndexWithNonce(w1, html)
	nonce1 := extractNonceFromCSP(t, w1.Result().Header.Get("Content-Security-Policy"))

	w2 := httptest.NewRecorder()
	serveIndexWithNonce(w2, html)
	nonce2 := extractNonceFromCSP(t, w2.Result().Header.Get("Content-Security-Policy"))

	if nonce1 == nonce2 {
		t.Errorf("two requests produced the same nonce: %q", nonce1)
	}
}

func TestServeIndexWithNonce_NilHTML(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	serveIndexWithNonce(w, nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("nil html: status = %d, want 503", w.Code)
	}
}

func TestServeIndexWithNonce_RaceOK(t *testing.T) {
	// Run many concurrent requests to detect shared-state data races.
	// The -race detector will catch any unsynchronised access.
	html := []byte(sampleHTML)
	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			serveIndexWithNonce(w, html)
			csp := w.Result().Header.Get("Content-Security-Policy")
			if csp == "" {
				// Can't call t.Fatal from a goroutine; use t.Error instead.
				t.Error("goroutine: CSP header missing")
			}
		}()
	}
	wg.Wait()
}

// extractNonceFromCSP parses 'nonce-<value>' from a CSP string and returns
// the nonce value.  Fails the test if no nonce is found.
func extractNonceFromCSP(t *testing.T, csp string) string {
	t.Helper()
	for _, token := range strings.Fields(csp) {
		token = strings.Trim(token, "';,")
		if strings.HasPrefix(token, "nonce-") {
			return strings.TrimPrefix(token, "nonce-")
		}
	}
	t.Fatalf("no nonce-<value> found in CSP header: %q", csp)
	return ""
}
