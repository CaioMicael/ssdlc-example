// Package store: time entry (cronômetro) persistence, built on top of the
// tasks schema and error conventions from store.go.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"modernc.org/sqlite"
)

// ErrNotTrackable is returned when starting a timer on a task that is done
// or archived.
var ErrNotTrackable = errors.New("task is not trackable")

// ErrNoActiveTimer is returned when stopping a timer while none is active.
var ErrNoActiveTimer = errors.New("no active timer")

// ErrEntryNotFound is returned when a time entry id does not exist.
var ErrEntryNotFound = errors.New("time entry not found")

// ErrEntryActive is returned when attempting to update or delete a running
// (unfinished) time entry.
var ErrEntryActive = errors.New("time entry is active")

// ErrEntryOverlap is returned when an edited time entry interval overlaps
// another entry, of any task.
var ErrEntryOverlap = errors.New("time entry overlaps another entry")

// now returns the current time in UTC. It is a variable so tests can
// substitute a deterministic clock; production always uses the server
// clock (time.Now).
var now = func() time.Time {
	return time.Now().UTC()
}

const timeEntriesSchema = `
CREATE TABLE IF NOT EXISTS time_entries (
  id         TEXT PRIMARY KEY,
  task_id    TEXT NOT NULL REFERENCES tasks(id),
  started_at TEXT NOT NULL,
  ended_at   TEXT,
  active     INTEGER,
  CHECK (ended_at IS NULL OR ended_at > started_at),
  CHECK ((ended_at IS NULL) = (active IS NOT NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_one_active ON time_entries(active);
CREATE INDEX IF NOT EXISTS idx_entries_task ON time_entries(task_id, started_at DESC);
`

// TimeEntry is a single timer record. EndedAt and DurationSeconds are nil
// while the entry is active (running).
type TimeEntry struct {
	ID              string  `json:"id"`
	TaskID          string  `json:"task_id"`
	StartedAt       string  `json:"started_at"`
	EndedAt         *string `json:"ended_at"`
	DurationSeconds *int64  `json:"duration_seconds"`
}

// timeLayout uses a fixed 9-digit fractional second (zeros, not nines), so
// every formatted timestamp has the same length. time_entries relies on
// plain TEXT ordering (the started_at/ended_at CHECK, the task index, and
// ORDER BY started_at) - RFC3339Nano's trimmed trailing zeros would make
// byte comparison disagree with chronological order (e.g. "...:00.5Z" >
// "...:00.500000001Z" lexicographically, despite being the earlier time).
const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

func formatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", s, err)
	}
	return t.UTC(), nil
}

// scanEntry scans one time_entries row into a TimeEntry, computing
// DurationSeconds when the entry is finished.
func scanEntry(row interface{ Scan(...any) error }) (TimeEntry, error) {
	var e TimeEntry
	var endedAt sql.NullString
	if err := row.Scan(&e.ID, &e.TaskID, &e.StartedAt, &endedAt); err != nil {
		return TimeEntry{}, err
	}
	if endedAt.Valid {
		e.EndedAt = &endedAt.String
		start, err := parseTime(e.StartedAt)
		if err != nil {
			return TimeEntry{}, err
		}
		end, err := parseTime(endedAt.String)
		if err != nil {
			return TimeEntry{}, err
		}
		dur := int64(end.Sub(start).Seconds())
		e.DurationSeconds = &dur
	}
	return e, nil
}

// activeEntryTx returns the currently running entry (active=1), if any.
func activeEntryTx(ctx context.Context, tx *sql.Tx) (TimeEntry, bool, error) {
	row := tx.QueryRowContext(ctx,
		`SELECT id, task_id, started_at, ended_at FROM time_entries WHERE active = 1`)
	e, err := scanEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TimeEntry{}, false, nil
	}
	if err != nil {
		return TimeEntry{}, false, fmt.Errorf("query active entry: %w", err)
	}
	return e, true, nil
}

// safeEndTime returns end, unless it is not strictly after the entry's
// started_at (e.g. two clock reads landing on the same tick under a coarse
// system clock resolution), in which case it returns started_at+1ns. The
// server clock stays the source of truth; this only breaks exact ties so
// the schema's `ended_at > started_at` CHECK always holds.
func safeEndTime(startedAtStr string, end time.Time) (time.Time, error) {
	started, err := parseTime(startedAtStr)
	if err != nil {
		return time.Time{}, err
	}
	if !end.After(started) {
		return started.Add(time.Nanosecond), nil
	}
	return end, nil
}

