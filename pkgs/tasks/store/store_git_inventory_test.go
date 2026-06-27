package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func openGitRepo(t *testing.T, main string) *gitwork.Repository {
	t.Helper()
	repo, err := gitwork.New().OpenRepository(context.Background(), main)
	if err != nil {
		t.Fatalf("OpenRepository: %v", err)
	}
	return repo
}

func addWorktreeOpts(branch string, create bool) gitwork.AddWorktreeOptions {
	return gitwork.AddWorktreeOptions{Branch: branch, CreateBranch: create}
}

func TestStore_CreateGlobalGitRepository_normalizesMainPath(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo := openGitRepo(t, main)
	wtPath := filepath.Join(filepath.Dir(main), "wt-register-main")
	if _, err := gitSvc.AddWorktree(ctx, repo, wtPath, addWorktreeOpts("from-linked", true)); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	created, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: wtPath}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGlobalGitRepository from linked path: %v", err)
	}
	wantMain, _ := filepath.Abs(main)
	if filepath.Clean(created.Path) != filepath.Clean(wantMain) {
		t.Fatalf("Path=%q want main %q", created.Path, wantMain)
	}
	if created.GitCommonDir == "" {
		t.Fatal("GitCommonDir empty")
	}
}

func TestStore_CreateGlobalGitRepository_duplicateCommonDir(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	if _, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: main}, gitSvc); err != nil {
		t.Fatalf("first register: %v", err)
	}
	repo := openGitRepo(t, main)
	wtPath := filepath.Join(filepath.Dir(main), "wt-dup")
	if _, err := gitSvc.AddWorktree(ctx, repo, wtPath, addWorktreeOpts("dup", true)); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	_, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: wtPath}, gitSvc)
	if domain.GitErrCode(err) != domain.GitCodeDuplicate {
		t.Fatalf("duplicate common dir: got %v want duplicate", err)
	}
}

func TestStore_ProbeGitWorktree(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repoRow, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	repo := openGitRepo(t, main)
	wtPath := filepath.Join(filepath.Dir(main), "wt-probe")
	if _, err := gitSvc.AddWorktree(ctx, repo, wtPath, addWorktreeOpts("probe", true)); err != nil {
		t.Fatal(err)
	}
	foreign := initGitRepo(t)
	linked, err := s.ProbeGitWorktree(ctx, repoRow.ID, wtPath, gitSvc)
	if err != nil || !linked.Linked || linked.Registered {
		t.Fatalf("linked unregistered: %+v err=%v", linked, err)
	}
	unlinked, err := s.ProbeGitWorktree(ctx, repoRow.ID, foreign, gitSvc)
	if err != nil || unlinked.Linked {
		t.Fatalf("foreign repo: %+v err=%v", unlinked, err)
	}
}
