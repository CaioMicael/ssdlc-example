package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CaioMicael/ssdlc-example/backend/internal/store"
)

// newTestStore opens a real SQLite store on a temp-file database, so the
// task route tests exercise the real store contract. The store is closed
// automatically at the end of the test.
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "tasks.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// doRequest performs an HTTP round trip against router and decodes a JSON
// response body into out (when out is non-nil). It returns the raw response
// recorder for status/body assertions.
func doRequest(t *testing.T, router http.Handler, method, target string, body any, out any) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		switch b := body.(type) {
		case []byte:
			reader = bytes.NewReader(b)
		case string:
			reader = bytes.NewReader([]byte(b))
		default:
			raw, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal request body: %v", err)
			}
			reader = bytes.NewReader(raw)
		}
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if out != nil && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
		}
	}

	return rec
}

type taskEnvelope struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Archived     bool   `json:"archived"`
	TotalSeconds int64  `json:"total_seconds"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type taskListEnvelope struct {
	Tasks []taskEnvelope `json:"tasks"`
}

type errorEnvelope struct {
	Error struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields"`
	} `json:"error"`
}

// AC1: creating a task with a valid title returns 201 with the created
// task, defaulted to status todo, archived false, total_seconds 0.
func TestCreateTask_ValidTitle_Returns201WithDefaults(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var got taskEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "Escrever relatorio", "description": "detalhes"}, &got)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201, body=%s", rec.Code, rec.Body.String())
	}
	if got.ID == "" {
		t.Fatalf("id = %q, want non-empty", got.ID)
	}
	if got.Title != "Escrever relatorio" {
		t.Fatalf("title = %q, want %q", got.Title, "Escrever relatorio")
	}
	if got.Description != "detalhes" {
		t.Fatalf("description = %q, want %q", got.Description, "detalhes")
	}
	if got.Status != "todo" {
		t.Fatalf("status = %q, want %q", got.Status, "todo")
	}
	if got.Archived != false {
		t.Fatalf("archived = %v, want false", got.Archived)
	}
	if got.TotalSeconds != 0 {
		t.Fatalf("total_seconds = %d, want 0", got.TotalSeconds)
	}
	if got.CreatedAt == "" || got.UpdatedAt == "" {
		t.Fatalf("created_at/updated_at must be set, got %q / %q", got.CreatedAt, got.UpdatedAt)
	}
}

// AC2: an empty title (after trim) returns 422 VALIDATION_ERROR with
// fields.title set.
func TestCreateTask_EmptyTitle_Returns422WithFieldsTitle(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var got errorEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "   "}, &got)

	if rec.Code != 422 {
		t.Fatalf("status = %d, want 422, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("code = %q, want VALIDATION_ERROR", got.Error.Code)
	}
	if _, ok := got.Error.Fields["title"]; !ok {
		t.Fatalf("fields = %v, want a %q entry", got.Error.Fields, "title")
	}
}

// AC2: a title over 200 characters returns 422 VALIDATION_ERROR with
// fields.title set.
func TestCreateTask_TitleTooLong_Returns422WithFieldsTitle(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var got errorEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": strings.Repeat("a", 201)}, &got)

	if rec.Code != 422 {
		t.Fatalf("status = %d, want 422, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("code = %q, want VALIDATION_ERROR", got.Error.Code)
	}
	if _, ok := got.Error.Fields["title"]; !ok {
		t.Fatalf("fields = %v, want a %q entry", got.Error.Fields, "title")
	}
}

// AC3: a description over 2000 characters returns 422 VALIDATION_ERROR with
// fields.description set.
func TestCreateTask_DescriptionTooLong_Returns422WithFieldsDescription(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var got errorEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "ok", "description": strings.Repeat("a", 2001)}, &got)

	if rec.Code != 422 {
		t.Fatalf("status = %d, want 422, body=%s", rec.Code, rec.Body.String())
	}
	if _, ok := got.Error.Fields["description"]; !ok {
		t.Fatalf("fields = %v, want a %q entry", got.Error.Fields, "description")
	}
}

