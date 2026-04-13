package api_test

// Integration tests for the Tasks HTTP API.
// Each test builds a fresh fixture via newTestServer (api_integration_test.go)
// so there is no shared state across tests.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// createTask is a small helper that POSTs a task and returns the parsed body.
// It fails the test if the server returns anything other than 201.
func createTask(t *testing.T, f *testFixture, project, title string) taskJSON {
	t.Helper()
	resp := f.do(t, http.MethodPost,
		"/api/projects/"+project+"/tasks",
		map[string]string{"title": title},
	)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create task %q: status %d (body=%s)", title, resp.StatusCode, readBody(t, resp))
	}
	var task taskJSON
	decodeJSON(t, resp, &task)
	return task
}

// taskJSON mirrors the API response shape for a task. Defined here so the
// test stays independent of any internal type renames in tasks.go.
type taskJSON struct {
	ID       string  `json:"id"`
	Project  string  `json:"project"`
	Title    string  `json:"title"`
	Status   string  `json:"status"`
	Position float64 `json:"position"`
}

func TestCreateTask(t *testing.T) {
	f := newTestServer(t)
	resp := f.do(t, http.MethodPost,
		"/api/projects/myproject/tasks",
		map[string]string{"title": "first task"},
	)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	var task taskJSON
	decodeJSON(t, resp, &task)

	if task.ID == "" {
		t.Errorf("expected non-empty ID")
	}
	if task.Title != "first task" {
		t.Errorf("title = %q, want %q", task.Title, "first task")
	}
	if task.Status != "todo" {
		t.Errorf("default status = %q, want %q", task.Status, "todo")
	}
	if task.Position <= 0 {
		t.Errorf("position = %v, want > 0", task.Position)
	}
}

func TestListTasks_Empty(t *testing.T) {
	f := newTestServer(t)
	resp := f.do(t, http.MethodGet, "/api/projects/myproject/tasks", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var tasks []taskJSON
	decodeJSON(t, resp, &tasks)
	if len(tasks) != 0 {
		t.Errorf("expected empty list, got %d", len(tasks))
	}
}

// TestListTasks_AfterCreate creates three tasks and expects the listing to
// return all three in the same order they were created (NextTaskPosition
// monotonically increases).
func TestListTasks_AfterCreate(t *testing.T) {
	f := newTestServer(t)
	createTask(t, f, "myproject", "alpha")
	createTask(t, f, "myproject", "beta")
	createTask(t, f, "myproject", "gamma")

	resp := f.do(t, http.MethodGet, "/api/projects/myproject/tasks", nil)
	var tasks []taskJSON
	decodeJSON(t, resp, &tasks)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
	want := []string{"alpha", "beta", "gamma"}
	for i, w := range want {
		if tasks[i].Title != w {
			t.Errorf("tasks[%d].Title = %q, want %q", i, tasks[i].Title, w)
		}
	}
}

func TestUpdateTask_Status(t *testing.T) {
	f := newTestServer(t)
	created := createTask(t, f, "myproject", "do the thing")

	newStatus := "done"
	resp := f.do(t, http.MethodPatch,
		"/api/projects/myproject/tasks/"+created.ID,
		map[string]interface{}{"status": newStatus},
	)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, readBody(t, resp))
	}
	var updated taskJSON
	decodeJSON(t, resp, &updated)
	if updated.Status != newStatus {
		t.Errorf("status = %q, want %q", updated.Status, newStatus)
	}
}

// TestUpdateTask_Position moves the third task ahead of the first by setting
// its position to 0.5 and verifies the listing reflects the new order.
func TestUpdateTask_Position(t *testing.T) {
	f := newTestServer(t)
	a := createTask(t, f, "myproject", "a") // pos 1
	_ = createTask(t, f, "myproject", "b")  // pos 2
	c := createTask(t, f, "myproject", "c") // pos 3

	newPos := 0.5
	resp := f.do(t, http.MethodPatch,
		"/api/projects/myproject/tasks/"+c.ID,
		map[string]interface{}{"position": newPos},
	)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", resp.StatusCode, readBody(t, resp))
	}

	resp = f.do(t, http.MethodGet, "/api/projects/myproject/tasks", nil)
	var tasks []taskJSON
	decodeJSON(t, resp, &tasks)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != c.ID {
		t.Errorf("expected task 'c' first after reorder, got %q", tasks[0].Title)
	}
	if tasks[1].ID != a.ID {
		t.Errorf("expected task 'a' second after reorder, got %q", tasks[1].Title)
	}
}

func TestDeleteTask(t *testing.T) {
	f := newTestServer(t)
	task := createTask(t, f, "myproject", "delete me")

	resp := f.do(t, http.MethodDelete, "/api/projects/myproject/tasks/"+task.ID, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want 204 (body=%s)", resp.StatusCode, readBody(t, resp))
	}

	resp = f.do(t, http.MethodGet, "/api/projects/myproject/tasks", nil)
	var tasks []taskJSON
	decodeJSON(t, resp, &tasks)
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", len(tasks))
	}
}

// TestTaskRenumbering forces fractional-index exhaustion by repeatedly
// inserting tasks with positions that halve the gap between two anchors.
// When the smallest gap drops below 0.001 the API renumbers everything
// to integer positions and returns a {renumbered:true, tasks:[…]} envelope.
//
// The test asserts (a) the renumber envelope is eventually returned, and
// (b) every position in the result is a clean integer ≥ 1.
func TestTaskRenumbering(t *testing.T) {
	f := newTestServer(t)
	// Two anchor tasks at positions 1 and 2 (assigned by NextTaskPosition).
	createTask(t, f, "myproject", "anchor-low")
	high := createTask(t, f, "myproject", "anchor-high")

	// Move "anchor-high" closer and closer to "anchor-low" until renumbering
	// fires. Each step halves the gap. After ~12 iterations the gap is well
	// below the 0.001 threshold and the server must collapse the index.
	var lastResp *http.Response
	pos := 1.5
	for i := 0; i < 20; i++ {
		lastResp = f.do(t, http.MethodPatch,
			"/api/projects/myproject/tasks/"+high.ID,
			map[string]interface{}{"position": pos},
		)
		if lastResp.StatusCode != http.StatusOK {
			t.Fatalf("PATCH status = %d on iter %d (body=%s)", lastResp.StatusCode, i, readBody(t, lastResp))
		}

		// Peek at the body to detect the renumber envelope.
		body := readBody(t, lastResp)
		if containsRenumberFlag(body) {
			// Renumbering happened — verify all positions are integers.
			tasks := parseRenumberedTasks(t, body)
			if len(tasks) < 2 {
				t.Fatalf("renumber envelope has %d tasks, want >= 2", len(tasks))
			}
			for i, task := range tasks {
				if task.Position != float64(int(task.Position)) || task.Position < 1 {
					t.Errorf("tasks[%d].Position = %v, want integer >= 1", i, task.Position)
				}
			}
			return
		}
		pos = 1.0 + (pos-1.0)/2
	}
	t.Fatalf("renumber envelope never returned after 20 iterations")
}

// containsRenumberFlag tolerates either compact or pretty JSON encoding so the
// test does not couple to a specific encoder configuration.
func containsRenumberFlag(body string) bool {
	return strings.Contains(body, `"renumbered":true`) ||
		strings.Contains(body, `"renumbered": true`)
}

func parseRenumberedTasks(t *testing.T, body string) []taskJSON {
	t.Helper()
	var env struct {
		Renumbered bool       `json:"renumbered"`
		Tasks      []taskJSON `json:"tasks"`
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("decode renumber envelope: %v (body=%s)", err, body)
	}
	return env.Tasks
}
