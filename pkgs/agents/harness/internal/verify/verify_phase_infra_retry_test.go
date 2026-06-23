package verify_test

import (
	"context"
	"sync/atomic"
	"testing"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)
// TestWorker_VerifyPhase_carriesPassesAcrossRetries pins PR2's
// retry-efficiency contract WITHOUT breaking the docs-promised atomic
// decision: when attempt 1 passes c1 and fails c2, and attempt 2
// passes c2, the cycle terminates `succeeded` and BOTH completion
// rows land. Per-attempt state is held in memory (processState.previouslyPassed)
// so nothing is committed to task_checklist_completions before
// terminal-success.
func TestWorker_VerifyPhase_carriesPassesAcrossRetries(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.createReadyTask(ctx, "verify-carry")
	c1, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion one", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add c1: %v", err)
	}
	c2, err := h.store.AddChecklistItem(ctx, tsk.ID, "criterion two", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add c2: %v", err)
	}

	maxRetries := 2
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
		// Attempt 1 reports both criteria as claimed done. Attempt 2
		// only reports c2 — c1 was passed on attempt 1 so the prompt
		// excludes it from the expected-IDs set, and including a
		// stale c1 entry is no longer required.
		ids := []string{c1.ID, c2.ID}
		if n >= 2 {
			ids = []string{c2.ID}
		}
		writeCriteriaReportFor(t, reportDir, cycles[0].ID, ids)
	}}
	execRunner.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "exec ok", nil, ""))

	var verifyAttempt atomic.Int32
	verifyRunner := runnerfake.New()
	verifyHook := &hookRunner{Runner: verifyRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseVerify {
			return
		}
		cycles, _ := h.store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) == 0 {
			return
		}
		n := verifyAttempt.Add(1)
		// Attempt 1: c1 verified, c2 fails. Attempt 2: c2 verified.
		// (c1 is locked from attempt 1 and not in the expected set.)
		switch n {
		case 1:
			writePartialVerifyReport(t, reportDir, cycles[0].ID, map[string]bool{
				c1.ID: true, c2.ID: false,
			})
		default:
			writePartialVerifyReport(t, reportDir, cycles[0].ID, map[string]bool{
				c2.ID: true,
			})
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
	bg := context.Background()
	items, err := h.store.ListChecklistForSubject(bg, tsk.ID)
	if err != nil {
		t.Fatalf("list checklist: %v", err)
	}
	doneCount := 0
	for _, it := range items {
		if it.Done {
			doneCount++
		}
	}
	if doneCount != 2 {
		t.Fatalf("expected both criteria done, got %d (items=%+v)", doneCount, items)
	}

	// Per-attempt verdict rows must survive in
	// task_cycle_verify_reports / task_cycle_criteria_reports so the
	// SPA's verdict block can render the retry timeline. The
	// carry-passes lock must NOT erase prior-attempt evidence: c1's
	// attempt 1 row should still be there alongside c2's attempt 2 row.
	cycles, err := h.store.ListCyclesForTask(bg, tsk.ID, 5)
	if err != nil {
		t.Fatalf("list cycles: %v", err)
	}
	if len(cycles) == 0 {
		t.Fatalf("no cycles recorded")
	}
	cycleID := cycles[0].ID
	verifyRows, err := h.store.ListVerifyReportsForCycle(bg, cycleID)
	if err != nil {
		t.Fatalf("list verify reports: %v", err)
	}
	if len(verifyRows) < 2 {
		t.Fatalf("expected ≥2 verify rows (one per attempted criterion), got %d", len(verifyRows))
	}
	criteriaRows, err := h.store.ListCriteriaReportsForCycle(bg, cycleID)
	if err != nil {
		t.Fatalf("list criteria reports: %v", err)
	}
	if len(criteriaRows) < 2 {
		t.Fatalf("expected ≥2 criteria rows, got %d", len(criteriaRows))
	}
}
