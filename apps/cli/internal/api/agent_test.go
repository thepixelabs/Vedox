package api

// Tests for the Doc Agent HTTP endpoints:
//
//	POST /api/agent/install   — handleAgentInstall
//	POST /api/agent/uninstall — handleAgentUninstall
//	GET  /api/agent/list      — handleAgentList
//
// Two layers of coverage:
//
//  1. Direct error-path tests for handleAgentInstall / handleAgentUninstall
//     using the existing real adapters (503 + malformed-input cases). These
//     need no installer seam — they reject the request before the adapter
//     constructor runs.
//
//  2. Behavioural tests that drive the handler down the Probe → Plan →
//     Install → Save path using a stub ProviderInstaller injected through
//     the Server.SetInstallerFactoryOverride seam. The stub never touches
//     the OS keychain or the user's real ~/.claude directory, so these tests
//     are deterministic on every platform and CI runner.
//
// End-to-end install/uninstall against the real Claude/Codex/Copilot/Gemini
// adapters lives in apps/cli/internal/providers/*_test.go and the CLI
// integration tests in cmd/.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/providers"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// ---------------------------------------------------------------------------
// Fixture
// ---------------------------------------------------------------------------

// agentFixture builds a test server. keyStore controls whether the agent
// handlers are enabled (nil = 503, non-nil = enabled). When stub is non-nil,
// every call to s.buildInstaller dispatches through stub instead of building
// a real provider adapter — see TestAgent_RoundTrip for the round-trip flow.
type agentFixture struct {
	server  *httptest.Server
	srv     *Server
	homeDir string
	stub    *stubInstaller
}

func newAgentFixture(t *testing.T, keyStore providers.KeyIssuer) *agentFixture {
	t.Helper()
	return newAgentFixtureWithStub(t, keyStore, nil)
}

// newAgentFixtureWithStub is the richer constructor used by behavioural tests.
// Pass a configured stubInstaller and the fixture wires it into the server's
// installerFactoryOverride. The same stub instance is returned via the
// fixture so tests can read recorded calls and toggle errors mid-test.
func newAgentFixtureWithStub(t *testing.T, keyStore providers.KeyIssuer, stub *stubInstaller) *agentFixture {
	t.Helper()

	wsRoot := t.TempDir()
	adapter, err := store.NewLocalAdapter(wsRoot, nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}
	wsDB, err := db.Open(db.Options{WorkspaceRoot: wsRoot})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = wsDB.Close() })

	srv := NewServer(adapter, wsDB, wsRoot, scanner.NewJobStore(), ai.NewJobStore(3), store.NewProjectRegistry(), agentauth.PassthroughAuth())
	if keyStore != nil {
		srv.SetKeyStore(keyStore)
	}
	// Redirect userHome to TempDir so any accidental receipt-store access
	// stays isolated from the developer's real ~/.vedox directory.
	srv.SetHomeDirOverride(wsRoot)

	if stub != nil {
		// Build a real ReceiptStore rooted under the same override home so
		// that handleAgentList (which constructs its own ReceiptStore from
		// s.userHome()) sees receipts the install handler wrote.
		recStore, err := providers.NewReceiptStore(filepath.Join(wsRoot, ".vedox"))
		if err != nil {
			t.Fatalf("NewReceiptStore: %v", err)
		}
		stub.receiptStore = recStore
		srv.SetInstallerFactoryOverride(stub.factory)
	}

	mux := http.NewServeMux()
	srv.Mount(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return &agentFixture{
		server:  ts,
		srv:     srv,
		homeDir: wsRoot,
		stub:    stub,
	}
}

