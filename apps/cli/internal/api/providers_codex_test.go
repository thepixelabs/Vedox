package api

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestCodex_RoundTrip(t *testing.T) {
	s, _, home := newProviderTestServer(t)

	// GET on empty → empty values, "global" scope.
	rec := callHandler(t, s.handleGetCodexConfig, "GET", "/", nil,
		map[string]string{"project": "myproject"})
	if rec.Code != 200 {
		t.Fatalf("GET %d (%s)", rec.Code, rec.Body.String())
	}
	var got codexConfigResponse
	decodeRec(t, rec, &got)
	if got.Scope != "global" {
		t.Errorf("scope = %q, want global", got.Scope)
	}
	if got.ConfigEtag != "" {
		t.Errorf("expected empty etag, got %q", got.ConfigEtag)
	}

	// PUT settings (typed).
	approval := "auto-edit"
	sandbox := "workspace-write"
	body := mustJSON(t, putCodexSettingsRequest{
		ApprovalMode: &approval, Sandbox: &sandbox, Etag: "",
	})
	rec = callHandler(t, s.handlePutCodexSettings, "PUT", "/", body,
		map[string]string{"project": "myproject"})
	if rec.Code != 200 {
		t.Fatalf("PUT settings %d (%s)", rec.Code, rec.Body.String())
	}
	var put1 etagOnlyResponse
	decodeRec(t, rec, &put1)
	if put1.Etag == "" {
		t.Error("expected non-empty etag")
	}

	// File should exist at home/.codex/config.toml with mode 0600.
	cfgPath := filepath.Join(home, ".codex", "config.toml")
	info, err := os.Stat(cfgPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %o, want 0600", info.Mode().Perm())
	}

	// GET again → values populated, etag stable.
	rec = callHandler(t, s.handleGetCodexConfig, "GET", "/", nil,
		map[string]string{"project": "myproject"})
	decodeRec(t, rec, &got)
	if got.ApprovalMode != "auto-edit" || got.Sandbox != "workspace-write" {
		t.Errorf("settings not persisted: %+v", got)
	}
	if got.ConfigEtag != put1.Etag {
		t.Errorf("etag mismatch")
	}

	// Stale etag → 409.
	body = mustJSON(t, putCodexSettingsRequest{ApprovalMode: &approval, Etag: "stale"})
	rec = callHandler(t, s.handlePutCodexSettings, "PUT", "/", body,
		map[string]string{"project": "myproject"})
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}

	// Invalid approvalMode → 400.
	bad := "yolo"
	body = mustJSON(t, putCodexSettingsRequest{ApprovalMode: &bad, Etag: put1.Etag})
	rec = callHandler(t, s.handlePutCodexSettings, "PUT", "/", body,
		map[string]string{"project": "myproject"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCodex_GlobalScopeInvariant(t *testing.T) {
	// Two different :project values must observe the same etag — they read
	// the same global file.
	s, root, _ := newProviderTestServer(t)
	if err := os.MkdirAll(filepath.Join(root, "other"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Seed via a write through project A.
	approval := "suggest"
	body := mustJSON(t, putCodexSettingsRequest{ApprovalMode: &approval, Etag: ""})
	rec := callHandler(t, s.handlePutCodexSettings, "PUT", "/", body,
		map[string]string{"project": "myproject"})
	if rec.Code != 200 {
		t.Fatalf("seed %d (%s)", rec.Code, rec.Body.String())
	}

	rec = callHandler(t, s.handleGetCodexConfig, "GET", "/", nil,
		map[string]string{"project": "myproject"})
	var a codexConfigResponse
	decodeRec(t, rec, &a)

	rec = callHandler(t, s.handleGetCodexConfig, "GET", "/", nil,
		map[string]string{"project": "other"})
	var b codexConfigResponse
	decodeRec(t, rec, &b)

	if a.ConfigEtag != b.ConfigEtag {
		t.Errorf("etags differ across projects: %q vs %q", a.ConfigEtag, b.ConfigEtag)
	}
	if a.ApprovalMode != b.ApprovalMode {
		t.Errorf("approvalMode differs across projects")
	}
}

func TestCodex_MCPRoundTrip(t *testing.T) {
	s, _, _ := newProviderTestServer(t)

	body := mustJSON(t, putCodexMCPRequest{
		Servers: map[string]any{"foo": map[string]any{"command": "node"}},
		Etag:    "",
	})
	rec := callHandler(t, s.handlePutCodexMCP, "PUT", "/", body,
		map[string]string{"project": "myproject"})
	if rec.Code != 200 {
		t.Fatalf("PUT mcp %d (%s)", rec.Code, rec.Body.String())
	}

	rec = callHandler(t, s.handleGetCodexConfig, "GET", "/", nil,
		map[string]string{"project": "myproject"})
	var got codexConfigResponse
	decodeRec(t, rec, &got)
	if _, ok := got.MCP.Servers["foo"]; !ok {
		t.Errorf("expected foo server, got %+v", got.MCP.Servers)
	}
}
