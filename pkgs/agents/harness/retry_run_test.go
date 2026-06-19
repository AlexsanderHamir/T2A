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

func TestGitResetHardClean_resetsAndCleansUntracked(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	gitInit(t, dir)
	ctx := context.Background()
	base, err := runGit(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "dirty.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gitResetHardClean(ctx, dir, base); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("untracked file should be removed, stat err=%v", err)
	}
}

func TestResolveFreshRetryAnchor_fromExecutePhaseDetails(t *testing.T) {
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "t", InitialPrompt: "p", Priority: domain.PriorityMedium, Status: domain.StatusFailed,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	cycle, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	exec, err := st.StartPhase(ctx, cycle.ID, domain.PhaseExecute, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	details, _ := json.Marshal(map[string]any{
		"git": map[string]string{"cycle_base_sha": "abc123deadbeef"},
	})
	if _, err := st.CompletePhase(ctx, store.CompletePhaseInput{
		CycleID: cycle.ID, PhaseSeq: exec.PhaseSeq,
		Status: domain.PhaseStatusSucceeded, Details: details, By: domain.ActorAgent,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.TerminateCycle(ctx, cycle.ID, domain.CycleStatusFailed, "x", domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	h := New(st, runnerfake.New(), Options{WorkingDir: t.TempDir()})
	anchor, err := h.resolveFreshRetryAnchor(ctx, cycle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if anchor != "abc123deadbeef" {
		t.Fatalf("anchor=%q", anchor)
	}
}

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	go func() {
		h.RunWithRetry(ctx, tsk, &domain.PendingRetry{Mode: domain.RetryResume, ParentCycleID: parent.ID})
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, call := range r.Calls() {
			if strings.Contains(call.Prompt, "Worker resume notice") {
				cancel()
				cycles, err := st.ListCyclesForTask(context.Background(), tsk.ID, 10)
				if err != nil {
					t.Fatal(err)
				}
				if len(cycles) < 2 {
					t.Fatalf("cycles=%d want >=2", len(cycles))
				}
				if !strings.Contains(string(cycles[0].MetaJSON), `"retry_mode":"resume"`) {
					t.Fatalf("meta=%s", cycles[0].MetaJSON)
				}
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("resume prompt not observed")
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
