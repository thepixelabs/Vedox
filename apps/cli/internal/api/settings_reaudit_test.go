package api

// Re-audit tests for PUT /api/settings body-size, depth, and key-validation
// limits that wave-0 did not enforce.

import (
	"net/http"
	"strings"
	"testing"
)

// TestSettingsPut_OversizedBody asserts that a 1 MB settings payload is
// rejected before it can exhaust daemon memory.
func TestSettingsPut_OversizedBody(t *testing.T) {
	f := newSettingsFixture(t)

	// Craft a JSON object whose single value is ~400 KB of text (under the
	// 256 KB MaxBytesReader ceiling would be accepted; 400 KB must be rejected).
	big := `{"huge":"` + strings.Repeat("x", 400*1024) + `"}`

	req, _ := http.NewRequest(http.MethodPut, f.server.URL+"/api/settings",
		strings.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 || resp.StatusCode >= 500 {
		t.Fatalf("oversized settings body: got %d, want 4xx", resp.StatusCode)
	}
}

// TestSettingsPut_DeepNesting rejects a JSON object nested beyond the depth
// cap. This guards the file-write path (and any future consumer of the saved
// prefs file) against stack blow-out or parser DoS.
func TestSettingsPut_DeepNesting(t *testing.T) {
	f := newSettingsFixture(t)

	// Build a JSON bomb: 100 nested arrays as the value of a single key.
	depth := 100
	value := strings.Repeat("[", depth) + strings.Repeat("]", depth)
	payload := `{"deep":` + value + `}`

	req, _ := http.NewRequest(http.MethodPut, f.server.URL+"/api/settings",
		strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("deeply-nested payload: got %d, want 400", resp.StatusCode)
	}
}

// TestSettingsPut_ShallowNestingAccepted confirms the depth cap is generous
// enough for any legitimate preferences shape (depth-5 is already well over
// what the editor uses today).
func TestSettingsPut_ShallowNestingAccepted(t *testing.T) {
	f := newSettingsFixture(t)

	payload := `{"editor":{"theme":{"mode":"dark","accent":"blue"}}}`

	req, _ := http.NewRequest(http.MethodPut, f.server.URL+"/api/settings",
		strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://localhost:5151")
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("PUT: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("shallow settings payload: got %d, want 200", resp.StatusCode)
	}
}
