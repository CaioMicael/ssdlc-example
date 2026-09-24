package httpapi

import (
	"net/http"
	"testing"
	"time"
)

type entriesListEnvelope struct {
	Entries []timeEntryEnvelope `json:"entries"`
}

// startAndStop starts and immediately stops a timer on taskID via HTTP,
// returning the finished entry's id.
func startAndStop(t *testing.T, router http.Handler, taskID string) string {
	t.Helper()
	var started timeEntryEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks/"+taskID+"/timer/start", nil, &started)
	if rec.Code != 201 {
		t.Fatalf("start setup: status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}
	rec = doRequest(t, router, "POST", "/api/v1/timer/stop", nil, nil)
	if rec.Code != 200 {
		t.Fatalf("stop setup: status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	return started.ID
}

// AC1: listing a task's time entries returns 200 with duration_seconds set
// for the finished entry.
func TestListEntries_Returns200WithDurationSeconds(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)
	entryID := startAndStop(t, router, taskID)

	var got entriesListEnvelope
	rec := doRequest(t, router, "GET", "/api/v1/tasks/"+taskID+"/time-entries", nil, &got)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if len(got.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(got.Entries))
	}
	if got.Entries[0].ID != entryID {
		t.Fatalf("entry id = %q, want %q", got.Entries[0].ID, entryID)
	}
	if got.Entries[0].DurationSeconds == nil {
		t.Fatalf("duration_seconds = nil, want set")
	}
}

// Edge case (unknown id -> resource's not-found code): listing entries for
// an unknown task returns 404.
func TestListEntries_UnknownTask_Returns404(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var got errorEnvelope
	rec := doRequest(t, router, "GET", "/api/v1/tasks/does-not-exist/time-entries", nil, &got)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "TASK_NOT_FOUND" {
		t.Fatalf("code = %q, want TASK_NOT_FOUND", got.Error.Code)
	}
}

// AC3: patching started_at/ended_at recalculates duration_seconds and
// responds 200.
func TestUpdateEntry_ValidRange_Returns200WithRecalculatedDuration(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)
	entryID := startAndStop(t, router, taskID)

	started := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	ended := started.Add(30 * time.Minute)
	body := map[string]string{
		"started_at": started.Format(time.RFC3339),
		"ended_at":   ended.Format(time.RFC3339),
	}

	var got timeEntryEnvelope
	rec := doRequest(t, router, "PATCH", "/api/v1/time-entries/"+entryID, body, &got)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if got.DurationSeconds == nil || *got.DurationSeconds != 1800 {
		t.Fatalf("duration_seconds = %v, want 1800", got.DurationSeconds)
	}
}

// AC4: an ended_at not after started_at returns 422 VALIDATION_ERROR.
func TestUpdateEntry_EndedNotAfterStarted_Returns422(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)
	entryID := startAndStop(t, router, taskID)

	ts := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	body := map[string]string{
		"started_at": ts.Format(time.RFC3339),
		"ended_at":   ts.Format(time.RFC3339),
	}

	var got errorEnvelope
	rec := doRequest(t, router, "PATCH", "/api/v1/time-entries/"+entryID, body, &got)

	if rec.Code != 422 {
		t.Fatalf("status = %d, want 422, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("code = %q, want VALIDATION_ERROR", got.Error.Code)
	}
}

// AC5: an ended_at after the server clock returns 422 VALIDATION_ERROR.
func TestUpdateEntry_FutureEndedAt_Returns422(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)
	entryID := startAndStop(t, router, taskID)

	future := time.Now().UTC().Add(24 * time.Hour)
	body := map[string]string{"ended_at": future.Format(time.RFC3339)}

	var got errorEnvelope
	rec := doRequest(t, router, "PATCH", "/api/v1/time-entries/"+entryID, body, &got)

	if rec.Code != 422 {
		t.Fatalf("status = %d, want 422, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("code = %q, want VALIDATION_ERROR", got.Error.Code)
	}
}

// AC6: an edited interval that overlaps another entry returns 422
// TIME_ENTRY_OVERLAP.
func TestUpdateEntry_Overlap_Returns422TimeEntryOverlap(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)

	entryA := startAndStop(t, router, taskID)
	aStart := time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)
	aEnd := aStart.Add(1 * time.Hour)
	if _, err := s.UpdateEntry(t.Context(), entryA, &aStart, &aEnd); err != nil {
		t.Fatalf("UpdateEntry A setup: %v", err)
	}

	entryB := startAndStop(t, router, taskID)
	body := map[string]string{
		"started_at": aStart.Add(30 * time.Minute).Format(time.RFC3339),
		"ended_at":   aStart.Add(90 * time.Minute).Format(time.RFC3339),
	}

	var got errorEnvelope
	rec := doRequest(t, router, "PATCH", "/api/v1/time-entries/"+entryB, body, &got)

	if rec.Code != 422 {
		t.Fatalf("status = %d, want 422, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "TIME_ENTRY_OVERLAP" {
		t.Fatalf("code = %q, want TIME_ENTRY_OVERLAP", got.Error.Code)
	}
}

