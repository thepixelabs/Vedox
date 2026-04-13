package api

// Provider config — Claude Code (.claude/CLAUDE.md, .claude/settings.json).
//
// All write paths go through atomicWrite, which calls assertNoSymlinkAncestor
// before any I/O. This prevents a malicious ".claude" symlink inside a cloned
// project from redirecting Vedox writes to an arbitrary file (e.g. ~/.ssh/id_rsa).
//
// Etag is always recomputed from the current file bytes on disk — clients never
// supply the authoritative etag. Missing files return empty content and an
// empty etag; the conflict check treats that as a valid "initial write" base.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

const maxProviderBody = 1 << 20 // 1 MiB — matches the doc write ceiling.

// ── Shared helpers ────────────────────────────────────────────────────────────

// computeEtag returns a hex-encoded sha256 of data. Empty input produces an
// empty string so callers can distinguish "file does not exist" from "file is
// empty" when communicating conflict-check baselines to the frontend.
func computeEtag(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// readFileOrEmpty reads absPath. A missing file returns (nil, "", nil) so
// handlers can uniformly render "not yet created" as empty content + empty etag.
func readFileOrEmpty(absPath string) ([]byte, string, error) {
	data, err := os.ReadFile(absPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", nil
		}
		return nil, "", err
	}
	return data, computeEtag(data), nil
}

// assertNoSymlinkAncestor walks every path component from boundary down to
// target and returns VDX-005 if any directory in the chain — or the target
// itself — is a symlink. This is the core defense against a malicious
// ".claude" entry in a cloned repo redirecting writes to ~/.ssh or similar.
//
// Both inputs must be absolute. If target does not start with boundary this
// returns VDX-005 immediately.
func assertNoSymlinkAncestor(boundary, target string) error {
	boundary = filepath.Clean(boundary)
	target = filepath.Clean(target)

	if boundary != target {
		rel, err := filepath.Rel(boundary, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			slog.Warn("provider: target escapes boundary",
				"code", "VDX-005", "boundary", boundary, "target", target)
			return fmt.Errorf("target path escapes boundary")
		}
	}

	// Walk each intermediate component. We Lstat everything from boundary to
	// target inclusive — a symlink AT the target is just as dangerous as a
	// symlink in an ancestor directory.
	rel, err := filepath.Rel(boundary, target)
	if err != nil {
		return err
	}

	// Check the boundary itself first.
	if err := lstatAndReject(boundary); err != nil {
		return err
	}
	if rel == "." {
		return nil
	}

	parts := strings.Split(rel, string(os.PathSeparator))
	cur := boundary
	for _, part := range parts {
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Component doesn't exist yet — that's fine for writes that
				// will mkdir it. The rename target at the end is created by
				// atomicWrite itself; symlink-squatting there is impossible
				// because we create it via O_EXCL-equivalent rename semantics.
				return nil
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			slog.Warn("provider: symlink in path rejected",
				"code", "VDX-005", "component", cur)
			return fmt.Errorf("symlink ancestor rejected")
		}
	}
	return nil
}

func lstatAndReject(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		slog.Warn("provider: symlink boundary rejected",
			"code", "VDX-005", "path", path)
		return fmt.Errorf("symlink boundary rejected")
	}
	return nil
}

// atomicWrite writes data to absPath atomically via temp file + fsync + rename.
// It always calls assertNoSymlinkAncestor(boundary, absPath) before any I/O.
// Directories are created with dirMode; the file lands with fileMode.
func atomicWrite(boundary, absPath string, data []byte, dirMode, fileMode os.FileMode) error {
	if err := assertNoSymlinkAncestor(boundary, absPath); err != nil {
		return err
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}
	// Re-check after MkdirAll — a racing actor could have introduced a symlink.
	if err := assertNoSymlinkAncestor(boundary, absPath); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".vedox-provider-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmpPath, fileMode); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, absPath); err != nil {
		cleanup()
		return err
	}
	return nil
}

