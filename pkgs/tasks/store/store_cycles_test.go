package store

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

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

	if _, err := s.TerminateCycle(ctx, first.ID, domain.CycleStatusSucceeded, "ok"); err != nil {
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
	if _, err := s.TerminateCycle(ctx, parent.ID, domain.CycleStatusFailed, "x"); err != nil {
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
	_, err = s.TerminateCycle(ctx, c.ID, domain.CycleStatusRunning, "")
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
	if _, err := s.TerminateCycle(ctx, c.ID, domain.CycleStatusSucceeded, ""); err != nil {
		t.Fatal(err)
	}
	_, err = s.TerminateCycle(ctx, c.ID, domain.CycleStatusFailed, "")
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
	if _, err := s.StartPhase(ctx, c.ID, domain.PhaseDiagnose); err != nil {
		t.Fatal(err)
	}
	_, err = s.TerminateCycle(ctx, c.ID, domain.CycleStatusSucceeded, "")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("terminate with running phase err = %v, want ErrInvalidInput", err)
	}
}

func TestStore_GetCycle_and_ListCyclesForTask(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)

	c1, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.TerminateCycle(ctx, c1.ID, domain.CycleStatusFailed, ""); err != nil {
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

func TestStore_StartPhase_enforces_state_machine(t *testing.T) {
	s, ctx := newCycleStore(t)
	tsk := mustCreateTask(t, s, ctx)
	c, err := s.StartCycle(ctx, StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := s.StartPhase(ctx, c.ID, domain.PhaseExecute); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("first phase Execute err = %v, want ErrInvalidInput", err)
	}

	d, err := s.StartPhase(ctx, c.ID, domain.PhaseDiagnose)
	if err != nil {
		t.Fatalf("start diagnose: %v", err)
	}
	if d.PhaseSeq != 1 {
		t.Fatalf("diagnose phase_seq = %d, want 1", d.PhaseSeq)
	}

	if _, err := s.StartPhase(ctx, c.ID, domain.PhaseExecute); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("start execute while diagnose running err = %v, want ErrInvalidInput", err)
	}

	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: c.ID, PhaseSeq: d.PhaseSeq, Status: domain.PhaseStatusSucceeded}); err != nil {
		t.Fatalf("complete diagnose: %v", err)
	}

	if _, err := s.StartPhase(ctx, c.ID, domain.PhaseVerify); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("diagnose -> verify err = %v, want ErrInvalidInput", err)
	}

	e, err := s.StartPhase(ctx, c.ID, domain.PhaseExecute)
	if err != nil {
		t.Fatalf("start execute: %v", err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: c.ID, PhaseSeq: e.PhaseSeq, Status: domain.PhaseStatusSucceeded}); err != nil {
		t.Fatal(err)
	}

	v, err := s.StartPhase(ctx, c.ID, domain.PhaseVerify)
	if err != nil {
		t.Fatalf("start verify: %v", err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: c.ID, PhaseSeq: v.PhaseSeq, Status: domain.PhaseStatusFailed}); err != nil {
		t.Fatal(err)
	}

	e2, err := s.StartPhase(ctx, c.ID, domain.PhaseExecute)
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
	if _, err := s.TerminateCycle(ctx, c.ID, domain.CycleStatusAborted, ""); err != nil {
		t.Fatal(err)
	}
	_, err = s.StartPhase(ctx, c.ID, domain.PhaseDiagnose)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestStore_StartPhase_rejects_invalid_inputs(t *testing.T) {
	s, ctx := newCycleStore(t)
	if _, err := s.StartPhase(ctx, "", domain.PhaseDiagnose); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("empty cycle err = %v, want ErrInvalidInput", err)
	}
	if _, err := s.StartPhase(ctx, "x", domain.Phase("nope")); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("bad phase err = %v, want ErrInvalidInput", err)
	}
	if _, err := s.StartPhase(ctx, "missing", domain.PhaseDiagnose); !errors.Is(err, domain.ErrNotFound) {
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
	d, err := s.StartPhase(ctx, c.ID, domain.PhaseDiagnose)
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
	d, err := s.StartPhase(ctx, c.ID, domain.PhaseDiagnose)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: c.ID, PhaseSeq: d.PhaseSeq, Status: domain.PhaseStatusSucceeded}); err != nil {
		t.Fatal(err)
	}
	_, err = s.CompletePhase(ctx, CompletePhaseInput{CycleID: c.ID, PhaseSeq: d.PhaseSeq, Status: domain.PhaseStatusFailed})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("double complete err = %v, want ErrInvalidInput", err)
	}
}

func TestStore_CompletePhase_rejects_invalid_inputs(t *testing.T) {
	s, ctx := newCycleStore(t)
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: "", PhaseSeq: 1, Status: domain.PhaseStatusSucceeded}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("empty cycle err = %v, want ErrInvalidInput", err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: "x", PhaseSeq: 0, Status: domain.PhaseStatusSucceeded}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("zero seq err = %v, want ErrInvalidInput", err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: "x", PhaseSeq: 1, Status: domain.PhaseStatusRunning}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("non-terminal status err = %v, want ErrInvalidInput", err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: "missing", PhaseSeq: 1, Status: domain.PhaseStatusSucceeded}); !errors.Is(err, domain.ErrNotFound) {
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

	d, err := s.StartPhase(ctx, c.ID, domain.PhaseDiagnose)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CompletePhase(ctx, CompletePhaseInput{CycleID: c.ID, PhaseSeq: d.PhaseSeq, Status: domain.PhaseStatusSucceeded}); err != nil {
		t.Fatal(err)
	}
	e, err := s.StartPhase(ctx, c.ID, domain.PhaseExecute)
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
	if _, err := s.StartPhase(ctx, c.ID, domain.PhaseDiagnose); err != nil {
		t.Fatal(err)
	}

	if _, err := s.Delete(ctx, tsk.ID, domain.ActorUser); err != nil {
		t.Fatalf("delete task: %v", err)
	}

	if _, err := s.GetCycle(ctx, c.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("cycle after task delete err = %v, want ErrNotFound", err)
	}
}