// AC7: patching an active (still running) entry returns 409
// TIME_ENTRY_ACTIVE.
func TestUpdateEntry_Active_Returns409TimeEntryActive(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)

	var started timeEntryEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks/"+taskID+"/timer/start", nil, &started)
	if rec.Code != 201 {
		t.Fatalf("start setup: status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}

	ts := time.Now().UTC().Add(-time.Hour)
	body := map[string]string{"started_at": ts.Format(time.RFC3339)}

	var got errorEnvelope
	rec = doRequest(t, router, "PATCH", "/api/v1/time-entries/"+started.ID, body, &got)

	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "TIME_ENTRY_ACTIVE" {
		t.Fatalf("code = %q, want TIME_ENTRY_ACTIVE", got.Error.Code)
	}
}

// AC9: patching an unknown entry id returns 404 TIME_ENTRY_NOT_FOUND.
func TestUpdateEntry_UnknownID_Returns404(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var got errorEnvelope
	rec := doRequest(t, router, "PATCH", "/api/v1/time-entries/does-not-exist", map[string]string{}, &got)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "TIME_ENTRY_NOT_FOUND" {
		t.Fatalf("code = %q, want TIME_ENTRY_NOT_FOUND", got.Error.Code)
	}
}

// AC8: deleting a finished entry removes it and responds 204.
func TestDeleteEntry_Finished_Returns204(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)
	entryID := startAndStop(t, router, taskID)

	rec := doRequest(t, router, "DELETE", "/api/v1/time-entries/"+entryID, nil, nil)
	if rec.Code != 204 {
		t.Fatalf("status = %d, want 204, body=%s", rec.Code, rec.Body.String())
	}

	entries, err := s.ListEntries(t.Context(), taskID)
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("entries after delete = %d, want 0", len(entries))
	}
}

// AC7: deleting an active (still running) entry returns 409
// TIME_ENTRY_ACTIVE.
func TestDeleteEntry_Active_Returns409TimeEntryActive(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)
	taskID := createTaskForTimer(t, router)

	var started timeEntryEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks/"+taskID+"/timer/start", nil, &started)
	if rec.Code != 201 {
		t.Fatalf("start setup: status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}

	var got errorEnvelope
	rec = doRequest(t, router, "DELETE", "/api/v1/time-entries/"+started.ID, nil, &got)

	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "TIME_ENTRY_ACTIVE" {
		t.Fatalf("code = %q, want TIME_ENTRY_ACTIVE", got.Error.Code)
	}
}

// Edge case (unknown id -> resource's not-found code): deleting an unknown
// entry returns 404 TIME_ENTRY_NOT_FOUND.
func TestDeleteEntry_UnknownID_Returns404(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var got errorEnvelope
	rec := doRequest(t, router, "DELETE", "/api/v1/time-entries/does-not-exist", nil, &got)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "TIME_ENTRY_NOT_FOUND" {
		t.Fatalf("code = %q, want TIME_ENTRY_NOT_FOUND", got.Error.Code)
	}
}
