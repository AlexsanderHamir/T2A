package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"gorm.io/gorm"
)

// --- Shared helpers ------------------------------------------------------

func newCycleStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	return NewStore(tasktestdb.OpenSQLite(t)), context.Background()
}

func mustCreateTask(t *testing.T, s *Store, ctx context.Context) *domain.Task {
	t.Helper()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	return tsk
}

// --- StartCycle / TerminateCycle -----------------------------------------

func TestStore_StartCycle_assigns_attempt_seq_monotonically(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)

	first, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle 1: %v", err)
	}
	if first.AttemptSeq != 1 {
		t.Fatalf("first attempt_seq = %d, want 1", first.AttemptSeq)
	}
	if first.Status != domain.CycleStatusRunning {
		t.Fatalf("first status = %q, want running", first.Status)
	}
	if string(first.MetaJSON) != "{}" {
		t.Fatalf("first meta_json = %q, want {}", string(first.MetaJSON))
	}

	if _, err := s.TerminateCycle(ctx, first.ID, domain.CycleStatusSucceeded, "ok", domain.ActorAgent); err != nil {
		t.Fatalf("terminate first: %v", err)
	}

	second, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle 2: %v", err)
	}
	if second.AttemptSeq != 2 {
		t.Fatalf("second attempt_seq = %d, want 2", second.AttemptSeq)
	}
}

func TestStore_StartCycle_rejects_concurrent_running(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)

	if _, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent}); err != nil {
		t.Fatalf("first start: %v", err)
	}
	_, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("second start err = %v, want ErrInvalidInput", err)
	}
}

func TestStore_StartCycle_rejects_unknown_task(t *testing.T) {
	s, ctx := newCycleStore(t)
	_, err := s.StartCycle(ctx, StartCycleInput{TaskID: "does-not-exist", TriggeredBy: domain.ActorAgent})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestStore_StartCycle_rejects_invalid_actor(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	_, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.Actor("bot")})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestStore_StartCycle_parent_must_belong_to_same_task(t *testing.T) {
	s, ctx := newCycleStore(t)
	taskA := mustCreateTask(t, s, ctx)
	taskB := mustCreateTask(t, s, ctx)

	parent, err := s.StartCycle(ctx, StartCycleInput{TaskID: taskA.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("seed parent on taskA: %v", err)
	}
	if _, err := s.TerminateCycle(ctx, parent.ID, domain.CycleStatusFailed, "x", domain.ActorAgent); err != nil {
		t.Fatalf("terminate parent: %v", err)
	}

	pid := parent.ID
	_, err = s.StartCycle(ctx, StartCycleInput{TaskID: taskB.ID, TriggeredBy: domain.ActorAgent, ParentCycleID: &pid})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("cross-task parent err = %v, want ErrInvalidInput", err)
	}
}