// Edge case: a malformed JSON body returns 400 INVALID_JSON.
func TestCreateTask_MalformedJSON_Returns400(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var got errorEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks", "{not json", &got)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "INVALID_JSON" {
		t.Fatalf("code = %q, want INVALID_JSON", got.Error.Code)
	}
}

// Edge case: a body over 1 MB returns 413 PAYLOAD_TOO_LARGE.
func TestCreateTask_OversizedBody_Returns413(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	huge := fmt.Sprintf(`{"title":"a","description":"%s"}`, strings.Repeat("a", 2*1024*1024))

	var got errorEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks", huge, &got)

	if rec.Code != 413 {
		t.Fatalf("status = %d, want 413, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "PAYLOAD_TOO_LARGE" {
		t.Fatalf("code = %q, want PAYLOAD_TOO_LARGE", got.Error.Code)
	}
}

// AC4: listing returns the created (non-archived) tasks.
func TestListTasks_ReturnsCreatedTasks(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "Tarefa 1"}, nil)
	doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "Tarefa 2"}, nil)

	var got taskListEnvelope
	rec := doRequest(t, router, "GET", "/api/v1/tasks", nil, &got)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if len(got.Tasks) != 2 {
		t.Fatalf("len(tasks) = %d, want 2", len(got.Tasks))
	}
}

// AC5: listing filtered by status returns only tasks with that status.
func TestListTasks_FilteredByStatus_ReturnsOnlyMatching(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var todoTask taskEnvelope
	doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "Ficara todo"}, &todoTask)

	var inProgressTask taskEnvelope
	doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "Vai mudar"}, &inProgressTask)
	doRequest(t, router, "PATCH", "/api/v1/tasks/"+inProgressTask.ID, map[string]string{"status": "in_progress"}, nil)

	var got taskListEnvelope
	rec := doRequest(t, router, "GET", "/api/v1/tasks?status=in_progress", nil, &got)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if len(got.Tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(got.Tasks))
	}
	if got.Tasks[0].ID != inProgressTask.ID {
		t.Fatalf("returned task id = %q, want %q", got.Tasks[0].ID, inProgressTask.ID)
	}
	if got.Tasks[0].Status != "in_progress" {
		t.Fatalf("returned task status = %q, want in_progress", got.Tasks[0].Status)
	}
}

// Edge case (design.md error table): an invalid status filter value returns
// 422 VALIDATION_ERROR.
func TestListTasks_InvalidStatusFilter_Returns422(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var got errorEnvelope
	rec := doRequest(t, router, "GET", "/api/v1/tasks?status=bogus", nil, &got)

	if rec.Code != 422 {
		t.Fatalf("status = %d, want 422, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("code = %q, want VALIDATION_ERROR", got.Error.Code)
	}
	if _, ok := got.Error.Fields["status"]; !ok {
		t.Fatalf("fields = %v, want a %q entry", got.Error.Fields, "status")
	}
}

// P2-AC4: listing with archived=true returns only archived tasks.
func TestListTasks_ArchivedTrue_ReturnsOnlyArchived(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var active taskEnvelope
	doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "Ativa"}, &active)

	var archived taskEnvelope
	doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "Vai arquivar"}, &archived)
	doRequest(t, router, "POST", "/api/v1/tasks/"+archived.ID+"/archive", nil, nil)

	var got taskListEnvelope
	rec := doRequest(t, router, "GET", "/api/v1/tasks?archived=true", nil, &got)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if len(got.Tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(got.Tasks))
	}
	if got.Tasks[0].ID != archived.ID {
		t.Fatalf("returned task id = %q, want %q", got.Tasks[0].ID, archived.ID)
	}
	if !got.Tasks[0].Archived {
		t.Fatalf("returned task archived = false, want true")
	}
}

