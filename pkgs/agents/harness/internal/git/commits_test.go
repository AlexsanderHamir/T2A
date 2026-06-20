package git

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestEvaluateExecuteCommitGates_dirtyTreeWithCommitsAdmits(t *testing.T) {
	t.Parallel()
	skipIfNoGit(t)
	ctx := context.Background()
	dir := t.TempDir()
	gitInit(t, dir)
	repo := NewExecRepo()
	base, err := repo.Run(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "work.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", dir, "-c", "user.email=t@e.local", "-c", "user.name=t", "add", "work.go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", dir, "-c", "user.email=t@e.local", "-c", "user.name=t", "commit", "-m", "feat: work")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	head, err := repo.Run(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	svc := NewService(st, repo, "")
	snap := PhaseSnapshot{Worktree: dir, CycleBaseSHA: base}
	entries := []store.CycleCommitEntry{{SHA: head, Worktree: dir}}
	failReason, err := svc.evaluateExecuteCommitGates(ctx, snap, "cycle-1", entries)
	if err != nil {
		t.Fatal(err)
	}
	if failReason != "" {
		t.Fatalf("dirty tree with commits should admit, got reason=%q", failReason)
	}
	if err := os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIngestExecuteCommits_gitOnlyIgnoresBadCriteriaReport(t *testing.T) {
	t.Parallel()
	skipIfNoGit(t)
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
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
	gitInit(t, dir)
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

	reportDir := t.TempDir()
	cycleDir := filepath.Join(reportDir, cycle.ID)
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	badReport := []byte(`{"schema_version":1,"function":"oops","criteria":[]}`)
	if err := os.WriteFile(filepath.Join(cycleDir, "criteria-report.json"), badReport, 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(st, repo, reportDir)
	snap := PhaseSnapshot{
		Repo:         dir,
		Worktree:     dir,
		BaseSHA:      base,
		CycleBaseSHA: base,
	}
	outcome, err := svc.IngestExecuteCommits(ctx, tsk.ID, cycle, 1, snap, nil, domain.RetryFresh, nil)
	if err != nil {
		t.Fatalf("ingest err: %v", err)
	}
	if outcome.FailReason != "" {
		t.Fatalf("want no fail reason, got %q", outcome.FailReason)
	}
	if outcome.CommitCount != 1 {
		t.Fatalf("commit_count=%d want 1", outcome.CommitCount)
	}
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

func TestIngestExecuteCommits_v1GoldenReport(t *testing.T) {
	t.Parallel()
	skipIfNoGit(t)
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
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
	gitInit(t, dir)
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

	reportDir := t.TempDir()
	cycleDir := filepath.Join(reportDir, cycle.ID)
	if err := os.MkdirAll(cycleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "reports", "testdata", "criteria_report_v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cycleDir, "criteria-report.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(st, repo, reportDir)
	snap := PhaseSnapshot{
		Repo:         dir,
		Worktree:     dir,
		BaseSHA:      base,
		CycleBaseSHA: base,
	}
	outcome, err := svc.IngestExecuteCommits(ctx, tsk.ID, cycle, 1, snap, nil, domain.RetryFresh, nil)
	if err != nil {
		t.Fatalf("ingest err: %v", err)
	}
	if outcome.FailReason != "" {
		t.Fatalf("want no fail reason, got %q", outcome.FailReason)
	}
	if outcome.CommitCount != 1 {
		t.Fatalf("commit_count=%d want 1", outcome.CommitCount)
	}
}
