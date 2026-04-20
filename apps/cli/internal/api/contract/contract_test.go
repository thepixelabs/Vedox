// Package contract pins every public HTTP endpoint of the Vedox daemon API.
//
// Each test asserts four stable properties of an endpoint:
//  1. The URL path is registered and responds (not 404/405).
//  2. The HTTP method is correct.
//  3. Required request fields are rejected when missing (mutating endpoints).
//  4. Response JSON top-level keys are stable.
//  5. Auth requirement is stable (bootstrap token, HMAC agent auth, or open).
//
// Tests use a real chi router via api.NewServer(...).Mount(mux) and
// httptest.NewRecorder — no network, no external process.
//
// Note: t.Parallel() is deliberately NOT used at the top level. The
// modernc.org/sqlite driver (a pure-Go CGO-free port) races on
// sqlite3_initialize during concurrent open calls — a known upstream
// issue. The existing api/ test suite takes the same approach. Individual
// subtests within a single test function can safely call t.Parallel()
// because they share the fixture (already-opened DB).
package contract

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"time"

	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/api"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/providers"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// testBootstrapToken is the 64-hex-char token used for bootstrap-token–gated
// endpoints. Must be exactly 64 hex characters.
const testBootstrapToken = "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"

// allowedOrigin is the CORS origin the middleware accepts for mutating verbs.
const allowedOrigin = "http://localhost:5151"

// ---------------------------------------------------------------------------
// Fixture
// ---------------------------------------------------------------------------

type fixture struct {
	mux           *http.ServeMux
	srv           *api.Server
	workspaceRoot string
	globalDB      *db.GlobalDB
	jobStore      *scanner.JobStore
}

// newFixture builds a fully-wired API server backed by temp dirs.
// A real GlobalDB is injected so repo/analytics endpoints work.
// Bootstrap token is set for browse/repos-create/register.
func newFixture(t *testing.T) *fixture {
	t.Helper()

	raw := t.TempDir()
	wsRoot, err := filepath.EvalSymlinks(raw)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	adapter, err := store.NewLocalAdapter(wsRoot, nil)
	if err != nil {
		t.Fatalf("store.NewLocalAdapter: %v", err)
	}

	wsDB, err := db.Open(db.Options{WorkspaceRoot: wsRoot})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = wsDB.Close() })

	gdbPath := filepath.Join(wsRoot, "global.db")
	gdb, err := db.OpenGlobalDB(gdbPath)
	if err != nil {
		t.Fatalf("db.OpenGlobalDB: %v", err)
	}
	t.Cleanup(func() { _ = gdb.Close() })

	jobStore := scanner.NewJobStore()
	srv := api.NewServer(
		adapter,
		wsDB,
		wsRoot,
		jobStore,
		ai.NewJobStore(3),
		store.NewProjectRegistry(),
		agentauth.PassthroughAuth(),
	)
	srv.SetGlobalDB(gdb)
	srv.SetBootstrapToken(testBootstrapToken)
	srv.SetHomeDirOverride(wsRoot)
	// Inject a stub KeyStore so agent/list works and agent/install reaches
	// buildInstaller (returning 400 unknown provider) rather than returning 503.
	srv.SetKeyStore(newStubKeyStore())

	mux := http.NewServeMux()
	srv.Mount(mux)

	return &fixture{
		mux:           mux,
		srv:           srv,
		workspaceRoot: wsRoot,
		globalDB:      gdb,
		jobStore:      jobStore,
	}
}

// ---------------------------------------------------------------------------
// Stub KeyStore — satisfies providers.KeyIssuer without touching the OS
// keychain. Keys are in-memory and deterministic.
// ---------------------------------------------------------------------------

type stubKeyStore struct{}

func newStubKeyStore() providers.KeyIssuer { return &stubKeyStore{} }

func (s *stubKeyStore) IssueKey(name, project, pathPrefix string) (string, string, error) {
	return "stub-key-id", "stub-secret-" + name, nil
}

func (s *stubKeyStore) RevokeKey(keyID string) error { return nil }

// ---------------------------------------------------------------------------
// Request helpers
// ---------------------------------------------------------------------------

func (f *fixture) get(t *testing.T, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	f.mux.ServeHTTP(w, req)
	return w
}

func (f *fixture) post(t *testing.T, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", allowedOrigin)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	f.mux.ServeHTTP(w, req)
	return w
}

func (f *fixture) put(t *testing.T, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(http.MethodPut, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", allowedOrigin)
	w := httptest.NewRecorder()
	f.mux.ServeHTTP(w, req)
	return w
}

// bearerHeader returns an Authorization header map for bootstrap-token tests.
func bearerHeader(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

// decodeJSON decodes the response body into the given pointer.
func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON response (status %d): %v\nbody: %s", w.Code, err, w.Body.String())
	}
}

