package store

import (
	"context"
	"path/filepath"
	"runtime"
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

func TestWorktreePathKey_matchesSlashAndCaseVariants(t *testing.T) {
	t.Parallel()
	if worktreePathKey(`C:\repo\main`) != worktreePathKey(`C:/repo/main`) {
		t.Fatal("slash variants should match")
	}
	if worktreePathKey(`/repo/main`) != worktreePathKey(`/repo/main/`) {
		t.Fatal("trailing slash should be ignored")
	}
	if runtime.GOOS == "windows" {
		if worktreePathKey(`C:\Repo\Main`) != worktreePathKey(`c:/repo/main`) {
			t.Fatal("case variants should match on Windows")
		}
	}
}

func TestFindWorktreeInInventory_normalizesPath(t *testing.T) {
	t.Parallel()
	rows := []WorktreeInventoryRow{
		{Path: `C:/Users/dev/app`, Branch: "main", IsMain: true},
	}
	got, ok := FindWorktreeInInventory(rows, `C:\Users\dev\app`)
	if !ok || got.Path != `C:/Users/dev/app` {
		t.Fatalf("FindWorktreeInInventory: ok=%v got=%+v", ok, got)
	}
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
	mainProbe, err := s.ProbeGitWorktree(ctx, repoRow.ID, main, gitSvc)
	if err != nil || !mainProbe.Linked || !mainProbe.Registered || !mainProbe.IsMain {
		t.Fatalf("seeded main: %+v err=%v", mainProbe, err)
	}
	unlinked, err := s.ProbeGitWorktree(ctx, repoRow.ID, foreign, gitSvc)
	if err != nil || unlinked.Linked {
		t.Fatalf("foreign repo: %+v err=%v", unlinked, err)
	}
}

func TestRepoWorktreeInventory_incompleteDiscoverRowNotRegistered(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(filepath.Dir(main), "wt-incomplete")
	repoGit := openGitRepo(t, main)
	if _, err := gitSvc.AddWorktree(ctx, repoGit, wtPath, addWorktreeOpts("incomplete", true)); err != nil {
		t.Fatal(err)
	}
	out, err := s.ReconcileGitRepository(ctx, "", repo.ID, ReconcileGitInput{
		AllowDiscover: true,
	}, gitSvc)
	if err != nil {
		t.Fatalf("ReconcileGitRepository: %v", err)
	}
	if out.Report.WorktreesAdded != 1 {
		t.Fatalf("worktrees_added=%d want 1 (linked only, not main)", out.Report.WorktreesAdded)
	}
	rows, err := s.RepoWorktreeInventory(ctx, repo, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	var linkedRow *WorktreeInventoryRow
	for i := range rows {
		if worktreePathKey(rows[i].Path) == worktreePathKey(wtPath) {
			linkedRow = &rows[i]
			break
		}
	}
	if linkedRow == nil {
		t.Fatalf("inventory missing linked worktree at %s: %+v", wtPath, rows)
	}
	if linkedRow.Registered {
		t.Fatal("discovered row without branch_id must not count as registered")
	}
}

func TestRegisterExistingGitWorktree_completesIncompleteDiscoverRow(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(filepath.Dir(main), "wt-complete")
	repoGit := openGitRepo(t, main)
	if _, err := gitSvc.AddWorktree(ctx, repoGit, wtPath, addWorktreeOpts("complete-me", true)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReconcileGitRepository(ctx, "", repo.ID, ReconcileGitInput{AllowDiscover: true}, gitSvc); err != nil {
		t.Fatal(err)
	}
	wt, err := s.RegisterExistingGitWorktree(ctx, repo.ID, wtPath, "feature", BindBranchInput{
		Name: "complete-me",
	}, gitSvc)
	if err != nil {
		t.Fatalf("RegisterExistingGitWorktree: %v", err)
	}
	if wt.BranchID == "" {
		t.Fatal("expected branch_id after completing registration")
	}
	if wt.Name != "feature" {
		t.Fatalf("name=%q want feature", wt.Name)
	}
	probe, err := s.ProbeGitWorktree(ctx, repo.ID, wtPath, gitSvc)
	if err != nil || !probe.Registered {
		t.Fatalf("probe after register: %+v err=%v", probe, err)
	}
}

func TestRegisterExistingGitWorktree_rejectsDuplicateRegisteredPath(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	wtPath := filepath.Join(filepath.Dir(main), "wt-dup-register")
	repoGit := openGitRepo(t, main)
	if _, err := gitSvc.AddWorktree(ctx, repoGit, wtPath, addWorktreeOpts("dup-branch", true)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RegisterExistingGitWorktree(ctx, repo.ID, wtPath, "first", BindBranchInput{
		Name: "dup-branch",
	}, gitSvc); err != nil {
		t.Fatalf("first register: %v", err)
	}
	_, err = s.RegisterExistingGitWorktree(ctx, repo.ID, wtPath, "second", BindBranchInput{
		Name: "dup-branch",
	}, gitSvc)
	if domain.GitErrCode(err) != domain.GitCodePathExists {
		t.Fatalf("duplicate register: got %v want path_exists", err)
	}
}

func TestRepoWorktreeInventory_registeredOnlyWhenBranchBound(t *testing.T) {
	s, ctx, gitSvc := gitTestStore(t)
	main := initGitRepo(t)
	repo, err := s.CreateGlobalGitRepository(ctx, CreateGitRepositoryInput{Path: main}, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.RepoWorktreeInventory(ctx, repo, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	var mainRow *WorktreeInventoryRow
	for i := range rows {
		if rows[i].IsMain {
			mainRow = &rows[i]
			break
		}
	}
	if mainRow == nil {
		t.Fatal("inventory missing main checkout")
	}
	if !mainRow.Registered {
		t.Fatal("seeded main worktree with branch_id must count as registered")
	}
	wtPath := filepath.Join(filepath.Dir(main), "wt-unreg-inv")
	repoGit := openGitRepo(t, main)
	if _, err := gitSvc.AddWorktree(ctx, repoGit, wtPath, addWorktreeOpts("unreg-inv", true)); err != nil {
		t.Fatal(err)
	}
	rows, err = s.RepoWorktreeInventory(ctx, repo, gitSvc)
	if err != nil {
		t.Fatal(err)
	}
	var linkedRow *WorktreeInventoryRow
	for i := range rows {
		if worktreePathKey(rows[i].Path) == worktreePathKey(wtPath) {
			linkedRow = &rows[i]
			break
		}
	}
	if linkedRow == nil {
		t.Fatalf("inventory missing linked worktree at %s", wtPath)
	}
	if linkedRow.Registered {
		t.Fatal("unregistered linked worktree must not count as registered")
	}
}
