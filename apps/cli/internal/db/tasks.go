package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ── Public types ──────────────────────────────────────────────────────────────

// Task is the canonical DB-layer representation of a task row.
type Task struct {
	ID        string
	Project   string
	Title     string
	Status    string  // "todo" | "in-progress" | "done"
	Position  float64 // fractional indexing; ordered ascending
	CreatedAt string  // RFC3339
	UpdatedAt string  // RFC3339
}

// ErrTaskNotFound is returned when a task row matching the given project+id
// does not exist.
var ErrTaskNotFound = fmt.Errorf("vedox: task not found")

// ── Read operations ────────────────────────────────────────────────────────────

// ListTasks returns all tasks for a project ordered by position ascending.
func (s *Store) ListTasks(ctx context.Context, project string) ([]Task, error) {
	rows, err := s.readDB.QueryContext(ctx,
		`SELECT id, project, title, status, position, created_at, updated_at
		 FROM tasks
		 WHERE project = ?
		 ORDER BY position ASC`,
		project,
	)
	if err != nil {
		return nil, fmt.Errorf("vedox: ListTasks: %w", err)
	}
	defer rows.Close()

	var out []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Project, &t.Title, &t.Status, &t.Position, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("vedox: ListTasks scan: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vedox: ListTasks rows: %w", err)
	}
	if out == nil {
		out = []Task{}
	}
	return out, nil
}

// GetTask fetches a single task by project + id.
func (s *Store) GetTask(ctx context.Context, project, id string) (Task, error) {
	var t Task
	err := s.readDB.QueryRowContext(ctx,
		`SELECT id, project, title, status, position, created_at, updated_at
		 FROM tasks WHERE project = ? AND id = ?`,
		project, id,
	).Scan(&t.ID, &t.Project, &t.Title, &t.Status, &t.Position, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return Task{}, ErrTaskNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("vedox: GetTask: %w", err)
	}
	return t, nil
}

// NextTaskPosition returns max(position)+1.0 for tasks in the project,
// or 1.0 if no tasks exist yet.
func (s *Store) NextTaskPosition(ctx context.Context, project string) (float64, error) {
	var maxPos sql.NullFloat64
	if err := s.readDB.QueryRowContext(ctx,
		`SELECT MAX(position) FROM tasks WHERE project = ?`,
		project,
	).Scan(&maxPos); err != nil {
		return 0, fmt.Errorf("vedox: NextTaskPosition: %w", err)
	}
	if !maxPos.Valid {
		return 1.0, nil
	}
	return maxPos.Float64 + 1.0, nil
}

// ── Write operations ───────────────────────────────────────────────────────────

// InsertTask creates a new task row.
func (s *Store) InsertTask(ctx context.Context, t Task) error {
	return s.writer.submit(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(
			`INSERT INTO tasks(id, project, title, status, position, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			t.ID, t.Project, t.Title, t.Status, t.Position, t.CreatedAt, t.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert task: %w", err)
		}
		return nil
	})
}

// UpdateTask applies a partial update to a task. Only non-nil pointer fields
// are written. Returns ErrTaskNotFound if no row matched.
func (s *Store) UpdateTask(ctx context.Context, project, id string, title, status *string, position *float64) (Task, error) {
	if title == nil && status == nil && position == nil {
		return s.GetTask(ctx, project, id)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var setClauses []string
	var args []interface{}

	if title != nil {
		setClauses = append(setClauses, "title = ?")
		args = append(args, *title)
	}
	if status != nil {
		setClauses = append(setClauses, "status = ?")
		args = append(args, *status)
	}
	if position != nil {
		setClauses = append(setClauses, "position = ?")
		args = append(args, *position)
	}
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, now)
	args = append(args, project, id)

	query := fmt.Sprintf(
		`UPDATE tasks SET %s WHERE project = ? AND id = ?`,
		strings.Join(setClauses, ", "),
	)

	var rowsAffected int64
	if err := s.writer.submit(ctx, func(tx *sql.Tx) error {
		res, err := tx.Exec(query, args...)
		if err != nil {
			return fmt.Errorf("update task: %w", err)
		}
		rowsAffected, _ = res.RowsAffected()
		return nil
	}); err != nil {
		return Task{}, err
	}
	if rowsAffected == 0 {
		return Task{}, ErrTaskNotFound
	}
	return s.GetTask(ctx, project, id)
}

// DeleteTask removes a task row. Returns ErrTaskNotFound if no row matched.
func (s *Store) DeleteTask(ctx context.Context, project, id string) error {
	var rowsAffected int64
	if err := s.writer.submit(ctx, func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`DELETE FROM tasks WHERE project = ? AND id = ?`,
			project, id,
		)
		if err != nil {
			return fmt.Errorf("delete task: %w", err)
		}
		rowsAffected, _ = res.RowsAffected()
		return nil
	}); err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrTaskNotFound
	}
	return nil
}

// RenumberTasks assigns sequential integer positions (1, 2, 3, ...) to all
// tasks in the project ordered by their current position. Returns the updated
// list. This is called when the minimum gap between adjacent positions collapses
// below 0.001 (fractional index exhaustion).
func (s *Store) RenumberTasks(ctx context.Context, project string, tasks []Task) ([]Task, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if err := s.writer.submit(ctx, func(tx *sql.Tx) error {
		for i, t := range tasks {
			if _, err := tx.Exec(
				`UPDATE tasks SET position = ?, updated_at = ? WHERE id = ?`,
				float64(i+1), now, t.ID,
			); err != nil {
				return fmt.Errorf("renumber task %s: %w", t.ID, err)
			}
			tasks[i].Position = float64(i + 1)
			tasks[i].UpdatedAt = now
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return tasks, nil
}
