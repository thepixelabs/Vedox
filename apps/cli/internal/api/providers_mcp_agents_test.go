package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSafeAgentFilename(t *testing.T) {
	cases := []struct {
		name string
		in   string
		ok   bool
	}{
		{"plain", "agent.md", true},
		{"traversal", "../evil.md", false},
		{"hidden", ".hidden.md", false},
		{"no_suffix", "noext", false},
		{"control_char", "agent\x01.md", false},
		{"too_long", strings.Repeat("a", 130) + ".md", false},
		{"backslash", "foo\\bar.md", false},
		{"slash", "foo/bar.md", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := safeAgentFilename(tc.in)
			gotOk := err == nil
			if gotOk != tc.ok {
				t.Errorf("safeAgentFilename(%q) ok=%v want %v (err=%v)", tc.in, gotOk, tc.ok, err)
			}
		})
	}
}

func TestClaudeMCP_RoundTrip(t *testing.T) {
	s, _, _ := newProviderTestServer(t)

	// GET on empty → empty servers.
	rec := callHandler(t, s.handleGetClaudeMCP, "GET", "/", nil,
		map[string]string{"project": "myproject"})
	if rec.Code != 200 {
		t.Fatalf("GET %d (%s)", rec.Code, rec.Body.String())
	}
	var got claudeMCPResponse
	decodeRec(t, rec, &got)
	if len(got.Servers) != 0 || got.Etag != "" {
		t.Errorf("expected empty mcp, got %+v", got)
	}

	// PUT initial.
	body := mustJSON(t, putMCPRequest{
		Servers: map[string]any{"foo": map[string]any{"command": "node"}},
		Etag:    "",
	})
	rec = callHandler(t, s.handlePutClaudeMCP, "PUT", "/", body,
		map[string]string{"project": "myproject"})
	if rec.Code != 200 {
		t.Fatalf("PUT %d (%s)", rec.Code, rec.Body.String())
	}
	var put1 etagOnlyResponse
	decodeRec(t, rec, &put1)

	// GET → servers come back.
	rec = callHandler(t, s.handleGetClaudeMCP, "GET", "/", nil,
		map[string]string{"project": "myproject"})
	decodeRec(t, rec, &got)
	if _, ok := got.Servers["foo"]; !ok {
		t.Errorf("expected foo server, got %+v", got.Servers)
	}
	if got.Etag != put1.Etag {
		t.Errorf("etag mismatch")
	}

	// Stale etag → 409.
	body = mustJSON(t, putMCPRequest{Servers: map[string]any{}, Etag: "stale"})
	rec = callHandler(t, s.handlePutClaudeMCP, "PUT", "/", body,
		map[string]string{"project": "myproject"})
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}
}

func TestAgents_CRUD(t *testing.T) {
	s, _, _ := newProviderTestServer(t)

	// List on empty → empty list.
	rec := callHandler(t, s.handleListAgents, "GET", "/", nil,
		map[string]string{"project": "myproject"})
	if rec.Code != 200 {
		t.Fatalf("list %d (%s)", rec.Code, rec.Body.String())
	}

	// Create an agent.
	body := mustJSON(t, createAgentRequest{
		Filename: "reviewer.md", Name: "Reviewer",
		Description: "Reviews code", Version: "1.0", Body: "Hello body.",
	})
	rec = callHandler(t, s.handleCreateAgent, "POST", "/", body,
		map[string]string{"project": "myproject"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d (%s)", rec.Code, rec.Body.String())
	}

	// List → contains it.
	rec = callHandler(t, s.handleListAgents, "GET", "/", nil,
		map[string]string{"project": "myproject"})
	var listResp struct {
		Agents []agentSummary `json:"agents"`
	}
	decodeRec(t, rec, &listResp)
	if len(listResp.Agents) != 1 || listResp.Agents[0].Name != "Reviewer" {
		t.Errorf("unexpected list: %+v", listResp.Agents)
	}

	// GET single.
	rec = callHandler(t, s.handleGetAgent, "GET", "/", nil,
		map[string]string{"project": "myproject", "filename": "reviewer.md"})
	if rec.Code != 200 {
		t.Fatalf("get %d (%s)", rec.Code, rec.Body.String())
	}
	var full agentFull
	decodeRec(t, rec, &full)
	if full.Body != "Hello body." {
		t.Errorf("body = %q", full.Body)
	}

	// PUT update.
	body = mustJSON(t, updateAgentRequest{
		Name: "Reviewer 2", Description: "v2", Version: "2.0",
		Body: "New body.", Etag: full.Etag,
	})
	rec = callHandler(t, s.handlePutAgent, "PUT", "/", body,
		map[string]string{"project": "myproject", "filename": "reviewer.md"})
	if rec.Code != 200 {
		t.Fatalf("put %d (%s)", rec.Code, rec.Body.String())
	}

	// Stale etag → 409.
	body = mustJSON(t, updateAgentRequest{Name: "x", Etag: "stale"})
	rec = callHandler(t, s.handlePutAgent, "PUT", "/", body,
		map[string]string{"project": "myproject", "filename": "reviewer.md"})
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rec.Code)
	}

	// DELETE → 204.
	rec = callHandler(t, s.handleDeleteAgent, "DELETE", "/", nil,
		map[string]string{"project": "myproject", "filename": "reviewer.md"})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestAgent_RejectsBadFilename(t *testing.T) {
	s, _, _ := newProviderTestServer(t)
	rec := callHandler(t, s.handleGetAgent, "GET", "/", nil,
		map[string]string{"project": "myproject", "filename": "../evil.md"})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestAgent_PreservesExtraFrontmatter(t *testing.T) {
	s, root, _ := newProviderTestServer(t)
	// Seed an agent file with an unknown frontmatter key.
	dir := filepath.Join(root, "myproject", ".claude", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw := "---\nname: Old\ndescription: Old desc\nversion: '0.1'\nmodel: claude-3\n---\nbody here\n"
	if err := os.WriteFile(filepath.Join(dir, "x.md"), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	// GET to grab etag.
	rec := callHandler(t, s.handleGetAgent, "GET", "/", nil,
		map[string]string{"project": "myproject", "filename": "x.md"})
	var full agentFull
	decodeRec(t, rec, &full)

	// PUT new content.
	body := mustJSON(t, updateAgentRequest{
		Name: "New", Description: "New desc", Version: "0.2",
		Body: "new body", Etag: full.Etag,
	})
	rec = callHandler(t, s.handlePutAgent, "PUT", "/", body,
		map[string]string{"project": "myproject", "filename": "x.md"})
	if rec.Code != 200 {
		t.Fatalf("put %d (%s)", rec.Code, rec.Body.String())
	}

	// File on disk must still mention "model: claude-3".
	data, _ := os.ReadFile(filepath.Join(dir, "x.md"))
	if !strings.Contains(string(data), "model:") {
		t.Errorf("unknown frontmatter key 'model' was stripped: %s", string(data))
	}
}
