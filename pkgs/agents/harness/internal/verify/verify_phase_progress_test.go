package verify_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)
// TestWorker_VerifyPhase_persistsAndPublishesProgressEventsUnderVerifyPhaseSeq
// pins the SPA Activity-panel P3 visibility property: progress events
// emitted by the verify runner MUST be persisted under the verify
// phase row's seq so the per-phase filter renders them. Today's V1 had
// zero P3 stream events because the verify runner.Request had no
// OnProgress callback.
func TestWorker_VerifyPhase_persistsAndPublishesProgressEventsUnderVerifyPhaseSeq(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "verify-progress")
	item, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion one", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add checklist item: %v", err)
	}

	workDir := t.TempDir()
	reportDir := t.TempDir()

	execRunner := runnerfake.New()
	execHook := &hookRunner{Runner: execRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseExecute {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeCriteriaReport(t, reportDir, cycles[0].ID, []string{item.ID})
		}
	}}
	execRunner.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "exec ok", nil, ""))

	verifyRunner := runnerfake.New()
	var verifyProgressFired atomic.Bool
	verifyHook := &hookRunner{Runner: verifyRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseVerify {
			return
		}
		if req.OnProgress != nil {
			verifyProgressFired.Store(true)
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeVerifyReport(t, reportDir, cycles[0].ID, []string{item.ID})
		}
	}}
	verifyRunner.Script(tsk.ID, domain.PhaseVerify, runner.NewResult(
		domain.PhaseStatusSucceeded, "verify ok", nil, ""))

	done := h.startHarnessRun(ctx, tsk, execHook, harness.Options{
		WorkingDir:   workDir,
		ReportDir:    reportDir,
		VerifyRunner: verifyHook,
	})
	h.waitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	cancel()
	<-done
	if !verifyProgressFired.Load() {
		t.Fatal("verify runner.Request.OnProgress was nil; progress wiring missing")
	}

	bg := context.Background()
	cycles, _ := h.store.ListCyclesForTask(bg, tsk.ID, 1)
	if len(cycles) != 1 {
		t.Fatalf("cycle count = %d, want 1", len(cycles))
	}
	phases, _ := h.store.ListPhasesForCycle(bg, cycles[0].ID)
	var verifyPhaseSeq int64
	for _, p := range phases {
		if p.Phase == domain.PhaseVerify {
			verifyPhaseSeq = p.PhaseSeq
		}
	}
	if verifyPhaseSeq == 0 {
		t.Fatalf("no verify phase row found; phases=%+v", phases)
	}

	deadline := time.Now().Add(2 * time.Second)
	var verifyEvents int
	for time.Now().Before(deadline) {
		events, err := h.store.ListCycleStreamEvents(bg, cycles[0].ID, 0, 50)
		if err != nil {
			t.Fatalf("list cycle stream events: %v", err)
		}
		verifyEvents = 0
		for _, ev := range events {
			if ev.PhaseSeq == verifyPhaseSeq {
				verifyEvents++
			}
		}
		if verifyEvents > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if verifyEvents == 0 {
		t.Fatalf("no stream events under verify phase_seq=%d (P3 panel would be empty)", verifyPhaseSeq)
	}
}
