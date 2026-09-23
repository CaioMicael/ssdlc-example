// Package store provides SQLite-backed persistence for tasks.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	// StatusTodo is the default status assigned to a newly created task.
	StatusTodo = "todo"
	// StatusInProgress marks a task currently being worked on.
	StatusInProgress = "in_progress"
	// StatusDone marks a finished task.
	StatusDone = "done"

	titleMaxLen       = 200
	descriptionMaxLen = 2000

	defaultListLimit = 50
)

// ErrNotFound is returned when a task id does not exist.
var ErrNotFound = errors.New("task not found")

// ErrArchived is returned when attempting to update an archived task.
var ErrArchived = errors.New("task is archived")

// ValidationError carries the offending field and a human-readable message,
// so the HTTP layer can build the `fields` map in the error body.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Task is a single task record, with json tags matching the API model.
type Task struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Archived    bool   `json:"archived"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// Filter selects which tasks List returns.
type Filter struct {
	Status   string
	Archived bool
	Limit    int
	Offset   int
}

// Patch describes a partial update to a task. Only non-nil fields are
// applied.
type Patch struct {
	Title       *string
	Description *string
	Status      *string
}

// Store wraps a SQLite database handle holding the tasks table.
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS tasks (
  id          TEXT PRIMARY KEY,
  title       TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  status      TEXT NOT NULL CHECK (status IN ('todo','in_progress','done')),
  archived    INTEGER NOT NULL DEFAULT 0 CHECK (archived IN (0,1)),
  created_at  TEXT NOT NULL,
  updated_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_tasks_archived_updated ON tasks(archived, updated_at DESC);
`

// Open opens (creating if needed) the SQLite database at path with WAL mode
// and a 5s busy timeout, runs the schema migration, and returns the store.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migration: %w", err)
	}
	if _, err := db.Exec(timeEntriesSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migration: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

// Ping checks that the database is reachable.
func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// validateTitle trims title and checks its length is within 1..200.
func validateTitle(title string) (string, error) {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return "", &ValidationError{Field: "title", Message: "title must not be empty"}
	}
	if len(trimmed) > titleMaxLen {
		return "", &ValidationError{Field: "title", Message: "title must be at most 200 characters"}
	}
	return trimmed, nil
}

// validateDescription checks description length is within 0..2000.
func validateDescription(description string) error {
	if len(description) > descriptionMaxLen {
		return &ValidationError{Field: "description", Message: "description must be at most 2000 characters"}
	}
	return nil
}

// validateStatus checks status is one of the allowed values.
func validateStatus(status string) error {
	switch status {
	case StatusTodo, StatusInProgress, StatusDone:
		return nil
	default:
		return &ValidationError{Field: "status", Message: "status must be one of todo, in_progress, done"}
	}
}

// newUUIDv4 generates a random UUID version 4 using crypto/rand.
func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// Create validates title and description, then inserts a new task with
// StatusTodo and archived=false, returning the created task.
func (s *Store) Create(ctx context.Context, title, description string) (Task, error) {
	trimmedTitle, err := validateTitle(title)
	if err != nil {
		return Task{}, err
	}
	if err := validateDescription(description); err != nil {
		return Task{}, err
	}

	id, err := newUUIDv4()
	if err != nil {
		return Task{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	task := Task{
		ID:          id,
		Title:       trimmedTitle,
		Description: description,
		Status:      StatusTodo,
		Archived:    false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO tasks (id, title, description, status, archived, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Title, task.Description, task.Status, boolToInt(task.Archived), task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		return Task{}, fmt.Errorf("insert task: %w", err)
	}

	return task, nil
}

// List returns tasks matching the filter, ordered by updated_at descending.
// A zero Limit defaults to 50.
func (s *Store) List(ctx context.Context, filter Filter) ([]Task, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	query := `SELECT id, title, description, status, archived, created_at, updated_at
	          FROM tasks WHERE archived = ?`
	args := []any{boolToInt(filter.Archived)}

	if filter.Status != "" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}

	query += ` ORDER BY updated_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	tasks := make([]Task, 0)
	for rows.Next() {
		var t Task
		var archived int
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &archived, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		t.Archived = archived != 0
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	return tasks, nil
}

// Get returns the task with the given id, or ErrNotFound if it does not
// exist.
func (s *Store) Get(ctx context.Context, id string) (Task, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, title, description, status, archived, created_at, updated_at
		 FROM tasks WHERE id = ?`, id,
	)

	var t Task
	var archived int
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &archived, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("get task: %w", err)
	}
	t.Archived = archived != 0

	return t, nil
}