func TestStore_TerminateCycle_rejects_non_terminal_status(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.TerminateCycle(ctx, c.ID, domain.CycleStatusRunning, "", domain.ActorAgent)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestStore_TerminateCycle_rejects_already_terminal(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.TerminateCycle(ctx, c.ID, domain.CycleStatusSucceeded, "", domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	_, err = s.TerminateCycle(ctx, c.ID, domain.CycleStatusFailed, "", domain.ActorAgent)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("re-terminate err = %v, want ErrInvalidInput", err)
	}
}

func TestStore_TerminateCycle_rejects_when_phase_running(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartPhase(ctx, c.ID, domain.PhaseDiagnose, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	_, err = s.TerminateCycle(ctx, c.ID, domain.CycleStatusSucceeded, "", domain.ActorAgent)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("terminate with running phase err = %v, want ErrInvalidInput", err)
	}
}

func TestStore_TerminateCycle_rejects_invalid_actor(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.TerminateCycle(ctx, c.ID, domain.CycleStatusSucceeded, "", domain.Actor("bot"))
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestStore_GetCycle_and_ListCyclesForTask(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)

	c1, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.TerminateCycle(ctx, c1.ID, domain.CycleStatusFailed, "", domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	c2, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.GetCycle(ctx, c2.ID)
	if err != nil {
		t.Fatalf("get cycle: %v", err)
	}
	if got.ID != c2.ID || got.AttemptSeq != 2 {
		t.Fatalf("get cycle = %+v", got)
	}

	list, err := s.ListCyclesForTask(ctx, tsk.ID, 0)
	if err != nil {
		t.Fatalf("list cycles: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("list len = %d, want 2", len(list))
	}
	if list[0].AttemptSeq != 2 || list[1].AttemptSeq != 1 {
		t.Fatalf("list order = [%d, %d], want [2, 1]", list[0].AttemptSeq, list[1].AttemptSeq)
	}

	if _, err := s.GetCycle(ctx, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing get err = %v, want ErrNotFound", err)
	}
	if _, err := s.ListCyclesForTask(ctx, "missing", 0); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing list err = %v, want ErrNotFound", err)
	}
}

// TestStore_ListCyclesForTaskBefore_keysetCursor pins the cursor
// semantics added alongside GET /tasks/{id}/cycles?before_attempt_seq=
// in Session 30. Three cycles seeded with attempt_seq 1/2/3 (the store
// assigns max+1 in order); the keyset call MUST return strictly older
// cycles than the cursor (i.e. attempt_seq < cursor) and never the
// cursor row itself, otherwise paginating clients would see duplicates
// across pages. The non-positive-cursor branch MUST behave identically
// to ListCyclesForTask so the new method can serve both first-page and
// next-page requests through a single store entrypoint.
func TestStore_ListCyclesForTaskBefore_keysetCursor(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)

	for i := 0; i < 3; i++ {
		c, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
		if err != nil {
			t.Fatalf("start cycle #%d: %v", i+1, err)
		}
		if _, err := s.TerminateCycle(ctx, c.ID, domain.CycleStatusSucceeded, "", domain.ActorAgent); err != nil {
			t.Fatalf("terminate cycle #%d: %v", i+1, err)
		}
	}

	t.Run("zeroBeforeBehavesLikeListForTask", func(t *testing.T) {
		got, err := s.ListCyclesForTaskBefore(ctx, tsk.ID, 0, 0)
		if err != nil {
			t.Fatalf("zero-before err = %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("zero-before len = %d, want 3 (matches ListCyclesForTask)", len(got))
		}
		if got[0].AttemptSeq != 3 || got[2].AttemptSeq != 1 {
			t.Fatalf("zero-before order = [%d ... %d], want [3 ... 1]", got[0].AttemptSeq, got[2].AttemptSeq)
		}
	})

	t.Run("strictlyLessThanCursor", func(t *testing.T) {
		got, err := s.ListCyclesForTaskBefore(ctx, tsk.ID, 3, 50)
		if err != nil {
			t.Fatalf("before=3 err = %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("before=3 len = %d, want 2 (cycles 2 and 1)", len(got))
		}
		if got[0].AttemptSeq != 2 || got[1].AttemptSeq != 1 {
			t.Fatalf("before=3 order = [%d, %d], want [2, 1]", got[0].AttemptSeq, got[1].AttemptSeq)
		}
		for _, c := range got {
			if c.AttemptSeq >= 3 {
				t.Fatalf("cursor row leaked: attempt_seq=%d should be < 3 (strict)", c.AttemptSeq)
			}
		}
	})

	t.Run("limitOneEnablesPaging", func(t *testing.T) {
		first, err := s.ListCyclesForTaskBefore(ctx, tsk.ID, 0, 1)
		if err != nil || len(first) != 1 || first[0].AttemptSeq != 3 {
			t.Fatalf("first page = %+v err=%v, want [{3}]", first, err)
		}
		second, err := s.ListCyclesForTaskBefore(ctx, tsk.ID, first[0].AttemptSeq, 1)
		if err != nil || len(second) != 1 || second[0].AttemptSeq != 2 {
			t.Fatalf("second page = %+v err=%v, want [{2}]", second, err)
		}
		third, err := s.ListCyclesForTaskBefore(ctx, tsk.ID, second[0].AttemptSeq, 1)
		if err != nil || len(third) != 1 || third[0].AttemptSeq != 1 {
			t.Fatalf("third page = %+v err=%v, want [{1}]", third, err)
		}
		end, err := s.ListCyclesForTaskBefore(ctx, tsk.ID, third[0].AttemptSeq, 1)
		if err != nil {
			t.Fatalf("end-of-stream err = %v", err)
		}
		if len(end) != 0 {
			t.Fatalf("end-of-stream len = %d, want 0", len(end))
		}
	})

	t.Run("missingTaskMapsToNotFound", func(t *testing.T) {
		if _, err := s.ListCyclesForTaskBefore(ctx, "missing", 1, 1); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestStore_AppendAndListCycleStreamEvents_ordersAndPages(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	cycle, phase := mustCreateCycleWithExecutePhase(t, s, ctx, tsk.ID)

	for i := 0; i < 3; i++ {
		ev, err := s.AppendCycleStreamEvent(ctx, AppendCycleStreamEventInput{
			TaskID:   tsk.ID,
			CycleID:  cycle.ID,
			PhaseSeq: phase.PhaseSeq,
			Source:   "cursor",
			Kind:     "message",
			Message:  fmt.Sprintf("event %d", i+1),
			Payload:  []byte(`{"kind":"message"}`),
		})
		if err != nil {
			t.Fatalf("append stream event %d: %v", i+1, err)
		}
		if ev.StreamSeq != int64(i+1) {
			t.Fatalf("stream_seq=%d want %d", ev.StreamSeq, i+1)
		}
	}

	got, err := s.ListCycleStreamEvents(ctx, cycle.ID, 1, 2)
	if err != nil {
		t.Fatalf("list stream events: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if got[0].StreamSeq != 2 || got[1].StreamSeq != 3 {
		t.Fatalf("stream order = [%d,%d] want [2,3]", got[0].StreamSeq, got[1].StreamSeq)
	}
	if got[0].Message != "event 2" {
		t.Fatalf("message=%q want event 2", got[0].Message)
	}
}

func TestStore_CycleStreamEventsCascadeWhenTaskDeleted(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	cycle, phase := mustCreateCycleWithExecutePhase(t, s, ctx, tsk.ID)
	if _, err := s.AppendCycleStreamEvent(ctx, AppendCycleStreamEventInput{
		TaskID:   tsk.ID,
		CycleID:  cycle.ID,
		PhaseSeq: phase.PhaseSeq,
		Source:   "cursor",
		Kind:     "message",
		Payload:  []byte(`{}`),
	}); err != nil {
		t.Fatalf("append stream event: %v", err)
	}
	if _, _, err := s.Delete(ctx, tsk.ID, domain.ActorUser); err != nil {
		t.Fatalf("delete task: %v", err)
	}
	if _, err := s.ListCycleStreamEvents(ctx, cycle.ID, 0, 10); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("list after delete err=%v want ErrNotFound", err)
	}
}

func mustCreateCycleWithExecutePhase(t *testing.T, s *Store, ctx context.Context, taskID string) (*domain.TaskCycle, *domain.TaskCyclePhase) {
	t.Helper()
	cycle, err := s.StartCycle(ctx, StartCycleInput{TaskID: taskID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	diag, err := s.StartPhase(ctx, cycle.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start diagnose: %v", err)
	}
	summary := "skip"
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{
		CycleID:  cycle.ID,
		PhaseSeq: diag.PhaseSeq,
		Status:   domain.PhaseStatusSkipped,
		Summary:  &summary,
		By:       domain.ActorAgent,
	}); err != nil {
		t.Fatalf("complete diagnose: %v", err)
	}
	phase, err := s.StartPhase(ctx, cycle.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start execute: %v", err)
	}
	return cycle, phase
}

// --- StartPhase / CompletePhase / ListPhasesForCycle ---------------------

func TestStore_StartPhase_enforces_state_machine(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.StartPhase(ctx, c.ID, domain.PhaseExecute, domain.ActorAgent); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("first phase Execute err = %v, want ErrInvalidInput", err)
	}

	d, err := s.StartPhase(ctx, c.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start diagnose: %v", err)
	}
	if d.PhaseSeq != 1 {
		t.Fatalf("diagnose phase_seq = %d, want 1", d.PhaseSeq)
	}

	if _, err := s.StartPhase(ctx, c.ID, domain.PhaseExecute, domain.ActorAgent); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("start execute while diagnose running err = %v, want ErrInvalidInput", err)
	}

	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: c.ID, PhaseSeq: d.PhaseSeq, Status: domain.PhaseStatusSucceeded, By: domain.ActorAgent}); err != nil {
		t.Fatalf("complete diagnose: %v", err)
	}

	if _, err := s.StartPhase(ctx, c.ID, domain.PhaseVerify, domain.ActorAgent); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("diagnose -> verify err = %v, want ErrInvalidInput", err)
	}

	e, err := s.StartPhase(ctx, c.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start execute: %v", err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: c.ID, PhaseSeq: e.PhaseSeq, Status: domain.PhaseStatusSucceeded, By: domain.ActorAgent}); err != nil {
		t.Fatal(err)
	}

	v, err := s.StartPhase(ctx, c.ID, domain.PhaseVerify, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start verify: %v", err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: c.ID, PhaseSeq: v.PhaseSeq, Status: domain.PhaseStatusFailed, By: domain.ActorAgent}); err != nil {
		t.Fatal(err)
	}

	e2, err := s.StartPhase(ctx, c.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		t.Fatalf("corrective execute after failing verify: %v", err)
	}
	if e2.PhaseSeq != 4 {
		t.Fatalf("corrective execute phase_seq = %d, want 4", e2.PhaseSeq)
	}
}

func TestStore_StartPhase_rejects_on_terminal_cycle(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.TerminateCycle(ctx, c.ID, domain.CycleStatusAborted, "", domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	_, err = s.StartPhase(ctx, c.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestStore_StartPhase_rejects_invalid_inputs(t *testing.T) {
	s, ctx := newCycleStore(t)
	if _, err := s.StartPhase(ctx, "", domain.PhaseDiagnose, domain.ActorAgent); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("empty cycle err = %v, want ErrInvalidInput", err)
	}
	if _, err := s.StartPhase(ctx, "x", domain.Phase("nope"), domain.ActorAgent); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("bad phase err = %v, want ErrInvalidInput", err)
	}
	if _, err := s.StartPhase(ctx, "missing", domain.PhaseDiagnose, domain.ActorAgent); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing cycle err = %v, want ErrNotFound", err)
	}
}

func TestStore_CompletePhase_updates_summary_and_details(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.StartPhase(ctx, c.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	summary := "scoped to package store"
	out, err := s.CompletePhase(ctx, CompletePhaseInput{
		CycleID:  c.ID,
		PhaseSeq: d.PhaseSeq,
		Status:   domain.PhaseStatusSucceeded,
		Summary:  &summary,
		Details:  []byte(`{"files":3}`),
		By:       domain.ActorAgent,
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if out.Status != domain.PhaseStatusSucceeded {
		t.Fatalf("status = %q", out.Status)
	}
	if out.EndedAt == nil {
		t.Fatal("ended_at unset")
	}
	if out.Summary == nil || *out.Summary != summary {
		t.Fatalf("summary = %v", out.Summary)
	}
	if string(out.DetailsJSON) != `{"files":3}` {
		t.Fatalf("details = %q", string(out.DetailsJSON))
	}
}

func TestStore_CompletePhase_rejects_double_complete(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.StartPhase(ctx, c.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: c.ID, PhaseSeq: d.PhaseSeq, Status: domain.PhaseStatusSucceeded, By: domain.ActorAgent}); err != nil {
		t.Fatal(err)
	}
	_, err = s.CompletePhase(ctx, CompletePhaseInput{CycleID: c.ID, PhaseSeq: d.PhaseSeq, Status: domain.PhaseStatusFailed, By: domain.ActorAgent})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("double complete err = %v, want ErrInvalidInput", err)
	}
}

func TestStore_CompletePhase_rejects_invalid_inputs(t *testing.T) {
	s, ctx := newCycleStore(t)
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: "", PhaseSeq: 1, Status: domain.PhaseStatusSucceeded, By: domain.ActorAgent}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("empty cycle err = %v, want ErrInvalidInput", err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: "x", PhaseSeq: 0, Status: domain.PhaseStatusSucceeded, By: domain.ActorAgent}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("zero seq err = %v, want ErrInvalidInput", err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: "x", PhaseSeq: 1, Status: domain.PhaseStatusRunning, By: domain.ActorAgent}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("non-terminal status err = %v, want ErrInvalidInput", err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: "missing", PhaseSeq: 1, Status: domain.PhaseStatusSucceeded, By: domain.ActorAgent}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing cycle err = %v, want ErrNotFound", err)
	}
}

func TestStore_ListPhasesForCycle_returns_in_seq_order(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}

	empty, err := s.ListPhasesForCycle(ctx, c.ID)
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("empty list len = %d", len(empty))
	}

	d, err := s.StartPhase(ctx, c.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: c.ID, PhaseSeq: d.PhaseSeq, Status: domain.PhaseStatusSucceeded, By: domain.ActorAgent}); err != nil {
		t.Fatal(err)
	}
	e, err := s.StartPhase(ctx, c.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.ListPhasesForCycle(ctx, c.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("list len = %d, want 2", len(got))
	}
	if got[0].ID != d.ID || got[1].ID != e.ID {
		t.Fatalf("order = [%s, %s], want [%s, %s]", got[0].ID, got[1].ID, d.ID, e.ID)
	}
	if got[0].PhaseSeq != 1 || got[1].PhaseSeq != 2 {
		t.Fatalf("seqs = [%d, %d], want [1, 2]", got[0].PhaseSeq, got[1].PhaseSeq)
	}
}

