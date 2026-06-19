package harness

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
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

	h := New(st, runnerfake.New(), Options{})
	cp, err := h.reconstructCheckpoint(ctx, cycle)
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if cp.entry != resumeEntryExecute {
		t.Fatalf("entry = %v, want execute resume", cp.entry)
	}
	if _, ok := cp.previouslyPassed[item.ID]; !ok {
		t.Fatalf("expected locked criterion %s", item.ID)
	}
	if cp.verifyAttempt != 1 {
		t.Fatalf("verifyAttempt = %d, want 1", cp.verifyAttempt)
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

	h := New(st, runnerfake.New(), Options{})
	cp, err := h.reconstructCheckpoint(ctx, cycle)
	if err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if cp.entry != resumeEntryVerifyOnly {
		t.Fatalf("entry = %v, want verify-only resume", cp.entry)
	}
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

func TestAppendResumeNotice_andCommitPolicy(t *testing.T) {
	t.Parallel()
	started := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	cycle := &domain.TaskCycle{ID: "cycle-1", StartedAt: started}
	known := []domain.TaskCycleCommit{
		{Seq: 1, SHA: "abc123def456", Message: "feat: add health check"},
	}
	prompt := appendResumeNotice("base", cycle, domain.PhaseExecute, known)
	for _, frag := range []string{"Worker resume notice", "cycle-1", "abc123def456", "base"} {
		if !containsSubstr(prompt, frag) {
			t.Fatalf("resume notice missing %q in %q", frag, prompt)
		}
	}
	withCommit := appendGitCommitPolicy("", false)
	if !containsSubstr(withCommit, "Git commits (required)") || !containsSubstr(withCommit, "criteria-report.json") {
		t.Fatalf("commit policy missing required block: %q", withCommit)
	}
	if containsSubstr(withCommit, "t2a:cycle") {
		t.Fatalf("commit policy must not mention t2a markers: %q", withCommit)
	}
	resumePolicy := appendGitCommitPolicy("", true)
	for _, frag := range []string{"cycle_base_sha..HEAD", "prior attempts", "empty array"} {
		if !containsSubstr(resumePolicy, frag) {
			t.Fatalf("resume commit policy missing %q in %q", frag, resumePolicy)
		}
	}
	opRetry := appendOperatorRetryResumeNotice("base", cycle, known)
	for _, frag := range []string{"Operator retry", "cycle-1", "abc123def456", "do **not** list them", "base"} {
		if !containsSubstr(opRetry, frag) {
			t.Fatalf("operator retry notice missing %q in %q", frag, opRetry)
		}
	}
	dir := t.TempDir()
	initGitRepoForDiffTest(t, dir)
	diff := verifyDiffSection(dir)
	if containsSubstr(diff, "(diff unavailable") {
		t.Fatalf("verify diff unavailable for git repo: %q", diff)
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
