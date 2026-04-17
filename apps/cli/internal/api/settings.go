package api

// handlers for user preferences endpoints:
//
//	GET /api/settings — read ~/.vedox/user-prefs.json (returns defaults when absent)
//	PUT /api/settings — PATCH-merge the incoming body into stored prefs and write
//	                    atomically at 0600 (R3 semantics: caller sends only the
//	                    categories it wants to change; unknown keys are preserved)
//
// The file is always located at <userHome>/.vedox/user-prefs.json.
// In tests, homeDirOverride on Server redirects the path to a temp directory.

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
)

const (
	// vedoxDirName is the dot-directory that holds all Vedox user state.
	vedoxDirName = ".vedox"
	// userPrefsFile is the filename within vedoxDirName for user preferences.
	userPrefsFile = "user-prefs.json"
	// userPrefsMode is the file permission for user-prefs.json.
	// 0600 = owner read+write only; no group or world access.
	userPrefsMode = 0o600
)

// userPrefsPath returns the absolute path to ~/.vedox/user-prefs.json,
// using s.userHome() so that tests can redirect it to a temp directory.
func (s *Server) userPrefsPath() (string, error) {
	home, err := s.userHome()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, vedoxDirName, userPrefsFile), nil
}

// handleGetSettings implements GET /api/settings.
//
// It reads ~/.vedox/user-prefs.json and returns its contents as JSON. When the
// file does not exist an empty JSON object {} is returned (200, not 404) so the
// frontend always gets a parseable response and can merge it with its own
// defaults. Any other read error returns 500.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	path, err := s.userPrefsPath()
	if err != nil {
		slog.Error("settings/get: resolve prefs path", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			"could not resolve user preferences path")
		return
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		// First run — no file yet. Return an empty object so the frontend can
		// merge it with its compiled-in defaults without any special-casing.
		writeJSON(w, http.StatusOK, map[string]interface{}{})
		return
	}
	if err != nil {
		slog.Error("settings/get: read prefs file", "path", path, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			"could not read user preferences")
		return
	}

	// Validate that the file is well-formed JSON before echoing it back.
	// We use json.RawMessage so we don't re-encode and potentially reorder keys.
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		slog.Error("settings/get: prefs file is malformed JSON",
			"path", path, "error", err.Error())
		// Treat a corrupt file like a missing one — return empty defaults.
		writeJSON(w, http.StatusOK, map[string]interface{}{})
		return
	}

	writeJSON(w, http.StatusOK, raw)
}

// handlePutSettings implements PUT /api/settings (PATCH semantics, per R3).
//
// The request body must be a JSON object. Each top-level key in the body is
// merged shallowly into the stored prefs: keys present in the body overwrite
// the stored value for that key; keys absent from the body are preserved. This
// mirrors how the frontend updatePrefs('editor', { spellCheck: true }) call
// only touches the 'editor' category.
//
// Write is atomic: we write to a temp file and rename, ensuring readers never
// see a partially-written file. The final file is 0600.
func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	path, err := s.userPrefsPath()
	if err != nil {
		slog.Error("settings/put: resolve prefs path", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			"could not resolve user preferences path")
		return
	}

	// Reject oversized bodies before any allocation. 256 KB is generous for a
	// user-preferences JSON blob (typical size is <2 KB). This prevents a
	// DoS-via-huge-payload that would otherwise fill daemon memory with the
	// map[string]json.RawMessage and the merged re-marshal buffer.
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)

	// Decode incoming body into a generic map so we preserve unknown future keys.
	// DisallowUnknownFields is deliberately NOT set — the schema is intentionally
	// open for forward-compatible preferences. Instead we bound depth by virtue
	// of storing values as json.RawMessage (no recursive decode).
	var incoming map[string]json.RawMessage
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&incoming); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-400", "invalid JSON body")
		return
	}
	// Reject trailing JSON (smuggling a second document).
	if dec.More() {
		writeError(w, http.StatusBadRequest, "VDX-400", "request body must be a single JSON object")
		return
	}
	// Validate each top-level value is well-formed JSON. json.RawMessage skips
	// validation; we re-parse each value and cap nesting depth to prevent a
	// "JSON bomb" (deeply nested arrays/objects) from exhausting stack memory
	// when the frontend later consumes the stored file.
	const maxDepth = 32
	for k, v := range incoming {
		if !utf8ValidString(k) {
			writeError(w, http.StatusBadRequest, "VDX-400", "settings key must be valid UTF-8")
			return
		}
		if len(k) > 256 {
			writeError(w, http.StatusBadRequest, "VDX-400", "settings key too long")
			return
		}
		if err := validateJSONDepth(v, maxDepth); err != nil {
			writeError(w, http.StatusBadRequest, "VDX-400",
				"settings value rejected: "+err.Error())
			return
		}
	}

	// Load existing prefs (may not exist yet).
	existing := make(map[string]json.RawMessage)
	existingData, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		slog.Error("settings/put: read existing prefs", "path", path, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			"could not read existing user preferences")
		return
	}
	if err == nil {
		// Best-effort parse — if the file is corrupt we start fresh.
		if jsonErr := json.Unmarshal(existingData, &existing); jsonErr != nil {
			slog.Warn("settings/put: existing prefs file is malformed; starting fresh",
				"path", path, "error", jsonErr.Error())
			existing = make(map[string]json.RawMessage)
		}
	}

	// PATCH-merge: incoming values overwrite matching keys; other keys survive.
	for k, v := range incoming {
		existing[k] = v
	}

	// Serialise merged result.
	merged, err := json.Marshal(existing)
	if err != nil {
		slog.Error("settings/put: marshal merged prefs", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			"could not serialise user preferences")
		return
	}

	// Ensure the directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		slog.Error("settings/put: mkdir vedox dir", "dir", dir, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			"could not create preferences directory")
		return
	}

	// Atomic write: write to a temp file in the same directory, then rename.
	// Same-directory temp ensures the rename is always on the same filesystem,
	// making it a rename(2) rather than a cross-device copy.
	tmp, err := os.CreateTemp(dir, "user-prefs-*.json.tmp")
	if err != nil {
		slog.Error("settings/put: create temp file", "dir", dir, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			"could not write user preferences")
		return
	}
	tmpName := tmp.Name()

	// Clean up the temp file on any error path. After a successful rename the
	// file is gone, so Remove on a missing path is a no-op.
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpName)
		}
	}()

	// Set 0600 on the open fd (fchmod) before writing — avoids the TOCTOU race
	// that a post-close chmod on a named path would introduce.
	if err := tmp.Chmod(userPrefsMode); err != nil {
		_ = tmp.Close()
		slog.Error("settings/put: chmod temp file", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			"could not set permissions on preferences file")
		return
	}

	if _, err := tmp.Write(merged); err != nil {
		_ = tmp.Close()
		slog.Error("settings/put: write temp file", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			"could not write user preferences")
		return
	}
	if err := tmp.Close(); err != nil {
		slog.Error("settings/put: close temp file", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			"could not flush user preferences")
		return
	}

	// Atomic rename.
	if err := os.Rename(tmpName, path); err != nil {
		slog.Error("settings/put: rename temp to final", "tmp", tmpName, "dst", path, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-500",
			"could not save user preferences")
		return
	}
	success = true

	// Return the merged result so the caller can confirm exactly what was stored.
	var raw json.RawMessage = merged
	writeJSON(w, http.StatusOK, raw)
}
