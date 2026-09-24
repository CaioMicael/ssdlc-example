package httpapi

import (
	"net/http"

	"github.com/CaioMicael/ssdlc-example/backend/internal/store"
)

// timeEntryResponse is the JSON shape returned for a single time entry.
// EndedAt and DurationSeconds are omitted (null) while the entry is active.
type timeEntryResponse struct {
	ID              string  `json:"id"`
	TaskID          string  `json:"task_id"`
	StartedAt       string  `json:"started_at"`
	EndedAt         *string `json:"ended_at"`
	DurationSeconds *int64  `json:"duration_seconds"`
}

func toTimeEntryResponse(e store.TimeEntry) timeEntryResponse {
	return timeEntryResponse{
		ID:              e.ID,
		TaskID:          e.TaskID,
		StartedAt:       e.StartedAt,
		EndedAt:         e.EndedAt,
		DurationSeconds: e.DurationSeconds,
	}
}

// registerTimerRoutes adds the timer start/stop/active routes to mux.
func registerTimerRoutes(mux *http.ServeMux, s TaskStore) {
	mux.HandleFunc("POST /api/v1/tasks/{id}/timer/start", handleStartTimer(s))
	mux.HandleFunc("POST /api/v1/timer/stop", handleStopTimer(s))
	mux.HandleFunc("GET /api/v1/timer", handleGetTimer(s))
}

// handleStartTimer starts a timer on the task in the URL. It responds 201
// when a new entry is created, or 200 with the existing entry when that
// task's timer was already running (spec P1 "Cronômetro" AC1-3).
func handleStartTimer(s TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		entry, created, err := s.StartTimer(r.Context(), id)
		if err != nil {
			writeStoreError(w, err)
			return
		}

		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		writeJSON(w, status, toTimeEntryResponse(entry))
	}
}

// handleStopTimer stops the currently active timer, if any (spec P1
// "Cronômetro" AC5-6).
func handleStopTimer(s TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entry, err := s.StopTimer(r.Context())
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toTimeEntryResponse(entry))
	}
}

// activeTimerResponse is the JSON shape returned by GET /api/v1/timer when
// a timer is running.
type activeTimerResponse struct {
	Entry timeEntryResponse `json:"entry"`
	Task  taskResponse      `json:"task"`
}

// handleGetTimer returns the active timer and its task, or a literal JSON
// null when no timer is running (spec P1 "Cronômetro" AC8).
func handleGetTimer(s TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entry, task, ok, err := s.ActiveTimer(r.Context())
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if !ok {
			writeJSON(w, http.StatusOK, nil)
			return
		}

		taskResp, err := toTaskResponse(r.Context(), s, task)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, activeTimerResponse{Entry: toTimeEntryResponse(entry), Task: taskResp})
	}
}