// objectKeys returns the top-level key set of a JSON object response.
func objectKeys(t *testing.T, w *httptest.ResponseRecorder) map[string]struct{} {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&m); err != nil {
		t.Fatalf("decode JSON object (status %d): %v\nbody: %s", w.Code, err, w.Body.String())
	}
	out := make(map[string]struct{}, len(m))
	for k := range m {
		out[k] = struct{}{}
	}
	return out
}

// hasKeys asserts that every expected key is present in the response JSON object.
func hasKeys(t *testing.T, w *httptest.ResponseRecorder, expected ...string) {
	t.Helper()
	got := objectKeys(t, w)
	var gotList []string
	for k := range got {
		gotList = append(gotList, k)
	}
	for _, want := range expected {
		if _, ok := got[want]; !ok {
			t.Errorf("response missing key %q; got keys: %v\nbody: %s", want, gotList, w.Body.String())
		}
	}
}

func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status: want %d, got %d\nbody: %s", want, w.Code, w.Body.String())
	}
}

// assertWrongMethod verifies that the path is registered (not 404) but
// rejects the given method with a non-2xx response.
func (f *fixture) assertWrongMethod(t *testing.T, path, wrongMethod string) {
	t.Helper()
	req := httptest.NewRequest(wrongMethod, path, nil)
	req.Header.Set("Origin", allowedOrigin)
	w := httptest.NewRecorder()
	f.mux.ServeHTTP(w, req)
	if w.Code == http.StatusNotFound {
		t.Errorf("assertWrongMethod(%q): path not registered at all (got 404 for method %q)", path, wrongMethod)
	}
	if w.Code >= 200 && w.Code < 300 {
		t.Errorf("assertWrongMethod(%q): wrong method %q returned 2xx — route does not enforce method", path, wrongMethod)
	}
}

func createFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func createDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// waitForScan blocks until the job store reports a completed (or failed) scan
// for the given job ID, or until 5 seconds elapse. This is called after
// POST /api/scan to prevent the background scan goroutine from writing to
// the TempDir after the test's t.Cleanup removes it (which causes Go's test
// runner to report "TempDir RemoveAll cleanup: directory not empty").
func (f *fixture) waitForScan(t *testing.T, jobID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if job, ok := f.jobStore.Snapshot(jobID); ok {
			if job.Status == "completed" || job.Status == "failed" {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Logf("waitForScan: job %s did not reach terminal state within 5s", jobID)
}

// ---------------------------------------------------------------------------
// 1. GET /api/health
// ---------------------------------------------------------------------------

func TestContract_Health(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		w := f.get(t, "/api/health", nil)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("method_GET_only", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/health", http.MethodPost)
	})

	t.Run("response_keys_stable", func(t *testing.T) {
		w := f.get(t, "/api/health", nil)
		assertStatus(t, w, http.StatusOK)
		hasKeys(t, w, "status")
	})

	t.Run("auth_open", func(t *testing.T) {
		// No token — must still return 200.
		w := f.get(t, "/api/health", nil)
		assertStatus(t, w, http.StatusOK)
	})
}

// ---------------------------------------------------------------------------
// 2. GET /api/projects
// ---------------------------------------------------------------------------

func TestContract_Projects_List(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		w := f.get(t, "/api/projects", nil)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("method_GET_only", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/projects", http.MethodPut)
	})

	t.Run("response_is_json_array", func(t *testing.T) {
		w := f.get(t, "/api/projects", nil)
		assertStatus(t, w, http.StatusOK)
		var arr []json.RawMessage
		if err := json.NewDecoder(w.Body).Decode(&arr); err != nil {
			t.Fatalf("expected JSON array: %v\nbody: %s", err, w.Body.String())
		}
	})

	t.Run("auth_open", func(t *testing.T) {
		w := f.get(t, "/api/projects", nil)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("GET /api/projects must be open; got %d", w.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// 3. POST /api/projects
// ---------------------------------------------------------------------------

func TestContract_Projects_Create(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		w := f.post(t, "/api/projects", map[string]string{"name": "contract-test-proj"}, nil)
		if w.Code != http.StatusCreated && w.Code != http.StatusConflict {
			t.Errorf("POST /api/projects: expected 201 or 409, got %d\nbody: %s", w.Code, w.Body.String())
		}
	})

	t.Run("required_name_missing_returns_400", func(t *testing.T) {
		w := f.post(t, "/api/projects", map[string]string{}, nil)
		assertStatus(t, w, http.StatusBadRequest)
		hasKeys(t, w, "code", "message")
	})

	t.Run("response_keys_on_create", func(t *testing.T) {
		w := f.post(t, "/api/projects", map[string]string{"name": "contract-proj-keys"}, nil)
		if w.Code != http.StatusCreated {
			t.Skipf("project creation returned %d; skipping key check", w.Code)
		}
		hasKeys(t, w, "name", "path", "docCount")
	})

	t.Run("auth_open_no_bootstrap_needed", func(t *testing.T) {
		// POST /api/projects is NOT bootstrap-token–gated.
		w := f.post(t, "/api/projects", map[string]string{"name": "contract-auth-probe"}, nil)
		if w.Code == http.StatusUnauthorized {
			t.Errorf("POST /api/projects must not require bootstrap token; got 401")
		}
	})
}

// ---------------------------------------------------------------------------
// 4. GET /api/scan
// ---------------------------------------------------------------------------

func TestContract_Scan_Summary(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		w := f.get(t, "/api/scan", nil)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("method_GET_not_PUT", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/scan", http.MethodPut)
	})

	t.Run("response_key_projects", func(t *testing.T) {
		w := f.get(t, "/api/scan", nil)
		assertStatus(t, w, http.StatusOK)
		hasKeys(t, w, "projects")
	})

	t.Run("auth_open", func(t *testing.T) {
		w := f.get(t, "/api/scan", nil)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("GET /api/scan must be open; got %d", w.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// 5. POST /api/scan
// ---------------------------------------------------------------------------

func TestContract_Scan_Start(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		w := f.post(t, "/api/scan", nil, nil)
		assertStatus(t, w, http.StatusAccepted)
		var resp struct{ JobID string `json:"jobId"` }
		_ = json.NewDecoder(w.Body).Decode(&resp)
		if resp.JobID != "" {
			f.waitForScan(t, resp.JobID)
		}
	})

	t.Run("method_POST_not_PUT", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/scan", http.MethodPut)
	})

	t.Run("response_key_jobId", func(t *testing.T) {
		w := f.post(t, "/api/scan", nil, nil)
		assertStatus(t, w, http.StatusAccepted)
		hasKeys(t, w, "jobId")
		var resp struct{ JobID string `json:"jobId"` }
		_ = json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&resp)
		if resp.JobID != "" {
			f.waitForScan(t, resp.JobID)
		}
	})

	t.Run("auth_open", func(t *testing.T) {
		w := f.post(t, "/api/scan", nil, nil)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("POST /api/scan must be open; got %d", w.Code)
		}
		var resp struct{ JobID string `json:"jobId"` }
		_ = json.NewDecoder(strings.NewReader(w.Body.String())).Decode(&resp)
		if resp.JobID != "" {
			f.waitForScan(t, resp.JobID)
		}
	})
}