// AC6: PATCH with title/status updates only the sent fields and persists
// the change (200, then visible via GET-equivalent list).
func TestUpdateTask_TitleAndStatus_Returns200AndPersists(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var created taskEnvelope
	doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "Original", "description": "desc original"}, &created)

	var updated taskEnvelope
	rec := doRequest(t, router, "PATCH", "/api/v1/tasks/"+created.ID, map[string]string{"title": "Novo titulo", "status": "done"}, &updated)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if updated.Title != "Novo titulo" {
		t.Fatalf("title = %q, want %q", updated.Title, "Novo titulo")
	}
	if updated.Status != "done" {
		t.Fatalf("status = %q, want done", updated.Status)
	}
	if updated.Description != "desc original" {
		t.Fatalf("description = %q, want unchanged %q", updated.Description, "desc original")
	}

	// Persistence check: fetch through the store directly, not just trust
	// the PATCH response.
	persisted, err := s.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("s.Get: %v", err)
	}
	if persisted.Title != "Novo titulo" || persisted.Status != "done" {
		t.Fatalf("persisted task = %+v, want title=Novo titulo status=done", persisted)
	}
}

// AC7: an invalid status value on PATCH returns 422 with fields.status.
func TestUpdateTask_InvalidStatus_Returns422WithFieldsStatus(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var created taskEnvelope
	doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "Original"}, &created)

	var got errorEnvelope
	rec := doRequest(t, router, "PATCH", "/api/v1/tasks/"+created.ID, map[string]string{"status": "bogus"}, &got)

	if rec.Code != 422 {
		t.Fatalf("status = %d, want 422, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("code = %q, want VALIDATION_ERROR", got.Error.Code)
	}
	if _, ok := got.Error.Fields["status"]; !ok {
		t.Fatalf("fields = %v, want a %q entry", got.Error.Fields, "status")
	}
}

// AC8: PATCH on an unknown id returns 404 TASK_NOT_FOUND.
func TestUpdateTask_UnknownID_Returns404(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var got errorEnvelope
	rec := doRequest(t, router, "PATCH", "/api/v1/tasks/nao-existe", map[string]string{"title": "x"}, &got)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "TASK_NOT_FOUND" {
		t.Fatalf("code = %q, want TASK_NOT_FOUND", got.Error.Code)
	}
}

// P2-AC5: PATCH on an archived task returns 409 TASK_ARCHIVED.
func TestUpdateTask_ArchivedTask_Returns409(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var created taskEnvelope
	doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "Vai arquivar"}, &created)
	doRequest(t, router, "POST", "/api/v1/tasks/"+created.ID+"/archive", nil, nil)

	var got errorEnvelope
	rec := doRequest(t, router, "PATCH", "/api/v1/tasks/"+created.ID, map[string]string{"title": "tentativa"}, &got)

	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "TASK_ARCHIVED" {
		t.Fatalf("code = %q, want TASK_ARCHIVED", got.Error.Code)
	}
}

// P2-AC1/AC4: archiving a task returns 200 and the task disappears from the
// default (non-archived) list.
func TestArchiveTask_Returns200AndDisappearsFromDefaultList(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var created taskEnvelope
	doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "Vai sumir"}, &created)

	var archived taskEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks/"+created.ID+"/archive", nil, &archived)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if !archived.Archived {
		t.Fatalf("archived = false, want true")
	}

	var list taskListEnvelope
	doRequest(t, router, "GET", "/api/v1/tasks", nil, &list)
	for _, task := range list.Tasks {
		if task.ID == created.ID {
			t.Fatalf("archived task %q still present in default list", created.ID)
		}
	}
}

// P2-AC3: restoring a task returns 200 and the task returns to the default
// list.
func TestRestoreTask_Returns200AndReturnsToDefaultList(t *testing.T) {
	s := newTestStore(t)
	router := NewRouter(t.TempDir(), s)

	var created taskEnvelope
	doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "Vai e volta"}, &created)
	doRequest(t, router, "POST", "/api/v1/tasks/"+created.ID+"/archive", nil, nil)

	var restored taskEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks/"+created.ID+"/restore", nil, &restored)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if restored.Archived {
		t.Fatalf("archived = true, want false")
	}

	var list taskListEnvelope
	doRequest(t, router, "GET", "/api/v1/tasks", nil, &list)
	found := false
	for _, task := range list.Tasks {
		if task.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("restored task %q not present in default list", created.ID)
	}
}