// looksLikeHujson is a cheap heuristic: true when data contains any `//` line
// comment or `/* */` block comment outside of obvious string contexts. We do
// not try to be precise — a false positive only triggers a slog warning.
func looksLikeHujson(data []byte) bool {
	// Strip strings naively: any occurrence of `//` or `/*` anywhere flags it.
	// This over-reports when a URL appears inside a JSON string value, but
	// the cost is a single WARN log, never data loss.
	if bytes.Contains(data, []byte("//")) {
		return true
	}
	if bytes.Contains(data, []byte("/*")) {
		return true
	}
	return false
}

// ── Claude paths & request shapes ─────────────────────────────────────────────

func claudeDir(projectDir string) string {
	return filepath.Join(projectDir, ".claude")
}
func claudeMemoryPath(projectDir string) string {
	return filepath.Join(claudeDir(projectDir), "CLAUDE.md")
}
func claudeSettingsPath(projectDir string) string {
	return filepath.Join(claudeDir(projectDir), "settings.json")
}

type claudeGetResponse struct {
	Memory      claudeMemoryBlock      `json:"memory"`
	Permissions claudePermissionsBlock `json:"permissions"`
	Scope       string                 `json:"scope"`
}
type claudeMemoryBlock struct {
	Content string `json:"content"`
	Etag    string `json:"etag"`
}
type claudePermissionsBlock struct {
	Raw  map[string]any `json:"raw"`
	Etag string         `json:"etag"`
}

type putMemoryRequest struct {
	Content string `json:"content"`
	Etag    string `json:"etag"`
}
type putPermissionsRequest struct {
	Permissions map[string]any `json:"permissions"`
	Etag        string         `json:"etag"`
}

