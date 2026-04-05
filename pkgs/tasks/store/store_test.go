package store

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/internal/testdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres"
	"gorm.io/gorm"
)

func strPtr(s string) *string { return &s }

func TestStore_Create_rejects_empty_title(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	_, err := s.Create(context.Background(), CreateTaskInput{Priority: domain.PriorityMedium, Title: "   "}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_invalid_status(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	st := domain.Status("nope")
	_, err := s.Create(context.Background(), CreateTaskInput{Priority: domain.PriorityMedium, Title: "ok", Status: st}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_missing_priority(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	_, err := s.Create(context.Background(), CreateTaskInput{Title: "ok"}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_invalid_priority(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	pr := domain.Priority("nope")
	_, err := s.Create(context.Background(), CreateTaskInput{Title: "ok", Priority: pr}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_invalid_actor(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	_, err := s.Create(context.Background(), CreateTaskInput{Priority: domain.PriorityMedium, Title: "ok"}, domain.Actor("system"))
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_uses_explicit_id(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	id := "custom-id-1"
	got, err := s.Create(context.Background(), CreateTaskInput{ID: id, Title: "t", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id {
		t.Fatalf("id %q", got.ID)
	}
}

func TestStore_Get_not_found(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	_, err := s.Get(context.Background(), "00000000-0000-0000-0000-000000000099")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestStore_Get_rejects_empty_id(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	_, err := s.Get(context.Background(), "  ")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_List_pagination_and_limit_cap(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
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

func TestStore_Update_rejects_no_fields(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
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
	s := NewStore(testdb.OpenSQLite(t))
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
	s := NewStore(testdb.OpenSQLite(t))
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
	s := NewStore(testdb.OpenSQLite(t))
	st := domain.StatusRunning
	_, err := s.Update(context.Background(), "00000000-0000-0000-0000-000000000088", UpdateTaskInput{Status: &st}, domain.ActorUser)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestStore_Delete_not_found(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	_, err := s.Delete(context.Background(), "00000000-0000-0000-0000-000000000077", domain.ActorUser)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestStore_Delete_rejects_empty_id(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	_, err := s.Delete(context.Background(), "", domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_duplicate_primary_key_fails(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	id := "dup"
	if _, err := s.Create(ctx, CreateTaskInput{ID: id, Title: "a", Priority: domain.PriorityMedium}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	_, err := s.Create(ctx, CreateTaskInput{ID: id, Title: "b", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err == nil {
		t.Fatal("expected error on duplicate id")
	}
	if errors.Is(err, domain.ErrInvalidInput) || errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unexpected sentinel: %v", err)
	}
}

func TestStore_events_recorded_for_create_and_title_change(t *testing.T) {
	db := testdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "first"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx, tsk.ID, UpdateTaskInput{Title: strPtr("second")}, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := db.Model(&domain.TaskEvent{}).Where("task_id = ?", tsk.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Fatalf("task_events rows %d want >= 2", n)
	}
}

func TestStore_ListTaskEvents_ordered_and_empty(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "a"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := s.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Type != domain.EventTaskCreated {
		t.Fatalf("events %#v", evs)
	}
	if _, err := s.Update(ctx, tsk.ID, UpdateTaskInput{Title: strPtr("b")}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	evs2, err := s.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs2) != 2 || evs2[0].Seq >= evs2[1].Seq {
		t.Fatalf("seq order %#v", evs2)
	}
}

func TestStore_TaskEventCount_and_LastEventSeq(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "a"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.TaskEventCount(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("count %d want 1", n)
	}
	last, err := s.LastEventSeq(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if last < 1 {
		t.Fatalf("last seq %d want >= 1", last)
	}
	if _, err := s.Update(ctx, tsk.ID, UpdateTaskInput{Title: strPtr("b")}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	n2, err := s.TaskEventCount(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 2 {
		t.Fatalf("count %d want 2", n2)
	}
	last2, err := s.LastEventSeq(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if last2 <= last {
		t.Fatalf("last2 %d should exceed last %d", last2, last)
	}
}

func TestStore_TaskEventCount_LastEventSeq_invalid_id(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	_, err := s.TaskEventCount(ctx, "")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("TaskEventCount empty id: got %v", err)
	}
	_, err = s.LastEventSeq(ctx, "  ")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("LastEventSeq empty id: got %v", err)
	}
}

func TestStore_ListTaskEvents_not_found_task_still_empty_id(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	_, err := s.ListTaskEvents(context.Background(), "")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_GetTaskEvent_returns_row_and_not_found(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "a"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	evs, err := s.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	got, err := s.GetTaskEvent(ctx, tsk.ID, evs[0].Seq)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != domain.EventTaskCreated || got.Seq != evs[0].Seq {
		t.Fatalf("got %#v", got)
	}
	_, err = s.GetTaskEvent(ctx, tsk.ID, 999)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing seq: got %v want ErrNotFound", err)
	}
}

func TestStore_GetTaskEvent_rejects_empty_id_and_bad_seq(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	_, err := s.GetTaskEvent(context.Background(), "  ", 1)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("empty id: got %v want ErrInvalidInput", err)
	}
	_, err = s.GetTaskEvent(context.Background(), "00000000-0000-0000-0000-000000000001", 0)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("seq 0: got %v want ErrInvalidInput", err)
	}
}

func TestStore_AppendTaskEventResponseMessage(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "a"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendTaskEvent(ctx, tsk.ID, domain.EventApprovalRequested, domain.ActorAgent, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	err = s.AppendTaskEventResponseMessage(ctx, tsk.ID, 2, " yes ", domain.Actor("system"))
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid actor: got %v", err)
	}
	err = s.AppendTaskEventResponseMessage(ctx, tsk.ID, 2, "   ", domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("empty text: got %v", err)
	}
	err = s.AppendTaskEventResponseMessage(ctx, tsk.ID, 1, "nope", domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("wrong type seq 1: got %v", err)
	}
	if err := s.AppendTaskEventResponseMessage(ctx, tsk.ID, 2, "Approved", domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTaskEvent(ctx, tsk.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserResponse == nil || *got.UserResponse != "Approved" {
		t.Fatalf("got %#v", got.UserResponse)
	}
	if got.UserResponseAt == nil {
		t.Fatal("expected UserResponseAt to be set")
	}
	th := ThreadEntriesForDisplay(got)
	if len(th) != 1 || th[0].By != domain.ActorUser || th[0].Body != "Approved" {
		t.Fatalf("thread %#v", th)
	}
	if err := s.AppendTaskEventResponseMessage(ctx, tsk.ID, 2, "Thanks", domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	got2, err := s.GetTaskEvent(ctx, tsk.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	th2 := ThreadEntriesForDisplay(got2)
	if len(th2) != 2 {
		t.Fatalf("want 2 thread entries, got %#v", th2)
	}
	if th2[1].By != domain.ActorAgent || th2[1].Body != "Thanks" {
		t.Fatalf("second entry %#v", th2[1])
	}
}

func TestStore_ListTaskEventsPageCursor_keyset_order_and_flags(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "a"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendTaskEvent(ctx, tsk.ID, domain.EventSyncPing, domain.ActorUser, nil); err != nil {
		t.Fatal(err)
	}
	head, err := s.ListTaskEventsPageCursor(ctx, tsk.ID, 1, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if head.Total != 2 {
		t.Fatalf("total %d want 2", head.Total)
	}
	if len(head.Events) != 1 || head.Events[0].Type != domain.EventSyncPing {
		t.Fatalf("head page: %#v", head.Events)
	}
	if !head.HasMoreOlder || head.HasMoreNewer {
		t.Fatalf("head flags newer=%v older=%v", head.HasMoreNewer, head.HasMoreOlder)
	}
	if head.RangeStart != 1 || head.RangeEnd != 1 {
		t.Fatalf("range %d-%d", head.RangeStart, head.RangeEnd)
	}
	before := head.Events[0].Seq
	older, err := s.ListTaskEventsPageCursor(ctx, tsk.ID, 10, &before, nil)
	if err != nil {
		t.Fatal(err)
	}
	if older.Total != 2 || len(older.Events) != 1 || older.Events[0].Type != domain.EventTaskCreated {
		t.Fatalf("before cursor: %#v", older.Events)
	}
	if !older.HasMoreNewer || older.HasMoreOlder {
		t.Fatalf("older page flags newer=%v older=%v", older.HasMoreNewer, older.HasMoreOlder)
	}
	minSeq := older.Events[0].Seq
	newer, err := s.ListTaskEventsPageCursor(ctx, tsk.ID, 10, nil, &minSeq)
	if err != nil {
		t.Fatal(err)
	}
	if len(newer.Events) != 1 || newer.Events[0].Type != domain.EventSyncPing {
		t.Fatalf("after cursor: %#v", newer.Events)
	}
}

func TestStore_ListTaskEventsPageCursor_rejects_both_cursors(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "a"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	before := int64(2)
	after := int64(1)
	_, err = s.ListTaskEventsPageCursor(ctx, tsk.ID, 10, &before, &after)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_ApprovalPending_respects_order(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "a"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := s.ApprovalPending(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending {
		t.Fatal("want no approval before events")
	}
	if err := s.AppendTaskEvent(ctx, tsk.ID, domain.EventApprovalRequested, domain.ActorUser, nil); err != nil {
		t.Fatal(err)
	}
	pending, err = s.ApprovalPending(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !pending {
		t.Fatal("want pending after request")
	}
	if err := s.AppendTaskEvent(ctx, tsk.ID, domain.EventApprovalGranted, domain.ActorUser, nil); err != nil {
		t.Fatal(err)
	}
	pending, err = s.ApprovalPending(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending {
		t.Fatal("want cleared after grant")
	}
}

func TestStore_AppendTaskEvent_appends_row_and_not_found(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "a"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendTaskEvent(ctx, tsk.ID, domain.EventSyncPing, domain.ActorUser, nil); err != nil {
		t.Fatal(err)
	}
	evs, err := s.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("want 2 events, got %d", len(evs))
	}
	if evs[1].Type != domain.EventSyncPing {
		t.Fatalf("want sync_ping, got %q", evs[1].Type)
	}
	err = s.AppendTaskEvent(ctx, "00000000-0000-0000-0000-000000000099", domain.EventSyncPing, domain.ActorUser, nil)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestStore_Update_rejects_invalid_actor(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
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
	s := NewStore(testdb.OpenSQLite(t))
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

func TestMigrate_second_call_succeeds(t *testing.T) {
	db := testdb.OpenSQLite(t)
	if err := postgres.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
}

func TestNewStore_roundTrip(t *testing.T) {
	db := testdb.OpenSQLite(t)
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

func TestStore_List_empty_table(t *testing.T) {
	db := testdb.OpenSQLite(t)
	s := NewStore(db)
	got, err := s.ListFlat(context.Background(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("len %d", len(got))
	}
}

func TestStore_Delete_cascades_events(t *testing.T) {
	db := testdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Delete(ctx, tsk.ID, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	err = db.Where("task_id = ?", tsk.ID).First(&domain.TaskEvent{}).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected events removed, got err=%v", err)
	}
}

func TestStore_Update_done_blockedWhenChildNotDone(t *testing.T) {
	db := testdb.OpenSQLite(t)
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

func TestStore_Delete_blockedWhenChildrenExist(t *testing.T) {
	db := testdb.OpenSQLite(t)
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
	_, err = s.Delete(ctx, parent.ID, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Delete_child_appends_subtask_removed_on_parent(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
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
	parentNotify, err := s.Delete(ctx, child.ID, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if parentNotify != parent.ID {
		t.Fatalf("notify parent %q want %q", parentNotify, parent.ID)
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

func TestStore_ListRootForest_empty_nonNilSlice(t *testing.T) {
	db := testdb.OpenSQLite(t)
	s := NewStore(db)
	got, err := s.ListRootForest(context.Background(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("want empty non-nil slice so JSON encodes as [] not null")
	}
	if len(got) != 0 {
		t.Fatalf("len %d", len(got))
	}
}

func TestStore_ListRootForest_nested(t *testing.T) {
	db := testdb.OpenSQLite(t)
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
	forest, err := s.ListRootForest(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
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

func TestStore_Create_child_appends_subtask_event_on_parent(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
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
	s := NewStore(testdb.OpenSQLite(t))
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

func TestStore_Ping_ok(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	if err := s.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}