// fakeFailingStore forces an unexpected (non-typed) store error, so the
// 500 mapping can be tested without depending on a real driver failure.
type fakeFailingStore struct{}

func (fakeFailingStore) Create(ctx context.Context, title, description string) (store.Task, error) {
	return store.Task{}, fmt.Errorf("driver connection reset by peer at 10.0.0.5:5432")
}

func (fakeFailingStore) List(ctx context.Context, filter store.Filter) ([]store.Task, error) {
	return nil, fmt.Errorf("driver connection reset by peer at 10.0.0.5:5432")
}

func (fakeFailingStore) Get(ctx context.Context, id string) (store.Task, error) {
	return store.Task{}, fmt.Errorf("driver connection reset by peer at 10.0.0.5:5432")
}

func (fakeFailingStore) Update(ctx context.Context, id string, p store.Patch) (store.Task, error) {
	return store.Task{}, fmt.Errorf("driver connection reset by peer at 10.0.0.5:5432")
}

func (fakeFailingStore) SetArchived(ctx context.Context, id string, archived bool) (store.Task, error) {
	return store.Task{}, fmt.Errorf("driver connection reset by peer at 10.0.0.5:5432")
}

func (fakeFailingStore) Ping(ctx context.Context) error {
	return fmt.Errorf("driver connection reset by peer at 10.0.0.5:5432")
}

func (fakeFailingStore) StartTimer(ctx context.Context, taskID string) (store.TimeEntry, bool, error) {
	return store.TimeEntry{}, false, fmt.Errorf("driver connection reset by peer at 10.0.0.5:5432")
}

func (fakeFailingStore) StopTimer(ctx context.Context) (store.TimeEntry, error) {
	return store.TimeEntry{}, fmt.Errorf("driver connection reset by peer at 10.0.0.5:5432")
}

func (fakeFailingStore) ActiveTimer(ctx context.Context) (store.TimeEntry, store.Task, bool, error) {
	return store.TimeEntry{}, store.Task{}, false, fmt.Errorf("driver connection reset by peer at 10.0.0.5:5432")
}

func (fakeFailingStore) TotalSeconds(ctx context.Context, taskID string) (int64, error) {
	return 0, fmt.Errorf("driver connection reset by peer at 10.0.0.5:5432")
}

func (fakeFailingStore) ListEntries(ctx context.Context, taskID string) ([]store.TimeEntry, error) {
	return nil, fmt.Errorf("driver connection reset by peer at 10.0.0.5:5432")
}

func (fakeFailingStore) UpdateEntry(ctx context.Context, id string, startedAt, endedAt *time.Time) (store.TimeEntry, error) {
	return store.TimeEntry{}, fmt.Errorf("driver connection reset by peer at 10.0.0.5:5432")
}

func (fakeFailingStore) DeleteEntry(ctx context.Context, id string) error {
	return fmt.Errorf("driver connection reset by peer at 10.0.0.5:5432")
}

// Edge case: an unexpected store failure maps to 500 INTERNAL_ERROR with a
// generic message, never the underlying driver text.
func TestCreateTask_UnexpectedStoreError_Returns500WithGenericMessage(t *testing.T) {
	router := NewRouter(t.TempDir(), fakeFailingStore{})

	var got errorEnvelope
	rec := doRequest(t, router, "POST", "/api/v1/tasks", map[string]string{"title": "qualquer"}, &got)

	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500, body=%s", rec.Code, rec.Body.String())
	}
	if got.Error.Code != "INTERNAL_ERROR" {
		t.Fatalf("code = %q, want INTERNAL_ERROR", got.Error.Code)
	}
	if strings.Contains(rec.Body.String(), "driver connection reset") || strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Fatalf("response leaked driver error detail: %s", rec.Body.String())
	}
}
