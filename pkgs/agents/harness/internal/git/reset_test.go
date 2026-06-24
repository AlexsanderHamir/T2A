package git

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/gittest"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/storefake"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func TestResetHardClean_resetsAndCleansUntracked(t *testing.T) {
	gittest.SkipIfNoGit(t)
	dir := t.TempDir()
	gittest.Init(t, dir)
	ctx := context.Background()
	repo := NewExecRepo()
	base, err := repo.Run(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "dirty.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ResetHardClean(ctx, repo, dir, base); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("untracked file should be removed, stat err=%v", err)
	}
}

func TestResolveFreshRetryAnchor_fromExecutePhaseDetails(t *testing.T) {
	ctx := context.Background()
	st := storefake.New(t).Store
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
	svc := NewService(st, NewExecRepo(), "")
	anchor, err := svc.ResolveFreshRetryAnchor(ctx, t.TempDir(), cycle.ID)
	if err != nil {
		t.Fatal(err)
	}
	if anchor != "abc123deadbeef" {
		t.Fatalf("anchor=%q", anchor)
	}
}