type etagOnlyResponse struct {
	Etag string `json:"etag"`
}
type conflictResponse struct {
	Error       string `json:"error"`
	CurrentEtag string `json:"currentEtag"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (s *Server) handleGetClaudeConfig(w http.ResponseWriter, r *http.Request) {
	projectDir, ok := s.providerProjectDir(w, r)
	if !ok {
		return
	}

	memBytes, memEtag, err := readFileOrEmpty(claudeMemoryPath(projectDir))
	if err != nil {
		slog.Error("provider: read CLAUDE.md", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	permsBytes, permsEtag, err := readFileOrEmpty(claudeSettingsPath(projectDir))
	if err != nil {
		slog.Error("provider: read settings.json", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}

	permsRaw := map[string]any{}
	if len(permsBytes) > 0 {
		if err := json.Unmarshal(permsBytes, &permsRaw); err != nil {
			// Non-fatal: the file exists but isn't parseable JSON (likely
			// hujson). Return the raw bytes as a single "_raw" field so the
			// frontend can show something without crashing.
			slog.Warn("provider: settings.json is not standard JSON",
				"error", err.Error())
			permsRaw = map[string]any{"_raw": string(permsBytes)}
		}
	}

	writeJSON(w, http.StatusOK, claudeGetResponse{
		Memory:      claudeMemoryBlock{Content: string(memBytes), Etag: memEtag},
		Permissions: claudePermissionsBlock{Raw: permsRaw, Etag: permsEtag},
		Scope:       "project",
	})
}

func (s *Server) handlePutClaudeMemory(w http.ResponseWriter, r *http.Request) {
	projectDir, ok := s.providerProjectDir(w, r)
	if !ok {
		return
	}

	req, ok := decodeProviderBody[putMemoryRequest](w, r)
	if !ok {
		return
	}

	absPath := claudeMemoryPath(projectDir)
	current, currentEtag, err := readFileOrEmpty(absPath)
	if err != nil {
		slog.Error("provider: read CLAUDE.md", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	_ = current
	if currentEtag != req.Etag {
		writeJSON(w, http.StatusConflict, conflictResponse{
			Error: "conflict", CurrentEtag: currentEtag,
		})
		return
	}

	data := []byte(req.Content)
	if err := atomicWrite(projectDir, absPath, data, 0o755, 0o644); err != nil {
		s.writeProviderWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, etagOnlyResponse{Etag: computeEtag(data)})
}

func (s *Server) handlePutClaudePermissions(w http.ResponseWriter, r *http.Request) {
	projectDir, ok := s.providerProjectDir(w, r)
	if !ok {
		return
	}

	req, ok := decodeProviderBody[putPermissionsRequest](w, r)
	if !ok {
		return
	}

	absPath := claudeSettingsPath(projectDir)
	current, currentEtag, err := readFileOrEmpty(absPath)
	if err != nil {
		slog.Error("provider: read settings.json", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	if currentEtag != req.Etag {
		writeJSON(w, http.StatusConflict, conflictResponse{
			Error: "conflict", CurrentEtag: currentEtag,
		})
		return
	}

	// Round-trip: load existing JSON (if any) and only mutate "permissions".
	// Unknown keys are preserved. If the file has hujson comments, we warn —
	// the round-trip will drop them.
	merged := map[string]any{}
	if len(current) > 0 {
		if looksLikeHujson(current) {
			slog.Warn("provider: settings.json contains comments; comments will be lost on write")
		}
		if err := json.Unmarshal(current, &merged); err != nil {
			// Can't safely merge — refuse rather than silently dropping keys.
			slog.Warn("provider: cannot parse existing settings.json", "error", err.Error())
			writeError(w, http.StatusUnprocessableEntity, "VDX-000",
				"existing settings.json is not parseable JSON; please fix it manually before editing via Vedox")
			return
		}
	}
	if req.Permissions == nil {
		delete(merged, "permissions")
	} else {
		merged["permissions"] = req.Permissions
	}

	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		slog.Error("provider: marshal settings.json", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	out = append(out, '\n')

	if err := atomicWrite(projectDir, absPath, out, 0o755, 0o644); err != nil {
		s.writeProviderWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, etagOnlyResponse{Etag: computeEtag(out)})
}

// ── Provider handler helpers ──────────────────────────────────────────────────

// providerProjectDir validates the :project param and returns the project
// directory, writing a 400 on failure. The returned directory is the security
// boundary for subsequent atomicWrite calls.
func (s *Server) providerProjectDir(w http.ResponseWriter, r *http.Request) (string, bool) {
	project := chi.URLParam(r, "project")
	projectDir, err := s.safeProjectPath(project)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VDX-005", "invalid project name")
		return "", false
	}
	return projectDir, true
}

// decodeProviderBody reads at most maxProviderBody bytes and decodes the body
// as T. Size overflow returns 413 VDX-010; malformed JSON returns 400.
func decodeProviderBody[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	r.Body = http.MaxBytesReader(w, r.Body, maxProviderBody)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		// http.MaxBytesReader returns its error with a specific type; the
		// simplest check is to look at the error string, since we control the
		// one path that sets it.
		if strings.Contains(err.Error(), "request body too large") {
			writeError(w, http.StatusRequestEntityTooLarge, "VDX-010", "request body exceeds 1MB limit")
			return v, false
		}
		writeError(w, http.StatusBadRequest, "VDX-000", "could not read request body")
		return v, false
	}
	if err := json.Unmarshal(body, &v); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-000", "invalid JSON body")
		return v, false
	}
	return v, true
}

// writeProviderWriteError translates atomicWrite failures into a VDX response.
// Symlink-ancestor rejection is a 400/VDX-005; everything else is a 500.
func (s *Server) writeProviderWriteError(w http.ResponseWriter, err error) {
	msg := err.Error()
	if strings.Contains(msg, "symlink") || strings.Contains(msg, "escapes boundary") {
		writeError(w, http.StatusBadRequest, "VDX-005",
			"target path rejected: symlink or traversal detected")
		return
	}
	slog.Error("provider: atomic write failed", "error", msg)
	writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
}
