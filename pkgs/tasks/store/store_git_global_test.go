package store

import (
	"path/filepath"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func TestStore_GlobalGitRepository_andWorktreeBranch(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)

	repo, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGlobalGitRepository: %v", err)
	}
	if repo.DefaultBranch != "" {
		t.Fatalf("DefaultBranch=%q want empty (path-only registration)", repo.DefaultBranch)
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
	branches, err := s.ListGitBranchesByRepo(ctx, repo.ID)
	if err != nil || len(branches) == 0 {
		t.Fatalf("ListGitBranchesByRepo: %v len=%d", err, len(branches))
	}
	var branchID string
	for _, b := range branches {
		if b.Name == "feature-global" {
			branchID = b.ID
			break
		}
	}
	if branchID == "" {
		t.Fatal("feature-global branch not found")
	}
	list, err := s.ListWorktreeBranches(ctx, wt.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListWorktreeBranches after create: %v len=%d", err, len(list))
	}
	wb := list[0]
	if err := s.ValidateTaskWorktreeBranchBinding(ctx, nil, wb.ID); err != nil {
		t.Fatalf("ValidateTaskWorktreeBranchBinding: %v", err)
	}
	gitCtx, err := s.ResolveTaskGitContextFromAssociation(ctx, wb.ID)
	if err != nil {
		t.Fatalf("ResolveTaskGitContextFromAssociation: %v", err)
	}
	if gitCtx.WorktreePath == "" || gitCtx.BranchName != "feature-global" {
		t.Fatalf("context=%+v", gitCtx)
	}
	if err := s.SetActiveBranch(ctx, wt.ID, branchID); err != nil {
		t.Fatalf("SetActiveBranch: %v", err)
	}
	wt2Path := filepath.Join(filepath.Dir(main), "wt-global-2")
	wt2, err := s.CreateGitWorktreeForRepo(ctx, repo.ID, CreateGitWorktreeInput{
		Path:         wt2Path,
		Branch:       "feature-other",
		CreateBranch: true,
	}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitWorktreeForRepo wt2: %v", err)
	}
	err = s.SetActiveBranch(ctx, wt2.ID, branchID)
	if domain.GitErrCode(err) != domain.GitCodeBranchActiveElsewhere {
		t.Fatalf("SetActiveBranch elsewhere: got %v want branch_active_elsewhere", err)
	}
	if err := s.RemoveWorktreeBranch(ctx, wt.ID, branchID); err != nil {
		t.Fatalf("RemoveWorktreeBranch: %v", err)
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
	list, err := s.ListWorktreeBranches(ctx, wt.ID)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListWorktreeBranches after create: %v len=%d", err, len(list))
	}
	wb := list[0]
	pid := proj.ID
	if err := s.ValidateTaskWorktreeBranchBinding(ctx, &pid, wb.ID); err != nil {
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
	err = s.ValidateTaskWorktreeBranchBinding(ctx, &otherPID, wb.ID)
	if err == nil {
		t.Fatal("expected project_repo_mismatch for project tied to different repo")
	}
	if domain.GitErrCode(err) != domain.GitCodeProjectRepoMismatch {
		t.Fatalf("got %v want project_repo_mismatch", err)
	}
}
