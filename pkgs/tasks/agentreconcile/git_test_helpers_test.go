package agentreconcile

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// seedAgentReconcileGit initialises a temp git repo, registers it with the
// store, and returns the main worktree and branch ids.
// Skips when git is not on PATH (matches pkgs/tasks/store/facade_git_test.go).
func seedAgentReconcileGit(t *testing.T, st *store.Store) (worktreeID, branchID string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "main", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	for _, args := range [][]string{
		{"config", "user.email", "agentreconcile-test@test.local"},
		{"config", "user.name", "Agent Reconcile Test"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-m", "init", "--allow-empty").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v %s", err, out)
	}
	ctx := context.Background()
	gitSvc := gitwork.New()
	repoRow, err := st.CreateGitRepository(ctx, domain.DefaultProjectID, store.CreateGitRepositoryInput{Path: dir}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitRepository: %v", err)
	}
	wts, err := st.ListGitWorktrees(ctx, domain.DefaultProjectID, repoRow.ID)
	if err != nil || len(wts) == 0 {
		t.Fatalf("ListGitWorktrees: %v len=%d", err, len(wts))
	}
	if wts[0].BranchID == "" {
		t.Fatal("main worktree missing branch_id")
	}
	return wts[0].ID, wts[0].BranchID
}

// seedSecondWorktreeOnRepo adds a linked worktree on a new branch in the same repo.
func seedSecondWorktreeOnRepo(t *testing.T, st *store.Store, firstWorktreeID string) (secondWorktreeID string) {
	t.Helper()
	ctx := context.Background()
	wt, err := st.GetGitWorktreeByID(ctx, firstWorktreeID)
	if err != nil {
		t.Fatalf("GetGitWorktreeByID: %v", err)
	}
	repo, err := st.GetGitRepositoryByID(ctx, wt.RepositoryID)
	if err != nil {
		t.Fatalf("GetGitRepositoryByID: %v", err)
	}
	gitSvc := gitwork.New()
	if out, err := exec.Command("git", "-C", repo.Path, "branch", "feature-b").CombinedOutput(); err != nil {
		t.Fatalf("git branch feature-b: %v %s", err, out)
	}
	wt2Path := filepath.Join(filepath.Dir(repo.Path), "wt-feature-b")
	wt2, err := st.CreateGitWorktreeForRepo(ctx, repo.ID, store.CreateGitWorktreeInput{
		Path:         wt2Path,
		Branch:       "feature-b",
		CreateBranch: false,
	}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitWorktreeForRepo feature-b: %v", err)
	}
	return wt2.ID
}
