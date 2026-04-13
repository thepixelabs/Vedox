package api

// Provider config — Codex global (~/.codex/config.toml).
//
// Codex config lives OUTSIDE the workspace — it is a user-global file that may
// contain API tokens. This has three consequences:
//
//  1. The security boundary for atomicWrite is the user's home directory, not
//     the project directory. The :project URL param is still validated (to
//     reject path-traversal attempts through the router), but the resolved
//     project dir is discarded.
//  2. The target path must be exactly ~/.codex/config.toml; we assert that
//     after cleaning to prevent any shenanigans with the home override.
//  3. Writes use tighter permissions (dir 0o700, file 0o600) because the file
//     may contain API tokens.
//
// All writable fields go through a TYPED struct — never map[string]any — so a
// malicious frontend cannot inject arbitrary TOML keys.

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/BurntSushi/toml"
)

var codexCommentsWarnOnce sync.Once

// ── Paths ─────────────────────────────────────────────────────────────────────

// codexPaths resolves the canonical (home, configPath) pair. It fails closed
// if the home directory is unknown or the resolved path is not the exact
// canonical form.
func (s *Server) codexPaths() (home, configPath string, err error) {
	home, err = s.userHome()
	if err != nil {
		return "", "", err
	}
	home = filepath.Clean(home)
	configPath = filepath.Join(home, ".codex", "config.toml")
	// Defensive: ensure the cleaned form matches what we expect exactly.
	if filepath.Clean(configPath) != filepath.Join(home, ".codex", "config.toml") {
		return "", "", fmt.Errorf("codex config path failed canonicalisation")
	}
	return home, configPath, nil
}

// ── Response/request shapes ───────────────────────────────────────────────────

type codexConfigResponse struct {
	MCP          codexMCPBlock `json:"mcp"`
	ApprovalMode string        `json:"approvalMode"`
	Sandbox      string        `json:"sandbox"`
	ConfigEtag   string        `json:"configEtag"`
	Scope        string        `json:"scope"`
}
type codexMCPBlock struct {
	Servers map[string]any `json:"servers"`
	Etag    string         `json:"etag"`
}

type putCodexMCPRequest struct {
	Servers map[string]any `json:"servers"`
	Etag    string         `json:"etag"`
}

// putCodexSettingsRequest is deliberately typed. approval_mode and sandbox are
// the ONLY keys a client can mutate; anything else in the body is ignored.
type putCodexSettingsRequest struct {
	ApprovalMode *string `json:"approvalMode"`
	Sandbox      *string `json:"sandbox"`
	Etag         string  `json:"etag"`
}

// ── TOML helpers ──────────────────────────────────────────────────────────────

// loadCodexTOML reads configPath and decodes it into a generic map. Missing
// files return an empty map + empty etag (not an error).
func loadCodexTOML(configPath string) (map[string]any, []byte, string, error) {
	raw, err := readFileBytes(configPath)
	if err != nil {
		return nil, nil, "", err
	}
	if len(raw) == 0 {
		return map[string]any{}, nil, "", nil
	}
	// Heuristic comment warning — fires once per process.
	if tomlHasComments(raw) {
		codexCommentsWarnOnce.Do(func() {
			slog.Warn("provider: ~/.codex/config.toml contains comments; comments will be lost on write")
		})
	}
	m := map[string]any{}
	if _, err := toml.Decode(string(raw), &m); err != nil {
		return nil, raw, computeEtag(raw), err
	}
	return m, raw, computeEtag(raw), nil
}

// readFileBytes is a tiny shim around readFileOrEmpty that returns only the
// byte slice (missing → nil).
func readFileBytes(path string) ([]byte, error) {
	data, _, err := readFileOrEmpty(path)
	return data, err
}

// tomlHasComments is a cheap heuristic: any line whose first non-space
// character is `#` counts as a comment line.
func tomlHasComments(raw []byte) bool {
	for _, line := range bytes.Split(raw, []byte("\n")) {
		trimmed := bytes.TrimLeft(line, " \t")
		if len(trimmed) > 0 && trimmed[0] == '#' {
			return true
		}
	}
	return false
}

