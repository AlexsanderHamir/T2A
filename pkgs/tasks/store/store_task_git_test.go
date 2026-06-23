package store_test

import (
	"context"
	"os/exec"
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func TestStore_ValidateTaskGitBinding(t *testing.T) {
	ctx := context.Background()
	s := store.NewStore(tasktestdb.OpenSQLite(t))
	gitSvc := gitwork.New()
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "main", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}

	repoRow, err := s.CreateGitRepository(ctx, domain.DefaultProjectID, store.CreateGitRepositoryInput{Path: dir}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitRepository: %v", err)
	}
	wts, err := s.ListGitWorktrees(ctx, domain.DefaultProjectID, repoRow.ID)
	if err != nil || len(wts) == 0 {
		t.Fatalf("ListGitWorktrees: %v", err)
	}
	branches, err := s.ListGitBranches(ctx, domain.DefaultProjectID, repoRow.ID)
	if err != nil || len(branches) == 0 {
		t.Fatalf("ListGitBranches: %v", err)
	}
	pid := domain.DefaultProjectID
	if err := s.ValidateTaskGitBinding(ctx, &pid, wts[0].ID, branches[0].ID); err != nil {
		t.Fatalf("valid binding: %v", err)
	}
	if err := s.ValidateTaskGitBinding(ctx, &pid, wts[0].ID, "00000000-0000-0000-0000-000000000099"); err == nil {
		t.Fatal("expected branch not found")
	}
}

func TestStore_AgentWorkerGitIdle(t *testing.T) {
	ctx := context.Background()
	s := store.NewStore(tasktestdb.OpenSQLite(t))
	idle, reason, err := s.AgentWorkerGitIdle(ctx)
	if err != nil {
		t.Fatalf("AgentWorkerGitIdle: %v", err)
	}
	if !idle || reason != "no_repository_registered" {
		t.Fatalf("got idle=%v reason=%q", idle, reason)
	}
}

func TestStore_ResolveTaskGitContext(t *testing.T) {
	ctx := context.Background()
	s := store.NewStore(tasktestdb.OpenSQLite(t))
	gitSvc := gitwork.New()
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "main", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	repoRow, err := s.CreateGitRepository(ctx, domain.DefaultProjectID, store.CreateGitRepositoryInput{Path: dir}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	wts, _ := s.ListGitWorktrees(ctx, domain.DefaultProjectID, repoRow.ID)
	branches, _ := s.ListGitBranches(ctx, domain.DefaultProjectID, repoRow.ID)
	gitCtx, err := s.ResolveTaskGitContext(ctx, wts[0].ID, branches[0].ID)
	if err != nil {
		t.Fatalf("ResolveTaskGitContext: %v", err)
	}
	if gitCtx.WorktreePath == "" || gitCtx.BranchName == "" {
		t.Fatalf("got %#v", gitCtx)
	}
}