func TestStore_TaskDelete_cascades_to_cycles_and_phases(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartPhase(ctx, c.ID, domain.PhaseDiagnose, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.Delete(ctx, tsk.ID, domain.ActorUser); err != nil {
		t.Fatalf("delete task: %v", err)
	}

	if _, err := s.GetCycle(ctx, c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cycle after task delete err = %v, want ErrNotFound", err)
	}
}

// --- meta_json / details_json normalization (formerly *_meta_validate_test)

// TestStore_StartCycle_meta_normalizes_null asserts the documented invariant
// for task_cycles.meta_json (docs/EXECUTION-CYCLES.md §column conventions and
// docs/API-HTTP.md POST /tasks/{id}/cycles): the column never carries a
// non-object JSON value. The store must normalize the JSON literal "null"
// (semantically equivalent to "no meta provided") to the canonical "{}"
// rather than persisting the literal "null", which the API doc promises
// will never appear in responses.
func TestStore_StartCycle_meta_normalizes_null(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{
		TaskID:      tsk.ID,
		TriggeredBy: domain.ActorAgent,
		Meta:        []byte("null"),
	})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	if string(c.MetaJSON) != "{}" {
		t.Fatalf("meta_json = %q, want {} (null must normalize per documented invariant)", string(c.MetaJSON))
	}
}

