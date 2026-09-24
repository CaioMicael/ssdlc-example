package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"modernc.org/sqlite"
)

// withNow overrides the package clock for the duration of the test (or
// until the next withNow call), restoring the previous value on cleanup.
func withNow(t *testing.T, ts time.Time) {
	t.Helper()
	orig := now
	now = func() time.Time { return ts }
	t.Cleanup(func() { now = orig })
}

// AC1 (spec P1 Cronômetro #1): start with no active timer creates one with
// ended_at nil.
func TestStartTimer_NoActive_Creates(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	entry, created, err := s.StartTimer(ctx, task.ID)
	if err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}
	if !created {
		t.Error("created = false, want true")
	}
	if entry.TaskID != task.ID {
		t.Errorf("TaskID = %q, want %q", entry.TaskID, task.ID)
	}
	if entry.EndedAt != nil {
		t.Errorf("EndedAt = %v, want nil", entry.EndedAt)
	}
	if entry.StartedAt == "" {
		t.Error("StartedAt is empty, want a timestamp")
	}
}

// AC3 (spec P1 Cronômetro #3): start on a task that already has the active
// timer returns the existing entry with created=false, no second row.
func TestStartTimer_SameTask_ReturnsExistingNoCreate(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	first, _, err := s.StartTimer(ctx, task.ID)
	if err != nil {
		t.Fatalf("StartTimer() first error = %v", err)
	}

	second, created, err := s.StartTimer(ctx, task.ID)
	if err != nil {
		t.Fatalf("StartTimer() second error = %v", err)
	}
	if created {
		t.Error("created = true, want false")
	}
	if second.ID != first.ID {
		t.Errorf("ID = %q, want %q (same entry returned)", second.ID, first.ID)
	}

	entries, err := s.ListEntries(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListEntries() error = %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("len(entries) = %d, want 1 (no duplicate row)", len(entries))
	}
}