// getTaskTx reads a task within an existing transaction, mirroring Get.
func getTaskTx(ctx context.Context, tx *sql.Tx, id string) (Task, error) {
	row := tx.QueryRowContext(ctx,
		`SELECT id, title, description, status, archived, created_at, updated_at
		 FROM tasks WHERE id = ?`, id,
	)

	var t Task
	var archived int
	err := row.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &archived, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrNotFound
	}
	if err != nil {
		return Task{}, fmt.Errorf("get task: %w", err)
	}
	t.Archived = archived != 0

	return t, nil
}

// Update applies the non-nil fields of p to the task with the given id,
// bumps updated_at, and returns the updated task. It returns ErrNotFound if
// the task does not exist and ErrArchived if the task is archived. If the
// status changes to done and the task has an active timer, that timer is
// finished with the server clock in the same transaction (spec P1 AC11).
func (s *Store) Update(ctx context.Context, id string, p Patch) (Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	task, err := getTaskTx(ctx, tx, id)
	if err != nil {
		return Task{}, err
	}
	if task.Archived {
		return Task{}, ErrArchived
	}

	if p.Title != nil {
		trimmedTitle, err := validateTitle(*p.Title)
		if err != nil {
			return Task{}, err
		}
		task.Title = trimmedTitle
	}
	if p.Description != nil {
		if err := validateDescription(*p.Description); err != nil {
			return Task{}, err
		}
		task.Description = *p.Description
	}
	if p.Status != nil {
		if err := validateStatus(*p.Status); err != nil {
			return Task{}, err
		}
		task.Status = *p.Status
	}

	task.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)

	_, err = tx.ExecContext(ctx,
		`UPDATE tasks SET title = ?, description = ?, status = ?, updated_at = ? WHERE id = ?`,
		task.Title, task.Description, task.Status, task.UpdatedAt, task.ID,
	)
	if err != nil {
		return Task{}, fmt.Errorf("update task: %w", err)
	}

	if p.Status != nil && task.Status == StatusDone {
		if err := finishActiveEntryForTaskTx(ctx, tx, task.ID); err != nil {
			return Task{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return Task{}, fmt.Errorf("commit: %w", err)
	}

	return task, nil
}

// SetArchived sets the archived flag on the task with the given id, keeping
// every other field, and bumps updated_at. It is idempotent: setting the
// same value it already has still succeeds and still bumps updated_at.
// Archiving a task with an active timer finishes that timer with the server
// clock in the same transaction (spec P2 AC2).
func (s *Store) SetArchived(ctx context.Context, id string, archived bool) (Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	task, err := getTaskTx(ctx, tx, id)
	if err != nil {
		return Task{}, err
	}

	task.Archived = archived
	task.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)

	_, err = tx.ExecContext(ctx,
		`UPDATE tasks SET archived = ?, updated_at = ? WHERE id = ?`,
		boolToInt(task.Archived), task.UpdatedAt, task.ID,
	)
	if err != nil {
		return Task{}, fmt.Errorf("set archived: %w", err)
	}

	if archived {
		if err := finishActiveEntryForTaskTx(ctx, tx, task.ID); err != nil {
			return Task{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return Task{}, fmt.Errorf("commit: %w", err)
	}

	return task, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