// TestStore_StartCycle_meta_rejects_non_object_json asserts that meta values
// that are valid JSON but not objects (string, number, array, bool) are
// rejected as ErrInvalidInput. The contract is "opaque JSON object" — silent
// coercion would let the column hold values that violate the documented
// shape, leading to client parsers that crash on the wire later.
func TestStore_StartCycle_meta_rejects_non_object_json(t *testing.T) {
	cases := []struct {
		name string
		body []byte
	}{
		{"string", []byte(`"foo"`)},
		{"number", []byte(`123`)},
		{"array", []byte(`[1,2,3]`)},
		{"bool", []byte(`true`)},
		{"malformed", []byte(`{not-json`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, ctx := newCycleStore(t)
			tsk := mustCreateTask(t, s, ctx)
			_, err := s.StartCycle(ctx, StartCycleInput{
				TaskID:      tsk.ID,
				TriggeredBy: domain.ActorAgent,
				Meta:        tc.body,
			})
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("err = %v, want ErrInvalidInput for meta=%s", err, string(tc.body))
			}
		})
	}
}

// TestStore_StartCycle_meta_passes_through_object asserts that a well-formed
// JSON object meta survives the normalize step unchanged (byte-equal, after
// the json package's canonical form). This pins the happy-path contract so
// the validation logic above does not regress into rejecting valid input.
func TestStore_StartCycle_meta_passes_through_object(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{
		TaskID:      tsk.ID,
		TriggeredBy: domain.ActorAgent,
		Meta:        []byte(`{"runner":"cursor-cli"}`),
	})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	if string(c.MetaJSON) != `{"runner":"cursor-cli"}` {
		t.Fatalf("meta_json = %q, want {\"runner\":\"cursor-cli\"}", string(c.MetaJSON))
	}
}

