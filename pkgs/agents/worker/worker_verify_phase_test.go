package worker_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/worker"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// TestWorker_VerifyPhase_opensWhileExecuteIsTerminal pins the fix for
// the bug where the worker called StartPhase(verify) while the execute
// phase was still in `running`, tripping the cycle's "no running phase"
// invariant inside the transaction. The verify phase must always open
// AFTER execute is terminal so the cycle's phase ledger reflects the
// real sequence and the verify→execute retry transition is legal.
//
// Symptom this test guards against: every cycle with verification
// enabled would terminate with `execute_phase_start_failed` on the
// retry attempt because the state machine forbids execute→execute.
func TestWorker_VerifyPhase_opensWhileExecuteIsTerminal(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "verify-phase")

	// One retry only, so the loop runs at most twice. The runner never
	// writes criteria-report.json so verification fails on every attempt
	// — the point of the test is the phase ledger, not the verdict.
	maxRetries := 1
	if _, err := h.store.UpdateSettings(ctx, store.SettingsPatch{VerifyMaxRetries: &maxRetries}); err != nil {
		t.Fatalf("set verify max retries: %v", err)
	}

	if _, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion one", "", domain.ActorUser); err != nil {
		t.Fatalf("add checklist item: %v", err)
	}

	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "ran cleanly",
		json.RawMessage(`{"ok":true}`), "",
	))

	// Use a temp WorkingDir so the worker's .t2a/<cycle>/ paths land
	// somewhere isolated and parseCriteriaReport hits ErrCriteriaReportMissing
	// deterministically (no stray files from earlier test runs).
	_, done := h.startWorker(ctx, r, worker.Options{WorkingDir: t.TempDir()})
	final := h.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("worker exit err: %v", err)
	}

	if final.Status != domain.StatusFailed {
		t.Fatalf("task status = %q, want failed", final.Status)
	}

	bg := context.Background()
	cycle := assertCycleStatus(t, h.store, tsk.ID, 1, domain.CycleStatusFailed)

	phases, err := h.store.ListPhasesForCycle(bg, cycle.ID)
	if err != nil {
		t.Fatalf("list phases: %v", err)
	}

	// Expected ledger: diagnose(skipped) → execute(succeeded) →
	// verify(failed) → execute(succeeded) → verify(failed).
	wantSeq := []struct {
		phase  domain.Phase
		status domain.PhaseStatus
	}{
		{domain.PhaseDiagnose, domain.PhaseStatusSkipped},
		{domain.PhaseExecute, domain.PhaseStatusSucceeded},
		{domain.PhaseVerify, domain.PhaseStatusFailed},
		{domain.PhaseExecute, domain.PhaseStatusSucceeded},
		{domain.PhaseVerify, domain.PhaseStatusFailed},
	}
	if len(phases) != len(wantSeq) {
		t.Fatalf("phase count = %d, want %d (got %+v)", len(phases), len(wantSeq), phases)
	}
	for i, want := range wantSeq {
		if phases[i].Phase != want.phase || phases[i].Status != want.status {
			t.Errorf("phase[%d] = %q/%q, want %q/%q",
				i, phases[i].Phase, phases[i].Status, want.phase, want.status)
		}
	}

	// Execute must NEVER fail with the synthetic reason that fired before
	// the fix. Walk cycle_failed events; the worker stamps the terminal
	// reason in the event's Data JSON.
	events, err := h.store.ListTaskEvents(bg, tsk.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	sawVerificationFailed := false
	for _, e := range events {
		if e.Type != domain.EventCycleFailed {
			continue
		}
		body := string(e.Data)
		if strings.Contains(body, "execute_phase_start_failed") {
			t.Errorf("cycle_failed carries execute_phase_start_failed (regression of the verify-phase bug): %s", body)
		}
		if strings.Contains(body, "verification_failed") {
			sawVerificationFailed = true
		}
	}
	if !sawVerificationFailed {
		t.Errorf("expected at least one cycle_failed event with reason=verification_failed; got events=%+v", events)
	}

	// Runner must have been invoked twice for execute (initial + 1
	// retry). If the state machine rejected the retry, only one call
	// would have landed.
	executeCalls := 0
	for _, c := range r.Calls() {
		if c.Phase == domain.PhaseExecute {
			executeCalls++
		}
	}
	if executeCalls != 2 {
		t.Fatalf("execute runner calls = %d, want 2 (initial + retry)", executeCalls)
	}
}
