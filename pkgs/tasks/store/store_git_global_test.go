package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func TestStore_GlobalGitRepository_andWorktreeBinding(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)

	repo, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGlobalGitRepository: %v", err)
	}
	all, err := s.ListAllGitRepositories(ctx)
	if err != nil || len(all) != 1 {
		t.Fatalf("ListAllGitRepositories: %v len=%d", err, len(all))
	}
	wtPath := filepath.Join(filepath.Dir(main), "wt-global")
	wt, err := s.CreateGitWorktreeForRepo(ctx, repo.ID, CreateGitWorktreeInput{
		Path:         wtPath,
		Branch:       "feature-global",
		CreateBranch: true,
	}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitWorktreeForRepo: %v", err)
	}
	if wt.BranchID == "" {
		t.Fatal("worktree missing branch_id after create")
	}
	if err := s.ValidateTaskWorktreeBinding(ctx, nil, wt.ID); err != nil {
		t.Fatalf("ValidateTaskWorktreeBinding: %v", err)
	}
	gitCtx, err := s.ResolveTaskGitContext(ctx, wt.ID)
	if err != nil {
		t.Fatalf("ResolveTaskGitContext: %v", err)
	}
	if gitCtx.WorktreePath == "" || gitCtx.BranchName != "feature-global" {
		t.Fatalf("context=%+v", gitCtx)
	}
	wt2Path := filepath.Join(filepath.Dir(main), "wt-global-2")
	_, err = s.CreateGitWorktreeForRepo(ctx, repo.ID, CreateGitWorktreeInput{
		Path:         wt2Path,
		Branch:       "feature-global",
		CreateBranch: true,
	}, gitSvc)
	if domain.GitErrCode(err) != domain.GitCodeBranchBoundToWorktree {
		t.Fatalf("duplicate branch on second worktree: got %v want branch_bound_to_worktree", err)
	}
	if err := s.DeleteGitWorktreeByID(ctx, wt.ID, true, gitSvc); err != nil {
		t.Fatalf("DeleteGitWorktreeByID: %v", err)
	}
	if err := s.DeleteGlobalGitRepository(ctx, repo.ID); err != nil {
		t.Fatalf("DeleteGlobalGitRepository: %v", err)
	}
}

func TestStore_ProjectRepositoryBinding(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	repoID := repo.ID
	proj, err := s.CreateProject(ctx, CreateProjectInput{
		Name:         "Overlay",
		RepositoryID: &repoID,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	byRepo, err := s.ListProjectsByRepository(ctx, repo.ID)
	if err != nil || len(byRepo) != 1 {
		t.Fatalf("ListProjectsByRepository: %v len=%d", err, len(byRepo))
	}
	wtPath := filepath.Join(filepath.Dir(main), "wt-proj")
	wt, err := s.CreateGitWorktreeForRepo(ctx, repo.ID, CreateGitWorktreeInput{
		Path:         wtPath,
		Branch:       "proj-branch",
		CreateBranch: true,
	}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	pid := proj.ID
	if err := s.ValidateTaskWorktreeBinding(ctx, &pid, wt.ID); err != nil {
		t.Fatalf("valid project binding: %v", err)
	}
	otherMain := initGitRepo(t)
	otherRepo, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: otherMain}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	otherRepoID := otherRepo.ID
	otherProj, err := s.CreateProject(ctx, CreateProjectInput{
		Name:         "Other overlay",
		RepositoryID: &otherRepoID,
	})
	if err != nil {
		t.Fatal(err)
	}
	otherPID := otherProj.ID
	err = s.ValidateTaskWorktreeBinding(ctx, &otherPID, wt.ID)
	if err == nil {
		t.Fatal("expected project_repo_mismatch for project tied to different repo")
	}
	if domain.GitErrCode(err) != domain.GitCodeProjectRepoMismatch {
		t.Fatalf("got %v want project_repo_mismatch", err)
	}
}

func TestDeleteGitWorktreeByID_rejectsRunningTask(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(filepath.Dir(main), "wt-running-global")
	wt, err := s.CreateGitWorktreeForRepo(ctx, repo.ID, CreateGitWorktreeInput{
		Path:         wtPath,
		Branch:       "running-global",
		CreateBranch: true,
	}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	wtID := wt.ID
	task := domain.Task{
		ID:            "task-global-running",
		Title:         "running",
		InitialPrompt: "x",
		Status:        domain.StatusRunning,
		Priority:      domain.PriorityMedium,
		Runner:        "cursor",
		WorktreeID:    &wtID,
	}
	if err := s.db.WithContext(ctx).Create(&task).Error; err != nil {
		t.Fatal(err)
	}
	err = s.DeleteGitWorktreeByID(ctx, wt.ID, true, gitSvc)
	if domain.GitErrCode(err) != domain.GitCodeHasRunningTask {
		t.Fatalf("got %v want has_running_task", err)
	}
}

func TestRelocateGitWorktree_updatesRegisteredPath(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(filepath.Dir(main), "wt-reloc-store")
	wt, err := s.CreateGitWorktreeForRepo(ctx, repo.ID, CreateGitWorktreeInput{
		Path:         wtPath,
		Branch:       "reloc-store",
		CreateBranch: true,
	}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	movedPath := filepath.Join(filepath.Dir(main), "wt-reloc-store-moved")
	runGitStore(t, main, "worktree", "move", wtPath, movedPath)
	t.Cleanup(func() { _ = os.RemoveAll(movedPath) })

	got, err := s.RelocateGitWorktree(ctx, wt.ID, movedPath, gitSvc)
	if err != nil {
		t.Fatalf("RelocateGitWorktree: %v", err)
	}
	if worktreePathKey(got.Path) != worktreePathKey(movedPath) {
		t.Fatalf("path=%q want %q", got.Path, movedPath)
	}
}

func TestCreateGitBranchForRepo_global(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	br, err := s.CreateGitBranchForRepo(ctx, repo.ID, CreateGitBranchInput{
		Name:       "feature-global-branch",
		StartPoint: "main",
	}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitBranchForRepo: %v", err)
	}
	if br.Name != "feature-global-branch" || br.HeadSHA == "" {
		t.Fatalf("branch=%+v", br)
	}
}