// ---------------------------------------------------------------------------
// 6. GET /api/scan/{jobId}
// ---------------------------------------------------------------------------

func TestContract_Scan_Status(t *testing.T) {
	f := newFixture(t)

	// Obtain a real job ID by starting a scan and wait for it to finish so
	// the background goroutine does not write to TempDir after cleanup.
	wStart := f.post(t, "/api/scan", nil, nil)
	if wStart.Code != http.StatusAccepted {
		t.Fatalf("POST /api/scan: expected 202, got %d", wStart.Code)
	}
	var started struct {
		JobID string `json:"jobId"`
	}
	decodeJSON(t, wStart, &started)
	if started.JobID == "" {
		t.Fatal("POST /api/scan returned empty jobId")
	}
	f.waitForScan(t, started.JobID)

	t.Run("path_stable", func(t *testing.T) {
		w := f.get(t, "/api/scan/"+started.JobID, nil)
		if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
			t.Errorf("GET /api/scan/{jobId}: expected 200 or 404, got %d", w.Code)
		}
	})

	t.Run("unknown_job_id_returns_404", func(t *testing.T) {
		w := f.get(t, "/api/scan/no-such-job-id-xyz", nil)
		assertStatus(t, w, http.StatusNotFound)
		hasKeys(t, w, "code", "message")
	})

	t.Run("auth_open", func(t *testing.T) {
		w := f.get(t, "/api/scan/"+started.JobID, nil)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("GET /api/scan/{jobId} must be open; got %d", w.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// 7. GET /api/repos
// ---------------------------------------------------------------------------

func TestContract_Repos_List(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		w := f.get(t, "/api/repos", nil)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("method_GET_only", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/repos", http.MethodPut)
	})

	t.Run("response_is_json_array", func(t *testing.T) {
		w := f.get(t, "/api/repos", nil)
		assertStatus(t, w, http.StatusOK)
		var arr []json.RawMessage
		if err := json.NewDecoder(w.Body).Decode(&arr); err != nil {
			t.Fatalf("expected JSON array: %v", err)
		}
	})

	t.Run("auth_open", func(t *testing.T) {
		w := f.get(t, "/api/repos", nil)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("GET /api/repos must be open; got %d", w.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// 8. POST /api/repos
// ---------------------------------------------------------------------------

func TestContract_Repos_Create(t *testing.T) {
	f := newFixture(t)

	validBody := map[string]string{
		"name":      "contract-repo",
		"type":      "private",
		"root_path": f.workspaceRoot,
	}

	t.Run("path_stable", func(t *testing.T) {
		w := f.post(t, "/api/repos", validBody, nil)
		if w.Code != http.StatusCreated && w.Code != http.StatusConflict {
			t.Errorf("POST /api/repos: expected 201 or 409, got %d\nbody: %s", w.Code, w.Body.String())
		}
	})

	t.Run("required_name_missing_returns_400", func(t *testing.T) {
		w := f.post(t, "/api/repos",
			map[string]string{"type": "private", "root_path": f.workspaceRoot}, nil)
		assertStatus(t, w, http.StatusBadRequest)
		hasKeys(t, w, "code", "message")
	})

	t.Run("required_root_path_missing_returns_400", func(t *testing.T) {
		w := f.post(t, "/api/repos",
			map[string]string{"name": "x", "type": "private"}, nil)
		assertStatus(t, w, http.StatusBadRequest)
		hasKeys(t, w, "code", "message")
	})

	t.Run("required_type_missing_returns_400", func(t *testing.T) {
		w := f.post(t, "/api/repos",
			map[string]string{"name": "x", "root_path": f.workspaceRoot}, nil)
		assertStatus(t, w, http.StatusBadRequest)
		hasKeys(t, w, "code", "message")
	})

	t.Run("invalid_type_returns_400", func(t *testing.T) {
		w := f.post(t, "/api/repos",
			map[string]string{"name": "x", "type": "bad-type", "root_path": f.workspaceRoot}, nil)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("response_keys_on_201", func(t *testing.T) {
		body := map[string]string{
			"name":      "contract-repo-keys",
			"type":      "private",
			"root_path": f.workspaceRoot,
		}
		w := f.post(t, "/api/repos", body, nil)
		if w.Code != http.StatusCreated {
			t.Skipf("POST /api/repos returned %d; skipping key check", w.Code)
		}
		hasKeys(t, w, "id", "name", "type", "root_path", "status", "created_at", "updated_at")
	})

	t.Run("auth_open_no_bootstrap_needed", func(t *testing.T) {
		// POST /api/repos is NOT bootstrap-token–gated (FIX-SEC-07 only gates
		// /repos/create and /repos/register).
		w := f.post(t, "/api/repos", validBody, nil)
		if w.Code == http.StatusUnauthorized {
			t.Errorf("POST /api/repos must not require bootstrap token; got 401")
		}
	})
}

// ---------------------------------------------------------------------------
// 9. POST /api/repos/create — bootstrap-token gated
// ---------------------------------------------------------------------------

func TestContract_Repos_CreateWithInit(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable_returns_401_without_token", func(t *testing.T) {
		// 401 proves the route is registered AND auth middleware is active.
		w := f.post(t, "/api/repos/create",
			map[string]string{"name": "x", "path": f.workspaceRoot}, nil)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("requires_bootstrap_token", func(t *testing.T) {
		w := f.post(t, "/api/repos/create",
			map[string]string{"name": "x", "path": f.workspaceRoot}, nil)
		assertStatus(t, w, http.StatusUnauthorized)
		hasKeys(t, w, "code", "message")
	})

	t.Run("wrong_token_returns_401", func(t *testing.T) {
		wrong := strings.Repeat("0", 64)
		w := f.post(t, "/api/repos/create",
			map[string]string{"name": "x", "path": f.workspaceRoot},
			bearerHeader(wrong))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("required_name_missing_returns_400", func(t *testing.T) {
		w := f.post(t, "/api/repos/create",
			map[string]string{"path": f.workspaceRoot},
			bearerHeader(testBootstrapToken))
		assertStatus(t, w, http.StatusBadRequest)
		hasKeys(t, w, "code", "message")
	})

	t.Run("required_path_missing_returns_400", func(t *testing.T) {
		w := f.post(t, "/api/repos/create",
			map[string]string{"name": "x"},
			bearerHeader(testBootstrapToken))
		assertStatus(t, w, http.StatusBadRequest)
		hasKeys(t, w, "code", "message")
	})
}

// ---------------------------------------------------------------------------
// 10. POST /api/repos/register — bootstrap-token gated
// ---------------------------------------------------------------------------

func TestContract_Repos_Register(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable_returns_401_without_token", func(t *testing.T) {
		w := f.post(t, "/api/repos/register",
			map[string]string{"path": f.workspaceRoot}, nil)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("requires_bootstrap_token", func(t *testing.T) {
		w := f.post(t, "/api/repos/register",
			map[string]string{"path": f.workspaceRoot}, nil)
		assertStatus(t, w, http.StatusUnauthorized)
		hasKeys(t, w, "code", "message")
	})

	t.Run("wrong_token_returns_401", func(t *testing.T) {
		wrong := strings.Repeat("0", 64)
		w := f.post(t, "/api/repos/register",
			map[string]string{"path": f.workspaceRoot},
			bearerHeader(wrong))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("required_path_missing_returns_400", func(t *testing.T) {
		w := f.post(t, "/api/repos/register",
			map[string]string{},
			bearerHeader(testBootstrapToken))
		assertStatus(t, w, http.StatusBadRequest)
		hasKeys(t, w, "code", "message")
	})
}

// ---------------------------------------------------------------------------
// 11. GET /api/agent/list
// ---------------------------------------------------------------------------

func TestContract_Agent_List(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		w := f.get(t, "/api/agent/list", nil)
		if w.Code == http.StatusNotFound {
			t.Errorf("GET /api/agent/list: path not registered (got 404)")
		}
	})

	t.Run("method_GET_only", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/agent/list", http.MethodPost)
	})

	t.Run("response_is_json_array_when_200", func(t *testing.T) {
		w := f.get(t, "/api/agent/list", nil)
		if w.Code != http.StatusOK {
			t.Skipf("agent/list returned %d; skipping array check", w.Code)
		}
		var arr []json.RawMessage
		if err := json.NewDecoder(w.Body).Decode(&arr); err != nil {
			t.Fatalf("expected JSON array: %v", err)
		}
	})

	t.Run("auth_open_no_bootstrap_needed", func(t *testing.T) {
		w := f.get(t, "/api/agent/list", nil)
		if w.Code == http.StatusUnauthorized {
			t.Errorf("GET /api/agent/list must not require bootstrap token; got 401")
		}
	})
}

// ---------------------------------------------------------------------------
// 12. POST /api/agent/install
// ---------------------------------------------------------------------------

func TestContract_Agent_Install(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		// Unknown provider → 400, not 404.
		w := f.post(t, "/api/agent/install",
			map[string]string{"provider": "unknown-xyz"}, nil)
		if w.Code == http.StatusNotFound {
			t.Errorf("POST /api/agent/install: path not registered (got 404)")
		}
	})

	t.Run("method_POST_only", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/agent/install", http.MethodGet)
	})

	t.Run("unknown_provider_returns_400", func(t *testing.T) {
		w := f.post(t, "/api/agent/install",
			map[string]string{"provider": "bad-provider"}, nil)
		assertStatus(t, w, http.StatusBadRequest)
		hasKeys(t, w, "code", "message")
	})

	t.Run("empty_provider_returns_400", func(t *testing.T) {
		w := f.post(t, "/api/agent/install", map[string]string{}, nil)
		assertStatus(t, w, http.StatusBadRequest)
		hasKeys(t, w, "code", "message")
	})

	t.Run("auth_open_no_bootstrap_needed", func(t *testing.T) {
		w := f.post(t, "/api/agent/install",
			map[string]string{"provider": "bad"}, nil)
		if w.Code == http.StatusUnauthorized {
			t.Errorf("POST /api/agent/install must not require bootstrap token; got 401")
		}
	})
}

// ---------------------------------------------------------------------------
// 13. POST /api/agent/uninstall
// ---------------------------------------------------------------------------

func TestContract_Agent_Uninstall(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		w := f.post(t, "/api/agent/uninstall",
			map[string]string{"provider": "unknown-xyz"}, nil)
		if w.Code == http.StatusNotFound {
			t.Errorf("POST /api/agent/uninstall: path not registered (got 404)")
		}
	})

	t.Run("method_POST_only", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/agent/uninstall", http.MethodGet)
	})

	t.Run("unknown_provider_returns_400", func(t *testing.T) {
		w := f.post(t, "/api/agent/uninstall",
			map[string]string{"provider": "bad-provider"}, nil)
		assertStatus(t, w, http.StatusBadRequest)
		hasKeys(t, w, "code", "message")
	})

	t.Run("empty_provider_returns_400", func(t *testing.T) {
		w := f.post(t, "/api/agent/uninstall", map[string]string{}, nil)
		assertStatus(t, w, http.StatusBadRequest)
		hasKeys(t, w, "code", "message")
	})

	t.Run("auth_open_no_bootstrap_needed", func(t *testing.T) {
		w := f.post(t, "/api/agent/uninstall",
			map[string]string{"provider": "bad"}, nil)
		if w.Code == http.StatusUnauthorized {
			t.Errorf("POST /api/agent/uninstall must not require bootstrap token; got 401")
		}
	})
}

// ---------------------------------------------------------------------------
// 14. POST /api/onboarding/complete
// ---------------------------------------------------------------------------

func TestContract_Onboarding_Complete(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		w := f.post(t, "/api/onboarding/complete", nil, nil)
		assertStatus(t, w, http.StatusNoContent)
	})

	t.Run("method_POST_only", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/onboarding/complete", http.MethodGet)
	})

	t.Run("empty_body_accepted_returns_204", func(t *testing.T) {
		w := f.post(t, "/api/onboarding/complete", nil, nil)
		assertStatus(t, w, http.StatusNoContent)
	})

	t.Run("full_body_accepted_returns_204", func(t *testing.T) {
		body := map[string]interface{}{
			"skippedSteps":      []int{2, 3},
			"selectedProviders": []string{"claude"},
			"registeredRepos":   1,
		}
		w := f.post(t, "/api/onboarding/complete", body, nil)
		assertStatus(t, w, http.StatusNoContent)
	})

	t.Run("response_body_is_empty", func(t *testing.T) {
		w := f.post(t, "/api/onboarding/complete", nil, nil)
		assertStatus(t, w, http.StatusNoContent)
		if w.Body.Len() != 0 {
			t.Errorf("204 No Content must have empty body; got: %s", w.Body.String())
		}
	})

	t.Run("auth_open", func(t *testing.T) {
		w := f.post(t, "/api/onboarding/complete", nil, nil)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("POST /api/onboarding/complete must be open; got %d", w.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// 15. GET /api/settings
// ---------------------------------------------------------------------------

func TestContract_Settings_Get(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		w := f.get(t, "/api/settings", nil)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("method_GET_only", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/settings", http.MethodPost)
	})

	t.Run("response_is_json_object", func(t *testing.T) {
		w := f.get(t, "/api/settings", nil)
		assertStatus(t, w, http.StatusOK)
		var obj map[string]json.RawMessage
		if err := json.NewDecoder(w.Body).Decode(&obj); err != nil {
			t.Fatalf("expected JSON object: %v", err)
		}
	})

	t.Run("auth_open", func(t *testing.T) {
		w := f.get(t, "/api/settings", nil)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("GET /api/settings must be open; got %d", w.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// 16. PUT /api/settings
// ---------------------------------------------------------------------------

func TestContract_Settings_Put(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		w := f.put(t, "/api/settings", map[string]interface{}{"theme": "dark"})
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("method_PUT_not_DELETE", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/settings", http.MethodDelete)
	})

	t.Run("invalid_json_returns_400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/settings",
			strings.NewReader("not json at all"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", allowedOrigin)
		w := httptest.NewRecorder()
		f.mux.ServeHTTP(w, req)
		assertStatus(t, w, http.StatusBadRequest)
		hasKeys(t, w, "code", "message")
	})

	t.Run("response_is_merged_json_object", func(t *testing.T) {
		w := f.put(t, "/api/settings",
			map[string]interface{}{"editor": map[string]bool{"spellCheck": true}})
		assertStatus(t, w, http.StatusOK)
		var obj map[string]json.RawMessage
		if err := json.NewDecoder(w.Body).Decode(&obj); err != nil {
			t.Fatalf("expected JSON object: %v", err)
		}
	})

	t.Run("auth_open", func(t *testing.T) {
		w := f.put(t, "/api/settings", map[string]interface{}{})
		if w.Code == http.StatusUnauthorized {
			t.Errorf("PUT /api/settings must not require bootstrap token; got 401")
		}
	})
}

// ---------------------------------------------------------------------------
// 17. GET /api/graph
// ---------------------------------------------------------------------------

func TestContract_Graph(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		// With project param but no GraphStore → 503 (not 404).
		w := f.get(t, "/api/graph?project=testproj", nil)
		if w.Code == http.StatusNotFound {
			t.Errorf("GET /api/graph: path not registered (got 404)")
		}
	})

	t.Run("method_GET_only", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/graph", http.MethodPost)
	})

	t.Run("empty_project_aggregates_across_projects", func(t *testing.T) {
		// Empty project param aggregates across all registered projects.
		// Fixture has no GraphStore, so this surfaces as 503 (same as
		// the single-project path below), not 400.
		w := f.get(t, "/api/graph", nil)
		if w.Code == http.StatusBadRequest {
			t.Errorf("GET /api/graph without project param must aggregate, not 400")
		}
	})

	t.Run("no_graph_store_returns_503", func(t *testing.T) {
		// Fixture does not inject a GraphStore → handler returns 503.
		w := f.get(t, "/api/graph?project=x", nil)
		assertStatus(t, w, http.StatusServiceUnavailable)
	})

	t.Run("auth_open", func(t *testing.T) {
		w := f.get(t, "/api/graph?project=x", nil)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("GET /api/graph must be open; got %d", w.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// 18. GET /api/analytics/summary
// ---------------------------------------------------------------------------

func TestContract_Analytics_Summary(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		w := f.get(t, "/api/analytics/summary", nil)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("method_GET_only", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/analytics/summary", http.MethodPost)
	})

	t.Run("response_keys_stable", func(t *testing.T) {
		w := f.get(t, "/api/analytics/summary", nil)
		assertStatus(t, w, http.StatusOK)
		hasKeys(t, w,
			"total_docs",
			"docs_last_7_days",
			"docs_last_30_days",
			"agent_triggered_last_7_days",
			"agent_triggered_last_30_days",
			"change_velocity_7d",
			"change_velocity_30d",
			"docs_per_project",
			"pipeline_ready",
		)
	})

	t.Run("auth_open", func(t *testing.T) {
		w := f.get(t, "/api/analytics/summary", nil)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("GET /api/analytics/summary must be open; got %d", w.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// 19. GET /api/preview
// ---------------------------------------------------------------------------

func TestContract_Preview(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		// Missing url param → 400, not 404.
		w := f.get(t, "/api/preview", nil)
		if w.Code == http.StatusNotFound {
			t.Errorf("GET /api/preview: path not registered (got 404)")
		}
	})

	t.Run("method_GET_only", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/preview", http.MethodPost)
	})

	t.Run("url_param_required_returns_400", func(t *testing.T) {
		w := f.get(t, "/api/preview", nil)
		assertStatus(t, w, http.StatusBadRequest)
		hasKeys(t, w, "code", "message")
	})

	t.Run("invalid_scheme_returns_422", func(t *testing.T) {
		w := f.get(t, "/api/preview?url=http://not-vedox/file/foo.go", nil)
		assertStatus(t, w, http.StatusUnprocessableEntity)
		hasKeys(t, w, "code", "message")
	})

	t.Run("response_keys_on_valid_file", func(t *testing.T) {
		// Create a real file in the workspace so the handler can read it.
		if err := createFile(f.workspaceRoot+"/hello.go", "package main\n"); err != nil {
			t.Fatalf("createFile: %v", err)
		}
		w := f.get(t, "/api/preview?url=vedox://file/hello.go", nil)
		if w.Code != http.StatusOK {
			t.Skipf("preview returned %d; skipping key check\nbody: %s", w.Code, w.Body.String())
		}
		hasKeys(t, w, "file_path", "language", "content", "start_line", "end_line", "total_lines", "truncated")
	})

	t.Run("auth_open", func(t *testing.T) {
		w := f.get(t, "/api/preview", nil)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("GET /api/preview must be open; got %d", w.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// 20. GET /api/projects/{project}/git/status
// ---------------------------------------------------------------------------

func TestContract_GitStatus(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		w := f.get(t, "/api/projects/myproj/git/status", nil)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("method_GET_only", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/projects/myproj/git/status", http.MethodPost)
	})

	t.Run("response_keys_stable", func(t *testing.T) {
		w := f.get(t, "/api/projects/myproj/git/status", nil)
		assertStatus(t, w, http.StatusOK)
		hasKeys(t, w, "branch", "dirty", "ahead", "behind")
	})

	t.Run("auth_open", func(t *testing.T) {
		w := f.get(t, "/api/projects/myproj/git/status", nil)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("GET /api/projects/{p}/git/status must be open; got %d", w.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// 21. GET /api/projects/{project}/docs/.../history
// ---------------------------------------------------------------------------

func TestContract_DocHistory(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable", func(t *testing.T) {
		// Path ending in /history dispatches to handleDocHistory.
		// Without a real doc it returns 400 or 500 — but NOT 404.
		w := f.get(t, "/api/projects/myproj/docs/readme.md/history", nil)
		if w.Code == http.StatusNotFound {
			t.Errorf("GET ...docs/.../history: path not registered (got 404)")
		}
	})

	t.Run("method_GET_only", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/projects/myproj/docs/readme.md/history",
			http.MethodPost)
	})

	t.Run("response_keys_when_doc_exists", func(t *testing.T) {
		projDir := f.workspaceRoot + "/myproj"
		if err := createDir(projDir); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := createFile(projDir+"/readme.md", "# hello\n"); err != nil {
			t.Fatalf("createFile: %v", err)
		}
		w := f.get(t, "/api/projects/myproj/docs/readme.md/history", nil)
		if w.Code != http.StatusOK {
			t.Skipf("history returned %d; skipping key check", w.Code)
		}
		hasKeys(t, w, "docPath", "entries")
	})

	t.Run("auth_open", func(t *testing.T) {
		w := f.get(t, "/api/projects/myproj/docs/readme.md/history", nil)
		if w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden {
			t.Errorf("GET ...docs/.../history must be open; got %d", w.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// 22. GET /api/browse — bootstrap-token gated
// ---------------------------------------------------------------------------

func TestContract_Browse(t *testing.T) {
	f := newFixture(t)

	t.Run("path_stable_returns_401_without_token", func(t *testing.T) {
		// 401 = route registered + middleware active.
		w := f.get(t, "/api/browse", nil)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("method_GET_only", func(t *testing.T) {
		f.assertWrongMethod(t, "/api/browse", http.MethodPost)
	})

	t.Run("requires_bootstrap_token", func(t *testing.T) {
		w := f.get(t, "/api/browse", nil)
		assertStatus(t, w, http.StatusUnauthorized)
		hasKeys(t, w, "code", "message")
	})

	t.Run("wrong_token_returns_401", func(t *testing.T) {
		wrong := strings.Repeat("0", 64)
		w := f.get(t, "/api/browse", bearerHeader(wrong))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("valid_token_passes_middleware", func(t *testing.T) {
		// Workspace root is inside the temp homeDirOverride, so the handler
		// boundary check passes. Accept 200 or 403 (path-outside-home on CI).
		// The invariant is: valid token must NOT return 401.
		w := f.get(t, "/api/browse?path="+f.workspaceRoot,
			bearerHeader(testBootstrapToken))
		if w.Code == http.StatusUnauthorized {
			t.Errorf("valid token must pass middleware; got 401")
		}
	})

	t.Run("response_keys_on_200", func(t *testing.T) {
		w := f.get(t, "/api/browse?path="+f.workspaceRoot,
			bearerHeader(testBootstrapToken))
		if w.Code != http.StatusOK {
			t.Skipf("browse returned %d; skipping key check", w.Code)
		}
		hasKeys(t, w, "path", "parent", "directories")
	})
}

// ---------------------------------------------------------------------------
// 23. POST /api/voice/ptt — only registered when VoiceServer is injected
// ---------------------------------------------------------------------------

func TestContract_Voice_PTT(t *testing.T) {
	f := newFixture(t)

	t.Run("absent_without_voice_server", func(t *testing.T) {
		// When SetVoiceServer is not called, Mount does NOT register voice routes.
		// The contract: this path MUST NOT return 200 when no VoiceServer is wired.
		w := f.post(t, "/api/voice/ptt", nil, nil)
		if w.Code == http.StatusOK {
			t.Errorf("POST /api/voice/ptt must not return 200 without a VoiceServer")
		}
	})
}

// ---------------------------------------------------------------------------
// 24. GET /api/voice/status — only registered when VoiceServer is injected
// ---------------------------------------------------------------------------

func TestContract_Voice_Status(t *testing.T) {
	f := newFixture(t)

	t.Run("absent_without_voice_server", func(t *testing.T) {
		w := f.get(t, "/api/voice/status", nil)
		if w.Code == http.StatusOK {
			t.Errorf("GET /api/voice/status must not return 200 without a VoiceServer")
		}
	})
}

// ---------------------------------------------------------------------------
// Error body shape — cross-cutting contract
// ---------------------------------------------------------------------------

// TestContract_ErrorShape verifies that every 4xx/5xx response uses the
// canonical {"code":"VDX-xxx","message":"..."} shape, not bare strings or
// ad-hoc objects.
func TestContract_ErrorShape(t *testing.T) {
	f := newFixture(t)

	type errCase struct {
		name    string
		trigger func() *httptest.ResponseRecorder
	}

	cases := []errCase{
		{
			name: "projects_empty_name",
			trigger: func() *httptest.ResponseRecorder {
				return f.post(t, "/api/projects", map[string]string{}, nil)
			},
		},
		{
			name: "repos_missing_name",
			trigger: func() *httptest.ResponseRecorder {
				return f.post(t, "/api/repos",
					map[string]string{"type": "private", "root_path": f.workspaceRoot}, nil)
			},
		},
		{
			name: "browse_no_token",
			trigger: func() *httptest.ResponseRecorder {
				return f.get(t, "/api/browse", nil)
			},
		},
		{
			name: "repos_create_no_token",
			trigger: func() *httptest.ResponseRecorder {
				return f.post(t, "/api/repos/create",
					map[string]string{"name": "x"}, nil)
			},
		},
		{
			name: "repos_register_no_token",
			trigger: func() *httptest.ResponseRecorder {
				return f.post(t, "/api/repos/register",
					map[string]string{}, nil)
			},
		},
		{
			name: "graph_missing_project_param",
			trigger: func() *httptest.ResponseRecorder {
				return f.get(t, "/api/graph", nil)
			},
		},
		{
			name: "preview_missing_url",
			trigger: func() *httptest.ResponseRecorder {
				return f.get(t, "/api/preview", nil)
			},
		},
		{
			name: "settings_invalid_json",
			trigger: func() *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodPut, "/api/settings",
					strings.NewReader("bad json"))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Origin", allowedOrigin)
				w := httptest.NewRecorder()
				f.mux.ServeHTTP(w, req)
				return w
			},
		},
		{
			name: "scan_unknown_job_id",
			trigger: func() *httptest.ResponseRecorder {
				return f.get(t, "/api/scan/no-such-job-id", nil)
			},
		},
		{
			name: "agent_install_bad_provider",
			trigger: func() *httptest.ResponseRecorder {
				return f.post(t, "/api/agent/install",
					map[string]string{"provider": "bad"}, nil)
			},
		},
		{
			name: "agent_uninstall_bad_provider",
			trigger: func() *httptest.ResponseRecorder {
				return f.post(t, "/api/agent/uninstall",
					map[string]string{"provider": "bad"}, nil)
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			w := tc.trigger()
			if w.Code < 400 {
				return // Not an error — skip shape check.
			}
			body := w.Body.String()
			var errBody struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal([]byte(body), &errBody); err != nil {
				t.Errorf("%s: error response is not canonical JSON {code,message}: %v\nbody: %s",
					tc.name, err, body)
				return
			}
			if errBody.Code == "" {
				t.Errorf("%s: error response missing 'code' field\nbody: %s", tc.name, body)
			}
			if errBody.Message == "" {
				t.Errorf("%s: error response missing 'message' field\nbody: %s", tc.name, body)
			}
		})
	}
}
