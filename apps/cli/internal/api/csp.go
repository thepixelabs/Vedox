// Package api — CSP nonce helpers (E9 v2.1 path).
//
// This file implements per-request CSP nonces that replace the static
// 'unsafe-inline' directive previously required by SvelteKit's inline style
// emission.  The approach:
//
//  1. generateNonce generates 16 cryptographically random bytes and returns
//     them base64url-encoded (no padding) — RFC 2397 §2.1 compatible.
//
//  2. injectNonce rewrites the raw HTML bytes of the SvelteKit index.html,
//     adding a nonce="<value>" attribute to every opening <style> tag (both
//     bare <style> and <style ...> with existing attributes).  All other HTML
//     is copied verbatim so malformed documents are never silently discarded.
//
//  3. cspHeader builds the final CSP header value: when a nonce is present
//     'unsafe-inline' is replaced by 'nonce-<value>'; when the nonce is empty
//     the legacy 'unsafe-inline' fallback is retained so the editor is never
//     broken by an RNG failure.
//
// Inline style="" attributes (e.g. Shiki token colouring) are NOT rewritten
// by injectNonce — those are covered by a separate
// 'style-src-attr 'unsafe-inline'' directive that is narrower in scope than
// the old blanket 'unsafe-inline'.  Browsers that support nonces will enforce
// the nonce on <style> blocks while still permitting style="" attributes under
// the attr-level override.  This is the maximum achievable restriction without
// a Shiki nonce-emission patch.

package api

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// generateNonce returns a fresh 16-byte cryptographically random nonce
// encoded as URL-safe base64 without padding (22 characters).
// It returns an error only when crypto/rand fails, which is a fatal system
// condition; callers must treat an error as a signal to fall back to the
// 'unsafe-inline' CSP rather than abort the request.
func generateNonce() ([]byte, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("csp: nonce generation failed: %w", err)
	}
	enc := make([]byte, base64.RawURLEncoding.EncodedLen(len(raw)))
	base64.RawURLEncoding.Encode(enc, raw)
	return enc, nil
}

// injectNonce rewrites html, adding nonce="<nonce>" to every opening <style>
// tag.  It handles both <style> (bare) and <style ...> (with existing attrs).
// If nonce is empty the original html is returned unchanged.
//
// The rewrite is byte-level and allocation-efficient: it performs a single
// forward scan, copying unchanged spans as slices and only allocating for
// the inserted attribute bytes.
func injectNonce(html, nonce []byte) []byte {
	if len(nonce) == 0 {
		return html
	}

	// attr is the text we splice after "<style" in every match.
	attr := append([]byte(` nonce="`), append(nonce, '"')...)

	var out bytes.Buffer
	out.Grow(len(html) + 32) // pre-allocate; 32 is a conservative overestimate

	remaining := html
	for {
		// Find the next "<style" (case-sensitive — SvelteKit emits lowercase).
		idx := bytes.Index(remaining, []byte("<style"))
		if idx == -1 {
			out.Write(remaining)
			break
		}

		// Copy everything up to and including "<style".
		out.Write(remaining[:idx+len("<style")])
		remaining = remaining[idx+len("<style"):]

		// Emit the nonce attribute before whatever follows the tag name
		// (a space, '>', or '/').  This is correct for:
		//   <style>         → <style nonce="...">
		//   <style type=…>  → <style nonce="..." type=…>
		//   <style>         (no extra attrs)
		out.Write(attr)
		// remaining now starts just after "<style"; write it as-is.
	}

	return out.Bytes()
}

// cspWithNonce returns the Content-Security-Policy header value for a given
// per-request nonce.  When nonce is non-empty 'unsafe-inline' in style-src is
// replaced by 'nonce-<value>' (more restrictive); when nonce is empty the
// legacy CSPHeaderValue with 'unsafe-inline' is returned as a safe fallback.
//
// A 'style-src-attr' override is added to keep Shiki's inline style=""
// attributes working even after 'unsafe-inline' is removed from style-src.
func cspWithNonce(nonce []byte) string {
	if len(nonce) == 0 {
		return CSPHeaderValue
	}
	return fmt.Sprintf(
		"default-src 'self'; script-src 'self'; style-src 'self' 'nonce-%s'; style-src-attr 'unsafe-inline'; img-src 'self' data:; object-src 'none'; frame-ancestors 'none'",
		nonce,
	)
}
