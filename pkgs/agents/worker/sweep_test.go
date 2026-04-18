package worker_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
	"gorm.io/gorm"
)

// sweepHarness is a minimal store + helpers for the orphan-sweep tests.
// It does not bring up a worker; the sweep is a stateless function that
// runs once at startup before the worker loop.
type sweepHarness struct {
	t  *testing.T
	st *store.Store
	db *gorm.DB
}

func newSweepHarness(t *testing.T) *sweepHarness {
	t.Helper()
	db := tasktestdb.OpenSQLite(t)
	return &sweepHarness{t: t, st: store.NewStore(db), db: db}
}

func (h *sweepHarness) makeRunningTaskWithRunningCycleAndPhase(t *testing.T, ctx context.Context, title string, phase domain.Phase) (*domain.Task, *domain.TaskCycle, *domain.TaskCyclePhase) {
	t.Helper()
	tsk, err := h.st.Create(ctx, store.CreateTaskInput{
		Title:         title,
		InitialPrompt: "do work",
		Status:        domain.StatusReady,
		Priority:      domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	running := domain.StatusRunning
	if _, err := h.st.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &running}, domain.ActorAgent); err != nil {
		t.Fatalf("transition task to running: %v", err)
	}
	cycle, err := h.st.StartCycle(ctx, store.StartCycleInput{
		TaskID:      tsk.ID,
		TriggeredBy: domain.ActorAgent,
	})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	if phase == "" {
		return tsk, cycle, nil
	}
	ph, err := h.st.StartPhase(ctx, cycle.ID, phase, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start phase: %v", err)
	}
	return tsk, cycle, ph
}

func TestSweep_CleanDB_isNoOp(t *testing.T) {
	t.Parallel()
	h := newSweepHarness(t)
	ctx := context.Background()

	res, err := worker.SweepOrphanRunningCycles(ctx, h.st)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if res.CyclesAborted != 0 || res.PhasesFailed != 0 || res.TasksFailed != 0 {
		t.Fatalf("clean DB sweep result = %+v, want zero", res)
	}
}

