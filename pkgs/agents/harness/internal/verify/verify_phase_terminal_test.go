package verify_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)
// TestWorker_VerifyPhase_failsCycleWhenVerifyTampers pins the
// integrity-check contract. A verify runner that mutates source files
// MUST cause the cycle to terminate as verify_tampered with no
// retries, regardless of verify_max_retries. Tampering is verifier
// misbehaviour; retrying execute cannot fix it.
func TestWorker_VerifyPhase_failsCycleWhenVerifyTampers(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "verify-tampers")
	item, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion one", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add checklist item: %v", err)
	}

	maxRetries := 3
	if _, err := h.store.UpdateSettings(ctx, store.SettingsPatch{VerifyMaxRetries: &maxRetries}); err != nil {
		t.Fatalf("set verify max retries: %v", err)
	}

	workDir := t.TempDir()
	reportDir := t.TempDir()
	gitInitTestRepo(t, workDir)

	execRunner := runnerfake.New()
	execHook := &hookRunner{Runner: execRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseExecute {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeCriteriaReportWithGitWork(t, reportDir, cycles[0].ID, workDir, []string{item.ID})
		}
	}}
	execRunner.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "exec ok", nil, ""))

	verifyRunner := runnerfake.New().WithName("naughty-verify")
	verifyHook := &hookRunner{Runner: verifyRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseVerify {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeVerifyReport(t, reportDir, cycles[0].ID, []string{item.ID})
		}
		// Tamper: drop a stray file in the working dir root. After
		// PR1 the integrity-check whitelist is empty (reports live
		// outside RepoRoot), so any RepoRoot mutation is tampering.
		if err := os.WriteFile(filepath.Join(workDir, "MUTATED.txt"), []byte("hi"), 0o644); err != nil {
			t.Logf("tamper write: %v", err)
		}
	}}
	verifyRunner.Script(tsk.ID, domain.PhaseVerify, runner.NewResult(
		domain.PhaseStatusSucceeded, "verify ok", nil, ""))

	done := h.startHarnessRun(ctx, tsk, execHook, harness.Options{
		WorkingDir:   workDir,
		ReportDir:    reportDir,
		VerifyRunner: verifyHook,
	})
	final := h.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	cancel()
	<-done
	if final.Status != domain.StatusFailed {
		t.Fatalf("task status = %q, want failed", final.Status)
	}

	bg := context.Background()
	cycles, _ := h.store.ListCyclesForTask(bg, tsk.ID, 5)
	if len(cycles) != 1 {
		t.Fatalf("cycle count = %d, want 1 (no retries on tamper)", len(cycles))
	}
	if cycles[0].Status != domain.CycleStatusFailed {
		t.Fatalf("cycle status = %q, want failed", cycles[0].Status)
	}

	events, _ := h.store.ListTaskEvents(bg, tsk.ID)
	sawTampered := false
	for _, e := range events {
		if e.Type != domain.EventCycleFailed {
			continue
		}
		if strings.Contains(string(e.Data), "verify_tampered") {
			sawTampered = true
		}
	}
	if !sawTampered {
		t.Fatalf("expected cycle_failed event with reason=verify_tampered; events=%+v", events)
	}

	// Verify must have been invoked exactly once: tampering is
	// terminal, retries do not run.
	verifyCallCount := 0
	for _, c := range verifyRunner.Calls() {
		if c.Phase == domain.PhaseVerify {
			verifyCallCount++
		}
	}
	if verifyCallCount != 1 {
		t.Fatalf("verify runner verify calls = %d, want 1 (terminal-not-retryable)", verifyCallCount)
	}
}
// TestWorker_VerifyPhase_finalFailureWritesNoCompletions pins the
// atomic-decision contract: when retries are exhausted with at least
// one criterion still failing, NO completion rows land in
// task_checklist_completions even for criteria that passed on every
// attempt. previouslyPassed is in-memory only.
func TestWorker_VerifyPhase_finalFailureWritesNoCompletions(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "verify-no-completion")
	c1, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion one", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add c1: %v", err)
	}
	c2, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion two", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add c2: %v", err)
	}

	maxRetries := 1
	if _, err := h.store.UpdateSettings(ctx, store.SettingsPatch{VerifyMaxRetries: &maxRetries}); err != nil {
		t.Fatalf("set max retries: %v", err)
	}

	workDir := t.TempDir()
	reportDir := t.TempDir()
	var execAttempt atomic.Int32
	execRunner := runnerfake.New()
	execHook := &hookRunner{Runner: execRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseExecute {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) == 0 {
			return
		}
		n := execAttempt.Add(1)
		ids := []string{c1.ID, c2.ID}
		if n >= 2 {
			ids = []string{c2.ID}
		}
		writeCriteriaReportFor(t, reportDir, cycles[0].ID, ids)
	}}
	execRunner.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "exec ok", nil, ""))

	verifyRunner := runnerfake.New()
	verifyHook := &hookRunner{Runner: verifyRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseVerify {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) == 0 {
			return
		}
		// c1 always passes; c2 always fails. Both attempts.
		ids := map[string]bool{c1.ID: true, c2.ID: false}
		writePartialVerifyReport(t, reportDir, cycles[0].ID, ids)
	}}
	verifyRunner.Script(tsk.ID, domain.PhaseVerify, runner.NewResult(
		domain.PhaseStatusSucceeded, "verify ok", nil, ""))

	done := h.startHarnessRun(ctx, tsk, execHook, harness.Options{
		WorkingDir:   workDir,
		ReportDir:    reportDir,
		VerifyRunner: verifyHook,
	})
	h.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	cancel()
	<-done
	bg := context.Background()
	items, err := h.store.ListChecklistForSubject(bg, tsk.ID)
	if err != nil {
		t.Fatalf("list checklist: %v", err)
	}
	for _, it := range items {
		if it.Done {
			t.Errorf("expected NO completed items on terminal failure; %s is done", it.ID)
		}
	}
}
// TestWorker_VerifyPhase_terminateReasonIncludesFailingIDs pins the
// SPA-renderable failure detail: when retries exhaust, the cycle's
// terminate_reason carries the failing criterion IDs after the
// stable `verification_failed:` prefix.
func TestWorker_VerifyPhase_terminateReasonIncludesFailingIDs(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "verify-reason-ids")
	c1, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion one", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add c1: %v", err)
	}
	c2, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion two", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add c2: %v", err)
	}

	maxRetries := 0
	if _, err := h.store.UpdateSettings(ctx, store.SettingsPatch{VerifyMaxRetries: &maxRetries}); err != nil {
		t.Fatalf("set max retries: %v", err)
	}

	workDir := t.TempDir()
	reportDir := t.TempDir()
	execRunner := runnerfake.New()
	execHook := &hookRunner{Runner: execRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseExecute {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) == 0 {
			return
		}
		writeCriteriaReport(t, reportDir, cycles[0].ID, []string{c1.ID, c2.ID})
	}}
	execRunner.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "exec ok", nil, ""))

	verifyRunner := runnerfake.New()
	verifyHook := &hookRunner{Runner: verifyRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseVerify {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) == 0 {
			return
		}
		writePartialVerifyReport(t, reportDir, cycles[0].ID, map[string]bool{
			c1.ID: false, c2.ID: false,
		})
	}}
	verifyRunner.Script(tsk.ID, domain.PhaseVerify, runner.NewResult(
		domain.PhaseStatusSucceeded, "verify ok", nil, ""))

	done := h.startHarnessRun(ctx, tsk, execHook, harness.Options{
		WorkingDir:   workDir,
		ReportDir:    reportDir,
		VerifyRunner: verifyHook,
	})
	h.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	cancel()
	<-done
	bg := context.Background()
	events, err := h.store.ListTaskEvents(bg, tsk.ID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var reason string
	for _, e := range events {
		if e.Type != domain.EventCycleFailed {
			continue
		}
		var payload struct {
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(e.Data, &payload); err != nil {
			continue
		}
		if strings.HasPrefix(payload.Reason, "verification_failed") {
			reason = payload.Reason
		}
	}
	if reason == "" {
		t.Fatalf("no cycle_failed event with verification_failed reason; events=%+v", events)
	}
	if !strings.HasPrefix(reason, "verification_failed:") {
		t.Fatalf("reason must start with verification_failed:; got %q", reason)
	}
	// IDs are sorted; assert both appear regardless of seed order.
	if !strings.Contains(reason, c1.ID) || !strings.Contains(reason, c2.ID) {
		t.Fatalf("reason must include both failing IDs; got %q (c1=%s c2=%s)", reason, c1.ID, c2.ID)
	}
}
// TestWorker_VerifyPhase_repoRootMutationStillTampered pins the
// strengthened integrity contract: with the report-file allowlist
// removed in PR1, ANY mutation under RepoRoot during the verify pass
// is tampering. Even paths that mimic the legacy `.legacy-scratch/<cycleID>/...`
// shape are no longer tolerated — the verifier has no business
// touching the working tree.
func TestWorker_VerifyPhase_repoRootMutationStillTampered(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "verify-no-allowlist")
	item, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion one", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add checklist item: %v", err)
	}

	workDir := t.TempDir()
	reportDir := t.TempDir()
	gitInitTestRepo(t, workDir)

	execRunner := runnerfake.New()
	execHook := &hookRunner{Runner: execRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseExecute {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeCriteriaReportWithGitWork(t, reportDir, cycles[0].ID, workDir, []string{item.ID})
		}
	}}
	execRunner.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "exec ok", nil, ""))

	verifyRunner := runnerfake.New()
	verifyHook := &hookRunner{Runner: verifyRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseVerify {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeVerifyReport(t, reportDir, cycles[0].ID, []string{item.ID})
			// Drop a fake legacy-shaped artifact INSIDE the working
			// tree. Pre-PR1 this would have been tolerated by the
			// allowlist; post-PR1 it must trip integrity.
			legacyDir := filepath.Join(workDir, ".legacy-scratch", cycles[0].ID)
			_ = os.MkdirAll(legacyDir, 0o755)
			_ = os.WriteFile(filepath.Join(legacyDir, "verify-report.json"), []byte("{}"), 0o644)
		}
	}}
	verifyRunner.Script(tsk.ID, domain.PhaseVerify, runner.NewResult(
		domain.PhaseStatusSucceeded, "verify ok", nil, ""))

	done := h.startHarnessRun(ctx, tsk, execHook, harness.Options{
		WorkingDir:   workDir,
		ReportDir:    reportDir,
		VerifyRunner: verifyHook,
	})
	h.waitTaskStatus(ctx, tsk.ID, domain.StatusFailed)
	cancel()
	<-done
	events, _ := h.store.ListTaskEvents(context.Background(), tsk.ID)
	sawTampered := false
	for _, e := range events {
		if e.Type == domain.EventCycleFailed && strings.Contains(string(e.Data), "verify_tampered") {
			sawTampered = true
		}
	}
	if !sawTampered {
		t.Fatalf("expected verify_tampered cycle_failed event after legacy-shaped RepoRoot write; events=%+v", events)
	}
}