// TestStore_CompletePhase_details_normalizes_and_validates pins the same
// invariant for task_cycle_phases.details_json: null normalizes to "{}",
// non-object JSON is rejected, and an object passes through.
func TestStore_CompletePhase_details_normalizes_and_validates(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	cycle, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	phase, err := s.StartPhase(ctx, cycle.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start phase: %v", err)
	}

	// null details normalize to "{}".
	out, err := s.CompletePhase(ctx, CompletePhaseInput{
		CycleID:  cycle.ID,
		PhaseSeq: phase.PhaseSeq,
		Status:   domain.PhaseStatusSucceeded,
		Details:  []byte("null"),
		By:       domain.ActorAgent,
	})
	if err != nil {
		t.Fatalf("complete phase with null details: %v", err)
	}
	if string(out.DetailsJSON) != "{}" {
		t.Fatalf("details_json = %q, want {}", string(out.DetailsJSON))
	}

	// Start a fresh cycle so we can run another phase end-to-end.
	if _, err := s.TerminateCycle(ctx, cycle.ID, domain.CycleStatusSucceeded, "", domain.ActorAgent); err != nil {
		t.Fatalf("terminate cycle: %v", err)
	}
	cycle2, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle 2: %v", err)
	}
	phase2, err := s.StartPhase(ctx, cycle2.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start phase 2: %v", err)
	}
	_, err = s.CompletePhase(ctx, CompletePhaseInput{
		CycleID:  cycle2.ID,
		PhaseSeq: phase2.PhaseSeq,
		Status:   domain.PhaseStatusSucceeded,
		Details:  []byte(`[1,2]`),
		By:       domain.ActorAgent,
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("complete phase with array details err = %v, want ErrInvalidInput", err)
	}
}

