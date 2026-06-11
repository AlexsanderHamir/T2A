package harness_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/harness"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/runnerfake"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func seedInterruptedExecute(t *testing.T, st *store.Store, ctx context.Context) (*domain.Task, *domain.TaskCycle, string) {
	t.Helper()
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "resume", InitialPrompt: "do the thing", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	running := domain.StatusRunning
	if _, err := st.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &running}, domain.ActorAgent); err != nil {
		t.Fatalf("update running: %v", err)
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
		t.Fatalf("complete execute interrupt: %v", err)
	}
	item, err := st.AddChecklistItem(ctx, tsk.ID, "criterion one", domain.ActorUser)
	if err != nil {
		t.Fatalf("add checklist: %v", err)
	}
	if err := st.UpsertVerifyReports(ctx, cycle.ID, 1, []store.VerifyReportEntry{
		{CriterionID: item.ID, Verified: true, VerifierKind: domain.VerifierAgentSelf, Reasoning: "locked"},
	}); err != nil {
		t.Fatalf("upsert verify: %v", err)
	}
	tsk, _ = st.Get(ctx, tsk.ID)
	return tsk, cycle, item.ID
}

func TestHarness_Resume_afterInterruptedExecute_composesResumePrompt(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st := store.NewStore(tasktestdb.OpenSQLite(t))
	tsk, cycle, criterionID := seedInterruptedExecute(t, st, ctx)

	promptCh := make(chan string, 1)
	phaseCh := make(chan domain.Phase, 1)
	inner := runnerfake.New()
	r := &hookRunner{
		Runner: inner,
		preRun: func(req runner.Request) {
			select {
			case promptCh <- req.Prompt:
			default:
			}
			select {
			case phaseCh <- req.Phase:
			default:
			}
			cancel()
		},
	}

	h := harness.New(st, r, harness.Options{ReportDir: t.TempDir()})
	done := make(chan struct{})
	go func() {
		h.Resume(ctx, tsk, cycle)
		close(done)
	}()

	var prompt string
	select {
	case prompt = <-promptCh:
	case <-time.After(pollTimeout):
		t.Fatal("timeout waiting for resume execute prompt")
	}
	select {
	case phase := <-phaseCh:
		if phase != domain.PhaseExecute {
			t.Fatalf("first runner phase = %q, want execute", phase)
		}
	case <-time.After(pollTimeout):
		t.Fatal("timeout waiting for runner phase")
	}

	for _, frag := range []string{"Worker resume notice", cycle.ID, "t2a:cycle=" + cycle.ID, "Already verified", criterionID, "do the thing"} {
		if !strings.Contains(prompt, frag) {
			t.Fatalf("resume prompt missing %q\nprompt=%q", frag, prompt)
		}
	}

	phases, err := st.ListPhasesForCycle(context.Background(), cycle.ID)
	if err != nil {
		t.Fatalf("list phases: %v", err)
	}
	if len(phases) < 2 {
		t.Fatalf("expected new execute phase after resume start, got %d phases", len(phases))
	}
	if phases[len(phases)-1].Phase != domain.PhaseExecute {
		t.Fatalf("last phase = %+v, want execute", phases[len(phases)-1])
	}

	select {
	case <-done:
	case <-time.After(pollTimeout):
		t.Fatal("timeout waiting for Resume to finish after cancel")
	}
}
