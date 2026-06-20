package resume

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/git"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness/internal/prompt"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestReconstructCheckpoint_lockedCriteriaAndVerifyAttempt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "checkpoint", InitialPrompt: "work", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	item, err := st.AddChecklistItem(ctx, tsk.ID, "criterion one", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add checklist: %v", err)
	}
	running := domain.StatusRunning
	if _, err := st.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &running}, domain.ActorAgent); err != nil {
		t.Fatalf("update: %v", err)
	}
	cycle, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	exec, err := st.StartPhase(ctx, cycle.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start execute: %v", err)
	}
	summary := domain.PhaseInterruptReason
	if _, err := st.CompletePhase(ctx, store.CompletePhaseInput{
		CycleID: cycle.ID, PhaseSeq: exec.PhaseSeq,
		Status: domain.PhaseStatusFailed, Summary: &summary, By: domain.ActorAgent,
	}); err != nil {
		t.Fatalf("complete execute: %v", err)
	}
	if err := st.UpsertVerifyReports(ctx, cycle.ID, 1, []store.VerifyReportEntry{
		{CriterionID: item.ID, Verified: true, VerifierKind: domain.VerifierAgentSelf, Reasoning: "ok"},
	}); err != nil {
		t.Fatalf("upsert verify: %v", err)
	}

	svc := NewService(st, Options{})
	cp, err := svc.ReconstructCheckpoint(ctx, cycle)
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if cp.Entry != EntryExecute {
		t.Fatalf("entry = %v, want execute resume", cp.Entry)
	}
	if _, ok := cp.PreviouslyPassed[item.ID]; !ok {
		t.Fatalf("expected locked criterion %s", item.ID)
	}
	if cp.VerifyAttempt != 1 {
		t.Fatalf("verifyAttempt = %d, want 1", cp.VerifyAttempt)
	}
}

func TestReconstructCheckpoint_interruptedVerify(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "verify resume", InitialPrompt: "work", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	running := domain.StatusRunning
	if _, err := st.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &running}, domain.ActorAgent); err != nil {
		t.Fatalf("update: %v", err)
	}
	cycle, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	exec, err := st.StartPhase(ctx, cycle.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start execute: %v", err)
	}
	if _, err := st.CompletePhase(ctx, store.CompletePhaseInput{
		CycleID: cycle.ID, PhaseSeq: exec.PhaseSeq,
		Status: domain.PhaseStatusSucceeded, By: domain.ActorAgent,
	}); err != nil {
		t.Fatalf("complete execute: %v", err)
	}
	verify, err := st.StartPhase(ctx, cycle.ID, domain.PhaseVerify, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start verify: %v", err)
	}
	summary := domain.PhaseInterruptReason
	if _, err := st.CompletePhase(ctx, store.CompletePhaseInput{
		CycleID: cycle.ID, PhaseSeq: verify.PhaseSeq,
		Status: domain.PhaseStatusFailed, Summary: &summary, By: domain.ActorAgent,
	}); err != nil {
		t.Fatalf("complete verify interrupt: %v", err)
	}

	svc := NewService(st, Options{})
	cp, err := svc.ReconstructCheckpoint(ctx, cycle)
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if cp.Entry != EntryVerifyOnly {
		t.Fatalf("entry = %v, want verify-only resume", cp.Entry)
	}
}

func TestLoadContinuationBundle_verifyOnlyWhenExecuteSucceeded(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "verify-only parent", InitialPrompt: "work", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	item, err := st.AddChecklistItem(ctx, tsk.ID, "criterion", nil, domain.ActorUser)
	if err != nil {
		t.Fatalf("add checklist: %v", err)
	}
	cycle, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	exec, err := st.StartPhase(ctx, cycle.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start execute: %v", err)
	}
	if _, err := st.CompletePhase(ctx, store.CompletePhaseInput{
		CycleID: cycle.ID, PhaseSeq: exec.PhaseSeq,
		Status: domain.PhaseStatusSucceeded, By: domain.ActorAgent,
	}); err != nil {
		t.Fatalf("complete execute: %v", err)
	}
	verify, err := st.StartPhase(ctx, cycle.ID, domain.PhaseVerify, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start verify: %v", err)
	}
	summary := verificationFailedReason + ": criterion failed"
	if _, err := st.CompletePhase(ctx, store.CompletePhaseInput{
		CycleID: cycle.ID, PhaseSeq: verify.PhaseSeq,
		Status: domain.PhaseStatusFailed, Summary: &summary, By: domain.ActorAgent,
	}); err != nil {
		t.Fatalf("complete verify: %v", err)
	}
	if _, err := st.TerminateCycle(ctx, cycle.ID, domain.CycleStatusFailed, verificationFailedReason, domain.ActorAgent); err != nil {
		t.Fatalf("terminate: %v", err)
	}
	when := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	if err := st.UpsertCycleCommits(ctx, tsk.ID, cycle.ID, []store.CycleCommitEntry{{
		PhaseSeq: 1, Seq: 1, Repo: "/repo", Worktree: "/repo", Branch: "main",
		SHA: "abc1234567890abcdef1234567890abcdef1234", CommittedAt: when, Message: "feat",
		Status: domain.CommitEligible,
	}}); err != nil {
		t.Fatalf("upsert commits: %v", err)
	}
	if err := st.UpsertVerifyReports(ctx, cycle.ID, 1, []store.VerifyReportEntry{
		{CriterionID: item.ID, Verified: false, VerifierKind: domain.VerifierVerifyAgent, Reasoning: "still failing"},
	}); err != nil {
		t.Fatalf("upsert verify: %v", err)
	}

	svc := NewService(st, Options{WorkingDir: t.TempDir()})
	bundle, err := svc.LoadContinuationBundle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	if bundle.Entry != EntryVerifyOnly {
		t.Fatalf("entry=%v want verify-only", bundle.Entry)
	}
	if !bundle.Sufficient {
		t.Fatalf("expected sufficient continuation data")
	}
}

