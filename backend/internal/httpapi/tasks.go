package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/CaioMicael/ssdlc-example/backend/internal/store"
)

// maxBodyBytes is the request body size limit (1 MB) enforced on every
// endpoint that reads a JSON body.
const maxBodyBytes = 1 << 20

// pageSize is the fixed number of tasks returned per page in the list
// endpoint.
const pageSize = 50

// TaskStore is the subset of store.Store's behavior the task handlers need.
// Declared consumer-side so tests can supply a fake without depending on the
// SQLite driver.
type TaskStore interface {
	Create(ctx context.Context, title, description string) (store.Task, error)
	List(ctx context.Context, filter store.Filter) ([]store.Task, error)
	Get(ctx context.Context, id string) (store.Task, error)
	Update(ctx context.Context, id string, p store.Patch) (store.Task, error)
	SetArchived(ctx context.Context, id string, archived bool) (store.Task, error)
	Ping(ctx context.Context) error
}

// taskResponse is the JSON shape returned for a single task. total_seconds
// is always 0 in this slice: time entries are introduced in a later slice
// (fatia 2) and are not persisted yet.
type taskResponse struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Archived     bool   `json:"archived"`
	TotalSeconds int64  `json:"total_seconds"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func toTaskResponse(t store.Task) taskResponse {
	return taskResponse{
		ID:           t.ID,
		Title:        t.Title,
		Description:  t.Description,
		Status:       t.Status,
		Archived:     t.Archived,
		TotalSeconds: 0,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
	}
}

type createTaskRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type patchTaskRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

// registerTaskRoutes adds the task CRUD and archive/restore routes to mux.
func registerTaskRoutes(mux *http.ServeMux, s TaskStore) {
	mux.HandleFunc("POST /api/v1/tasks", handleCreateTask(s))
	mux.HandleFunc("GET /api/v1/tasks", handleListTasks(s))
	mux.HandleFunc("PATCH /api/v1/tasks/{id}", handleUpdateTask(s))
	mux.HandleFunc("POST /api/v1/tasks/{id}/archive", handleArchiveTask(s))
	mux.HandleFunc("POST /api/v1/tasks/{id}/restore", handleRestoreTask(s))
}

func handleCreateTask(s TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createTaskRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}

		task, err := s.Create(r.Context(), req.Title, req.Description)
		if err != nil {
			writeStoreError(w, err)
			return
		}

		writeJSON(w, http.StatusCreated, toTaskResponse(task))
	}
}

func handleListTasks(s TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		status := q.Get("status")
		if status != "" && !isValidStatus(status) {
			msg := "status must be one of todo, in_progress, done"
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", msg, map[string]string{"status": msg})
			return
		}

		archived := false
		if v := q.Get("archived"); v != "" {
			parsed, err := strconv.ParseBool(v)
			if err == nil {
				archived = parsed
			}
		}

		page := 1
		if v := q.Get("page"); v != "" {
			if p, err := strconv.Atoi(v); err == nil && p > 0 {
				page = p
			}
		}

		filter := store.Filter{
			Status:   status,
			Archived: archived,
			Limit:    pageSize,
			Offset:   (page - 1) * pageSize,
		}

		tasks, err := s.List(r.Context(), filter)
		if err != nil {
			writeStoreError(w, err)
			return
		}

		resp := make([]taskResponse, 0, len(tasks))
		for _, t := range tasks {
			resp = append(resp, toTaskResponse(t))
		}

		writeJSON(w, http.StatusOK, map[string]any{"tasks": resp})
	}
}

func isValidStatus(status string) bool {
	switch status {
	case store.StatusTodo, store.StatusInProgress, store.StatusDone:
		return true
	default:
		return false
	}
}

func handleUpdateTask(s TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req patchTaskRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}

		patch := store.Patch{
			Title:       req.Title,
			Description: req.Description,
			Status:      req.Status,
		}

		task, err := s.Update(r.Context(), id, patch)
		if err != nil {
			writeStoreError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, toTaskResponse(task))
	}
}

func handleArchiveTask(s TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		task, err := s.SetArchived(r.Context(), id, true)
		if err != nil {
			writeStoreError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, toTaskResponse(task))
	}
}

func handleRestoreTask(s TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		task, err := s.SetArchived(r.Context(), id, false)
		if err != nil {
			writeStoreError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, toTaskResponse(task))
	}
}

// decodeJSONBody enforces the 1 MB body limit and decodes r.Body's JSON into
// v. On failure it writes the appropriate error response (413 or 400) and
// returns false; callers must stop processing the request in that case.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body exceeds 1 MB", nil)
			return false
		}
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body is not valid JSON", nil)
		return false
	}

	return true
}

// writeStoreError maps a store error to the corresponding HTTP error
// response. Unexpected errors are logged with their detail and answered
// with a generic message, never the driver text.
func writeStoreError(w http.ResponseWriter, err error) {
	var verr *store.ValidationError
	switch {
	case errors.As(err, &verr):
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", verr.Message, map[string]string{verr.Field: verr.Message})
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "TASK_NOT_FOUND", "task not found", nil)
	case errors.Is(err, store.ErrArchived):
		writeError(w, http.StatusConflict, "TASK_ARCHIVED", "task is archived", nil)
	default:
		slog.Error("unexpected store error", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", nil)
	}
}

type errorResponse struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, message string, fields map[string]string) {
	writeJSON(w, status, errorResponse{Error: errorDetail{Code: code, Message: message, Fields: fields}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