// --- Dual-write to task_events (formerly *_dualwrite_test) ---------------
//
// Stage 3 invariant: every cycle/phase mutation appends a matching
// task_events audit row in the same SQL transaction. The tests below pin
// that contract end-to-end: payload shape, actor mirroring, monotonic
// task_events.seq across mixed operations, event_seq backfill on phases,
// and atomic rollback when the mirror insert itself fails.
//
// Adding a new cycle/phase mutation? Add it to this table and the audit
// row assertions force you to wire the mirror at the same time.

type cycleEventCheck struct {
	wantType    domain.EventType
	wantBy      domain.Actor
	wantPayload map[string]any
}

func decodeEventPayload(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode event payload %q: %v", string(raw), err)
	}
	return m
}

func assertSubset(t *testing.T, got, want map[string]any, label string) {
	t.Helper()
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			t.Fatalf("%s: missing key %q in payload %v", label, k, got)
		}
		// JSON numbers decode as float64; normalize int comparisons.
		if wn, ok := wv.(int); ok {
			gn, ok := gv.(float64)
			if !ok {
				t.Fatalf("%s: key %q expected number, got %T (%v)", label, k, gv, gv)
			}
			if int(gn) != wn {
				t.Fatalf("%s: key %q = %v, want %d", label, k, gv, wn)
			}
			continue
		}
		if fmt.Sprint(gv) != fmt.Sprint(wv) {
			t.Fatalf("%s: key %q = %v, want %v", label, k, gv, wv)
		}
	}
}

func loadEventBySeq(t *testing.T, db *gorm.DB, taskID string, seq int64) domain.TaskEvent {
	t.Helper()
	var ev domain.TaskEvent
	if err := db.Where("task_id = ? AND seq = ?", taskID, seq).First(&ev).Error; err != nil {
		t.Fatalf("load event task=%s seq=%d: %v", taskID, seq, err)
	}
	return ev
}

func assertEvent(t *testing.T, db *gorm.DB, taskID string, seq int64, want cycleEventCheck) domain.TaskEvent {
	t.Helper()
	ev := loadEventBySeq(t, db, taskID, seq)
	if ev.Type != want.wantType {
		t.Fatalf("seq=%d type=%q, want %q", seq, ev.Type, want.wantType)
	}
	if ev.By != want.wantBy {
		t.Fatalf("seq=%d by=%q, want %q", seq, ev.By, want.wantBy)
	}
	got := decodeEventPayload(t, []byte(ev.Data))
	assertSubset(t, got, want.wantPayload, fmt.Sprintf("event seq=%d", seq))
	return ev
}

func TestStore_DualWrite_StartCycle_emits_cycle_started(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk := mustCreateTask(t, s, ctx)

	beforeSeq, _ := lastEventSeqRaw(t, db, tsk.ID)
	cyc, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}

	assertEvent(t, db, tsk.ID, beforeSeq+1, cycleEventCheck{
		wantType: domain.EventCycleStarted,
		wantBy:   domain.ActorAgent,
		wantPayload: map[string]any{
			"cycle_id":     cyc.ID,
			"attempt_seq":  int(cyc.AttemptSeq),
			"triggered_by": "agent",
		},
	})
}

func TestStore_DualWrite_StartCycle_includes_parent_when_set(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk := mustCreateTask(t, s, ctx)

	parent, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.TerminateCycle(ctx, parent.ID, domain.CycleStatusFailed, "first attempt", domain.ActorAgent); err != nil {
		t.Fatal(err)
	}

	pid := parent.ID
	child, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent, ParentCycleID: &pid})
	if err != nil {
		t.Fatal(err)
	}

	startedSeq, _ := lastEventSeqRaw(t, db, tsk.ID)
	got := decodeEventPayload(t, []byte(loadEventBySeq(t, db, tsk.ID, startedSeq).Data))
	if got["parent_cycle_id"] != parent.ID {
		t.Fatalf("parent_cycle_id = %v, want %s", got["parent_cycle_id"], parent.ID)
	}
	if got["cycle_id"] != child.ID {
		t.Fatalf("cycle_id = %v, want %s", got["cycle_id"], child.ID)
	}
}

