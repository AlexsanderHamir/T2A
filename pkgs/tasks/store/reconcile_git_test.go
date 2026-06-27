package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

func TestReconcileGitRepository_needsBootstrapWhenPathMissing(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGitRepository(ctx, domain.DefaultProjectID, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	renamed := filepath.Join(filepath.Dir(main), "renamed-main")
	if err := os.Rename(main, renamed); err != nil {
		t.Fatalf("rename main: %v", err)
	}
	t.Cleanup(func() { _ = os.Rename(renamed, main) })

	out, err := s.ReconcileGitRepository(ctx, domain.DefaultProjectID, repo.ID, ReconcileGitInput{
		AllowRemove: true,
	}, gitSvc)
	if err != nil {
		t.Fatalf("ReconcileGitRepository: %v", err)
	}
	if out.Status != reconcileStatusNeedsBootstrapPath {
		t.Fatalf("status=%q want %q", out.Status, reconcileStatusNeedsBootstrapPath)
	}
}

func TestReconcileGitRepository_mainRenamed_withBootstrap(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGitRepository(ctx, domain.DefaultProjectID, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	wtsBefore, err := s.ListGitWorktrees(ctx, domain.DefaultProjectID, repo.ID)
	if err != nil || len(wtsBefore) == 0 {
		t.Fatalf("worktrees before: %v len=%d", err, len(wtsBefore))
	}
	mainID := wtsBefore[0].ID

	renamed := filepath.Join(filepath.Dir(main), "renamed-main-bootstrap")
	if err := os.Rename(main, renamed); err != nil {
		t.Fatalf("rename main: %v", err)
	}
	t.Cleanup(func() { _ = os.Rename(renamed, main) })

	out, err := s.ReconcileGitRepository(ctx, domain.DefaultProjectID, repo.ID, ReconcileGitInput{
		BootstrapPath: renamed,
		RepairGit:     true,
		AllowRemove:   true,
	}, gitSvc)
	if err != nil {
		t.Fatalf("ReconcileGitRepository: %v", err)
	}
	if out.Status != reconcileStatusOK {
		t.Fatalf("status=%q want ok", out.Status)
	}
	if !out.Report.RepoPathUpdated {
		t.Fatal("expected repo path update")
	}

	gotRepo, err := s.GetGitRepository(ctx, domain.DefaultProjectID, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if worktreePathKey(gotRepo.Path) != worktreePathKey(renamed) {
		t.Fatalf("repo path=%q want %q", gotRepo.Path, renamed)
	}
	gotWT, err := s.GetGitWorktree(ctx, domain.DefaultProjectID, mainID)
	if err != nil {
		t.Fatal(err)
	}
	if gotWT.ID != mainID {
		t.Fatalf("main worktree id changed: %q", gotWT.ID)
	}
	if worktreePathKey(gotWT.Path) != worktreePathKey(renamed) {
		t.Fatalf("main wt path=%q want %q", gotWT.Path, renamed)
	}
}

func TestReconcileGitRepository_linkedWorktreeMoved_preservesID(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGitRepository(ctx, domain.DefaultProjectID, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(filepath.Dir(main), "wt-move-src")
	wt, err := s.CreateGitWorktree(ctx, domain.DefaultProjectID, repo.ID, CreateGitWorktreeInput{
		Path:         wtPath,
		Branch:       "feature-move",
		CreateBranch: true,
	}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitWorktree: %v", err)
	}
	wtPath2 := filepath.Join(filepath.Dir(main), "wt-move-dst")
	runGitStore(t, main, "worktree", "move", wtPath, wtPath2)
	t.Cleanup(func() {
		_ = os.RemoveAll(wtPath2)
	})

	out, err := s.ReconcileGitRepository(ctx, domain.DefaultProjectID, repo.ID, ReconcileGitInput{
		RepairGit:   true,
		AllowRemove: true,
	}, gitSvc)
	if err != nil {
		t.Fatalf("ReconcileGitRepository: %v", err)
	}
	if out.Report.WorktreesPathUpdated < 1 {
		t.Fatalf("expected path update report=%+v", out.Report)
	}
	got, err := s.GetGitWorktree(ctx, domain.DefaultProjectID, wt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != wt.ID {
		t.Fatalf("worktree id changed")
	}
	if worktreePathKey(got.Path) != worktreePathKey(wtPath2) {
		t.Fatalf("path=%q want %q", got.Path, wtPath2)
	}
}

func TestReconcileGitRepository_bootstrapWrongRepo(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	mainA := initGitRepo(t)
	runGitStore(t, mainA, "commit", "--allow-empty", "-m", "marker-a")
	repoA, err := s.CreateGitRepository(ctx, domain.DefaultProjectID, CreateGitRepositoryInput{Path: mainA}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	branches, err := s.ListGitBranches(ctx, domain.DefaultProjectID, repoA.ID)
	if err != nil || len(branches) == 0 || strings.TrimSpace(branches[0].HeadSHA) == "" {
		t.Fatalf("branches for verify: %v len=%d", err, len(branches))
	}
	mainB := initGitRepo(t)
	renamed := filepath.Join(filepath.Dir(mainA), "gone-a")
	if err := os.Rename(mainA, renamed); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(renamed, mainA) })

	_, err = s.ReconcileGitRepository(ctx, domain.DefaultProjectID, repoA.ID, ReconcileGitInput{
		BootstrapPath: mainB,
		AllowRemove:   true,
	}, gitSvc)
	if domain.GitErrCode(err) != domain.GitCodeBootstrapMismatch {
		t.Fatalf("got %v want bootstrap_mismatch", err)
	}
}

func TestStore_CreateGitRepository_setsGitCommonDirAndSingleBranch(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	runGitStore(t, main, "branch", "extra")
	repo, err := s.CreateGitRepository(ctx, domain.DefaultProjectID, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitRepository: %v", err)
	}
	if repo.GitCommonDir == "" {
		t.Fatal("GitCommonDir empty")
	}
	branches, err := s.ListGitBranches(ctx, domain.DefaultProjectID, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 1 {
		t.Fatalf("len(branches)=%d want 1 bound branch only", len(branches))
	}
	if branches[0].Name != "main" {
		t.Fatalf("branch name=%q want main", branches[0].Name)
	}
}

func TestReconcileGitRepository_pathMatch_reportsCheckoutMismatch(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGitRepository(ctx, domain.DefaultProjectID, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(filepath.Dir(main), "wt-checkout")
	wt, err := s.CreateGitWorktree(ctx, domain.DefaultProjectID, repo.ID, CreateGitWorktreeInput{
		Path:         wtPath,
		Branch:       "feature-bound",
		CreateBranch: true,
	}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitWorktree: %v", err)
	}
	runGitStore(t, wtPath, "checkout", "-b", "other-branch")

	out, err := s.ReconcileGitRepository(ctx, domain.DefaultProjectID, repo.ID, ReconcileGitInput{}, gitSvc)
	if err != nil {
		t.Fatalf("ReconcileGitRepository: %v", err)
	}
	if out.Status != reconcileStatusPartial {
		t.Fatalf("status=%q want partial", out.Status)
	}
	found := false
	for _, skip := range out.Report.WorktreesSkipped {
		if skip.WorktreeID == wt.ID && skip.Reason == "branch_checkout_mismatch" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected branch_checkout_mismatch skip report=%+v", out.Report)
	}
}

func TestReconcileGitRepository_dryRun_noWrites(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGitRepository(ctx, domain.DefaultProjectID, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.GetGitRepository(ctx, domain.DefaultProjectID, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.ReconcileGitRepository(ctx, domain.DefaultProjectID, repo.ID, ReconcileGitInput{
		DryRun:      true,
		AllowRemove: true,
	}, gitSvc)
	if err != nil {
		t.Fatalf("ReconcileGitRepository: %v", err)
	}
	if out.Status != reconcileStatusOK {
		t.Fatalf("status=%q", out.Status)
	}
	after, err := s.GetGitRepository(ctx, domain.DefaultProjectID, repo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Path != after.Path || before.UpdatedAt != after.UpdatedAt {
		t.Fatal("dry run modified repository row")
	}
}