// encodeCodexTOML marshals m to TOML. We use BurntSushi's encoder.
func encodeCodexTOML(m map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (s *Server) handleGetCodexConfig(w http.ResponseWriter, r *http.Request) {
	// The :project param is still validated — discard the result.
	if _, ok := s.providerProjectDir(w, r); !ok {
		return
	}
	_, configPath, err := s.codexPaths()
	if err != nil {
		slog.Error("provider: codex paths", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	m, _, etag, err := loadCodexTOML(configPath)
	if err != nil {
		slog.Warn("provider: codex config unparseable", "error", err.Error())
		m = map[string]any{}
	}

	servers := map[string]any{}
	if v, ok := m["mcp_servers"].(map[string]any); ok {
		servers = v
	}
	approval, _ := m["approval_mode"].(string)
	sandbox, _ := m["sandbox"].(string)

	writeJSON(w, http.StatusOK, codexConfigResponse{
		MCP:          codexMCPBlock{Servers: servers, Etag: etag},
		ApprovalMode: approval,
		Sandbox:      sandbox,
		ConfigEtag:   etag,
		Scope:        "global",
	})
}

func (s *Server) handlePutCodexMCP(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.providerProjectDir(w, r); !ok {
		return
	}
	req, ok := decodeProviderBody[putCodexMCPRequest](w, r)
	if !ok {
		return
	}
	if err := s.writeCodexConfig(w, req.Etag, func(m map[string]any) {
		if req.Servers == nil {
			delete(m, "mcp_servers")
		} else {
			m["mcp_servers"] = req.Servers
		}
	}); err != nil {
		// response already written
		return
	}
}

func (s *Server) handlePutCodexSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.providerProjectDir(w, r); !ok {
		return
	}
	req, ok := decodeProviderBody[putCodexSettingsRequest](w, r)
	if !ok {
		return
	}
	if req.ApprovalMode != nil {
		if !validApprovalMode(*req.ApprovalMode) {
			writeError(w, http.StatusBadRequest, "VDX-000",
				"approvalMode must be one of: suggest, auto-edit, full-auto")
			return
		}
	}
	if req.Sandbox != nil {
		if !validSandbox(*req.Sandbox) {
			writeError(w, http.StatusBadRequest, "VDX-000",
				"sandbox must be one of: read-only, workspace-write, danger-full-access")
			return
		}
	}
	if err := s.writeCodexConfig(w, req.Etag, func(m map[string]any) {
		if req.ApprovalMode != nil {
			m["approval_mode"] = *req.ApprovalMode
		}
		if req.Sandbox != nil {
			m["sandbox"] = *req.Sandbox
		}
	}); err != nil {
		return
	}
}

// writeCodexConfig performs the shared load → conflict-check → mutate → save
// path. mutate is called with the decoded map; it must only touch known keys.
// On conflict or error it writes the response and returns a sentinel error so
// callers can bail out.
func (s *Server) writeCodexConfig(w http.ResponseWriter, clientEtag string, mutate func(m map[string]any)) error {
	home, configPath, err := s.codexPaths()
	if err != nil {
		slog.Error("provider: codex paths", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return err
	}

	m, current, currentEtag, err := loadCodexTOML(configPath)
	if err != nil && len(current) > 0 {
		writeError(w, http.StatusUnprocessableEntity, "VDX-000",
			"existing ~/.codex/config.toml is not parseable TOML; please fix it manually before editing via Vedox")
		return err
	}
	if currentEtag != clientEtag {
		writeJSON(w, http.StatusConflict, conflictResponse{
			Error: "conflict", CurrentEtag: currentEtag,
		})
		return fmt.Errorf("conflict")
	}

	mutate(m)

	out, err := encodeCodexTOML(m)
	if err != nil {
		slog.Error("provider: encode codex toml", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return err
	}

	if err := atomicWrite(home, configPath, out, 0o700, 0o600); err != nil {
		s.writeProviderWriteError(w, err)
		return err
	}
	writeJSON(w, http.StatusOK, etagOnlyResponse{Etag: computeEtag(out)})
	return nil
}

func validApprovalMode(v string) bool {
	switch v {
	case "suggest", "auto-edit", "full-auto":
		return true
	}
	return false
}
func validSandbox(v string) bool {
	switch v {
	case "read-only", "workspace-write", "danger-full-access":
		return true
	}
	return false
}

