package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "app.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// AC1 (spec P1 #1): creating a task sets status=todo and archived=false.
func TestCreate_SetsDefaults(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	task, err := s.Create(ctx, "Buy milk", "2 liters")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if task.Status != StatusTodo {
		t.Errorf("Status = %q, want %q", task.Status, StatusTodo)
	}
	if task.Archived != false {
		t.Errorf("Archived = %v, want false", task.Archived)
	}
	if task.Title != "Buy milk" {
		t.Errorf("Title = %q, want %q", task.Title, "Buy milk")
	}
	if task.ID == "" {
		t.Error("ID is empty, want a generated UUID")
	}
	if task.CreatedAt == "" || task.UpdatedAt == "" {
		t.Error("CreatedAt/UpdatedAt are empty, want timestamps")
	}
}

// AC2 (spec P1 #2): empty title after trim -> ValidationError on field "title".
func TestCreate_EmptyTitleAfterTrim_ValidationError(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.Create(ctx, "   ", "desc")

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if ve.Field != "title" {
		t.Errorf("Field = %q, want %q", ve.Field, "title")
	}
}

// AC2 (spec P1 #2): title over 200 chars -> ValidationError on field "title".
func TestCreate_TitleOver200Chars_ValidationError(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	longTitle := make([]byte, 201)
	for i := range longTitle {
		longTitle[i] = 'a'
	}

	_, err := s.Create(ctx, string(longTitle), "")

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if ve.Field != "title" {
		t.Errorf("Field = %q, want %q", ve.Field, "title")
	}
}

// AC3 (spec P1 #3): description over 2000 chars -> ValidationError on field "description".
func TestCreate_DescriptionOver2000Chars_ValidationError(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	longDesc := make([]byte, 2001)
	for i := range longDesc {
		longDesc[i] = 'b'
	}

	_, err := s.Create(ctx, "Valid title", string(longDesc))

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if ve.Field != "description" {
		t.Errorf("Field = %q, want %q", ve.Field, "description")
	}
}

