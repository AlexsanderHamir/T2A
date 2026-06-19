package harness

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestRunWithRetry_freshStartsNewCycleWithParent(t *testing.T) {
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "fresh-retry", InitialPrompt: "work", Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddChecklistItem(ctx, tsk.ID, "criterion", nil, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	running := domain.StatusRunning
	if _, err := st.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &running}, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	parent, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.TerminateCycle(ctx, parent.ID, domain.CycleStatusFailed, "fail", domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	failed := domain.StatusFailed
	if _, err := st.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &failed}, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	running = domain.StatusRunning
	if _, err := st.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &running}, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}

	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(domain.PhaseStatusSucceeded, "ok", nil, ""))
	h := New(st, r, Options{WorkingDir: t.TempDir(), Clock: func() time.Time { return time.Unix(0, 0).UTC() }})
	h.RunWithRetry(ctx, tsk, &domain.PendingRetry{Mode: domain.RetryFresh, ParentCycleID: parent.ID})

	cycles, err := st.ListCyclesForTask(ctx, tsk.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) < 2 {
		t.Fatalf("cycles=%d want >=2", len(cycles))
	}
	child := cycles[0]
	if child.ParentCycleID == nil || *child.ParentCycleID != parent.ID {
		t.Fatalf("parent_cycle_id=%v want %s", child.ParentCycleID, parent.ID)
	}
	if !strings.Contains(string(child.MetaJSON), `"retry_mode":"fresh"`) {
		t.Fatalf("meta=%s want retry_mode fresh", child.MetaJSON)
	}
}

