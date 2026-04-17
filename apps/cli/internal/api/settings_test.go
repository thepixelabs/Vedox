package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/vedox/vedox/internal/agentauth"
	"github.com/vedox/vedox/internal/ai"
	"github.com/vedox/vedox/internal/db"
	"github.com/vedox/vedox/internal/scanner"
	"github.com/vedox/vedox/internal/store"
)

// settingsFixture builds a test HTTP server whose home directory is redirected
// to a temp directory so tests never touch ~/.vedox.
type settingsFixture struct {
	server  *httptest.Server
	homeDir string
	srv     *Server
}

func newSettingsFixture(t *testing.T) *settingsFixture {
	t.Helper()

	raw := t.TempDir()
	resolved, err := filepath.EvalSymlinks(raw)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	// Separate temp dir for the fake home directory.
	home := t.TempDir()
	homeResolved, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatalf("EvalSymlinks home: %v", err)
	}

	adapter, err := store.NewLocalAdapter(resolved, nil)
	if err != nil {
		t.Fatalf("NewLocalAdapter: %v", err)
	}

	dbStore, err := db.Open(db.Options{WorkspaceRoot: resolved})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = dbStore.Close() })

	apiSrv := NewServer(
		adapter,
		dbStore,
		resolved,
		scanner.NewJobStore(),
		ai.NewJobStore(3),
		store.NewProjectRegistry(),
		agentauth.PassthroughAuth(),
	)
	apiSrv.SetHomeDirOverride(homeResolved)

	mux := http.NewServeMux()
	apiSrv.Mount(mux)

	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	return &settingsFixture{
		server:  ts,
		homeDir: homeResolved,
		srv:     apiSrv,
	}
}

// prefsPath returns the path to user-prefs.json within the fixture's fake home.
func (f *settingsFixture) prefsPath() string {
	return filepath.Join(f.homeDir, vedoxDirName, userPrefsFile)
}