// AC4 (spec P1 #4): List orders by updated_at descending.
func TestList_OrdersByUpdatedAtDescending(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	first, err := s.Create(ctx, "First", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	second, err := s.Create(ctx, "Second", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Touch "first" so it becomes the most recently updated.
	title := "First updated"
	if _, err := s.Update(ctx, first.ID, Patch{Title: &title}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	tasks, err := s.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("len(tasks) = %d, want 2", len(tasks))
	}
	if tasks[0].ID != first.ID {
		t.Errorf("tasks[0].ID = %q, want %q (most recently updated first)", tasks[0].ID, first.ID)
	}
	if tasks[1].ID != second.ID {
		t.Errorf("tasks[1].ID = %q, want %q", tasks[1].ID, second.ID)
	}
}

// AC5 (spec P1 #5): List filters by status.
func TestList_FiltersByStatus(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	todoTask, err := s.Create(ctx, "Todo task", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	doneTask, err := s.Create(ctx, "Done task", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	doneStatus := StatusDone
	if _, err := s.Update(ctx, doneTask.ID, Patch{Status: &doneStatus}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	tasks, err := s.List(ctx, Filter{Status: StatusDone})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].ID != doneTask.ID {
		t.Errorf("tasks[0].ID = %q, want %q", tasks[0].ID, doneTask.ID)
	}
	if tasks[0].ID == todoTask.ID {
		t.Error("filtered result includes the todo task, want only done tasks")
	}
}

// ARC-01 (spec P2 #4): List filters by archived vs not.
func TestList_FiltersByArchived(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	active, err := s.Create(ctx, "Active task", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	archived, err := s.Create(ctx, "Archived task", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := s.SetArchived(ctx, archived.ID, true); err != nil {
		t.Fatalf("SetArchived() error = %v", err)
	}

	activeList, err := s.List(ctx, Filter{Archived: false})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(activeList) != 1 || activeList[0].ID != active.ID {
		t.Errorf("List(Archived: false) = %+v, want only %q", activeList, active.ID)
	}

	archivedList, err := s.List(ctx, Filter{Archived: true})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(archivedList) != 1 || archivedList[0].ID != archived.ID {
		t.Errorf("List(Archived: true) = %+v, want only %q", archivedList, archived.ID)
	}
}

// AC8 (spec P1 #8): Get on a missing id returns ErrNotFound.
func TestGet_MissingID_ErrNotFound(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	_, err := s.Get(ctx, "00000000-0000-4000-8000-000000000000")

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// AC6 (spec P1 #6): Update changes only the fields sent and bumps updated_at.
func TestUpdate_PartialUpdateChangesOnlySentFields_BumpsUpdatedAt(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	task, err := s.Create(ctx, "Original title", "Original description")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	newTitle := "New title"
	updated, err := s.Update(ctx, task.ID, Patch{Title: &newTitle})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if updated.Title != "New title" {
		t.Errorf("Title = %q, want %q", updated.Title, "New title")
	}
	if updated.Description != "Original description" {
		t.Errorf("Description = %q, want unchanged %q", updated.Description, "Original description")
	}
	if updated.Status != StatusTodo {
		t.Errorf("Status = %q, want unchanged %q", updated.Status, StatusTodo)
	}
	if updated.UpdatedAt == task.UpdatedAt {
		t.Errorf("UpdatedAt did not change, still %q", updated.UpdatedAt)
	}
	if updated.CreatedAt != task.CreatedAt {
		t.Errorf("CreatedAt = %q, want unchanged %q", updated.CreatedAt, task.CreatedAt)
	}
}

// AC7 (spec P1 #7): invalid status -> ValidationError on field "status".
func TestUpdate_InvalidStatus_ValidationError(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	task, err := s.Create(ctx, "Task", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	badStatus := "not_a_status"
	_, err = s.Update(ctx, task.ID, Patch{Status: &badStatus})

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if ve.Field != "status" {
		t.Errorf("Field = %q, want %q", ve.Field, "status")
	}
}

// ARC-02 (spec P2 #5): updating an archived task returns ErrArchived.
func TestUpdate_ArchivedTask_ErrArchived(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	task, err := s.Create(ctx, "Task", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := s.SetArchived(ctx, task.ID, true); err != nil {
		t.Fatalf("SetArchived() error = %v", err)
	}

	newTitle := "Should not apply"
	_, err = s.Update(ctx, task.ID, Patch{Title: &newTitle})

	if !errors.Is(err, ErrArchived) {
		t.Fatalf("err = %v, want ErrArchived", err)
	}
}

// ARC-01 (spec P2 #1, #3): archiving then restoring preserves title,
// description and status.
func TestSetArchived_ArchiveThenRestore_PreservesFields(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	task, err := s.Create(ctx, "Task to archive", "Some description")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	inProgress := StatusInProgress
	task, err = s.Update(ctx, task.ID, Patch{Status: &inProgress})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	archived, err := s.SetArchived(ctx, task.ID, true)
	if err != nil {
		t.Fatalf("SetArchived(true) error = %v", err)
	}
	if !archived.Archived {
		t.Error("Archived = false, want true")
	}

	restored, err := s.SetArchived(ctx, task.ID, false)
	if err != nil {
		t.Fatalf("SetArchived(false) error = %v", err)
	}
	if restored.Archived {
		t.Error("Archived = true, want false after restore")
	}
	if restored.Title != "Task to archive" {
		t.Errorf("Title = %q, want %q", restored.Title, "Task to archive")
	}
	if restored.Description != "Some description" {
		t.Errorf("Description = %q, want %q", restored.Description, "Some description")
	}
	if restored.Status != StatusInProgress {
		t.Errorf("Status = %q, want unchanged %q", restored.Status, StatusInProgress)
	}
}

// SetArchived is idempotent: archiving an already-archived task still
// succeeds.
func TestSetArchived_Idempotent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	task, err := s.Create(ctx, "Task", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	first, err := s.SetArchived(ctx, task.ID, true)
	if err != nil {
		t.Fatalf("SetArchived() first call error = %v", err)
	}
	second, err := s.SetArchived(ctx, task.ID, true)
	if err != nil {
		t.Fatalf("SetArchived() second call error = %v", err)
	}

	if !first.Archived || !second.Archived {
		t.Errorf("Archived = %v/%v, want true/true", first.Archived, second.Archived)
	}
}

// AC4 (spec P1 #4): List respects limit and offset.
func TestList_RespectsLimitAndOffset(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, err := s.Create(ctx, "Task", ""); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	page1, err := s.List(ctx, Filter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("len(page1) = %d, want 2", len(page1))
	}

	page2, err := s.List(ctx, Filter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("len(page2) = %d, want 2", len(page2))
	}

	if page1[0].ID == page2[0].ID || page1[1].ID == page2[0].ID {
		t.Error("page1 and page2 overlap, want distinct pages given the offset")
	}

	// The last page of 5 tasks with limit 2 holds the single remaining task.
	page3, err := s.List(ctx, Filter{Limit: 2, Offset: 4})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(page3) != 1 {
		t.Fatalf("len(page3) = %d, want 1", len(page3))
	}
}

// AC4 (spec P1 #4): List defaults to 50 per page when Limit is unset.
func TestList_DefaultLimitIsFifty(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := s.Create(ctx, "Task", ""); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	tasks, err := s.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("len(tasks) = %d, want 3 (all created tasks fit under the default limit of 50)", len(tasks))
	}
}

// AC4 / edge case "Paginação 50": the cap must truncate a real overflowing
// result set, not merely exist as a constant.
func TestList_CapsAtFiftyWithMoreRows(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	for i := 0; i < 51; i++ {
		if _, err := s.Create(ctx, "Task", ""); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	all, err := s.List(ctx, Filter{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(all) != 50 {
		t.Fatalf("len(List) = %d, want 50 (cap must truncate 51 rows)", len(all))
	}
}

// Edge case "Fuso horário": timestamps are RFC 3339 in UTC, not a local or
// loosely formatted string.
func TestCreate_TimestampsAreRFC3339UTC(t *testing.T) {
	s := openTestStore(t)

	task, err := s.Create(context.Background(), "Task", "")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	for label, value := range map[string]string{"created_at": task.CreatedAt, "updated_at": task.UpdatedAt} {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			t.Errorf("%s = %q, want RFC 3339: %v", label, value, err)
			continue
		}
		if _, offset := parsed.Zone(); offset != 0 {
			t.Errorf("%s = %q, want UTC (zero offset), got offset %d", label, value, offset)
		}
	}
}
