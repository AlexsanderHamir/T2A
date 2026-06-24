package agentreconcile

import (
	"context"
	"os/exec"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// seedAgentReconcileGit initialises a temp git repo, registers it with the
// store, associates the default worktree with its branch, and returns ids.
// Skips when git is not on PATH (matches pkgs/tasks/store/facade_git_test.go).
func seedAgentReconcileGit(t *testing.T, st *store.Store) (worktreeID, branchID, worktreeBranchID string) {
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
	branches, err := st.ListGitBranches(ctx, domain.DefaultProjectID, repoRow.ID)
	if err != nil || len(branches) == 0 {
		t.Fatalf("ListGitBranches: %v len=%d", err, len(branches))
	}
	wb, err := st.AssociateWorktreeBranch(ctx, store.AssociateWorktreeBranchInput{
		WorktreeID: wts[0].ID,
		BranchID:   branches[0].ID,
	})
	if err != nil {
		t.Fatalf("AssociateWorktreeBranch: %v", err)
	}
	return wts[0].ID, branches[0].ID, wb.ID
}