// finishEntryTx finalizes the given entry with the given end time.
func finishEntryTx(ctx context.Context, tx *sql.Tx, entryID string, endedAt time.Time) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE time_entries SET ended_at = ?, active = NULL WHERE id = ?`,
		formatTime(endedAt), entryID,
	)
	if err != nil {
		return fmt.Errorf("finish time entry: %w", err)
	}
	return nil
}

// finishActiveEntryForTaskTx finishes the active entry for taskID, if any,
// using the server clock. It is a no-op if there is no active entry or the
// active entry belongs to another task. Used to enforce that a task moving
// to done or being archived never leaves a timer running (spec P1 AC11, P2
// AC2).
func finishActiveEntryForTaskTx(ctx context.Context, tx *sql.Tx, taskID string) error {
	active, ok, err := activeEntryTx(ctx, tx)
	if err != nil {
		return err
	}
	if !ok || active.TaskID != taskID {
		return nil
	}
	endedAt, err := safeEndTime(active.StartedAt, now())
	if err != nil {
		return err
	}
	return finishEntryTx(ctx, tx, active.ID, endedAt)
}

// maxStartTimerAttempts bounds the retry-on-conflict loop in StartTimer.
// Two concurrent starts can both read "no active timer" before either
// commits; the loser's INSERT is rejected by the idx_one_active unique
// index (not by application logic - see the schema comment), and it must
// retry with a fresh read of the now-current state.
const maxStartTimerAttempts = 100

// SQLite result codes relevant to retrying a losing StartTimer attempt.
const (
	sqliteConstraintUnique = 2067 // SQLITE_CONSTRAINT_UNIQUE: idx_one_active rejected a second active row.
	sqliteBusy             = 5   // SQLITE_BUSY: another writer currently holds the lock.
	sqliteBusySnapshot     = 517 // SQLITE_BUSY_SNAPSHOT: this tx's read snapshot is stale; must restart.
)

// isRetryableStartTimerConflict reports whether err is a concurrency
// conflict that a fresh attempt (with a fresh read of the current state)
// can resolve: the idx_one_active unique index rejecting a losing INSERT,
// or the SQLite writer lock/snapshot being contended by another concurrent
// start. Any other error is returned to the caller as-is.
func isRetryableStartTimerConflict(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	switch sqliteErr.Code() {
	case sqliteConstraintUnique, sqliteBusy, sqliteBusySnapshot:
		return true
	default:
		return false
	}
}

// StartTimer starts a timer on taskID. If a timer is already running on the
// same task, it is returned unchanged with created=false. If a timer is
// running on a different task, that timer is finished with the server clock
// and the new one is created, in the same transaction. It returns
// ErrNotFound if the task does not exist and ErrNotTrackable if the task is
// done or archived.
//
// Concurrency: the invariant of at most one active timer is enforced by the
// idx_one_active unique index in the schema, not by this method checking
// before inserting (that check alone would lose a race between two
// simultaneous starts). When two starts race, the DB rejects the loser's
// INSERT with a unique constraint violation; StartTimer catches exactly
// that and retries with a fresh read.
func (s *Store) StartTimer(ctx context.Context, taskID string) (TimeEntry, bool, error) {
	for attempt := 0; attempt < maxStartTimerAttempts; attempt++ {
		entry, created, err := s.startTimerOnce(ctx, taskID)
		if isRetryableStartTimerConflict(err) {
			continue
		}
		return entry, created, err
	}
	return TimeEntry{}, false, fmt.Errorf("start timer: gave up after %d attempts under contention", maxStartTimerAttempts)
}

// startTimerOnce is a single attempt at StartTimer's logic. It returns a
// unique-constraint error unwrapped so the caller can detect and retry it.
func (s *Store) startTimerOnce(ctx context.Context, taskID string) (TimeEntry, bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TimeEntry{}, false, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var status string
	var archived int
	err = tx.QueryRowContext(ctx, `SELECT status, archived FROM tasks WHERE id = ?`, taskID).Scan(&status, &archived)
	if errors.Is(err, sql.ErrNoRows) {
		return TimeEntry{}, false, ErrNotFound
	}
	if err != nil {
		return TimeEntry{}, false, fmt.Errorf("get task: %w", err)
	}
	if status == StatusDone || archived != 0 {
		return TimeEntry{}, false, ErrNotTrackable
	}

	active, ok, err := activeEntryTx(ctx, tx)
	if err != nil {
		return TimeEntry{}, false, err
	}
	if ok {
		if active.TaskID == taskID {
			if err := tx.Commit(); err != nil {
				return TimeEntry{}, false, fmt.Errorf("commit: %w", err)
			}
			return active, false, nil
		}
		endedAt, err := safeEndTime(active.StartedAt, now())
		if err != nil {
			return TimeEntry{}, false, err
		}
		if err := finishEntryTx(ctx, tx, active.ID, endedAt); err != nil {
			return TimeEntry{}, false, err
		}
	}

	id, err := newUUIDv4()
	if err != nil {
		return TimeEntry{}, false, err
	}
	startedAt := formatTime(now())
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO time_entries (id, task_id, started_at, ended_at, active) VALUES (?, ?, ?, NULL, 1)`,
		id, taskID, startedAt,
	); err != nil {
		return TimeEntry{}, false, fmt.Errorf("insert time entry: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return TimeEntry{}, false, fmt.Errorf("commit: %w", err)
	}

	return TimeEntry{ID: id, TaskID: taskID, StartedAt: startedAt}, true, nil
}