// get issues GET /api/settings.
func (f *settingsFixture) getSettings(t *testing.T) *http.Response {
	t.Helper()
	resp, err := f.server.Client().Get(f.server.URL + "/api/settings")
	if err != nil {
		t.Fatalf("GET /api/settings: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// put issues PUT /api/settings with the given body.
// The Origin header is set to the CORS-allowlisted value so the CSRF middleware
// does not reject the request. In production the SvelteKit dev server sends
// this header automatically from http://localhost:5151.
func (f *settingsFixture) putSettings(t *testing.T, body interface{}) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPut, f.server.URL+"/api/settings", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("NewRequest PUT /api/settings: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("PUT /api/settings: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// decodeJSON decodes the response body into v. Rewinds via bodyStr if needed.
func decodeSettingsJSON(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GET /api/settings
// ---------------------------------------------------------------------------

// TestGetSettings_NoFile returns {} (empty object) with 200 when the
// prefs file does not exist yet.
func TestGetSettings_NoFile(t *testing.T) {
	f := newSettingsFixture(t)

	resp := f.getSettings(t)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got map[string]interface{}
	decodeSettingsJSON(t, resp, &got)
	if len(got) != 0 {
		t.Errorf("expected empty object, got %v", got)
	}
}

// TestGetSettings_ExistingFile returns the stored JSON when the file exists.
func TestGetSettings_ExistingFile(t *testing.T) {
	f := newSettingsFixture(t)

	// Pre-write a prefs file.
	dir := filepath.Join(f.homeDir, vedoxDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := map[string]interface{}{
		"appearance": map[string]interface{}{"theme": "ember"},
		"editor":     map[string]interface{}{"spellCheck": true},
	}
	data, _ := json.Marshal(want)
	if err := os.WriteFile(f.prefsPath(), data, 0o600); err != nil {
		t.Fatalf("write prefs: %v", err)
	}

	resp := f.getSettings(t)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got map[string]interface{}
	decodeSettingsJSON(t, resp, &got)

	appearance, ok := got["appearance"].(map[string]interface{})
	if !ok {
		t.Fatalf("appearance key missing or wrong type: %v", got)
	}
	if appearance["theme"] != "ember" {
		t.Errorf("theme = %v, want ember", appearance["theme"])
	}
}

// TestGetSettings_MalformedFile returns {} (empty object, not an error) when
// the prefs file contains invalid JSON. This prevents the UI from breaking
// due to a corrupted file.
func TestGetSettings_MalformedFile(t *testing.T) {
	f := newSettingsFixture(t)

	dir := filepath.Join(f.homeDir, vedoxDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(f.prefsPath(), []byte("not json {{{}"), 0o600); err != nil {
		t.Fatalf("write corrupt prefs: %v", err)
	}

	resp := f.getSettings(t)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got map[string]interface{}
	decodeSettingsJSON(t, resp, &got)
	if len(got) != 0 {
		t.Errorf("expected empty object on corrupt file, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// PUT /api/settings
// ---------------------------------------------------------------------------

// TestPutSettings_CreatesFile verifies that PUT creates the prefs file when it
// does not exist yet and returns the merged payload.
func TestPutSettings_CreatesFile(t *testing.T) {
	f := newSettingsFixture(t)

	body := map[string]interface{}{
		"appearance": map[string]interface{}{"theme": "paper"},
	}
	resp := f.putSettings(t, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// File must exist.
	if _, err := os.Stat(f.prefsPath()); os.IsNotExist(err) {
		t.Fatal("prefs file was not created")
	}

	// File must be 0600.
	info, err := os.Stat(f.prefsPath())
	if err != nil {
		t.Fatalf("stat prefs: %v", err)
	}
	if got := info.Mode().Perm(); got != userPrefsMode {
		t.Errorf("file mode = %04o, want %04o", got, userPrefsMode)
	}

	// Response body must reflect the stored content.
	var got map[string]interface{}
	decodeSettingsJSON(t, resp, &got)
	app, ok := got["appearance"].(map[string]interface{})
	if !ok {
		t.Fatalf("appearance missing: %v", got)
	}
	if app["theme"] != "paper" {
		t.Errorf("theme = %v, want paper", app["theme"])
	}
}

// TestPutSettings_MergesKeys verifies PATCH semantics: a PUT that only sends
// "editor" must not destroy an existing "appearance" key.
func TestPutSettings_MergesKeys(t *testing.T) {
	f := newSettingsFixture(t)

	// Write initial prefs with two categories.
	dir := filepath.Join(f.homeDir, vedoxDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	initial := map[string]interface{}{
		"appearance": map[string]interface{}{"theme": "graphite"},
		"editor":     map[string]interface{}{"spellCheck": false},
	}
	data, _ := json.Marshal(initial)
	if err := os.WriteFile(f.prefsPath(), data, 0o600); err != nil {
		t.Fatalf("write prefs: %v", err)
	}

	// PUT only changes "editor".
	patch := map[string]interface{}{
		"editor": map[string]interface{}{"spellCheck": true},
	}
	resp := f.putSettings(t, patch)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// "appearance" must survive.
	var got map[string]interface{}
	decodeSettingsJSON(t, resp, &got)

	app, ok := got["appearance"].(map[string]interface{})
	if !ok {
		t.Fatalf("appearance key was lost after PATCH: %v", got)
	}
	if app["theme"] != "graphite" {
		t.Errorf("theme = %v, want graphite (must be preserved)", app["theme"])
	}

	editor, ok := got["editor"].(map[string]interface{})
	if !ok {
		t.Fatalf("editor key missing: %v", got)
	}
	if editor["spellCheck"] != true {
		t.Errorf("spellCheck = %v, want true", editor["spellCheck"])
	}
}

// TestPutSettings_InvalidJSON returns 400 on a malformed request body.
func TestPutSettings_InvalidJSON(t *testing.T) {
	f := newSettingsFixture(t)

	req, err := http.NewRequest(http.MethodPut, f.server.URL+"/api/settings",
		bytes.NewBufferString("not json"))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// CSRF middleware requires an allowlisted Origin on mutating verbs.
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestPutSettings_Idempotent verifies that sending the same body twice yields
// the same stored result (atomicity + PATCH merge are stable under repetition).
func TestPutSettings_Idempotent(t *testing.T) {
	f := newSettingsFixture(t)

	body := map[string]interface{}{
		"voice": map[string]interface{}{"micEnabled": true},
	}

	for i := 0; i < 2; i++ {
		resp := f.putSettings(t, body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("PUT #%d: status = %d, want 200", i+1, resp.StatusCode)
		}
	}

	// Confirm file on disk has correct content.
	data, err := os.ReadFile(f.prefsPath())
	if err != nil {
		t.Fatalf("read prefs: %v", err)
	}
	var stored map[string]interface{}
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatalf("parse stored prefs: %v", err)
	}
	voice, ok := stored["voice"].(map[string]interface{})
	if !ok {
		t.Fatalf("voice key missing: %v", stored)
	}
	if voice["micEnabled"] != true {
		t.Errorf("micEnabled = %v, want true", voice["micEnabled"])
	}
}

// TestPutSettings_FileMode0600 confirms the written file always has mode 0600,
// even when the file did not previously exist (fchmod on temp fd path).
func TestPutSettings_FileMode0600(t *testing.T) {
	f := newSettingsFixture(t)

	resp := f.putSettings(t, map[string]interface{}{"notifications": map[string]interface{}{"soundEnabled": false}})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	info, err := os.Stat(f.prefsPath())
	if err != nil {
		t.Fatalf("stat prefs: %v", err)
	}
	if got := info.Mode().Perm(); got != userPrefsMode {
		t.Errorf("file mode = %04o, want %04o", got, userPrefsMode)
	}
}
