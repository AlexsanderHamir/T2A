package verify_test

import (
	"context"
	"encoding/json"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	if _, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion one", nil, domain.ActorUser); err != nil {
		t.Fatalf("add checklist item: %v", err)
	}

	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "ran cleanly",
		json.RawMessage(`{"ok":true}`), "",
	))

	// Use a temp WorkingDir so the worker's .legacy-scratch/<cycle>/ paths land
	// somewhere isolated and parseCriteriaReport hits ErrCriteriaReportMissing
	// deterministically (no stray files from earlier test runs).
	done := h.startHarnessRun(ctx, tsk, r, harness.Options{WorkingDir: t.TempDir()})
	final := h.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	<-done
	cancel()
	if final.Status != domain.StatusFailed {
		t.Fatalf("task status = %q, want failed", final.Status)
	}

	bg := context.Background()
	cycle := assertCycleStatus(t, h.store, tsk.ID, 1, domain.CycleStatusFailed)

	phases, err := h.store.ListPhasesForCycle(bg, cycle.ID)
	if err != nil {
		t.Fatalf("list phases: %v", err)
	}

	// Expected ledger: execute(succeeded) → verify(failed) →
	// execute(succeeded) → verify(failed).
	wantSeq := []struct {
		phase  domain.Phase
		status domain.PhaseStatus
	}{
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

// TestWorker_VerifyPhase_recordsDisagreementAsAgentSelfFailed pins the
// disagreement-via-derived-query contract from PR3: when the execute
// agent does NOT claim a criterion done, that surfaces on
// hamix_verify_verdict_total{verifier_kind="agent_self",verdict="failed"}.
// The same counter handles passes and the verifier's own verdicts;
// disagreement is the {agent_self,failed} slice.
func TestWorker_VerifyPhase_recordsDisagreementAsAgentSelfFailed(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "verify-disagreement")
	c1, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion one", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add c1: %v", err)
	}

	maxRetries := 0
	if _, err := h.store.UpdateSettings(ctx, store.SettingsPatch{VerifyMaxRetries: &maxRetries}); err != nil {
		t.Fatalf("set max retries: %v", err)
	}

	workDir := t.TempDir()
	reportDir := t.TempDir()
	r := runnerfake.New()
	hook := &hookRunner{Runner: r, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseExecute {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) == 0 {
			return
		}
		cdir := filepath.Join(reportDir, cycles[0].ID)
		if err := os.MkdirAll(cdir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		// claimed_done=false models the agent self-rejecting the criterion.
		body := `{"criteria":[{"id":"` + c1.ID + `","claimed_done":false,"evidence":"agent gave up"}]}`
		if err := os.WriteFile(filepath.Join(cdir, "criteria-report.json"), []byte(body), 0o644); err != nil {
			t.Fatalf("write criteria: %v", err)
		}
	}}
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "exec ok", nil, ""))

	metrics := newRecordingMetrics()
	done := h.startHarnessRun(ctx, tsk, hook, harness.Options{WorkingDir: workDir, ReportDir: reportDir, Metrics: metrics})
	h.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	<-done
	cancel()
	verdicts := metrics.verdictSnapshot()
	if len(verdicts) == 0 {
		t.Fatalf("expected at least one verdict recorded")
	}
	disagreements := 0
	for _, v := range verdicts {
		if v.Kind == domain.VerifierAgentSelf && !v.Passed {
			disagreements++
		}
	}
	if disagreements != 1 {
		t.Fatalf("agent_self/failed verdict count = %d, want 1; verdicts=%+v", disagreements, verdicts)
	}

	durations := metrics.verifyDurationSnapshot()
	if len(durations) == 0 {
		t.Fatalf("expected ObserveVerifyDuration to fire when verify ran")
	}

	retries := metrics.verifyRetriesSnapshot()
	if len(retries) == 0 || retries[len(retries)-1] != 0 {
		t.Fatalf("expected one retries observation = 0 (no retries); got %+v", retries)
	}
}