func TestStore_DualWrite_TerminateCycle_emits_completed_or_failed(t *testing.T) {
	cases := []struct {
		name       string
		status     domain.CycleStatus
		reason     string
		wantType   domain.EventType
		wantStatus string
		wantReason string
	}{
		{"succeeded", domain.CycleStatusSucceeded, "", domain.EventCycleCompleted, "succeeded", ""},
		{"failed", domain.CycleStatusFailed, "checks didn't pass", domain.EventCycleFailed, "failed", "checks didn't pass"},
		{"aborted", domain.CycleStatusAborted, "user cancelled", domain.EventCycleFailed, "aborted", "user cancelled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := tasktestdb.OpenSQLite(t)
			s := NewStore(db)
			ctx := context.Background()
			tsk := mustCreateTask(t, s, ctx)
			cyc, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
			if err != nil {
				t.Fatal(err)
			}

			if _, err := s.TerminateCycle(ctx, cyc.ID, tc.status, tc.reason, domain.ActorUser); err != nil {
				t.Fatal(err)
			}

			seq, _ := lastEventSeqRaw(t, db, tsk.ID)
			payload := map[string]any{
				"cycle_id":    cyc.ID,
				"attempt_seq": int(cyc.AttemptSeq),
				"status":      tc.wantStatus,
			}
			if tc.wantReason != "" {
				payload["reason"] = tc.wantReason
			}
			assertEvent(t, db, tsk.ID, seq, cycleEventCheck{
				wantType:    tc.wantType,
				wantBy:      domain.ActorUser,
				wantPayload: payload,
			})
		})
	}
}

func TestStore_DualWrite_StartPhase_backfills_event_seq(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk := mustCreateTask(t, s, ctx)
	cyc, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}

	ph, err := s.StartPhase(ctx, cyc.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if ph.EventSeq == nil {
		t.Fatal("phase.EventSeq nil after StartPhase")
	}

	ev := assertEvent(t, db, tsk.ID, *ph.EventSeq, cycleEventCheck{
		wantType: domain.EventPhaseStarted,
		wantBy:   domain.ActorAgent,
		wantPayload: map[string]any{
			"cycle_id":  cyc.ID,
			"phase":     "diagnose",
			"phase_seq": int(ph.PhaseSeq),
		},
	})
	if ev.Seq != *ph.EventSeq {
		t.Fatalf("event_seq = %d, mirror seq = %d", *ph.EventSeq, ev.Seq)
	}
}

func TestStore_DualWrite_CompletePhase_emits_terminal_mirror_and_updates_event_seq(t *testing.T) {
	cases := []struct {
		name     string
		status   domain.PhaseStatus
		summary  string
		wantType domain.EventType
	}{
		{"succeeded", domain.PhaseStatusSucceeded, "diagnosed scope", domain.EventPhaseCompleted},
		{"failed", domain.PhaseStatusFailed, "verify failed", domain.EventPhaseFailed},
		{"skipped", domain.PhaseStatusSkipped, "no checks needed", domain.EventPhaseSkipped},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := tasktestdb.OpenSQLite(t)
			s := NewStore(db)
			ctx := context.Background()
			tsk := mustCreateTask(t, s, ctx)
			cyc, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
			if err != nil {
				t.Fatal(err)
			}
			ph, err := s.StartPhase(ctx, cyc.ID, domain.PhaseDiagnose, domain.ActorAgent)
			if err != nil {
				t.Fatal(err)
			}
			startedEventSeq := *ph.EventSeq

			summary := tc.summary
			done, err := s.CompletePhase(ctx, CompletePhaseInput{
				CycleID:  cyc.ID,
				PhaseSeq: ph.PhaseSeq,
				Status:   tc.status,
				Summary:  &summary,
				By:       domain.ActorAgent,
			})
			if err != nil {
				t.Fatal(err)
			}
			if done.EventSeq == nil {
				t.Fatal("phase.EventSeq nil after CompletePhase")
			}
			if *done.EventSeq <= startedEventSeq {
				t.Fatalf("event_seq did not advance: completed=%d started=%d", *done.EventSeq, startedEventSeq)
			}

			assertEvent(t, db, tsk.ID, *done.EventSeq, cycleEventCheck{
				wantType: tc.wantType,
				wantBy:   domain.ActorAgent,
				wantPayload: map[string]any{
					"cycle_id":  cyc.ID,
					"phase":     "diagnose",
					"phase_seq": int(ph.PhaseSeq),
					"status":    string(tc.status),
					"summary":   tc.summary,
				},
			})
		})
	}
}

