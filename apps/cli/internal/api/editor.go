package api

// editor.go — serves the embedded SvelteKit editor with per-request CSP nonces.
//
// Route ownership:
//   - All requests NOT under /api/ or /healthz are served from EditorFS when it
//     is non-nil (release builds).
//   - Non-HTML assets (JS, CSS, fonts, …) are served verbatim by http.FileServer.
//   - The root "/" (and any unknown path) falls back to index.html so SvelteKit's
//     client-side router handles the URL — this is the standard SPA "serve-all"
//     pattern.
//   - index.html is loaded, nonce-injected, and written per request so that the
//     CSP nonce in the header and the nonce in <style> tags always match.
//
// Defensive rules:
//   - If EditorFS is nil (non-release build / dev mode), MountEditor is a no-op.
//   - If nonce generation fails (broken RNG) the page is served with 'unsafe-inline'
//     so the editor still works — security degrades gracefully rather than the
//     operator seeing a blank page.
//   - If index.html cannot be read from the FS the response is a plain-text 503.

import (
	"io/fs"
	"log/slog"
	"net/http"
)

// MountEditor registers the static-asset + index.html handler on mux.
// editorFS must be the sub-filesystem returned by webassets.GetEditorFS();
// when it is nil (non-release build) this function is a no-op.
//
// Every request that resolves to index.html gets:
//
//  1. A fresh 16-byte nonce injected into the Content-Security-Policy header.
//  2. The same nonce spliced into every <style> tag in the HTML body.
//
// Other assets (JS bundles, CSS files, fonts) bypass nonce injection and are
// served verbatim by http.FileServer, which handles ETags, Range requests, and
// MIME types automatically.
func MountEditor(mux *http.ServeMux, editorFS fs.FS) {
	if editorFS == nil {
		return
	}

	// Read index.html once at mount time — it never changes between requests.
	// We keep the raw bytes and inject a fresh nonce on every request.
	rawIndex, err := fs.ReadFile(editorFS, "index.html")
	if err != nil {
		slog.Warn("webassets: index.html not found in EditorFS; editor will return 503",
			"error", err)
		rawIndex = nil
	}

	// fileServer handles everything that is NOT index.html — JS, CSS, fonts, etc.
	// It will 404 for paths that do not exist in the FS; the catch-all handler
	// below redirects those to index.html.
	fileServer := http.FileServer(http.FS(editorFS))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Static assets: if the path maps to an existing non-directory file in
		// the FS (e.g. /_app/immutable/entry/…), serve it directly.
		if r.URL.Path != "/" {
			if f, ferr := editorFS.Open(r.URL.Path[1:]); ferr == nil {
				info, serr := f.Stat()
				_ = f.Close()
				if serr == nil && !info.IsDir() {
					fileServer.ServeHTTP(w, r)
					return
				}
			}
		}

		// Everything else (root, unknown paths, SvelteKit client-side routes)
		// gets index.html with a fresh nonce.
		serveIndexWithNonce(w, rawIndex)
	})
}

// serveIndexWithNonce generates a per-request nonce, injects it into rawHTML,
// sets the matching CSP header, and writes the HTML to w.  If rawHTML is nil
// (missing index.html) it returns 503.  If nonce generation fails it falls back
// to 'unsafe-inline' but still serves the page.
func serveIndexWithNonce(w http.ResponseWriter, rawHTML []byte) {
	if rawHTML == nil {
		http.Error(w, "editor not available (index.html missing from embedded assets)", http.StatusServiceUnavailable)
		return
	}

	nonce, err := generateNonce()
	if err != nil {
		// RNG failure — serve with unsafe-inline fallback so the editor is not
		// silently broken.  This path should never be reached in practice.
		slog.Error("csp: nonce generation failed; falling back to unsafe-inline", "error", err)
		nonce = nil
	}

	w.Header().Set("Content-Security-Policy", cspWithNonce(nonce))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	injected := injectNonce(rawHTML, nonce)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(injected)
}
