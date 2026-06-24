package verify_test

import (
	"context"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestWorker_VerifyPhase_repoRootStaysCleanThroughoutCycle pins PR1's
// headline UX promise: customer working trees no longer accumulate
// `.legacy-scratch/` scratch files. The worker writes scratch outside RepoRoot
// (Options.ReportDir) and never touches the operator's repo. Both
// pre- and post-cycle `git status --porcelain` MUST report the
// working tree as clean.
func TestWorker_VerifyPhase_repoRootStaysCleanThroughoutCycle(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.CreateReadyTask(ctx, "verify-clean-repo")
	item, err := h.Store.AddChecklistItem(ctx, tsk.ID, "criterion one", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add checklist item: %v", err)
	}

	workDir := t.TempDir()
	reportDir := t.TempDir()
	gitInitTestRepo(t, workDir)

	preStatus, preErr := exec.Command("git", "-C", workDir, "status", "--porcelain").CombinedOutput()
	if preErr != nil {
		t.Fatalf("pre git status: %v\n%s", preErr, preStatus)
	}
	if strings.TrimSpace(string(preStatus)) != "" {
		t.Fatalf("precondition failed: working tree not clean before cycle: %s", preStatus)
	}

	execRunner := runnerfake.New()
	execHook := &hookRunner{Runner: execRunner, preRun: func(req runner.Request) {
		if req.Phase != domain.PhaseExecute {
			return
		}
		cycles, _ := h.Store.ListCyclesForTask(context.Background(), req.TaskID, 1)
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
	postStatus, postErr := exec.Command("git", "-C", workDir, "status", "--porcelain").CombinedOutput()
	if postErr != nil {
		t.Fatalf("post git status: %v\n%s", postErr, postStatus)
	}
	if strings.TrimSpace(string(postStatus)) != "" {
		t.Fatalf("RepoRoot dirty after cycle: %q", postStatus)
	}
	if entries, err := os.ReadDir(workDir); err == nil {
		for _, e := range entries {
			if e.Name() == ".legacy-scratch" {
				t.Fatalf("RepoRoot still contains legacy .legacy-scratch/ dir; PR1 contract is broken")
			}
		}
	}
}

// TestWorker_terminateCycle_cleansReportDir pins PR1's GC contract:
// after the cycle terminates, <reportDir>/<cycleID>/ must be gone so
// disk use stays bounded across thousands of cycles. The previous
// .legacy-scratch/-under-RepoRoot scheme had no GC and would have grown
// unboundedly.
func TestWorker_terminateCycle_cleansReportDir(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tsk := h.CreateReadyTask(ctx, "verify-cleanup")
	item, err := h.Store.AddChecklistItem(ctx, tsk.ID, "criterion one", nil, domain.ActorUser)
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
		cycles, _ := h.Store.ListCyclesForTask(context.Background(), req.TaskID, 1)
		if len(cycles) > 0 {
			writeCriteriaReport(t, reportDir, cycles[0].ID, []string{item.ID})
		}
	}}
	execRunner.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(
		domain.PhaseStatusSucceeded, "exec ok", nil, ""))

	verifyRunner := runnerfake.New()
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
	cycles, _ := h.Store.ListCyclesForTask(context.Background(), tsk.ID, 1)
	if len(cycles) != 1 {
		t.Fatalf("cycle count = %d, want 1", len(cycles))
	}
	cycleScratch := filepath.Join(reportDir, cycles[0].ID)
	if _, err := os.Stat(cycleScratch); !os.IsNotExist(err) {
		t.Fatalf("expected per-cycle scratch dir gone after terminate; stat err=%v path=%s", err, cycleScratch)
	}
}