// post issues a JSON POST to the fixture server.
func (f *agentFixture) post(t *testing.T, path string, body interface{}) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, f.server.URL+path, strings.NewReader(string(b)))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// get issues a GET to the fixture server.
func (f *agentFixture) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := f.server.Client().Get(f.server.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// decodeError parses a writeError JSON body into a code/message pair.
func decodeError(t *testing.T, body io.Reader) errorResponse {
	t.Helper()
	var er errorResponse
	if err := json.NewDecoder(body).Decode(&er); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return er
}

// ---------------------------------------------------------------------------
// mockKeyIssuer — satisfies providers.KeyIssuer without touching the keychain.
// ---------------------------------------------------------------------------

type mockKeyIssuer struct{}

func (m *mockKeyIssuer) IssueKey(_, _, _ string) (string, string, error) {
	return "mock-key-id", "mock-secret", nil
}
func (m *mockKeyIssuer) RevokeKey(_ string) error { return nil }

// ---------------------------------------------------------------------------
// stubInstaller — controllable ProviderInstaller for behavioural tests.
//
// Each method consults the matching *Err field; if non-nil, it returns that
// error untouched. Otherwise it returns the stub's pre-configured success
// payload. Calls are recorded in the *Calls counters so tests can assert
// the handler followed the documented Probe → Plan → Install ordering.
//
// The stub is paired with a real *providers.ReceiptStore so the install
// handler can persist a receipt to disk (verified by the round-trip and
// 0o600-mode tests). The factory method satisfies the signature expected
// by Server.installerFactoryOverride.
// ---------------------------------------------------------------------------

type stubInstaller struct {
	// success payloads (used when the matching *Err is nil).
	probe   providers.ProbeResult
	plan    providers.InstallPlan
	receipt providers.InstallReceipt

	// forced errors (any non-nil short-circuits the matching method).
	probeErr     error
	planErr      error
	installErr   error
	uninstallErr error

	// observability — call counters incremented before any error is returned.
	probeCalls     int
	planCalls      int
	installCalls   int
	uninstallCalls int

	// receiptStore is wired by newAgentFixtureWithStub. Tests do not set it.
	receiptStore *providers.ReceiptStore
}

// factory matches the signature expected by SetInstallerFactoryOverride.
// It always returns the same stub so tests can read recorded state from a
// single instance regardless of how many times the handler called it.
func (s *stubInstaller) factory(_ string) (providers.ProviderInstaller, *providers.ReceiptStore, error) {
	return s, s.receiptStore, nil
}

func (s *stubInstaller) Probe(_ context.Context) (*providers.ProbeResult, error) {
	s.probeCalls++
	if s.probeErr != nil {
		return nil, s.probeErr
	}
	p := s.probe
	return &p, nil
}

func (s *stubInstaller) Plan(_ context.Context) (*providers.InstallPlan, error) {
	s.planCalls++
	if s.planErr != nil {
		return nil, s.planErr
	}
	p := s.plan
	return &p, nil
}

func (s *stubInstaller) Install(_ context.Context, _ *providers.InstallPlan) (*providers.InstallReceipt, error) {
	s.installCalls++
	if s.installErr != nil {
		return nil, s.installErr
	}
	r := s.receipt
	return &r, nil
}

func (s *stubInstaller) Uninstall(_ context.Context) error {
	s.uninstallCalls++
	if s.uninstallErr != nil {
		return s.uninstallErr
	}
	// Mirror the real Claude adapter: remove the receipt so handleAgentList
	// reflects the uninstall on the next call.
	if s.receiptStore != nil {
		_ = s.receiptStore.Delete(s.receipt.Provider)
	}
	return nil
}

// Repair and Verify are not exercised by the HTTP handlers under test; they
// satisfy the interface contract.
func (s *stubInstaller) Repair(_ context.Context) error { return nil }
func (s *stubInstaller) Verify(_ context.Context, _ *providers.InstallReceipt) (*providers.VerifyResult, error) {
	return &providers.VerifyResult{Healthy: true}, nil
}

// newClaudeStub returns a stubInstaller pre-configured with a realistic
// Claude install receipt. Tests tweak the *Err fields to drive specific paths.
func newClaudeStub() *stubInstaller {
	return &stubInstaller{
		probe: providers.ProbeResult{Installed: false},
		plan: providers.InstallPlan{
			Provider: providers.ProviderClaude,
			PlanHash: "stub-plan-hash",
		},
		receipt: providers.InstallReceipt{
			Provider:    providers.ProviderClaude,
			Version:     "2.0",
			AuthKeyID:   "stub-key-id-abc",
			DaemonURL:   "http://127.0.0.1:5150",
			SchemaHash:  "stub-schema-hash",
			FileHashes:  map[string]string{"/tmp/file-a": "h1", "/tmp/file-b": "h2"},
			InstalledAt: time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		},
	}
}

// ---------------------------------------------------------------------------
// POST /api/agent/install — 503 path (no keyStore)
// ---------------------------------------------------------------------------

// TestAgentInstall_NoKeyStore returns 503 when the server has no keyStore.
func TestAgentInstall_NoKeyStore(t *testing.T) {
	f := newAgentFixture(t, nil)

	resp := f.post(t, "/api/agent/install", map[string]string{"provider": "claude"})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// POST /api/agent/install — 400 paths
// ---------------------------------------------------------------------------

// TestAgentInstall_UnknownProvider returns 400 with VDX-400 for an unknown
// provider id.
func TestAgentInstall_UnknownProvider(t *testing.T) {
	f := newAgentFixture(t, &mockKeyIssuer{})

	resp := f.post(t, "/api/agent/install", map[string]string{"provider": "openai-gpt-99"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	er := decodeError(t, resp.Body)
	if er.Code != "VDX-400" {
		t.Errorf("code = %q, want VDX-400", er.Code)
	}
	if !strings.Contains(er.Message, "openai-gpt-99") {
		t.Errorf("message %q should echo the rejected provider", er.Message)
	}
}

// TestAgentInstall_EmptyProvider returns 400.
func TestAgentInstall_EmptyProvider(t *testing.T) {
	f := newAgentFixture(t, &mockKeyIssuer{})

	resp := f.post(t, "/api/agent/install", map[string]string{"provider": ""})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestAgentInstall_InvalidJSON returns 400.
func TestAgentInstall_InvalidJSON(t *testing.T) {
	f := newAgentFixture(t, &mockKeyIssuer{})

	req, _ := http.NewRequest(http.MethodPost, f.server.URL+"/api/agent/install",
		strings.NewReader("{not json}"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// POST /api/agent/install — installer error paths (Probe/Plan/Install)
// ---------------------------------------------------------------------------

// TestAgentInstall_InstallerInstallError returns 500 with VDX-500 when the
// stub installer's Install() reports an error. Probe and Plan must have run
// first per the documented Probe → Plan → Install order.
func TestAgentInstall_InstallerInstallError(t *testing.T) {
	stub := newClaudeStub()
	stub.installErr = errors.New("disk full while writing agent file")
	f := newAgentFixtureWithStub(t, &mockKeyIssuer{}, stub)

	resp := f.post(t, "/api/agent/install", map[string]string{"provider": "claude"})
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	er := decodeError(t, resp.Body)
	if er.Code != "VDX-500" {
		t.Errorf("code = %q, want VDX-500", er.Code)
	}
	if !strings.Contains(er.Message, "install failed") {
		t.Errorf("message %q should describe the failing stage", er.Message)
	}
	if !strings.Contains(er.Message, "disk full") {
		t.Errorf("message %q should propagate the underlying installer error", er.Message)
	}
	if stub.probeCalls != 1 || stub.planCalls != 1 || stub.installCalls != 1 {
		t.Errorf("unexpected call ordering: probe=%d plan=%d install=%d",
			stub.probeCalls, stub.planCalls, stub.installCalls)
	}
}

// TestAgentInstall_AlreadyInstalled returns 409 when Probe reports an
// existing install. Plan and Install must NOT run — the handler short-circuits.
func TestAgentInstall_AlreadyInstalled(t *testing.T) {
	stub := newClaudeStub()
	stub.probe.Installed = true
	f := newAgentFixtureWithStub(t, &mockKeyIssuer{}, stub)

	resp := f.post(t, "/api/agent/install", map[string]string{"provider": "claude"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	er := decodeError(t, resp.Body)
	if er.Code != "VDX-409" {
		t.Errorf("code = %q, want VDX-409", er.Code)
	}
	if stub.planCalls != 0 || stub.installCalls != 0 {
		t.Errorf("plan/install must not run after Probe says Installed=true: plan=%d install=%d",
			stub.planCalls, stub.installCalls)
	}
}

// TestAgentInstall_ReceiptSaveFailureStillReturns201 documents the
// "install succeeded, save best-effort" contract: when Save fails the
// handler logs and still returns 201. The receipt-save error must NOT be
// surfaced as a 500.
//
// We trigger a Save failure by planting a regular file at the path that the
// ReceiptStore expects to be a directory. os.MkdirAll then fails with
// ENOTDIR on Unix, the handler logs and continues to the 201 response.
func TestAgentInstall_ReceiptSaveFailureStillReturns201(t *testing.T) {
	stub := newClaudeStub()
	f := newAgentFixtureWithStub(t, &mockKeyIssuer{}, stub)

	receiptsDir := filepath.Join(f.homeDir, ".vedox", "install-receipts")
	if err := os.MkdirAll(filepath.Dir(receiptsDir), 0o700); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(receiptsDir, []byte("blocker"), 0o600); err != nil {
		t.Fatalf("plant blocker file: %v", err)
	}

	resp := f.post(t, "/api/agent/install", map[string]string{"provider": "claude"})
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 201 (Save failure must be best-effort); body=%s",
			resp.StatusCode, string(body))
	}
	// Confirm the blocker is still a regular file — Save was expected to fail.
	info, err := os.Lstat(receiptsDir)
	if err != nil {
		t.Fatalf("lstat receiptsDir: %v", err)
	}
	if info.IsDir() {
		t.Errorf("blocker was overwritten — Save was expected to fail, not succeed")
	}
}

// ---------------------------------------------------------------------------
// POST /api/agent/install — happy path with stub installer
// ---------------------------------------------------------------------------

// TestAgentInstall_HappyPath drives the handler through Probe → Plan →
// Install → Save → 201 and asserts the response body matches the receipt.
// The agentReceiptResponse must NEVER expose the secret material — only
// the AuthKeyID and a FileCount.
func TestAgentInstall_HappyPath(t *testing.T) {
	stub := newClaudeStub()
	f := newAgentFixtureWithStub(t, &mockKeyIssuer{}, stub)

	resp := f.post(t, "/api/agent/install", map[string]string{"provider": "claude"})
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 201; body=%s", resp.StatusCode, string(body))
	}

	var got agentReceiptResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Provider != "claude" {
		t.Errorf("Provider = %q, want claude", got.Provider)
	}
	if got.Version != "2.0" {
		t.Errorf("Version = %q, want 2.0", got.Version)
	}
	if got.AuthKeyID != "stub-key-id-abc" {
		t.Errorf("AuthKeyID = %q, want stub-key-id-abc", got.AuthKeyID)
	}
	if got.FileCount != len(stub.receipt.FileHashes) {
		t.Errorf("FileCount = %d, want %d", got.FileCount, len(stub.receipt.FileHashes))
	}
	// MED-05 guard: the response must not leak file paths or hash material.
	body, _ := json.Marshal(got)
	if strings.Contains(string(body), "fileHashes") || strings.Contains(string(body), "/tmp/file-a") {
		t.Errorf("response leaks raw file paths: %s", string(body))
	}
}

// ---------------------------------------------------------------------------
// POST /api/agent/uninstall — installer error path
// ---------------------------------------------------------------------------

// TestAgentUninstall_InstallerError returns 500 with VDX-500 when the
// stub installer's Uninstall reports an error.
func TestAgentUninstall_InstallerError(t *testing.T) {
	stub := newClaudeStub()
	stub.uninstallErr = errors.New("permission denied stripping CLAUDE.md")
	f := newAgentFixtureWithStub(t, &mockKeyIssuer{}, stub)

	resp := f.post(t, "/api/agent/uninstall", map[string]string{"provider": "claude"})
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	er := decodeError(t, resp.Body)
	if er.Code != "VDX-500" {
		t.Errorf("code = %q, want VDX-500", er.Code)
	}
	if !strings.Contains(er.Message, "permission denied") {
		t.Errorf("message %q should propagate the underlying error", er.Message)
	}
}

// ---------------------------------------------------------------------------
// POST /api/agent/uninstall — 503 path
// ---------------------------------------------------------------------------

func TestAgentUninstall_NoKeyStore(t *testing.T) {
	f := newAgentFixture(t, nil)

	resp := f.post(t, "/api/agent/uninstall", map[string]string{"provider": "claude"})
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

// TestAgentUninstall_UnknownProvider returns 400.
func TestAgentUninstall_UnknownProvider(t *testing.T) {
	f := newAgentFixture(t, &mockKeyIssuer{})

	resp := f.post(t, "/api/agent/uninstall", map[string]string{"provider": "unknown-bot"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// GET /api/agent/list
// ---------------------------------------------------------------------------

// TestAgentList_Empty returns [] when no receipts are on disk.
// The server's homeDirOverride points to a fresh TempDir so there are
// guaranteed to be no receipt files.
func TestAgentList_Empty(t *testing.T) {
	f := newAgentFixture(t, nil) // keyStore not required for list

	resp := f.get(t, "/api/agent/list")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got []agentListItem
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got == nil {
		t.Error("body must be [] not null for empty list")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 items, got %d", len(got))
	}
}

// ---------------------------------------------------------------------------
// Integration: install → list → uninstall → list round-trip
// ---------------------------------------------------------------------------

// TestAgent_RoundTrip_InstallListUninstall covers the full lifecycle a UI
// flow exercises:
//
//  1. POST /api/agent/install with a stub installer → 201, receipt persisted.
//  2. GET  /api/agent/list                          → 200, claude is listed.
//  3. POST /api/agent/uninstall                     → 200, receipt deleted.
//  4. GET  /api/agent/list                          → 200, list is empty.
//
// It verifies that the same ReceiptStore (located via homeDirOverride) is
// observed by both the install handler (which writes) and the list handler
// (which reads via a fresh ReceiptStore each call).
func TestAgent_RoundTrip_InstallListUninstall(t *testing.T) {
	stub := newClaudeStub()
	f := newAgentFixtureWithStub(t, &mockKeyIssuer{}, stub)

	// 1. Install.
	if resp := f.post(t, "/api/agent/install", map[string]string{"provider": "claude"}); resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("install status = %d, want 201; body=%s", resp.StatusCode, string(body))
	}

	// 2. List shows the new install.
	resp := f.get(t, "/api/agent/list")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status after install = %d, want 200", resp.StatusCode)
	}
	var listed []agentListItem
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("list returned %d items after install, want 1: %+v", len(listed), listed)
	}
	if listed[0].Provider != "claude" {
		t.Errorf("listed provider = %q, want claude", listed[0].Provider)
	}
	if listed[0].AuthKeyID != "stub-key-id-abc" {
		t.Errorf("listed AuthKeyID = %q, want stub-key-id-abc", listed[0].AuthKeyID)
	}

	// 3. Uninstall.
	if resp := f.post(t, "/api/agent/uninstall", map[string]string{"provider": "claude"}); resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("uninstall status = %d, want 200; body=%s", resp.StatusCode, string(body))
	}

	// 4. List is empty again.
	resp = f.get(t, "/api/agent/list")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status after uninstall = %d, want 200", resp.StatusCode)
	}
	var afterUninstall []agentListItem
	if err := json.NewDecoder(resp.Body).Decode(&afterUninstall); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(afterUninstall) != 0 {
		t.Errorf("list still has %d items after uninstall: %+v", len(afterUninstall), afterUninstall)
	}
}

// TestAgent_ReceiptFile_WrittenAt0600 verifies the on-disk receipt's mode.
// The receipt must be readable only by the owning user — it contains the
// HMAC key ID and managed file paths, and is written under ~/.vedox.
//
// Skipped on Windows where Unix permission semantics do not apply.
func TestAgent_ReceiptFile_WrittenAt0600(t *testing.T) {
	stub := newClaudeStub()
	f := newAgentFixtureWithStub(t, &mockKeyIssuer{}, stub)

	if resp := f.post(t, "/api/agent/install", map[string]string{"provider": "claude"}); resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("install status = %d, want 201; body=%s", resp.StatusCode, string(body))
	}

	receiptPath := filepath.Join(f.homeDir, ".vedox", "install-receipts", "claude.json")
	info, err := os.Stat(receiptPath)
	if err != nil {
		t.Fatalf("stat receipt: %v", err)
	}
	if isWindows() {
		t.Skip("Unix permission semantics not applicable on Windows")
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("receipt mode = %#o, want 0600 (owner-only read/write)", mode)
	}

	data, err := os.ReadFile(receiptPath)
	if err != nil {
		t.Fatalf("read receipt: %v", err)
	}
	if !strings.Contains(string(data), `"provider": "claude"`) {
		t.Errorf("receipt JSON missing provider field; got: %s", string(data))
	}
	if !strings.Contains(string(data), "stub-key-id-abc") {
		t.Errorf("receipt JSON missing AuthKeyID; got: %s", string(data))
	}
	// The receipt MUST NOT carry secret material.
	if strings.Contains(string(data), "mock-secret") {
		t.Errorf("receipt leaks secret value: %s", string(data))
	}
}

// isWindows is a tiny portability helper used by the 0o600 receipt test.
// It avoids importing runtime at the top of the file just for one constant.
func isWindows() bool {
	return os.PathSeparator == '\\'
}