// StopTimer finishes the currently active timer, if any, using the server
// clock. It returns ErrNoActiveTimer if there is none.
func (s *Store) StopTimer(ctx context.Context) (TimeEntry, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TimeEntry{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	active, ok, err := activeEntryTx(ctx, tx)
	if err != nil {
		return TimeEntry{}, err
	}
	if !ok {
		return TimeEntry{}, ErrNoActiveTimer
	}

	endedAt, err := safeEndTime(active.StartedAt, now())
	if err != nil {
		return TimeEntry{}, err
	}
	if err := finishEntryTx(ctx, tx, active.ID, endedAt); err != nil {
		return TimeEntry{}, err
	}
	if err := tx.Commit(); err != nil {
		return TimeEntry{}, fmt.Errorf("commit: %w", err)
	}

	endedStr := formatTime(endedAt)
	start, err := parseTime(active.StartedAt)
	if err != nil {
		return TimeEntry{}, err
	}
	dur := int64(endedAt.Sub(start).Seconds())
	active.EndedAt = &endedStr
	active.DurationSeconds = &dur
	return active, nil
}

// ActiveTimer returns the currently running entry and its task, or
// ok=false if there is none.
func (s *Store) ActiveTimer(ctx context.Context) (TimeEntry, Task, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, task_id, started_at, ended_at FROM time_entries WHERE active = 1`)
	entry, err := scanEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TimeEntry{}, Task{}, false, nil
	}
	if err != nil {
		return TimeEntry{}, Task{}, false, fmt.Errorf("query active entry: %w", err)
	}

	task, err := s.Get(ctx, entry.TaskID)
	if err != nil {
		return TimeEntry{}, Task{}, false, err
	}

	return entry, task, true, nil
}

// ListEntries returns the entries for taskID ordered by started_at
// descending, each with DurationSeconds computed when finished. It returns
// ErrNotFound if the task does not exist.
func (s *Store) ListEntries(ctx context.Context, taskID string) ([]TimeEntry, error) {
	if _, err := s.Get(ctx, taskID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, task_id, started_at, ended_at FROM time_entries WHERE task_id = ? ORDER BY started_at DESC`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("list time entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries := make([]TimeEntry, 0)
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("scan time entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list time entries: %w", err)
	}

	return entries, nil
}

// getEntryTx reads a time entry within an existing transaction.
func getEntryTx(ctx context.Context, tx *sql.Tx, id string) (TimeEntry, error) {
	row := tx.QueryRowContext(ctx,
		`SELECT id, task_id, started_at, ended_at FROM time_entries WHERE id = ?`, id)
	e, err := scanEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return TimeEntry{}, ErrEntryNotFound
	}
	if err != nil {
		return TimeEntry{}, fmt.Errorf("get time entry: %w", err)
	}
	return e, nil
}

