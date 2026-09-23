package httpapi

import (
	"net/http"
	"time"
)

// patchEntryRequest is the request body for PATCH /api/v1/time-entries/{id}.
// Nil fields keep their current value.
type patchEntryRequest struct {
	StartedAt *time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at"`
}

// registerEntryRoutes adds the time entry list/patch/delete routes to mux.
func registerEntryRoutes(mux *http.ServeMux, s TaskStore) {
	mux.HandleFunc("GET /api/v1/tasks/{id}/time-entries", handleListEntries(s))
	mux.HandleFunc("PATCH /api/v1/time-entries/{id}", handleUpdateEntry(s))
	mux.HandleFunc("DELETE /api/v1/time-entries/{id}", handleDeleteEntry(s))
}

// handleListEntries returns a task's time entries ordered by started_at
// descending (spec P1 "Consultar e corrigir apontamentos" AC1).
func handleListEntries(s TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		entries, err := s.ListEntries(r.Context(), id)
		if err != nil {
			writeStoreError(w, err)
			return
		}

		resp := make([]timeEntryResponse, 0, len(entries))
		for _, e := range entries {
			resp = append(resp, toTimeEntryResponse(e))
		}
		writeJSON(w, http.StatusOK, map[string]any{"entries": resp})
	}
}

// handleUpdateEntry edits a finished time entry's started_at/ended_at and
// responds with the recalculated duration (spec P1 "Consultar e corrigir
// apontamentos" AC3-7).
func handleUpdateEntry(s TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req patchEntryRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}

		entry, err := s.UpdateEntry(r.Context(), id, req.StartedAt, req.EndedAt)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, toTimeEntryResponse(entry))
	}
}

// handleDeleteEntry permanently removes a finished time entry (spec P1
// "Consultar e corrigir apontamentos" AC8-9).
func handleDeleteEntry(s TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		if err := s.DeleteEntry(r.Context(), id); err != nil {
			writeStoreError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
