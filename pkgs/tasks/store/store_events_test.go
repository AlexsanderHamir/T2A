package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/internal/testdb"
)

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

func TestStore_AppendTaskEventResponseMessage_concurrent_no_lost_updates(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "concurrent-thread"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendTaskEvent(ctx, tsk.ID, domain.EventApprovalRequested, domain.ActorAgent, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	const n = 8
	var wg sync.WaitGroup
	wg.Add(n)
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			msg := fmt.Sprintf("msg-%d", i)
			errs <- s.AppendTaskEventResponseMessage(ctx, tsk.ID, 2, msg, domain.ActorUser)
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatalf("append: %v", e)
		}
	}
	got, err := s.GetTaskEvent(ctx, tsk.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	th := ThreadEntriesForDisplay(got)
	if len(th) != n {
		t.Fatalf("want %d thread entries (no lost updates under concurrency), got %d entries: %#v", n, len(th), th)
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
