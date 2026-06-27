package store

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func gitTestStore(t *testing.T) (*Store, context.Context, gitwork.Service) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	return NewStore(tasktestdb.OpenSQLite(t)), context.Background(), gitwork.New()
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitStore(t, dir, "init", "-b", "main")
	runGitStore(t, dir, "config", "user.email", "t@example.com")
	runGitStore(t, dir, "config", "user.name", "Test")
	runGitStore(t, dir, "commit", "--allow-empty", "-m", "init")
	return dir
}

func runGitStore(t *testing.T, dir string, args ...string) {
	t.Helper()
	all := append([]string{"-C", dir}, args...)
	if out, err := exec.Command("git", all...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestStore_GitRepositoryCRUD(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)

	repo, err := s.CreateGitRepository(ctx, domain.DefaultProjectID, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitRepository: %v", err)
	}
	list, err := s.ListGitRepositories(ctx, domain.DefaultProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("len=%d want 1", len(list))
	}
	got, err := s.GetGitRepository(ctx, domain.DefaultProjectID, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path == "" {
		t.Fatal("empty path")
	}
	if err := s.DeleteGitRepository(ctx, domain.DefaultProjectID, repo.ID); err != nil {
		t.Fatalf("DeleteGitRepository: %v", err)
	}
}

func TestStore_GitRepository_notARepository(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	dir := t.TempDir()
	_, err := s.CreateGitRepository(ctx, domain.DefaultProjectID, CreateGitRepositoryInput{Path: dir}, gitSvc)
	if domain.GitErrCode(err) != domain.GitCodeNotARepository {
		t.Fatalf("got %v want not_a_git_repository", err)
	}
}

func TestStore_GitWorktreeAndBranch_roundtrip(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGitRepository(ctx, domain.DefaultProjectID, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(filepath.Dir(main), "wt-a")
	wt, err := s.CreateGitWorktree(ctx, domain.DefaultProjectID, repo.ID, CreateGitWorktreeInput{
		Path:         wtPath,
		Branch:       "feature-a",
		CreateBranch: true,
	}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitWorktree: %v", err)
	}
	wts, err := s.ListGitWorktrees(ctx, domain.DefaultProjectID, repo.ID)
	if err != nil || len(wts) != 2 {
		t.Fatalf("worktrees: %v len=%d", err, len(wts))
	}
	branch, err := s.CreateGitBranch(ctx, domain.DefaultProjectID, repo.ID, CreateGitBranchInput{
		Name:       "feature-b",
		StartPoint: "main",
	}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitBranch: %v", err)
	}
	branches, err := s.ListGitBranches(ctx, domain.DefaultProjectID, repo.ID)
	if err != nil || len(branches) < 2 {
		t.Fatalf("branches: %v len=%d", err, len(branches))
	}
	if err := s.DeleteGitWorktree(ctx, domain.DefaultProjectID, wt.ID, true, gitSvc); err != nil {
		t.Fatalf("DeleteGitWorktree: %v", err)
	}
	if err := s.DeleteGitBranch(ctx, domain.DefaultProjectID, branch.ID, true, gitSvc); err != nil {
		t.Fatalf("DeleteGitBranch: %v", err)
	}
}

func TestStore_GitDeleteGuard_runningTask(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGitRepository(ctx, domain.DefaultProjectID, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(filepath.Dir(main), "wt-guard")
	wt, err := s.CreateGitWorktree(ctx, domain.DefaultProjectID, repo.ID, CreateGitWorktreeInput{
		Path:         wtPath,
		Branch:       "guard-branch",
		CreateBranch: true,
	}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	branches, _ := s.ListGitBranches(ctx, domain.DefaultProjectID, repo.ID)
	var branchID string
	for _, b := range branches {
		if b.Name == "guard-branch" {
			branchID = b.ID
			break
		}
	}
	if branchID == "" {
		t.Fatal("guard-branch not found")
	}
	if wt.BranchID != branchID {
		t.Fatalf("worktree branch_id = %q want %q", wt.BranchID, branchID)
	}
	wtID := wt.ID
	task := domain.Task{
		ID:            "task-running-guard",
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
	err = s.DeleteGitWorktree(ctx, domain.DefaultProjectID, wt.ID, true, gitSvc)
	if ge := domain.GitErrCode(err); ge != domain.GitCodeHasRunningTask {
		t.Fatalf("got code %q want has_running_task (%v)", ge, err)
	}
}