func TestLoadContinuationBundle_carriesCriteriaReportProbeErr(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "criteria probe parent", InitialPrompt: "work", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	cycle, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	exec, err := st.StartPhase(ctx, cycle.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		t.Fatalf("start execute: %v", err)
	}
	probeErr := "criteria report invalid: unknown field function"
	details := git.MergeCriteriaReportProbeErr([]byte(`{"summary":"runner failed"}`), probeErr)
	summary := git.ExecuteInvalidCommitReason
	if _, err := st.CompletePhase(ctx, store.CompletePhaseInput{
		CycleID: cycle.ID, PhaseSeq: exec.PhaseSeq,
		Status: domain.PhaseStatusFailed, Summary: &summary, Details: details, By: domain.ActorAgent,
	}); err != nil {
		t.Fatalf("complete execute: %v", err)
	}
	if _, err := st.TerminateCycle(ctx, cycle.ID, domain.CycleStatusFailed, git.ExecuteInvalidCommitReason, domain.ActorAgent); err != nil {
		t.Fatalf("terminate: %v", err)
	}

	svc := NewService(st, Options{WorkingDir: t.TempDir()})
	bundle, err := svc.LoadContinuationBundle(ctx, cycle.ID)
	if err != nil {
		t.Fatalf("load bundle: %v", err)
	}
	if bundle.CriteriaReportProbeErr != probeErr {
		t.Fatalf("CriteriaReportProbeErr=%q want %q", bundle.CriteriaReportProbeErr, probeErr)
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
	svc := NewService(st, Options{})
	if _, err := svc.LoadCheckpointFromParent(ctx, cycle.ID); err == nil {
		t.Fatal("expected error for running parent cycle")
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
	svc := NewService(st, Options{})
	if err := svc.SeedCrossCycleExecuteFromParent(ctx, child, parent.ID); err != nil {
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

func TestReasonRemediation_executeGates(t *testing.T) {
	t.Parallel()
	tests := []struct {
		reason string
		want   string
	}{
		{git.ExecuteUncommittedWorkReason, "uncommitted"},
		{git.ExecuteNoCommitsReason, "at least one"},
		{git.ExecuteInvalidCommitReason, "repository"},
		{git.ExecuteRewrittenHistoryReason, "amend"},
	}
	for _, tc := range tests {
		got := git.ReasonRemediation(tc.reason)
		if got == "" {
			t.Fatalf("reason=%q got empty", tc.reason)
		}
		if tc.want != "" && !containsSubstr(got, tc.want) {
			t.Fatalf("reason=%q got=%q want substring %q", tc.reason, got, tc.want)
		}
	}
}

func TestFormatCommitsByStatusForResume_groups(t *testing.T) {
	t.Parallel()
	got := prompt.FormatCommitsByStatusForResume([]domain.TaskCycleCommit{
		{SHA: "abc", Status: domain.CommitEligible, Message: "ok"},
		{SHA: "def", Status: domain.CommitObserved, Message: "blocked", GateReason: git.ExecuteUncommittedWorkReason},
	})
	if !containsSubstr(got, "Eligible") || !containsSubstr(got, "Observed") {
		t.Fatalf("got=%q", got)
	}
	if !containsSubstr(got, "re-discover") {
		t.Fatalf("missing anti-discovery: %q", got)
	}
}

func containsSubstr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func initGitRepoForDiffTest(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	for _, args := range [][]string{
		{"init"},
		{"-c", "user.email=t@e.local", "-c", "user.name=t", "commit", "--allow-empty", "-m", "init"},
	} {
		if err := exec.Command("git", append([]string{"-C", dir}, args...)...).Run(); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}
}