func TestRunWithRetry_resumeCarriesPassedCriteria(t *testing.T) {
	ctx := context.Background()

	st := store.NewStore(tasktestdb.OpenSQLite(t))
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "resume-retry", InitialPrompt: "work", Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	item, err := st.AddChecklistItem(ctx, tsk.ID, "criterion", nil, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	running := domain.StatusRunning
	if _, err := st.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &running}, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	parent, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertVerifyReports(ctx, parent.ID, 1, []store.VerifyReportEntry{
		{CriterionID: item.ID, Verified: true, VerifierKind: domain.VerifierAgentSelf, Reasoning: "ok"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.TerminateCycle(ctx, parent.ID, domain.CycleStatusFailed, "verify fail", domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	failed := domain.StatusFailed
	if _, err := st.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &failed}, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	running = domain.StatusRunning
	if _, err := st.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &running}, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}

	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseExecute, runner.NewResult(domain.PhaseStatusSucceeded, "ok", nil, ""))
	h := New(st, r, Options{
		WorkingDir: t.TempDir(),
		Clock:      func() time.Time { return time.Unix(0, 0).UTC() },
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.RunWithRetry(ctx, tsk, &domain.PendingRetry{Mode: domain.RetryResume, ParentCycleID: parent.ID})
	}()

	deadline := time.Now().Add(5 * time.Second)
	var sawPrompt bool
	for time.Now().Before(deadline) {
		for _, call := range r.Calls() {
			if strings.Contains(call.Prompt, "Continuation") || strings.Contains(call.Prompt, "Operator retry") {
				sawPrompt = true
				break
			}
		}
		if sawPrompt {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !sawPrompt {
		t.Fatal("resume prompt not observed")
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RunWithRetry did not finish")
	}

	cycles, err := st.ListCyclesForTask(ctx, tsk.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) < 2 {
		t.Fatalf("cycles=%d want >=2", len(cycles))
	}
	if !strings.Contains(string(cycles[0].MetaJSON), `"retry_mode":"resume"`) {
		t.Fatalf("meta=%s", cycles[0].MetaJSON)
	}
}

func TestSeedCrossCycleExecuteFromParent_recordsSucceededExecute(t *testing.T) {
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "seed execute", InitialPrompt: "work", Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	exec, err := st.StartPhase(ctx, parent.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CompletePhase(ctx, store.CompletePhaseInput{
		CycleID: parent.ID, PhaseSeq: exec.PhaseSeq,
		Status: domain.PhaseStatusSucceeded, By: domain.ActorAgent,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.TerminateCycle(ctx, parent.ID, domain.CycleStatusFailed, verificationFailedReason, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	child, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	if err := New(st, runnerfake.New(), Options{}).seedCrossCycleExecuteFromParent(ctx, child, parent.ID); err != nil {
		t.Fatal(err)
	}
	phases, err := st.ListPhasesForCycle(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(phases) != 1 || phases[0].Phase != domain.PhaseExecute || phases[0].Status != domain.PhaseStatusSucceeded {
		t.Fatalf("phases=%+v", phases)
	}
}

func TestVerifyOnlyCrossCycleResume_runCycleLoopSkipsRunnerExecute(t *testing.T) {
	workDir := t.TempDir()
	gitInit(t, workDir)
	reportDir := t.TempDir()

	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "verify-only resume", InitialPrompt: "work", Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	item, err := st.AddChecklistItem(ctx, tsk.ID, "criterion", nil, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	running := domain.StatusRunning
	if _, err := st.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &running}, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	parent, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	exec, err := st.StartPhase(ctx, parent.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CompletePhase(ctx, store.CompletePhaseInput{
		CycleID: parent.ID, PhaseSeq: exec.PhaseSeq,
		Status: domain.PhaseStatusSucceeded, By: domain.ActorAgent,
	}); err != nil {
		t.Fatal(err)
	}
	verify, err := st.StartPhase(ctx, parent.ID, domain.PhaseVerify, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	summary := verificationFailedReason + ": failed"
	if _, err := st.CompletePhase(ctx, store.CompletePhaseInput{
		CycleID: parent.ID, PhaseSeq: verify.PhaseSeq,
		Status: domain.PhaseStatusFailed, Summary: &summary, By: domain.ActorAgent,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.TerminateCycle(ctx, parent.ID, domain.CycleStatusFailed, verificationFailedReason, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	if err := st.UpsertCycleCommits(ctx, tsk.ID, parent.ID, []store.CycleCommitEntry{{
		PhaseSeq: 1, Seq: 1, Repo: "/repo", Worktree: "/repo", Branch: "main",
		SHA: "abc1234567890abcdef1234567890abcdef1234", CommittedAt: when, Message: "feat",
		Status: domain.CommitEligible,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertCriteriaReports(ctx, parent.ID, 1, []store.CriteriaReportEntry{
		{CriterionID: item.ID, ClaimedDone: true, Evidence: "execute done"},
	}); err != nil {
		t.Fatal(err)
	}

	cp, err := New(st, runnerfake.New(), Options{WorkingDir: workDir}).loadCheckpointFromParent(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cp.Entry != resumeEntryVerifyOnly {
		t.Fatalf("entry=%v want verify-only", cp.Entry)
	}

	r := runnerfake.New()
	r.Script(tsk.ID, domain.PhaseVerify, runner.NewResult(domain.PhaseStatusSucceeded, "ok", nil, ""))
	h := New(st, r, Options{
		WorkingDir: workDir,
		ReportDir:  reportDir,
		Clock:      func() time.Time { return time.Unix(0, 0).UTC() },
	})
	state := processState{
		startedAt:        h.opts.Clock(),
		previouslyPassed: harnessVerdictsFromResume(cp.PreviouslyPassed),
		verifyFeedback:   cp.VerifyFeedback,
	}
	parentID := parent.ID
	child, ok := h.startCycle(ctx, tsk, &state, startCycleOpts{parentCycleID: &parentID, retryMode: domain.RetryResume})
	if !ok {
		t.Fatal("start child cycle failed")
	}
	if err := h.seedCrossCycleExecuteFromParent(ctx, child, parent.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.mirrorParentCriteriaForVerifyOnly(ctx, child.ID, parent.ID); err != nil {
		t.Fatal(err)
	}
	writeVerifyReportForTest(t, reportDir, child.ID, []string{item.ID})
	state.verifySnap, _ = h.loadVerificationSnapshot(ctx, tsk.ID)
	h.runCycleLoop(ctx, tsk, child, &state, cycleLoopOpts{
		skipFirstExecute: true,
		continuation:     cp.Continuation,
	})

	for _, call := range r.Calls() {
		if call.Phase == domain.PhaseExecute {
			t.Fatalf("verify-only resume must skip execute runner; got %+v", call)
		}
	}
	if len(r.Calls()) == 0 {
		t.Fatal("expected verify runner call")
	}
}

func TestLoadCheckpointFromParent_requiresTerminal(t *testing.T) {
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "t", InitialPrompt: "p", Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	cycle, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	h := New(st, runnerfake.New(), Options{})
	if _, err := h.loadCheckpointFromParent(ctx, cycle.ID); err == nil {
		t.Fatal("expected error for running parent cycle")
	}
}

func writeVerifyReportForTest(t *testing.T, reportDir, cycleID string, ids []string) {
	t.Helper()
	cdir := filepath.Join(reportDir, cycleID)
	if err := os.MkdirAll(cdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	type entry struct {
		ID        string `json:"id"`
		Verified  bool   `json:"verified"`
		Reasoning string `json:"reasoning"`
	}
	rep := struct {
		Criteria []entry `json:"criteria"`
	}{}
	for _, id := range ids {
		rep.Criteria = append(rep.Criteria, entry{
			ID:        id,
			Verified:  true,
			Reasoning: "verifier confirmed via diff inspection and file content review of the change set under test",
		})
	}
	b, _ := json.Marshal(rep)
	if err := os.WriteFile(filepath.Join(cdir, "verify-report.json"), b, 0o644); err != nil {
		t.Fatalf("write verify: %v", err)
	}
}