// AC2 (spec P1 Cronômetro #2): start on task B while A is active finishes A
// with the switch timestamp and creates B active, leaving exactly one
// active entry.
func TestStartTimer_OtherTaskActive_FinishesPreviousAndCreatesNew(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	taskA, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	taskB, err := s.Create(ctx, "Task B", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	entryA, _, err := s.StartTimer(ctx, taskA.ID)
	if err != nil {
		t.Fatalf("StartTimer(A) error = %v", err)
	}

	entryB, created, err := s.StartTimer(ctx, taskB.ID)
	if err != nil {
		t.Fatalf("StartTimer(B) error = %v", err)
	}
	if !created {
		t.Error("created = false, want true for B")
	}

	entriesA, err := s.ListEntries(ctx, taskA.ID)
	if err != nil {
		t.Fatalf("ListEntries(A) error = %v", err)
	}
	if len(entriesA) != 1 || entriesA[0].ID != entryA.ID {
		t.Fatalf("entries(A) = %+v, want exactly [%s] finished", entriesA, entryA.ID)
	}
	if entriesA[0].EndedAt == nil {
		t.Error("A's entry EndedAt = nil, want finished when B started")
	}

	active, activeTask, ok, err := s.ActiveTimer(ctx)
	if err != nil {
		t.Fatalf("ActiveTimer() error = %v", err)
	}
	if !ok {
		t.Fatal("ActiveTimer() ok = false, want true")
	}
	if active.ID != entryB.ID {
		t.Errorf("active.ID = %q, want %q (B)", active.ID, entryB.ID)
	}
	if activeTask.ID != taskB.ID {
		t.Errorf("activeTask.ID = %q, want %q", activeTask.ID, taskB.ID)
	}
}

// AC4 (spec P1 Cronômetro #4): starting on a done task is refused.
func TestStartTimer_TaskDone_ErrNotTrackable(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	doneStatus := StatusDone
	if _, err := s.Update(ctx, task.ID, Patch{Status: &doneStatus}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	_, _, err = s.StartTimer(ctx, task.ID)
	if !errors.Is(err, ErrNotTrackable) {
		t.Errorf("err = %v, want ErrNotTrackable", err)
	}
}

// AC4 (spec P1 Cronômetro #4): starting on an archived task is refused.
func TestStartTimer_TaskArchived_ErrNotTrackable(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := s.SetArchived(ctx, task.ID, true); err != nil {
		t.Fatalf("SetArchived() error = %v", err)
	}

	_, _, err = s.StartTimer(ctx, task.ID)
	if !errors.Is(err, ErrNotTrackable) {
		t.Errorf("err = %v, want ErrNotTrackable", err)
	}
}

// Unknown task id must surface the same ErrNotFound as the task store.
func TestStartTimer_UnknownTask_ErrNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, _, err := s.StartTimer(ctx, "does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// AC5 (spec P1 Cronômetro #5): stop finishes the active entry.
func TestStopTimer_FinishesActive(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	entry, _, err := s.StartTimer(ctx, task.ID)
	if err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}

	stopped, err := s.StopTimer(ctx)
	if err != nil {
		t.Fatalf("StopTimer() error = %v", err)
	}
	if stopped.ID != entry.ID {
		t.Errorf("ID = %q, want %q", stopped.ID, entry.ID)
	}
	if stopped.EndedAt == nil {
		t.Error("EndedAt = nil, want set")
	}
	if stopped.DurationSeconds == nil {
		t.Error("DurationSeconds = nil, want set")
	}

	_, _, ok, err := s.ActiveTimer(ctx)
	if err != nil {
		t.Fatalf("ActiveTimer() error = %v", err)
	}
	if ok {
		t.Error("ActiveTimer() ok = true, want false after stop")
	}
}

// AC6 (spec P1 Cronômetro #6): stop with no active timer is refused.
func TestStopTimer_NoActive_ErrNoActiveTimer(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.StopTimer(ctx)
	if !errors.Is(err, ErrNoActiveTimer) {
		t.Errorf("err = %v, want ErrNoActiveTimer", err)
	}
}

// AC7 (spec P1 Cronômetro #7): 20 concurrent starts on different tasks must
// leave exactly one active row. This exercises the DB directly, so it
// fails if the unique index on time_entries(active) is ever removed --
// without it, all 20 concurrent inserts would succeed and this would read
// back a count of 20 instead of 1.
func TestStartTimer_Concurrent20Starts_ExactlyOneActive(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	const n = 20
	taskIDs := make([]string, n)
	for i := 0; i < n; i++ {
		task, err := s.Create(ctx, fmt.Sprintf("Task %d", i), "")
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		taskIDs[i] = task.ID
	}

	var wg sync.WaitGroup
	for _, id := range taskIDs {
		wg.Add(1)
		go func(taskID string) {
			defer wg.Done()
			// Losers of the race may legitimately error (e.g. busy
			// timeout exhausted); the invariant under test is the row
			// count below, not each call's outcome.
			_, _, _ = s.StartTimer(ctx, taskID)
		}(id)
	}
	wg.Wait()

	var count int
	row := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM time_entries WHERE active = 1`)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count active: %v", err)
	}
	if count != 1 {
		t.Errorf("active count = %d, want exactly 1", count)
	}
}

// AC7 (spec P1 Cronômetro #7), design.md guarantee: the invariant is
// enforced by the idx_one_active unique index itself, independent of
// StartTimer's own logic. A raw INSERT that bypasses StartTimer entirely
// is still rejected once one active row exists - this is the mechanism the
// goroutine test above relies on, and it fails (the second insert would
// wrongly succeed) if idx_one_active is ever dropped from the schema.
func TestSchema_UniqueActiveIndex_RejectsSecondActiveRow(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	taskA, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	taskB, err := s.Create(ctx, "Task B", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO time_entries (id, task_id, started_at, ended_at, active) VALUES (?, ?, ?, NULL, 1)`,
		"raw-entry-a", taskA.ID, formatTime(now()),
	)
	if err != nil {
		t.Fatalf("first raw insert: %v", err)
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO time_entries (id, task_id, started_at, ended_at, active) VALUES (?, ?, ?, NULL, 1)`,
		"raw-entry-b", taskB.ID, formatTime(now()),
	)
	if err == nil {
		t.Fatal("second raw insert with active=1 succeeded, want the unique index to reject it")
	}
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) || sqliteErr.Code() != sqliteConstraintUnique {
		t.Errorf("err = %v, want SQLITE_CONSTRAINT_UNIQUE (2067)", err)
	}
}

// AC8 (spec P1 Cronômetro #8): ActiveTimer returns the running entry and
// its task.
func TestActiveTimer_WithRunning(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	entry, _, err := s.StartTimer(ctx, task.ID)
	if err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}

	gotEntry, gotTask, ok, err := s.ActiveTimer(ctx)
	if err != nil {
		t.Fatalf("ActiveTimer() error = %v", err)
	}
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if gotEntry.ID != entry.ID {
		t.Errorf("entry.ID = %q, want %q", gotEntry.ID, entry.ID)
	}
	if gotTask.ID != task.ID {
		t.Errorf("task.ID = %q, want %q", gotTask.ID, task.ID)
	}
}

// AC8 (spec P1 Cronômetro #8): ActiveTimer reports ok=false when nothing is
// running.
func TestActiveTimer_WithoutRunning(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, _, ok, err := s.ActiveTimer(ctx)
	if err != nil {
		t.Fatalf("ActiveTimer() error = %v", err)
	}
	if ok {
		t.Error("ok = true, want false")
	}
}

// AC11 (spec P1 Cronômetro #11): marking a task's status done finishes its
// active entry in the same transaction.
func TestUpdate_StatusDone_FinishesActiveEntry(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	entry, _, err := s.StartTimer(ctx, task.ID)
	if err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}

	doneStatus := StatusDone
	if _, err := s.Update(ctx, task.ID, Patch{Status: &doneStatus}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	_, _, ok, err := s.ActiveTimer(ctx)
	if err != nil {
		t.Fatalf("ActiveTimer() error = %v", err)
	}
	if ok {
		t.Error("ActiveTimer() ok = true, want false after marking task done")
	}

	entries, err := s.ListEntries(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListEntries() error = %v", err)
	}
	if len(entries) != 1 || entries[0].ID != entry.ID {
		t.Fatalf("entries = %+v, want exactly [%s]", entries, entry.ID)
	}
	if entries[0].EndedAt == nil {
		t.Error("EndedAt = nil, want finished")
	}
}

// P2 AC2: archiving a task with an active timer finishes it in the same
// transaction.
func TestSetArchived_True_FinishesActiveEntry(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	entry, _, err := s.StartTimer(ctx, task.ID)
	if err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}

	if _, err := s.SetArchived(ctx, task.ID, true); err != nil {
		t.Fatalf("SetArchived() error = %v", err)
	}

	_, _, ok, err := s.ActiveTimer(ctx)
	if err != nil {
		t.Fatalf("ActiveTimer() error = %v", err)
	}
	if ok {
		t.Error("ActiveTimer() ok = true, want false after archiving")
	}

	entries, err := s.ListEntries(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListEntries() error = %v", err)
	}
	if len(entries) != 1 || entries[0].ID != entry.ID || entries[0].EndedAt == nil {
		t.Fatalf("entries = %+v, want exactly [%s] finished", entries, entry.ID)
	}
}

// P1 Apontamentos AC1: entries are listed started_at descending, with
// duration for finished ones.
func TestListEntries_OrderedDescWithDuration(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	withNow(t, base)
	if _, _, err := s.StartTimer(ctx, task.ID); err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}
	withNow(t, base.Add(5*time.Minute))
	first, err := s.StopTimer(ctx)
	if err != nil {
		t.Fatalf("StopTimer() error = %v", err)
	}

	withNow(t, base.Add(10*time.Minute))
	if _, _, err := s.StartTimer(ctx, task.ID); err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}
	withNow(t, base.Add(30*time.Minute))
	second, err := s.StopTimer(ctx)
	if err != nil {
		t.Fatalf("StopTimer() error = %v", err)
	}

	entries, err := s.ListEntries(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListEntries() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].ID != second.ID {
		t.Errorf("entries[0].ID = %q, want %q (most recent started_at first)", entries[0].ID, second.ID)
	}
	if entries[1].ID != first.ID {
		t.Errorf("entries[1].ID = %q, want %q", entries[1].ID, first.ID)
	}
	if entries[0].DurationSeconds == nil || *entries[0].DurationSeconds != 1200 {
		t.Errorf("entries[0].DurationSeconds = %v, want 1200", entries[0].DurationSeconds)
	}
	if entries[1].DurationSeconds == nil || *entries[1].DurationSeconds != 300 {
		t.Errorf("entries[1].DurationSeconds = %v, want 300", entries[1].DurationSeconds)
	}
}

// Unknown task id for ListEntries surfaces ErrNotFound (design.md route
// table: GET .../time-entries -> 404).
func TestListEntries_UnknownTask_ErrNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.ListEntries(ctx, "does-not-exist")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// AC3 (spec P1 Apontamentos #3): updating an entry recalculates its
// duration.
func TestUpdateEntry_RecalculatesDuration(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	withNow(t, base)
	if _, _, err := s.StartTimer(ctx, task.ID); err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}
	withNow(t, base.Add(5*time.Minute))
	entry, err := s.StopTimer(ctx)
	if err != nil {
		t.Fatalf("StopTimer() error = %v", err)
	}

	withNow(t, base.Add(time.Hour)) // "now" must be after the new ended_at
	newEnd := base.Add(30 * time.Minute)
	updated, err := s.UpdateEntry(ctx, entry.ID, nil, &newEnd)
	if err != nil {
		t.Fatalf("UpdateEntry() error = %v", err)
	}
	if updated.DurationSeconds == nil || *updated.DurationSeconds != 1800 {
		t.Errorf("DurationSeconds = %v, want 1800", updated.DurationSeconds)
	}
	if updated.EndedAt == nil || *updated.EndedAt != formatTime(newEnd) {
		t.Errorf("EndedAt = %v, want %q", updated.EndedAt, formatTime(newEnd))
	}
}

// AC4 (spec P1 Apontamentos #4): ended_at <= started_at is a validation
// error.
func TestUpdateEntry_EndedBeforeOrEqualStarted_ValidationError(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	withNow(t, base)
	if _, _, err := s.StartTimer(ctx, task.ID); err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}
	withNow(t, base.Add(5*time.Minute))
	entry, err := s.StopTimer(ctx)
	if err != nil {
		t.Fatalf("StopTimer() error = %v", err)
	}

	withNow(t, base.Add(time.Hour))
	equalToStart := base
	_, err = s.UpdateEntry(ctx, entry.ID, nil, &equalToStart)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if ve.Field != "ended_at" {
		t.Errorf("Field = %q, want %q", ve.Field, "ended_at")
	}
}

// AC5 (spec P1 Apontamentos #5): ended_at after the server clock is a
// validation error.
func TestUpdateEntry_EndedInFuture_ValidationError(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	withNow(t, base)
	if _, _, err := s.StartTimer(ctx, task.ID); err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}
	withNow(t, base.Add(5*time.Minute)) // "now" stays here for the update below
	entry, err := s.StopTimer(ctx)
	if err != nil {
		t.Fatalf("StopTimer() error = %v", err)
	}

	future := base.Add(time.Hour)
	_, err = s.UpdateEntry(ctx, entry.ID, nil, &future)
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if ve.Field != "ended_at" {
		t.Errorf("Field = %q, want %q", ve.Field, "ended_at")
	}
}

// AC6 (spec P1 Apontamentos #6): an edited interval that overlaps another
// entry is rejected.
func TestUpdateEntry_Overlap_ErrEntryOverlap(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	// entry1: 10:00 - 10:10
	withNow(t, base)
	if _, _, err := s.StartTimer(ctx, task.ID); err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}
	withNow(t, base.Add(10*time.Minute))
	if _, err := s.StopTimer(ctx); err != nil {
		t.Fatalf("StopTimer() error = %v", err)
	}

	// entry2: 10:20 - 10:30
	withNow(t, base.Add(20*time.Minute))
	if _, _, err := s.StartTimer(ctx, task.ID); err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}
	withNow(t, base.Add(30*time.Minute))
	entry2, err := s.StopTimer(ctx)
	if err != nil {
		t.Fatalf("StopTimer() error = %v", err)
	}

	withNow(t, base.Add(time.Hour))
	// Move entry2's start into entry1's interval (10:05, inside [10:00,10:10)).
	newStart := base.Add(5 * time.Minute)
	_, err = s.UpdateEntry(ctx, entry2.ID, &newStart, nil)
	if !errors.Is(err, ErrEntryOverlap) {
		t.Errorf("err = %v, want ErrEntryOverlap", err)
	}
}

// AC6 boundary (spec P1 Apontamentos #6 + design risk): an interval that
// only touches another (end == start) is NOT an overlap.
func TestUpdateEntry_TouchingBoundary_Allowed(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	// entry1: 10:00 - 10:10
	withNow(t, base)
	if _, _, err := s.StartTimer(ctx, task.ID); err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}
	withNow(t, base.Add(10*time.Minute))
	if _, err := s.StopTimer(ctx); err != nil {
		t.Fatalf("StopTimer() error = %v", err)
	}

	// entry2: 10:20 - 10:30
	withNow(t, base.Add(20*time.Minute))
	if _, _, err := s.StartTimer(ctx, task.ID); err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}
	withNow(t, base.Add(30*time.Minute))
	entry2, err := s.StopTimer(ctx)
	if err != nil {
		t.Fatalf("StopTimer() error = %v", err)
	}

	withNow(t, base.Add(time.Hour))
	// Move entry2's start to exactly 10:10, touching entry1's end.
	newStart := base.Add(10 * time.Minute)
	updated, err := s.UpdateEntry(ctx, entry2.ID, &newStart, nil)
	if err != nil {
		t.Fatalf("UpdateEntry() error = %v, want touching boundary to be allowed", err)
	}
	if updated.StartedAt != formatTime(newStart) {
		t.Errorf("StartedAt = %q, want %q", updated.StartedAt, formatTime(newStart))
	}
}

// AC7 (spec P1 Apontamentos #7): editing the active entry is refused.
func TestUpdateEntry_ActiveEntry_ErrEntryActive(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	entry, _, err := s.StartTimer(ctx, task.ID)
	if err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}

	newStart := time.Now().UTC()
	_, err = s.UpdateEntry(ctx, entry.ID, &newStart, nil)
	if !errors.Is(err, ErrEntryActive) {
		t.Errorf("err = %v, want ErrEntryActive", err)
	}
}

// AC7 (spec P1 Apontamentos #7): deleting the active entry is refused.
func TestDeleteEntry_ActiveEntry_ErrEntryActive(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	entry, _, err := s.StartTimer(ctx, task.ID)
	if err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}

	err = s.DeleteEntry(ctx, entry.ID)
	if !errors.Is(err, ErrEntryActive) {
		t.Errorf("err = %v, want ErrEntryActive", err)
	}
}

// AC8 (spec P1 Apontamentos #8): deleting a finished entry removes it.
func TestDeleteEntry_Finished_Removes(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, _, err := s.StartTimer(ctx, task.ID); err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}
	entry, err := s.StopTimer(ctx)
	if err != nil {
		t.Fatalf("StopTimer() error = %v", err)
	}

	if err := s.DeleteEntry(ctx, entry.ID); err != nil {
		t.Fatalf("DeleteEntry() error = %v", err)
	}

	entries, err := s.ListEntries(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListEntries() error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("len(entries) = %d, want 0", len(entries))
	}
}

// AC9 (spec P1 Apontamentos #9): unknown id on update surfaces
// ErrEntryNotFound.
func TestUpdateEntry_UnknownID_ErrEntryNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	newEnd := time.Now().UTC()
	_, err := s.UpdateEntry(ctx, "does-not-exist", nil, &newEnd)
	if !errors.Is(err, ErrEntryNotFound) {
		t.Errorf("err = %v, want ErrEntryNotFound", err)
	}
}

// AC9 (spec P1 Apontamentos #9): unknown id on delete surfaces
// ErrEntryNotFound.
func TestDeleteEntry_UnknownID_ErrEntryNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	err := s.DeleteEntry(ctx, "does-not-exist")
	if !errors.Is(err, ErrEntryNotFound) {
		t.Errorf("err = %v, want ErrEntryNotFound", err)
	}
}

// P1 Apontamentos AC2: TotalSeconds sums only finished entries, ignoring
// the currently active one.
func TestTotalSeconds_SumsOnlyFinished(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	task, err := s.Create(ctx, "Task A", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	base := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	withNow(t, base)
	if _, _, err := s.StartTimer(ctx, task.ID); err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}
	withNow(t, base.Add(5*time.Minute))
	if _, err := s.StopTimer(ctx); err != nil {
		t.Fatalf("StopTimer() error = %v", err)
	}

	withNow(t, base.Add(10*time.Minute))
	if _, _, err := s.StartTimer(ctx, task.ID); err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}
	withNow(t, base.Add(25*time.Minute))
	if _, err := s.StopTimer(ctx); err != nil {
		t.Fatalf("StopTimer() error = %v", err)
	}

	// Third entry stays active; must not count towards the total.
	withNow(t, base.Add(time.Hour))
	if _, _, err := s.StartTimer(ctx, task.ID); err != nil {
		t.Fatalf("StartTimer() error = %v", err)
	}

	total, err := s.TotalSeconds(ctx, task.ID)
	if err != nil {
		t.Fatalf("TotalSeconds() error = %v", err)
	}
	want := int64(5*60 + 15*60)
	if total != want {
		t.Errorf("TotalSeconds() = %d, want %d", total, want)
	}
}
