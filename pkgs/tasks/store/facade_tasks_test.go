package store

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/kernel"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

// strPtr is a tiny helper used across the public-facade tests to take
// the address of a string literal. It lives here because facade_tasks_test.go
// is the largest consumer; other tests in this package can reuse it.
func strPtr(s string) *string { return &s }

// --- Create / Get / Update / Delete validation ----------------------------

func TestStore_Create_rejects_empty_title(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.Create(context.Background(), CreateTaskInput{Priority: domain.PriorityMedium, Title: "   "}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_invalid_status(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	st := domain.Status("nope")
	_, err := s.Create(context.Background(), CreateTaskInput{Priority: domain.PriorityMedium, Title: "ok", Status: st}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_missing_priority(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.Create(context.Background(), CreateTaskInput{Title: "ok"}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_invalid_priority(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	pr := domain.Priority("nope")
	_, err := s.Create(context.Background(), CreateTaskInput{Title: "ok", Priority: pr}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_defaults_task_type_to_general(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	got, err := s.Create(context.Background(), CreateTaskInput{Title: "ok", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if got.TaskType != domain.TaskTypeGeneral {
		t.Fatalf("task type %q", got.TaskType)
	}
}

func TestStore_Create_rejects_invalid_task_type(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	tt := domain.TaskType("nope")
	_, err := s.Create(context.Background(), CreateTaskInput{Title: "ok", Priority: domain.PriorityMedium, TaskType: tt}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_invalid_actor(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.Create(context.Background(), CreateTaskInput{Priority: domain.PriorityMedium, Title: "ok"}, domain.Actor("system"))
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_uses_explicit_id(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	id := "custom-id-1"
	got, err := s.Create(context.Background(), CreateTaskInput{ID: id, Title: "t", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id {
		t.Fatalf("id %q", got.ID)
	}
}

func TestStore_Create_duplicate_primary_key_fails(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	id := "dup"
	if _, err := s.Create(ctx, CreateTaskInput{ID: id, Title: "a", Priority: domain.PriorityMedium}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	_, err := s.Create(ctx, CreateTaskInput{ID: id, Title: "b", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err == nil {
		t.Fatal("expected error on duplicate id")
	}
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestStore_Get_not_found(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.Get(context.Background(), "00000000-0000-0000-0000-000000000099")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestStore_Get_rejects_empty_id(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.Get(context.Background(), "  ")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Update_rejects_no_fields(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Update(ctx, tsk.ID, UpdateTaskInput{}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Update_rejects_empty_title_patch(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Update(ctx, tsk.ID, UpdateTaskInput{Title: strPtr("  ")}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Update_changes_status_and_prompt(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	st := domain.StatusRunning
	pr := domain.PriorityHigh
	got, err := s.Update(ctx, tsk.ID, UpdateTaskInput{
		InitialPrompt: strPtr("p1"),
		Status:        &st,
		Priority:      &pr,
	}, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.StatusRunning || got.Priority != domain.PriorityHigh || got.InitialPrompt != "p1" {
		t.Fatalf("got %+v", got)
	}
}

func TestStore_Update_not_found(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	st := domain.StatusRunning
	_, err := s.Update(context.Background(), "00000000-0000-0000-0000-000000000088", UpdateTaskInput{Status: &st}, domain.ActorUser)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestStore_Update_rejects_invalid_actor(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	st := domain.StatusRunning
	_, err = s.Update(ctx, tsk.ID, UpdateTaskInput{Status: &st}, domain.Actor("nope"))
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Update_rejects_invalid_status_value(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	bad := domain.Status("invalid")
	_, err = s.Update(ctx, tsk.ID, UpdateTaskInput{Status: &bad}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Update_done_blockedWhenChildNotDone(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	parent, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "p"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	pid := parent.ID
	_, err = s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "c", ParentID: &pid}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	done := domain.StatusDone
	_, err = s.Update(ctx, parent.ID, UpdateTaskInput{Status: &done}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Delete_not_found(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, _, err := s.Delete(context.Background(), "00000000-0000-0000-0000-000000000077", domain.ActorUser)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestStore_Delete_rejects_empty_id(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, _, err := s.Delete(context.Background(), "", domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

// TestStore_Delete_cascadesSubtree pins the documented contract that
// a single Delete on a parent removes the parent and every descendant
// in BFS order. Replaces the prior `delete subtasks first` rejection
// (see docs/API-HTTP.md DELETE /tasks/{id}) so users no longer have
// to descend the tree manually from the SPA.
func TestStore_Delete_cascadesSubtree(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	parent, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "p"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	pid := parent.ID
	child, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "c", ParentID: &pid}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	cid := child.ID
	grand, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "gc", ParentID: &cid}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	deletedIDs, parentNotify, err := s.Delete(ctx, parent.ID, domain.ActorUser)
	if err != nil {
		t.Fatalf("cascade delete: %v", err)
	}
	if parentNotify != "" {
		t.Fatalf("parentNotify=%q want empty (root has no parent)", parentNotify)
	}
	if len(deletedIDs) != 3 {
		t.Fatalf("deletedIDs=%v want 3 ids (parent+child+grandchild)", deletedIDs)
	}
	want := map[string]bool{parent.ID: true, child.ID: true, grand.ID: true}
	for _, id := range deletedIDs {
		if !want[id] {
			t.Fatalf("unexpected id %q in deletedIDs=%v", id, deletedIDs)
		}
		delete(want, id)
	}
	if len(want) != 0 {
		t.Fatalf("missing ids from cascade: %v (got %v)", want, deletedIDs)
	}
	for _, id := range []string{parent.ID, child.ID, grand.ID} {
		if _, err := s.Get(ctx, id); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("Get(%s) after cascade err=%v want ErrNotFound", id, err)
		}
	}
}

func TestStore_Delete_cascades_events(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Delete(ctx, tsk.ID, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	err = db.Where("task_id = ?", tsk.ID).First(&domain.TaskEvent{}).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected events removed, got err=%v", err)
	}
}

func TestStore_Delete_child_appends_subtask_removed_on_parent(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	parent, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "p"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	pid := parent.ID
	child, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "kid", ParentID: &pid}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	deletedIDs, parentNotify, err := s.Delete(ctx, child.ID, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if parentNotify != parent.ID {
		t.Fatalf("notify parent %q want %q", parentNotify, parent.ID)
	}
	if len(deletedIDs) != 1 || deletedIDs[0] != child.ID {
		t.Fatalf("deletedIDs=%v want [%s]", deletedIDs, child.ID)
	}
	pEv, err := s.ListTaskEvents(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, e := range pEv {
		if e.Type == domain.EventSubtaskRemoved {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatalf("parent events: want subtask_removed, got %#v", pEv)
	}
}

// --- Parent/child + checklist-inherit append-events ----------------------

func TestStore_Create_child_appends_subtask_event_on_parent(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	parent, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "p"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	pid := parent.ID
	child, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "kid", ParentID: &pid}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	chEv, err := s.ListTaskEvents(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chEv) != 1 || chEv[0].Type != domain.EventTaskCreated {
		t.Fatalf("child events: %+v", chEv)
	}
	pEv, err := s.ListTaskEvents(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pEv) != 2 || pEv[0].Type != domain.EventTaskCreated || pEv[1].Type != domain.EventSubtaskAdded {
		t.Fatalf("parent events: %+v", pEv)
	}
}

func TestStore_Update_checklist_inherit_change_appends_event(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	parent, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "p"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	pid := parent.ID
	child, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "c", ParentID: &pid, ChecklistInherit: false}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	inherit := true
	if _, err := s.Update(ctx, child.ID, UpdateTaskInput{ChecklistInherit: &inherit}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	evs, err := s.ListTaskEvents(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, e := range evs {
		if e.Type == domain.EventChecklistInheritChanged {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatal("expected checklist_inherit_changed event")
	}
}

// --- List / Tree paths ---------------------------------------------------

func TestStore_List_pagination_and_limit_cap(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	for i := range 5 {
		title := string(rune('a' + i))
		if _, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: title}, domain.ActorUser); err != nil {
			t.Fatal(err)
		}
	}

	out, err := s.ListFlat(ctx, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("page1 len %d", len(out))
	}

	out2, err := s.ListFlat(ctx, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(out2) != 2 {
		t.Fatalf("page2 len %d", len(out2))
	}

	all, err := s.ListFlat(ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Fatalf("limit 0 normalized len %d", len(all))
	}

	capped, err := s.ListFlat(ctx, 500, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(capped) != 5 {
		t.Fatalf("over-limit cap: got %d tasks", len(capped))
	}
}

func TestStore_List_empty_table(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	got, err := s.ListFlat(context.Background(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("len %d", len(got))
	}
}

func TestStore_ListRootForest_empty_nonNilSlice(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	got, hasMore, err := s.ListRootForest(context.Background(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if hasMore {
		t.Fatal("unexpected hasMore")
	}
	if got == nil {
		t.Fatal("want empty non-nil slice so JSON encodes as [] not null")
	}
	if len(got) != 0 {
		t.Fatalf("len %d", len(got))
	}
}

func TestStore_ListRootForest_nested(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	p, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "root"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	pid := p.ID
	_, err = s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "kid", ParentID: &pid}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	forest, hasMore, err := s.ListRootForest(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if hasMore {
		t.Fatal("unexpected hasMore")
	}
	if len(forest) != 1 {
		t.Fatalf("roots %d", len(forest))
	}
	if len(forest[0].Children) != 1 {
		t.Fatalf("children %d", len(forest[0].Children))
	}
	if forest[0].Children[0].Title != "kid" {
		t.Fatalf("child title %q", forest[0].Children[0].Title)
	}
}

func TestStore_ListRootForest_hasMore_and_keyset(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	ids := []string{
		"10000000-0000-4000-8000-000000000001",
		"10000000-0000-4000-8000-000000000002",
		"10000000-0000-4000-8000-000000000003",
	}
	for _, id := range ids {
		if _, err := s.Create(ctx, CreateTaskInput{ID: id, Priority: domain.PriorityMedium, Title: "r"}, domain.ActorUser); err != nil {
			t.Fatal(err)
		}
	}
	got, hasMore, err := s.ListRootForest(ctx, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !hasMore || len(got) != 2 || got[0].ID != ids[0] || got[1].ID != ids[1] {
		t.Fatalf("page1: len=%d hasMore=%v", len(got), hasMore)
	}
	got2, hasMore2, err := s.ListRootForestAfter(ctx, 2, ids[1])
	if err != nil {
		t.Fatal(err)
	}
	if hasMore2 || len(got2) != 1 || got2[0].ID != ids[2] {
		t.Fatalf("page2: len=%d hasMore=%v", len(got2), hasMore2)
	}
}

func TestStore_GetTaskTree_rejects_chain_deeper_than_max(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	root, err := s.Create(ctx, CreateTaskInput{Title: "root", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	pid := root.ID
	for i := 0; i < MaxTaskTreeDepth; i++ {
		child, err := s.Create(ctx, CreateTaskInput{Title: fmt.Sprintf("d%d", i), Priority: domain.PriorityMedium, ParentID: &pid}, domain.ActorUser)
		if err != nil {
			t.Fatal(err)
		}
		pid = child.ID
	}
	if _, err := s.GetTaskTree(ctx, root.ID); err != nil {
		t.Fatalf("tree at max depth should succeed: %v", err)
	}
	if _, err := s.Create(ctx, CreateTaskInput{Title: "too-deep", Priority: domain.PriorityMedium, ParentID: &pid}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	_, err = s.GetTaskTree(ctx, root.ID)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

// --- Construction / migration --------------------------------------------

func TestMigrate_second_call_succeeds(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	if err := postgres.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
}

func TestNewStore_roundTrip(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	in, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "r"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.Get(ctx, in.ID)
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "r" {
		t.Fatalf("title %q", out.Title)
	}
}

// --- Ready-task notifier wiring ------------------------------------------

type spyReadyNotifier struct {
	calls int
	last  string
}

func (s *spyReadyNotifier) NotifyReadyTask(ctx context.Context, task domain.Task) error {
	s.calls++
	s.last = task.ID
	return nil
}

func TestSetReadyTaskNotifier_CreateReady(t *testing.T) {
	ctx := context.Background()
	st := NewStore(tasktestdb.OpenSQLite(t))
	var n spyReadyNotifier
	st.SetReadyTaskNotifier(&n)
	if _, err := st.Create(ctx, CreateTaskInput{Title: "x", Priority: domain.PriorityMedium}, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	if n.calls != 1 {
		t.Fatalf("notifier calls %d want 1", n.calls)
	}
}

func TestSetReadyTaskNotifier_CreateNonReady(t *testing.T) {
	ctx := context.Background()
	st := NewStore(tasktestdb.OpenSQLite(t))
	var n spyReadyNotifier
	st.SetReadyTaskNotifier(&n)
	if _, err := st.Create(ctx, CreateTaskInput{Title: "x", Priority: domain.PriorityMedium, Status: domain.StatusRunning}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if n.calls != 0 {
		t.Fatalf("notifier calls %d want 0", n.calls)
	}
}

func TestSetReadyTaskNotifier_UpdateTransitionToReady(t *testing.T) {
	ctx := context.Background()
	st := NewStore(tasktestdb.OpenSQLite(t))
	var n spyReadyNotifier
	st.SetReadyTaskNotifier(&n)
	tk, err := st.Create(ctx, CreateTaskInput{Title: "x", Priority: domain.PriorityMedium, Status: domain.StatusRunning}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	n.calls = 0
	ready := domain.StatusReady
	if _, err := st.Update(ctx, tk.ID, UpdateTaskInput{Status: &ready}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if n.calls != 1 || n.last != tk.ID {
		t.Fatalf("calls=%d last=%q", n.calls, n.last)
	}
}

// --- Operation-duration histogram ----------------------------------------

func storeOpHistogramSampleCount(op string) (uint64, error) {
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return 0, err
	}
	for _, mf := range mfs {
		if mf.GetName() != "taskapi_store_operation_duration_seconds" {
			continue
		}
		for _, m := range mf.GetMetric() {
			match := false
			for _, lp := range m.GetLabel() {
				if lp.GetName() == "op" && lp.GetValue() == op {
					match = true
					break
				}
			}
			if match {
				h := m.GetHistogram()
				if h == nil {
					continue
				}
				return h.GetSampleCount(), nil
			}
		}
	}
	return 0, nil
}

func TestStore_operation_duration_histogram_create_task(t *testing.T) {
	before, err := storeOpHistogramSampleCount(kernel.OpCreateTask)
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err = s.Create(context.Background(), CreateTaskInput{Title: "hist", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	after, err := storeOpHistogramSampleCount(kernel.OpCreateTask)
	if err != nil {
		t.Fatal(err)
	}
	if after < before+1 {
		t.Fatalf("create_task histogram sample_count: before=%d after=%d", before, after)
	}
}