func TestSweep_OrphanRunningCycle_isAbortedAndTaskWalkedToFailed(t *testing.T) {
	t.Parallel()
	h := newSweepHarness(t)
	ctx := context.Background()

	tsk, cycle, ph := h.makeRunningTaskWithRunningCycleAndPhase(t, ctx, "orphan-cycle", domain.PhaseDiagnose)

	res, err := worker.SweepOrphanRunningCycles(ctx, h.st)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if res.CyclesAborted != 1 {
		t.Fatalf("CyclesAborted = %d, want 1", res.CyclesAborted)
	}
	if res.PhasesFailed != 1 {
		t.Fatalf("PhasesFailed = %d, want 1", res.PhasesFailed)
	}
	if res.TasksFailed != 1 {
		t.Fatalf("TasksFailed = %d, want 1", res.TasksFailed)
	}

	gotCycle, err := h.st.GetCycle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("get cycle: %v", err)
	}
	if gotCycle.Status != domain.CycleStatusAborted {
		t.Fatalf("cycle status = %q, want aborted", gotCycle.Status)
	}

	phases, err := h.st.ListPhasesForCycle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("list phases: %v", err)
	}
	if len(phases) != 1 || phases[0].ID != ph.ID || phases[0].Status != domain.PhaseStatusFailed {
		t.Fatalf("phases = %+v, want one failed phase id=%s", phases, ph.ID)
	}

	gotTask, err := h.st.Get(ctx, tsk.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if gotTask.Status != domain.StatusFailed {
		t.Fatalf("task status = %q, want failed", gotTask.Status)
	}

	events, err := h.st.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	counts := eventTypeCounts(events)
	if counts[domain.EventCycleFailed] != 1 {
		t.Fatalf("cycle_failed mirror count = %d, want 1 (events=%+v)", counts[domain.EventCycleFailed], counts)
	}
	if counts[domain.EventPhaseFailed] != 1 {
		t.Fatalf("phase_failed mirror count = %d, want 1", counts[domain.EventPhaseFailed])
	}

	var found bool
	for _, e := range events {
		if e.Type != domain.EventCycleFailed {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(e.Data, &payload); err != nil {
			t.Fatalf("unmarshal cycle_failed payload: %v", err)
		}
		if payload["reason"] != worker.SweepReason {
			t.Fatalf("cycle_failed reason = %v, want %q", payload["reason"], worker.SweepReason)
		}
		if payload["status"] != string(domain.CycleStatusAborted) {
			t.Fatalf("cycle_failed status = %v, want aborted", payload["status"])
		}
		found = true
	}
	if !found {
		t.Fatalf("no cycle_failed mirror event found")
	}
}

func TestSweep_OrphanPhaseUnderTerminalCycle_isFailed(t *testing.T) {
	t.Parallel()
	h := newSweepHarness(t)
	ctx := context.Background()

	tsk, cycle, ph := h.makeRunningTaskWithRunningCycleAndPhase(t, ctx, "orphan-phase", domain.PhaseDiagnose)

	if _, err := h.st.CompletePhase(ctx, store.CompletePhaseInput{
		CycleID:  cycle.ID,
		PhaseSeq: ph.PhaseSeq,
		Status:   domain.PhaseStatusSucceeded,
		By:       domain.ActorAgent,
	}); err != nil {
		t.Fatalf("complete diagnose: %v", err)
	}
	exec, err := h.st.StartPhase(ctx, cycle.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start execute: %v", err)
	}
	if _, err := h.st.CompletePhase(ctx, store.CompletePhaseInput{
		CycleID:  cycle.ID,
		PhaseSeq: exec.PhaseSeq,
		Status:   domain.PhaseStatusSucceeded,
		By:       domain.ActorAgent,
	}); err != nil {
		t.Fatalf("complete execute: %v", err)
	}
	if _, err := h.st.TerminateCycle(ctx, cycle.ID, domain.CycleStatusSucceeded, "", domain.ActorAgent); err != nil {
		t.Fatalf("terminate cycle: %v", err)
	}

	tx := h.db.Exec(
		"UPDATE task_cycle_phases SET status = ?, ended_at = NULL WHERE phase_seq = ? AND cycle_id = ?",
		domain.PhaseStatusRunning, exec.PhaseSeq, cycle.ID,
	)
	if tx.Error != nil {
		t.Fatalf("synthesize orphan running phase: %v", tx.Error)
	}

	res, err := worker.SweepOrphanRunningCycles(ctx, h.st)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if res.PhasesFailed != 1 {
		t.Fatalf("PhasesFailed = %d, want 1", res.PhasesFailed)
	}
	if res.CyclesAborted != 0 {
		t.Fatalf("CyclesAborted = %d, want 0 (cycle already terminal)", res.CyclesAborted)
	}

	phases, err := h.st.ListPhasesForCycle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("list phases: %v", err)
	}
	for _, p := range phases {
		if p.Status == domain.PhaseStatusRunning {
			t.Fatalf("phase %d still running", p.PhaseSeq)
		}
	}

	if _, err := h.st.Get(ctx, tsk.ID); err != nil {
		t.Fatalf("get task: %v", err)
	}
}

func TestSweep_Idempotent(t *testing.T) {
	t.Parallel()
	h := newSweepHarness(t)
	ctx := context.Background()

	_, _, _ = h.makeRunningTaskWithRunningCycleAndPhase(t, ctx, "idemp", domain.PhaseDiagnose)

	first, err := worker.SweepOrphanRunningCycles(ctx, h.st)
	if err != nil {
		t.Fatalf("first sweep: %v", err)
	}
	if first.CyclesAborted != 1 {
		t.Fatalf("first sweep CyclesAborted = %d, want 1", first.CyclesAborted)
	}
	second, err := worker.SweepOrphanRunningCycles(ctx, h.st)
	if err != nil {
		t.Fatalf("second sweep: %v", err)
	}
	if second.CyclesAborted != 0 || second.PhasesFailed != 0 || second.TasksFailed != 0 {
		t.Fatalf("second sweep should be no-op, got %+v", second)
	}
}
