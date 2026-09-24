package httpapi

import (
	"net/http"
	"testing"
	"time"
)

type timeEntryEnvelope struct {
	ID              string  `json:"id"`
	TaskID          string  `json:"task_id"`
	StartedAt       string  `json:"started_at"`
	EndedAt         *string `json:"ended_at"`
	DurationSeconds *int64  `json:"duration_seconds"`
}

type activeTimerEnvelope struct {
	Entry timeEntryEnvelope `json:"entry"`
	Task  taskEnvelope      `json:"task"`
}

// createTaskForTimer creates a task via HTTP and returns its id.
func createTaskForTimer(t *testing.T, router http.Handler) string {
	t.Helper()
	var got taskEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "Tarefa"}, &got)
	if rec.Code != 201 {
		t.Fatalf("create task setup: status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}
	return got.ID
}

// AC1: starting a timer on a task with no active timer creates an entry and
// responds 201.
func TestStartTimer_NewTask_Returns201WithEntry(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)

	var got timeEntryEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks/"+taskID+"/timer/start", nil, &got)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}
	if got.ID == "" {
		t.Fatalf("id = %q, want non-empty", got.ID)
	}
	if got.TaskID != taskID {
		t.Fatalf("task_id = %q, want %q", got.TaskID, taskID)
	}
	if got.StartedAt == "" {
		t.Fatalf("started_at must be set")
	}
	if got.EndedAt != nil {
		t.Fatalf("ended_at = %v, want nil", got.EndedAt)
	}
}

