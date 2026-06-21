package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

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

func ensureParentHasCriterion(t *testing.T, ctx context.Context, s *Store, parentID string) {
	t.Helper()
	items, err := s.ListChecklistForSubject(ctx, parentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) > 0 {
		return
	}
	if _, err := s.AddChecklistItem(ctx, parentID, "test criterion", nil, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
}

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

func TestStore_Delete_not_found(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.Delete(context.Background(), "00000000-0000-0000-0000-000000000077", domain.ActorUser)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestStore_Delete_rejects_empty_id(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.Delete(context.Background(), "", domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Delete_removesTask(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	deletedIDs, err := s.Delete(ctx, tsk.ID, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if len(deletedIDs) != 1 || deletedIDs[0] != tsk.ID {
		t.Fatalf("deletedIDs=%v", deletedIDs)
	}
	if _, err := s.Get(ctx, tsk.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get after delete err=%v want ErrNotFound", err)
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
	if _, err := s.Delete(ctx, tsk.ID, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	err = db.Where("task_id = ?", tsk.ID).First(&domain.TaskEvent{}).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected events removed, got err=%v", err)
	}
}

// --- List paths ---------------------------------------------------

func TestStore_ListFlatPage_empty_nonNilSlice(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	got, hasMore, err := s.ListFlatPage(context.Background(), 10, 0, nil)
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

func TestStore_ListFlatPage_hasMore_and_keyset(t *testing.T) {
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
	got, hasMore, err := s.ListFlatPage(ctx, 2, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !hasMore || len(got) != 2 || got[0].ID != ids[2] || got[1].ID != ids[1] {
		t.Fatalf("page1: len=%d hasMore=%v ids=%v,%v want %s,%s", len(got), hasMore, got[0].ID, got[1].ID, ids[2], ids[1])
	}
	got2, hasMore2, err := s.ListFlatAfter(ctx, 2, ids[1])
	if err != nil {
		t.Fatal(err)
	}
	if hasMore2 || len(got2) != 1 || got2[0].ID != ids[0] {
		t.Fatalf("page2: len=%d hasMore=%v id=%s want %s", len(got2), hasMore2, got2[0].ID, ids[0])
	}
}

// TestStore_Get_wrappedRecordNotFoundStillMapsToErrNotFound pins the contract
// that Get's sentinel check uses errors.Is and not fragile == comparison.
func TestStore_Get_wrappedRecordNotFoundStillMapsToErrNotFound(t *testing.T) {
	t.Parallel()
	db := tasktestdb.OpenSQLite(t)
	const cb = "test_wrap_tasks_record_not_found"
	if err := db.Callback().Query().After("gorm:query").Register(cb, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "tasks" && errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			tx.Error = fmt.Errorf("wrapped: %w", tx.Error)
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Callback().Query().Remove(cb); err != nil {
			t.Logf("remove callback: %v", err)
		}
	})

	s := NewStore(db)
	_, err := s.Get(context.Background(), "00000000-0000-0000-0000-deadbeefdead")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("Get on wrapped ErrRecordNotFound: got err=%v, want errors.Is(domain.ErrNotFound)", err)
	}
}

// --- Construction / migration --------------------------------------------

func TestStore_List_pagination_and_limit_cap(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	for i := range 5 {
		title := string(rune('a' + i))
		if _, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: title}, domain.ActorUser); err != nil {
			t.Fatal(err)
		}
	}

	out, err := s.ListFlat(ctx, 2, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("page1 len %d", len(out))
	}

	out2, err := s.ListFlat(ctx, 2, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out2) != 2 {
		t.Fatalf("page2 len %d", len(out2))
	}

	all, err := s.ListFlat(ctx, 0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Fatalf("limit 0 normalized len %d", len(all))
	}

	capped, err := s.ListFlat(ctx, 500, 0, nil)
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
	got, err := s.ListFlat(context.Background(), 10, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("len %d", len(got))
	}
}

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

// TestStore_Create_doesNotNotifyWhenPickupInFuture pins the Stage 0
// invariant from docs/data-model.md: a freshly-created ready task
// whose pickup_not_before is in the future MUST NOT be pushed onto
// the in-memory ready queue. The reconcile loop, which honours the
// SQL filter, picks it up later when the time has passed. Without
// this gate, the worker would race the reconcile sweep and pick up
// a scheduled task immediately.
func TestStore_Create_doesNotNotifyWhenPickupInFuture(t *testing.T) {
	ctx := context.Background()
	st := NewStore(tasktestdb.OpenSQLite(t))
	var n spyReadyNotifier
	st.SetReadyTaskNotifier(&n)
	future := time.Now().UTC().Add(1 * time.Hour)
	if _, err := st.Create(ctx, CreateTaskInput{
		Title: "scheduled", Priority: domain.PriorityMedium,
		PickupNotBefore: &future,
	}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if n.calls != 0 {
		t.Fatalf("notifier calls=%d want 0 (future schedule must defer)", n.calls)
	}
}

// TestStore_Create_notifiesWhenPickupAlreadyPassed pins the
// other half of the invariant: a pickup time already in the past
// is a no-op deferral and behaves exactly like the unscheduled path.
// (Operator typo or backfill recovery.)
func TestStore_Create_notifiesWhenPickupAlreadyPassed(t *testing.T) {
	ctx := context.Background()
	st := NewStore(tasktestdb.OpenSQLite(t))
	var n spyReadyNotifier
	st.SetReadyTaskNotifier(&n)
	past := time.Now().UTC().Add(-1 * time.Minute)
	if _, err := st.Create(ctx, CreateTaskInput{
		Title: "already-due", Priority: domain.PriorityMedium,
		PickupNotBefore: &past,
	}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if n.calls != 1 {
		t.Fatalf("notifier calls=%d want 1 (past schedule should not defer)", n.calls)
	}
}

// TestStore_Update_doesNotNotifyOnReadyTransitionWhenPickupInFuture
// covers the symmetric Update path: a status transition into ready
// for a task that already carries a future pickup_not_before MUST
// also defer to the reconcile loop. Currently only Status is
// patchable via UpdateInput; PickupNotBefore via PATCH is a Stage 2
// concern. We exercise the gate by seeding pickup_not_before at
// create time and then transitioning the status afterwards.
func TestStore_Update_doesNotNotifyOnReadyTransitionWhenPickupInFuture(t *testing.T) {
	ctx := context.Background()
	st := NewStore(tasktestdb.OpenSQLite(t))
	var n spyReadyNotifier
	st.SetReadyTaskNotifier(&n)
	future := time.Now().UTC().Add(1 * time.Hour)
	tk, err := st.Create(ctx, CreateTaskInput{
		Title: "scheduled-running", Priority: domain.PriorityMedium,
		Status: domain.StatusRunning, PickupNotBefore: &future,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	n.calls = 0
	ready := domain.StatusReady
	if _, err := st.Update(ctx, tk.ID, UpdateTaskInput{Status: &ready}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if n.calls != 0 {
		t.Fatalf("notifier calls=%d want 0 (future schedule must defer on Update)", n.calls)
	}
}

// TestShouldNotifyReadyNow_unitTable pins the boundary cases of the
// gate so a future refactor cannot quietly invert the comparison.
// The "exactly equal" case mirrors ready.ListQueueCandidates'
// `pickup_not_before <= now` filter.
func TestShouldNotifyReadyNow_unitTable(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		t    *time.Time
		want bool
	}{
		{"nil/no-deferral", nil, true},
		{"past", ptrTime(now.Add(-time.Hour)), true},
		{"exactly-now", ptrTime(now), true},
		{"one-second-future", ptrTime(now.Add(time.Second)), false},
		{"hour-future", ptrTime(now.Add(time.Hour)), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ShouldNotifyReadyNow(c.t, now); got != c.want {
				t.Fatalf("ShouldNotifyReadyNow(%v, %v)=%v want %v",
					c.t, now, got, c.want)
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

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
