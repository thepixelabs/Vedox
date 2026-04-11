package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/vedox/vedox/internal/db"
)

// ── JSON shapes ───────────────────────────────────────────────────────────────

// taskResponse is the canonical JSON shape returned by all task endpoints.
// camelCase field names match the TypeScript Task interface on the frontend.
type taskResponse struct {
	ID        string  `json:"id"`
	Project   string  `json:"project"`
	Title     string  `json:"title"`
	Status    string  `json:"status"`
	Position  float64 `json:"position"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
}

// createTaskRequest is the JSON body accepted by POST /api/projects/:project/tasks.
type createTaskRequest struct {
	Title  string `json:"title"`
	Status string `json:"status"` // optional; defaults to "todo"
}

// updateTaskRequest is the JSON body accepted by PATCH /api/projects/:project/tasks/:id.
// All fields are optional; nil means "leave unchanged".
type updateTaskRequest struct {
	Title    *string  `json:"title"`
	Status   *string  `json:"status"`
	Position *float64 `json:"position"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// handleListTasks handles GET /api/projects/{project}/tasks.
// Returns all tasks for the project ordered by position ascending.
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	if err := validateTaskProjectName(project); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-005", "invalid project name")
		return
	}

	tasks, err := s.db.ListTasks(r.Context(), project)
	if err != nil {
		slog.Error("api.handleListTasks: query failed", "project", project, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, tasksToResponse(tasks))
}

// handleCreateTask handles POST /api/projects/{project}/tasks.
// Body: { title: string, status?: "todo"|"in-progress"|"done" }
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	if err := validateTaskProjectName(project); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-005", "invalid project name")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-000", "invalid JSON body")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "VDX-000", "title must not be empty")
		return
	}
	if req.Status == "" {
		req.Status = "todo"
	}
	if !validTaskStatus(req.Status) {
		writeError(w, http.StatusBadRequest, "VDX-000", "status must be todo, in-progress, or done")
		return
	}

	pos, err := s.db.NextTaskPosition(r.Context(), project)
	if err != nil {
		slog.Error("api.handleCreateTask: NextTaskPosition failed", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}

	t := db.Task{
		ID:      uuid.New().String(),
		Project: project,
		Title:   req.Title,
		Status:  req.Status,
		Position: pos,
	}
	// CreatedAt and UpdatedAt are set by InsertTask to avoid clock skew
	// between the caller and the DB layer. We set them here directly so the
	// response can echo them without an extra round-trip.
	now := nowRFC3339()
	t.CreatedAt = now
	t.UpdatedAt = now

	if err := s.db.InsertTask(r.Context(), t); err != nil {
		slog.Error("api.handleCreateTask: InsertTask failed", "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, taskToResponse(t))
}

// handleUpdateTask handles PATCH /api/projects/{project}/tasks/{id}.
// Accepts any subset of { title, status, position }.
//
// Position updates trigger a collapse check: if the minimum gap between adjacent
// task positions falls below 0.001, all tasks in the project are renumbered
// with integer positions and the full updated list is returned inside a
// { renumbered: true, tasks: [...] } envelope so the frontend can resync.
func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	taskID := chi.URLParam(r, "id")
	if err := validateTaskProjectName(project); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-005", "invalid project name")
		return
	}
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "VDX-000", "missing task id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	var req updateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-000", "invalid JSON body")
		return
	}

	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		req.Title = &trimmed
		if *req.Title == "" {
			writeError(w, http.StatusBadRequest, "VDX-000", "title must not be empty")
			return
		}
	}
	if req.Status != nil && !validTaskStatus(*req.Status) {
		writeError(w, http.StatusBadRequest, "VDX-000", "status must be todo, in-progress, or done")
		return
	}

	updated, err := s.db.UpdateTask(r.Context(), project, taskID, req.Title, req.Status, req.Position)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			writeError(w, http.StatusNotFound, "VDX-000", "task not found")
			return
		}
		slog.Error("api.handleUpdateTask: UpdateTask failed", "id", taskID, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}

	// After a position update check for fractional index exhaustion.
	if req.Position != nil {
		tasks, renumbered, renumErr := s.checkAndRenumber(r, project)
		if renumErr != nil {
			slog.Warn("api.handleUpdateTask: renumber check failed", "error", renumErr.Error())
			// Non-fatal — respond with the patched task.
			writeJSON(w, http.StatusOK, taskToResponse(updated))
			return
		}
		if renumbered {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"renumbered": true,
				"tasks":      tasksToResponse(tasks),
			})
			return
		}
	}

	writeJSON(w, http.StatusOK, taskToResponse(updated))
}

// handleDeleteTask handles DELETE /api/projects/{project}/tasks/{id}.
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	project := chi.URLParam(r, "project")
	taskID := chi.URLParam(r, "id")
	if err := validateTaskProjectName(project); err != nil {
		writeError(w, http.StatusBadRequest, "VDX-005", "invalid project name")
		return
	}
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "VDX-000", "missing task id")
		return
	}

	if err := s.db.DeleteTask(r.Context(), project, taskID); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			writeError(w, http.StatusNotFound, "VDX-000", "task not found")
			return
		}
		slog.Error("api.handleDeleteTask: DeleteTask failed", "id", taskID, "error", err.Error())
		writeError(w, http.StatusInternalServerError, "VDX-000", "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// checkAndRenumber queries the current task list and, if the minimum gap between
// adjacent positions is below 0.001, renumbers all tasks with integers 1, 2, 3, …
// Returns the (possibly renumbered) list and whether renumbering occurred.
func (s *Server) checkAndRenumber(r *http.Request, project string) ([]db.Task, bool, error) {
	tasks, err := s.db.ListTasks(r.Context(), project)
	if err != nil || len(tasks) < 2 {
		return tasks, false, err
	}

	minGap := tasks[1].Position - tasks[0].Position
	for i := 2; i < len(tasks); i++ {
		gap := tasks[i].Position - tasks[i-1].Position
		if gap < minGap {
			minGap = gap
		}
	}
	if minGap >= 0.001 {
		return tasks, false, nil
	}

	tasks, err = s.db.RenumberTasks(r.Context(), project, tasks)
	if err != nil {
		return nil, false, err
	}
	return tasks, true, nil
}

func taskToResponse(t db.Task) taskResponse {
	return taskResponse{
		ID:        t.ID,
		Project:   t.Project,
		Title:     t.Title,
		Status:    t.Status,
		Position:  t.Position,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
}

func tasksToResponse(tasks []db.Task) []taskResponse {
	out := make([]taskResponse, len(tasks))
	for i, t := range tasks {
		out[i] = taskToResponse(t)
	}
	return out
}

func validTaskStatus(s string) bool {
	return s == "todo" || s == "in-progress" || s == "done"
}

// validateTaskProjectName guards against path traversal in the project URL
// parameter. The api package's safeProjectPath helper has the same logic but
// requires a workspaceRoot; this lighter variant is sufficient for the DB
// layer which doesn't use the filesystem.
func validateTaskProjectName(name string) error {
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") {
		return fmt.Errorf("invalid project name")
	}
	return nil
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