// AC3: starting a timer again on the task that already has it running
// returns 200 with the existing entry, without creating a duplicate.
func TestStartTimer_SameTaskAgain_Returns200WithoutDuplicate(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)

	var first timeEntryEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks/"+taskID+"/timer/start", nil, &first)
	if rec.Code != 201 {
		t.Fatalf("first start: status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}

	var second timeEntryEnvelope
	rec = doRequest(t, router, "POST", "/api/v1/tasks/"+taskID+"/timer/start", nil, &second)
	if rec.Code != 200 {
		t.Fatalf("second start: status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if second.ID != first.ID {
		t.Fatalf("second start id = %q, want same as first %q (no duplicate)", second.ID, first.ID)
	}

	entries, err := s.ListEntries(t.Context(), taskID)
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries for task = %d, want 1 (no duplicate created)", len(entries))
	}
}

// AC2: starting a timer on task B while task A has an active timer finishes
// A's entry and creates B's, responding 201.
func TestStartTimer_OtherTaskFinishesPrevious_Returns201(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskA := createTaskForTimer(t, router)
	taskB := createTaskForTimer(t, router)

	rec := doRequest(t, router, "POST", "/api/v1/tasks/"+taskA+"/timer/start", nil, nil)
	if rec.Code != 201 {
		t.Fatalf("start A: status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}

	var gotB timeEntryEnvelope
	rec = doRequest(t, router, "POST", "/api/v1/tasks/"+taskB+"/timer/start", nil, &gotB)
	if rec.Code != 201 {
		t.Fatalf("start B: status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}
	if gotB.TaskID != taskB {
		t.Fatalf("task_id = %q, want %q", gotB.TaskID, taskB)
	}

	var timer activeTimerEnvelope
	rec = doRequest(t, router, "GET", "/api/v1/timer", nil, &timer)
	if rec.Code != 200 {
		t.Fatalf("get timer: status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if timer.Task.ID != taskB {
		t.Fatalf("active timer task = %q, want %q (B)", timer.Task.ID, taskB)
	}

	entriesA, err := s.ListEntries(t.Context(), taskA)
	if err != nil {
		t.Fatalf("ListEntries A: %v", err)
	}
	if len(entriesA) != 1 || entriesA[0].EndedAt == nil {
		t.Fatalf("task A entries = %+v, want exactly 1 finished entry", entriesA)
	}
}

// AC4: starting a timer on a done task is refused with 409
// TASK_NOT_TRACKABLE.
func TestStartTimer_DoneTask_Returns409TaskNotTrackable(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)

	rec := doRequest(t, router, "PATCH", "/api/v1/tasks/"+taskID, map[string]string{"status": "done"}, nil)
	if rec.Code != 200 {
		t.Fatalf("mark done: status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var got errorEnvelope
	rec = doRequest(t, router, "POST", "/api/v1/tasks/"+taskID+"/timer/start", nil, &got)
	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "TASK_NOT_TRACKABLE" {
		t.Fatalf("code = %q, want TASK_NOT_TRACKABLE", got.Error.Code)
	}
}

// AC4: starting a timer on an archived task is refused with 409
// TASK_NOT_TRACKABLE.
func TestStartTimer_ArchivedTask_Returns409TaskNotTrackable(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)

	rec := doRequest(t, router, "POST", "/api/v1/tasks/"+taskID+"/archive", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("archive: status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var got errorEnvelope
	rec = doRequest(t, router, "POST", "/api/v1/tasks/"+taskID+"/timer/start", nil, &got)
	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "TASK_NOT_TRACKABLE" {
		t.Fatalf("code = %q, want TASK_NOT_TRACKABLE", got.Error.Code)
	}
}

// Edge case (unknown id -> resource's not-found code): starting a timer on
// an unknown task id returns 404 TASK_NOT_FOUND.
func TestStartTimer_UnknownID_Returns404(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var got errorEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks/does-not-exist/timer/start", nil, &got)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "TASK_NOT_FOUND" {
		t.Fatalf("code = %q, want TASK_NOT_FOUND", got.Error.Code)
	}
}

// AC5: stopping the active timer sets ended_at and responds 200 with the
// finished entry.
func TestStopTimer_Returns200WithEndedAt(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)

	rec := doRequest(t, router, "POST", "/api/v1/tasks/"+taskID+"/timer/start", nil, nil)
	if rec.Code != 201 {
		t.Fatalf("start: status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}

	var got timeEntryEnvelope
	rec = doRequest(t, router, "POST", "/api/v1/timer/stop", nil, &got)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if got.EndedAt == nil || *got.EndedAt == "" {
		t.Fatalf("ended_at = %v, want set", got.EndedAt)
	}
	if got.DurationSeconds == nil {
		t.Fatalf("duration_seconds = nil, want set")
	}
}

// AC6: stopping with no active timer returns 409 NO_ACTIVE_TIMER.
func TestStopTimer_NoActive_Returns409NoActiveTimer(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var got errorEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/timer/stop", nil, &got)

	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "NO_ACTIVE_TIMER" {
		t.Fatalf("code = %q, want NO_ACTIVE_TIMER", got.Error.Code)
	}
}

// AC8: GET /api/v1/timer with no active timer returns 200 with a literal
// JSON null body.
func TestGetTimer_NoActive_ReturnsNull(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	rec := doRequest(t, router, "GET", "/api/v1/timer", nil, nil)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	trimmed := body
	for len(trimmed) > 0 && (trimmed[len(trimmed)-1] == '\n' || trimmed[len(trimmed)-1] == ' ') {
		trimmed = trimmed[:len(trimmed)-1]
	}
	if trimmed != "null" {
		t.Fatalf("body = %q, want literal null", body)
	}
}

// AC8: GET /api/v1/timer with an active timer returns 200 with the entry
// and its task.
func TestGetTimer_Active_ReturnsEntryAndTask(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)

	rec := doRequest(t, router, "POST", "/api/v1/tasks/"+taskID+"/timer/start", nil, nil)
	if rec.Code != 201 {
		t.Fatalf("start: status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}

	var got activeTimerEnvelope
	rec = doRequest(t, router, "GET", "/api/v1/timer", nil, &got)
	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if got.Entry.TaskID != taskID {
		t.Fatalf("entry.task_id = %q, want %q", got.Entry.TaskID, taskID)
	}
	if got.Task.ID != taskID {
		t.Fatalf("task.id = %q, want %q", got.Task.ID, taskID)
	}
}

// P1 "Consultar e corrigir apontamentos" AC2: a task's total_seconds is the
// real sum of its finished entries, not a hardcoded value.
func TestTotalSeconds_ReflectsFinishedEntries(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)

	var started timeEntryEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks/"+taskID+"/timer/start", nil, &started)
	if rec.Code != 201 {
		t.Fatalf("start: status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}

	rec = doRequest(t, router, "POST", "/api/v1/timer/stop", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("stop: status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	// Force a known 30-minute duration directly through the store (bypassing
	// the real clock, which would make an exact assertion flaky), then
	// verify the HTTP response reports that exact sum.
	base := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	end := base.Add(30 * time.Minute)
	if _, err := s.UpdateEntry(t.Context(), started.ID, &base, &end); err != nil {
		t.Fatalf("UpdateEntry setup: %v", err)
	}

	var got taskEnvelope
	rec = doRequest(t, router, "PATCH", "/api/v1/tasks/"+taskID, map[string]string{}, &got)
	if rec.Code != 200 {
		t.Fatalf("get task via patch: status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if got.TotalSeconds != 1800 {
		t.Fatalf("total_seconds = %d, want 1800", got.TotalSeconds)
	}
}