// overlapsTx reports whether [start, end) overlaps any time entry other
// than excludeID, across all tasks. An entry still active (no ended_at) is
// treated as open-ended for this check, since it has not finished yet.
// Touching intervals (one entry's end equal to the other's start) are not
// an overlap.
func overlapsTx(ctx context.Context, tx *sql.Tx, excludeID string, start, end time.Time) (bool, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT started_at, ended_at FROM time_entries WHERE id != ?`, excludeID)
	if err != nil {
		return false, fmt.Errorf("query entries for overlap: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var startedAtStr string
		var endedAt sql.NullString
		if err := rows.Scan(&startedAtStr, &endedAt); err != nil {
			return false, fmt.Errorf("scan entry for overlap: %w", err)
		}
		otherStart, err := parseTime(startedAtStr)
		if err != nil {
			return false, err
		}
		var otherEnd time.Time
		if endedAt.Valid {
			otherEnd, err = parseTime(endedAt.String)
			if err != nil {
				return false, err
			}
		} else {
			// Still running: open-ended, so it overlaps anything that
			// starts before "forever".
			otherEnd = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
		}

		if start.Before(otherEnd) && otherStart.Before(end) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("query entries for overlap: %w", err)
	}

	return false, nil
}

// UpdateEntry updates the started_at and/or ended_at of a finished entry,
// recalculating its duration. Nil fields keep the current value. It
// returns ErrEntryNotFound when the id does not exist, ErrEntryActive when
// the entry is still running, a *ValidationError when ended_at is not after
// started_at or is in the future, and ErrEntryOverlap when the resulting
// interval overlaps another entry.
func (s *Store) UpdateEntry(ctx context.Context, id string, startedAt, endedAt *time.Time) (TimeEntry, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return TimeEntry{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	entry, err := getEntryTx(ctx, tx, id)
	if err != nil {
		return TimeEntry{}, err
	}
	if entry.EndedAt == nil {
		return TimeEntry{}, ErrEntryActive
	}

	newStart, err := parseTime(entry.StartedAt)
	if err != nil {
		return TimeEntry{}, err
	}
	if startedAt != nil {
		newStart = startedAt.UTC()
	}

	newEnd, err := parseTime(*entry.EndedAt)
	if err != nil {
		return TimeEntry{}, err
	}
	if endedAt != nil {
		newEnd = endedAt.UTC()
	}

	if !newEnd.After(newStart) {
		return TimeEntry{}, &ValidationError{Field: "ended_at", Message: "ended_at must be after started_at"}
	}
	if newEnd.After(now()) {
		return TimeEntry{}, &ValidationError{Field: "ended_at", Message: "ended_at must not be in the future"}
	}

	overlap, err := overlapsTx(ctx, tx, id, newStart, newEnd)
	if err != nil {
		return TimeEntry{}, err
	}
	if overlap {
		return TimeEntry{}, ErrEntryOverlap
	}

	startedStr := formatTime(newStart)
	endedStr := formatTime(newEnd)
	if _, err := tx.ExecContext(ctx,
		`UPDATE time_entries SET started_at = ?, ended_at = ? WHERE id = ?`,
		startedStr, endedStr, id,
	); err != nil {
		return TimeEntry{}, fmt.Errorf("update time entry: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return TimeEntry{}, fmt.Errorf("commit: %w", err)
	}

	dur := int64(newEnd.Sub(newStart).Seconds())
	return TimeEntry{ID: id, TaskID: entry.TaskID, StartedAt: startedStr, EndedAt: &endedStr, DurationSeconds: &dur}, nil
}

// DeleteEntry permanently removes a finished time entry. It returns
// ErrEntryNotFound when the id does not exist and ErrEntryActive when the
// entry is still running.
func (s *Store) DeleteEntry(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	entry, err := getEntryTx(ctx, tx, id)
	if err != nil {
		return err
	}
	if entry.EndedAt == nil {
		return ErrEntryActive
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM time_entries WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete time entry: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// TotalSeconds returns the sum of DurationSeconds of all finished entries
// for taskID.
func (s *Store) TotalSeconds(ctx context.Context, taskID string) (int64, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT started_at, ended_at FROM time_entries WHERE task_id = ? AND ended_at IS NOT NULL`,
		taskID,
	)
	if err != nil {
		return 0, fmt.Errorf("sum time entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var total int64
	for rows.Next() {
		var startedAtStr, endedAtStr string
		if err := rows.Scan(&startedAtStr, &endedAtStr); err != nil {
			return 0, fmt.Errorf("scan time entry: %w", err)
		}
		start, err := parseTime(startedAtStr)
		if err != nil {
			return 0, err
		}
		end, err := parseTime(endedAtStr)
		if err != nil {
			return 0, err
		}
		total += int64(end.Sub(start).Seconds())
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("sum time entries: %w", err)
	}

	return total, nil
}