func TestStore_DualWrite_seq_is_monotonic_across_entrypoints(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk := mustCreateTask(t, s, ctx)

	// task_created (seq 1) is the only baseline event seeded by Create.
	cyc, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	ph, err := s.StartPhase(ctx, cyc.ID, domain.PhaseDiagnose, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: cyc.ID, PhaseSeq: ph.PhaseSeq, Status: domain.PhaseStatusSucceeded, By: domain.ActorAgent}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TerminateCycle(ctx, cyc.ID, domain.CycleStatusSucceeded, "", domain.ActorAgent); err != nil {
		t.Fatal(err)
	}

	var rows []domain.TaskEvent
	if err := db.Where("task_id = ?", tsk.ID).Order("seq ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	wantTypes := []domain.EventType{
		domain.EventTaskCreated,
		domain.EventCycleStarted,
		domain.EventPhaseStarted,
		domain.EventPhaseCompleted,
		domain.EventCycleCompleted,
	}
	if len(rows) != len(wantTypes) {
		t.Fatalf("event count = %d, want %d (%+v)", len(rows), len(wantTypes), rows)
	}
	for i, r := range rows {
		if r.Seq != int64(i+1) {
			t.Fatalf("row %d seq = %d, want %d", i, r.Seq, i+1)
		}
		if r.Type != wantTypes[i] {
			t.Fatalf("row %d type = %q, want %q", i, r.Type, wantTypes[i])
		}
	}
}

func TestStore_DualWrite_StartCycle_rolls_back_when_mirror_insert_fails(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk := mustCreateTask(t, s, ctx)

	// Sabotage the next task_events insert so the mirror append fails inside
	// the same transaction as the cycle insert. The cycle write must be
	// rolled back: zero new task_cycles rows AND the task_events stream
	// stays at its baseline length.
	const cb = "test_block_task_events_insert"
	if err := db.Callback().Create().Before("gorm:create").Register(cb, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "task_events" {
			_ = tx.AddError(errors.New("forced failure: task_events"))
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Callback().Create().Remove(cb); err != nil {
			t.Logf("remove callback: %v", err)
		}
	})

	beforeCycles := countRows(t, db, &domain.TaskCycle{})
	beforeEvents := countRows(t, db, &domain.TaskEvent{})

	_, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err == nil {
		t.Fatal("StartCycle: want error from mirror failure, got nil")
	}

	if got := countRows(t, db, &domain.TaskCycle{}); got != beforeCycles {
		t.Fatalf("task_cycles count = %d, want %d (cycle insert leaked despite mirror failure)", got, beforeCycles)
	}
	if got := countRows(t, db, &domain.TaskEvent{}); got != beforeEvents {
		t.Fatalf("task_events count = %d, want %d (rollback did not restore baseline)", got, beforeEvents)
	}
}

func TestStore_DualWrite_mirror_event_types_reject_user_response(t *testing.T) {
	mirrorTypes := []domain.EventType{
		domain.EventCycleStarted,
		domain.EventCycleCompleted,
		domain.EventCycleFailed,
		domain.EventPhaseStarted,
		domain.EventPhaseCompleted,
		domain.EventPhaseFailed,
		domain.EventPhaseSkipped,
	}
	for _, et := range mirrorTypes {
		if domain.EventTypeAcceptsUserResponse(et) {
			t.Fatalf("EventTypeAcceptsUserResponse(%q) = true; mirror events are observational and must reject user_response writes", et)
		}
	}
}

func lastEventSeqRaw(t *testing.T, db *gorm.DB, taskID string) (int64, error) {
	t.Helper()
	var max int64
	if err := db.Raw(`SELECT COALESCE(MAX(seq), 0) FROM task_events WHERE task_id = ?`, taskID).Scan(&max).Error; err != nil {
		t.Fatalf("last event seq: %v", err)
	}
	return max, nil
}

func countRows(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var n int64
	if err := db.Model(model).Count(&n).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return n
}
