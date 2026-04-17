package api

// FIX-SEC-07 acceptance tests for the mutating repo-onboarding endpoints:
//
//	POST /api/repos/create   — handleCreateRepoWithInit
//	POST /api/repos/register — handleRegisterRepo
//
// Before this fix, both endpoints were reachable by any local process that
// could speak to the daemon port — no credential was required. These tests
// pin the bootstrap-token guard in place:
//
//  1. No Authorization header             → 401 VDX-401
//  2. Wrong bearer value                  → 401 VDX-401
//  3. Non-Bearer scheme (e.g. Basic)      → 401 VDX-401
//  4. Correct Bearer token                → 201 (happy path still works)
//
// The tests speak HTTP directly (not through the fixture helper) because
// the helper auto-attaches the valid token; we need to exercise missing /
// malformed headers explicitly.

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// postRawRepos issues a raw POST to the reposFixture server without the
// Authorization header the standard helper injects. The Origin header is
// still set so the CORS middleware lets the request through. authHeader is
// set verbatim when non-empty so the caller can exercise the "wrong token"
// and "wrong scheme" variants.
func postRawRepos(t *testing.T, f *reposFixture, path, authHeader, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, f.server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// ---------------------------------------------------------------------------
// POST /api/repos/create — FIX-SEC-07 guard
// ---------------------------------------------------------------------------

// TestCreateRepo_NoToken_Returns401 verifies the unauthenticated caller is
// rejected. Before FIX-SEC-07 this returned 201 (or 503 when no GlobalDB).
func TestCreateRepo_NoToken_Returns401(t *testing.T) {
	f := newReposFixture(t)

	// Pick a path inside $HOME so the only thing that could fail is auth.
	repoPath := filepath.Join(homeTempDir(t), "no-token-create")
	body := `{"name":"no-token","type":"private","path":"` + repoPath + `"}`
	resp := postRawRepos(t, f, "/api/repos/create", "", body)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body=%s)", resp.StatusCode, drainBody(t, resp))
	}
	if body := drainBody(t, resp); !strings.Contains(body, "VDX-401") {
		t.Errorf("expected VDX-401 in body, got: %s", body)
	}
}

// TestCreateRepo_WrongToken_Returns401 verifies that a plausible but incorrect
// token is also rejected. Constant-time comparison must not leak whether the
// token was partially correct.
func TestCreateRepo_WrongToken_Returns401(t *testing.T) {
	f := newReposFixture(t)

	repoPath := filepath.Join(homeTempDir(t), "wrong-token-create")
	body := `{"name":"wrong","type":"private","path":"` + repoPath + `"}`
	resp := postRawRepos(t, f, "/api/repos/create", "Bearer deadbeef", body)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body=%s)", resp.StatusCode, drainBody(t, resp))
	}
	if body := drainBody(t, resp); !strings.Contains(body, "VDX-401") {
		t.Errorf("expected VDX-401 in body, got: %s", body)
	}
}

// TestCreateRepo_MalformedAuthScheme_Returns401 verifies that a non-Bearer
// Authorization value (Basic, Token, etc.) is rejected.
func TestCreateRepo_MalformedAuthScheme_Returns401(t *testing.T) {
	f := newReposFixture(t)

	repoPath := filepath.Join(homeTempDir(t), "basic-create")
	body := `{"name":"basic","type":"private","path":"` + repoPath + `"}`
	resp := postRawRepos(t, f, "/api/repos/create", "Basic "+testReposToken, body)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// TestCreateRepo_WithToken_Returns201 verifies the happy path still works
// when a correct Bearer token is supplied — proving the middleware is a
// gate, not a brick wall.
func TestCreateRepo_WithToken_Returns201(t *testing.T) {
	f := newReposFixture(t)

	repoPath := filepath.Join(homeTempDir(t), "with-token-create")
	body := `{"name":"with-token","type":"private","path":"` + repoPath + `"}`
	resp := postRawRepos(t, f, "/api/repos/create", "Bearer "+testReposToken, body)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body=%s)", resp.StatusCode, drainBody(t, resp))
	}
}

// ---------------------------------------------------------------------------
// POST /api/repos/register — FIX-SEC-07 guard
// ---------------------------------------------------------------------------

// TestRegisterRepo_NoToken_Returns401 verifies the unauthenticated caller is
// rejected before the handler reads the request body.
func TestRegisterRepo_NoToken_Returns401(t *testing.T) {
	f := newReposFixture(t)

	body := `{"path":"/tmp/whatever"}`
	resp := postRawRepos(t, f, "/api/repos/register", "", body)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body=%s)", resp.StatusCode, drainBody(t, resp))
	}
	if body := drainBody(t, resp); !strings.Contains(body, "VDX-401") {
		t.Errorf("expected VDX-401 in body, got: %s", body)
	}
}

// TestRegisterRepo_WrongToken_Returns401 verifies that a bad token is also
// rejected.
func TestRegisterRepo_WrongToken_Returns401(t *testing.T) {
	f := newReposFixture(t)

	body := `{"path":"/tmp/whatever"}`
	resp := postRawRepos(t, f, "/api/repos/register", "Bearer not-the-right-token", body)

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}
