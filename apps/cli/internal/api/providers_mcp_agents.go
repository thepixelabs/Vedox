package api

// Provider config — Claude Code MCP (.mcp.json) and Agents (.claude/agents/*.md).
//
// MCP is a JSON round-trip that preserves unknown keys (same pattern as
// settings.json). Agents are YAML-frontmatter + Markdown body files; we parse
// and re-emit them through yaml.v3 so unknown frontmatter keys survive reads
// but only the known fields are rewritten on PUT.

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

// ── Paths ─────────────────────────────────────────────────────────────────────

func mcpPath(projectDir string) string {
	return filepath.Join(projectDir, ".mcp.json")
}
func agentsDir(projectDir string) string {
	return filepath.Join(projectDir, ".claude", "agents")
}

// ── Request/response shapes ───────────────────────────────────────────────────

type claudeMCPResponse struct {
	Servers map[string]any `json:"servers"`
	Etag    string         `json:"etag"`
}
type putMCPRequest struct {
	Servers map[string]any `json:"servers"`
	Etag    string         `json:"etag"`
}

type agentSummary struct {
	Filename    string `json:"filename"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}
type agentFull struct {
	Filename    string `json:"filename"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Body        string `json:"body"`
	Etag        string `json:"etag"`
}

type createAgentRequest struct {
	Filename    string `json:"filename"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Body        string `json:"body"`
}
type updateAgentRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Body        string `json:"body"`
	Etag        string `json:"etag"`
}

// ── safeAgentFilename ─────────────────────────────────────────────────────────

// safeAgentFilename validates an agent filename. Rules:
//   - must end in ".md"
//   - no path separators (/ or \) and no ".."
//   - must not start with "." (no hidden files, no "..")
//   - at most 128 characters
//   - no control characters (< 0x20 or 0x7f)
//
// Returns the trimmed filename on success, or an error describing the reason
// for the reject. The returned error is user-safe.
func safeAgentFilename(name string) (string, error) {
	if name == "" {
		return "", errors.New("filename is empty")
	}
	if len(name) > 128 {
		return "", errors.New("filename too long (max 128)")
	}
	if strings.ContainsAny(name, `/\`) {
		return "", errors.New("filename contains path separators")
	}
	if strings.Contains(name, "..") {
		return "", errors.New("filename contains traversal sequence")
	}
	if strings.HasPrefix(name, ".") {
		return "", errors.New("filename must not start with '.'")
	}
	if !strings.HasSuffix(name, ".md") {
		return "", errors.New("filename must end with .md")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "", errors.New("filename contains control characters")
		}
	}
	return name, nil
}

// ── Agent file (de)serialisation ──────────────────────────────────────────────

// agentFrontmatter holds the known YAML keys we round-trip. Unknown keys are
// preserved in the `extra` map via a custom unmarshal path.
type agentFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
}

// parseAgentFile splits raw into its frontmatter block and body. A file without
// frontmatter delimiters is treated as all-body with empty frontmatter.
func parseAgentFile(raw []byte) (fm agentFrontmatter, extra map[string]any, body string, err error) {
	s := string(raw)
	// Accept either "---\n" or "---\r\n" as the opening delimiter.
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return agentFrontmatter{}, nil, s, nil
	}
	// Locate closing "---" on its own line.
	rest := strings.TrimPrefix(strings.TrimPrefix(s, "---\r\n"), "---\n")
	endIdx := strings.Index(rest, "\n---")
	if endIdx < 0 {
		return agentFrontmatter{}, nil, s, nil
	}
	yamlBlock := rest[:endIdx]
	afterEnd := rest[endIdx+len("\n---"):]
	// Strip the trailing newline(s) after the closing ---.
	afterEnd = strings.TrimPrefix(afterEnd, "\r\n")
	afterEnd = strings.TrimPrefix(afterEnd, "\n")

	all := map[string]any{}
	if err := yaml.Unmarshal([]byte(yamlBlock), &all); err != nil {
		return agentFrontmatter{}, nil, "", fmt.Errorf("parse frontmatter: %w", err)
	}
	if v, ok := all["name"].(string); ok {
		fm.Name = v
	}
	if v, ok := all["description"].(string); ok {
		fm.Description = v
	}
	if v, ok := all["version"].(string); ok {
		fm.Version = v
	}
	delete(all, "name")
	delete(all, "description")
	delete(all, "version")
	return fm, all, afterEnd, nil
}

// encodeAgentFile serialises frontmatter + extra unknown keys + body.
func encodeAgentFile(fm agentFrontmatter, extra map[string]any, body string) ([]byte, error) {
	merged := map[string]any{}
	for k, v := range extra {
		merged[k] = v
	}
	// Known keys overwrite extras so a malicious/extra "name" cannot shadow.
	merged["name"] = fm.Name
	merged["description"] = fm.Description
	merged["version"] = fm.Version

	yamlBytes, err := yaml.Marshal(merged)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString("---\n")
	b.Write(yamlBytes)
	b.WriteString("---\n")
	b.WriteString(body)
	return []byte(b.String()), nil
}

// ── MCP handlers ──────────────────────────────────────────────────────────────

func (s *Server) handleGetClaudeMCP(w http.ResponseWriter, r *http.Request) {
	projectDir, ok := s.providerProjectDir(w, r)
	if !ok {
		return
	}
	data, etag, err := readFileOrEmpty(mcpPath(projectDir))
	if err != nil {
		slog.Error("provider: read .mcp.json", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	servers := map[string]any{}
	if len(data) > 0 {
		var root map[string]any
		if err := json.Unmarshal(data, &root); err != nil {
			slog.Warn("provider: .mcp.json unparseable", "error", err.Error())
			root = map[string]any{}
		}
		if m, ok := root["mcpServers"].(map[string]any); ok {
			servers = m
		} else if m, ok := root["servers"].(map[string]any); ok {
			servers = m
		}
	}
	writeJSON(w, http.StatusOK, claudeMCPResponse{Servers: servers, Etag: etag})
}

func (s *Server) handlePutClaudeMCP(w http.ResponseWriter, r *http.Request) {
	projectDir, ok := s.providerProjectDir(w, r)
	if !ok {
		return
	}
	req, ok := decodeProviderBody[putMCPRequest](w, r)
	if !ok {
		return
	}
	absPath := mcpPath(projectDir)
	current, currentEtag, err := readFileOrEmpty(absPath)
	if err != nil {
		slog.Error("provider: read .mcp.json", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	if currentEtag != req.Etag {
		writeJSON(w, http.StatusConflict, conflictResponse{
			Error: "conflict", CurrentEtag: currentEtag,
		})
		return
	}

	root := map[string]any{}
	if len(current) > 0 {
		if looksLikeHujson(current) {
			slog.Warn("provider: .mcp.json contains comments; comments will be lost on write")
		}
		if err := json.Unmarshal(current, &root); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "VDX-000",
				"existing .mcp.json is not parseable JSON; please fix it manually before editing via Vedox")
			return
		}
	}
	if req.Servers == nil {
		root["mcpServers"] = map[string]any{}
	} else {
		root["mcpServers"] = req.Servers
	}

	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		slog.Error("provider: marshal .mcp.json", "error", err.Error())
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

// ── Agent handlers ────────────────────────────────────────────────────────────

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	projectDir, ok := s.providerProjectDir(w, r)
	if !ok {
		return
	}
	dir := agentsDir(projectDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, http.StatusOK, map[string]any{"agents": []agentSummary{}})
			return
		}
		slog.Error("provider: list agents", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	out := []agentSummary{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fn := e.Name()
		if _, err := safeAgentFilename(fn); err != nil {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, fn))
		if err != nil {
			continue
		}
		fm, _, _, perr := parseAgentFile(raw)
		if perr != nil {
			// Surface a best-effort row so the user can still see & fix it.
			out = append(out, agentSummary{Filename: fn})
			continue
		}
		out = append(out, agentSummary{
			Filename:    fn,
			Name:        fm.Name,
			Description: fm.Description,
			Version:     fm.Version,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Filename < out[j].Filename })
	writeJSON(w, http.StatusOK, map[string]any{"agents": out})
}

func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	projectDir, ok := s.providerProjectDir(w, r)
	if !ok {
		return
	}
	fn, err := safeAgentFilename(chi.URLParam(r, "filename"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VDX-005", "invalid agent filename")
		return
	}
	abs := filepath.Join(agentsDir(projectDir), fn)
	raw, etag, err := readFileOrEmpty(abs)
	if err != nil {
		slog.Error("provider: read agent", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	if len(raw) == 0 {
		writeError(w, http.StatusNotFound, "VDX-000", "agent not found")
		return
	}
	fm, _, body, perr := parseAgentFile(raw)
	if perr != nil {
		writeError(w, http.StatusUnprocessableEntity, "VDX-000", "agent file has invalid frontmatter")
		return
	}
	writeJSON(w, http.StatusOK, agentFull{
		Filename: fn, Name: fm.Name, Description: fm.Description,
		Version: fm.Version, Body: body, Etag: etag,
	})
}

func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	projectDir, ok := s.providerProjectDir(w, r)
	if !ok {
		return
	}
	req, ok := decodeProviderBody[createAgentRequest](w, r)
	if !ok {
		return
	}
	fn, err := safeAgentFilename(req.Filename)
	if err != nil {
		writeError(w, http.StatusBadRequest, "VDX-005", "invalid agent filename: "+err.Error())
		return
	}
	abs := filepath.Join(agentsDir(projectDir), fn)
	if _, err := os.Lstat(abs); err == nil {
		writeError(w, http.StatusConflict, "VDX-000", "agent already exists")
		return
	}
	data, err := encodeAgentFile(agentFrontmatter{
		Name: req.Name, Description: req.Description, Version: req.Version,
	}, nil, req.Body)
	if err != nil {
		slog.Error("provider: encode agent", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	if err := atomicWrite(projectDir, abs, data, 0o755, 0o644); err != nil {
		s.writeProviderWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"filename": fn, "etag": computeEtag(data),
	})
}

func (s *Server) handlePutAgent(w http.ResponseWriter, r *http.Request) {
	projectDir, ok := s.providerProjectDir(w, r)
	if !ok {
		return
	}
	fn, err := safeAgentFilename(chi.URLParam(r, "filename"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VDX-005", "invalid agent filename")
		return
	}
	req, ok := decodeProviderBody[updateAgentRequest](w, r)
	if !ok {
		return
	}
	abs := filepath.Join(agentsDir(projectDir), fn)
	current, currentEtag, err := readFileOrEmpty(abs)
	if err != nil {
		slog.Error("provider: read agent", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	if len(current) == 0 {
		writeError(w, http.StatusNotFound, "VDX-000", "agent not found")
		return
	}
	if currentEtag != req.Etag {
		writeJSON(w, http.StatusConflict, conflictResponse{
			Error: "conflict", CurrentEtag: currentEtag,
		})
		return
	}
	_, extra, _, perr := parseAgentFile(current)
	if perr != nil {
		writeError(w, http.StatusUnprocessableEntity, "VDX-000", "existing agent has invalid frontmatter")
		return
	}
	data, err := encodeAgentFile(agentFrontmatter{
		Name: req.Name, Description: req.Description, Version: req.Version,
	}, extra, req.Body)
	if err != nil {
		slog.Error("provider: encode agent", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	if err := atomicWrite(projectDir, abs, data, 0o755, 0o644); err != nil {
		s.writeProviderWriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, etagOnlyResponse{Etag: computeEtag(data)})
}

func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	projectDir, ok := s.providerProjectDir(w, r)
	if !ok {
		return
	}
	fn, err := safeAgentFilename(chi.URLParam(r, "filename"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "VDX-005", "invalid agent filename")
		return
	}
	abs := filepath.Join(agentsDir(projectDir), fn)
	// Symlink-ancestor check: the delete path must be inside the project with
	// no symlinks on the way down, same guarantee as writes.
	if err := assertNoSymlinkAncestor(projectDir, abs); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-005",
			"target path rejected: symlink or traversal detected")
		return
	}
	if err := os.Remove(abs); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		slog.Error("provider: delete agent", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
