package git

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/gittest"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents/harness/storefake"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func TestIngestExecuteCommits_fromClaims(t *testing.T) {
	t.Parallel()
	gittest.SkipIfNoGit(t)
	ctx := context.Background()
	st := storefake.New(t).Store
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "t", InitialPrompt: "p", Priority: domain.PriorityMedium, Status: domain.StatusReady,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	cycle, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	gittest.Init(t, dir)
	repo := NewExecRepo()
	base, err := repo.Run(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "f.go"}, {"commit", "-m", "feat"}} {
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=t@e.local", "-c", "user.name=t"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	head, err := repo.Run(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	head = head[:len(head)-1] // trim newline if any - rev-parse returns with newline
	if head == "" {
		head, _ = repo.Run(ctx, dir, "rev-parse", "HEAD")
	}
	head = trimLine(head)

	reportDir := t.TempDir()
	cycleDir := filepath.Join(reportDir, cycle.ID)
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	report := map[string]any{
		"schema_version": 1,
		"criteria":       []map[string]any{{"id": "c1", "claimed_done": true, "evidence": "done"}},
		"commits":        []map[string]any{{"sha": head}},
	}
	raw, _ := json.Marshal(report)
	if err := os.WriteFile(filepath.Join(cycleDir, "criteria-report.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(st, repo, reportDir)
	snap := PhaseSnapshot{
		Repo:         dir,
		Worktree:     dir,
		BaseSHA:      trimLine(base),
		CycleBaseSHA: trimLine(base),
	}
	outcome, err := svc.IngestExecuteCommits(ctx, tsk.ID, cycle, 1, snap, nil)
	if err != nil {
		t.Fatalf("ingest err: %v", err)
	}
	if outcome.FailReason != "" {
		t.Fatalf("want no fail reason, got %q", outcome.FailReason)
	}
	if outcome.CommitCount != 1 {
		t.Fatalf("commit_count=%d want 1", outcome.CommitCount)
	}
	rows, err := st.ListCommitsForTask(ctx, tsk.ID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("task commits: %v len=%d", err, len(rows))
	}
}

func TestIngestExecuteCommits_emptyClaimsContinues(t *testing.T) {
	t.Parallel()
	gittest.SkipIfNoGit(t)
	ctx := context.Background()
	st := storefake.New(t).Store
	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "t", InitialPrompt: "p", Priority: domain.PriorityMedium, Status: domain.StatusReady,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	cycle, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	gittest.Init(t, dir)
	repo := NewExecRepo()
	base, _ := repo.Run(ctx, dir, "rev-parse", "HEAD")
	svc := NewService(st, repo, t.TempDir())
	snap := PhaseSnapshot{Repo: dir, Worktree: dir, BaseSHA: trimLine(base), CycleBaseSHA: trimLine(base)}
	outcome, err := svc.IngestExecuteCommits(ctx, tsk.ID, cycle, 1, snap, nil)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.CommitCount != 0 || outcome.FailReason != "" {
		t.Fatalf("got %+v", outcome)
	}
}

func trimLine(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func TestCriteriaReportProbeErr_roundTrip(t *testing.T) {
	t.Parallel()
	base, _ := json.Marshal(map[string]any{"summary": "ok"})
	merged := MergeCriteriaReportProbeErr(base, "criteria report invalid: unknown field")
	got := CriteriaReportProbeErrFromPhaseDetails(merged)
	if got != "criteria report invalid: unknown field" {
		t.Fatalf("got %q", got)
	}
}
