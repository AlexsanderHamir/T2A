package verify_test

import (
	"context"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"testing"
)

// TestWorker_VerifyPhase_usesSeparateRunnerWhenConfigured pins the
// adversarial-separation contract: when Options.VerifyRunner is non-nil
// the verify pass MUST land on it, not on the execute runner. Without
// this the docs' verifier_kind=verify_agent claim of adversarial review
// is structurally false.
func TestWorker_VerifyPhase_usesSeparateRunnerWhenConfigured(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.CreateReadyTask(ctx, "verify-multi-runner")
	item, err := h.Store.AddChecklistItem(ctx, tsk.ID, "criterion one", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add checklist item: %v", err)
	}

	workDir := t.TempDir()
	reportDir := t.TempDir()

	execRunner := runnerfake.New().WithName("exec-runner")
	execHook := &hookRunner{Runner: execRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseExecute {
			return
		}
		cycles, _ := h.Store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeCriteriaReport(t, reportDir, cycles[0].ID, []string{item.ID})
		}
	}}
	execRunner.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "exec ok", nil, ""))

	verifyRunner := runnerfake.New().WithName("verify-runner")
	verifyHook := &hookRunner{Runner: verifyRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseVerify {
			return
		}
		cycles, _ := h.Store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeVerifyReport(t, reportDir, cycles[0].ID, []string{item.ID})
		}
	}}
	verifyRunner.Script(tsk.ID, domain.PhaseVerify, runner.NewResult(
		domain.PhaseStatusSucceeded, "verify ok", nil, ""))

	done := h.StartHarnessRun(ctx, tsk, execHook, harness.Options{
		WorkingDir:   workDir,
		ReportDir:    reportDir,
		VerifyRunner: verifyHook,
	})
	h.WaitTaskStatus(ctx, tsk.ID, domain.StatusDone)
	<-done
	cancel()
	execCalls := execRunner.Calls()
	for _, c := range execCalls {
		if c.Phase == domain.PhaseVerify {
			t.Fatalf("execute runner saw a verify request: %+v", c)
		}
	}
	verifyCalls := verifyRunner.Calls()
	if len(verifyCalls) != 1 || verifyCalls[0].Phase != domain.PhaseVerify {
		t.Fatalf("verify runner calls = %+v, want exactly 1 verify request", verifyCalls)
	}
}
