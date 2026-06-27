// Package gittest provides shared git repository bootstrap helpers for
// integration and handler tests. Centralizes init, skip-when-missing-git,
// and store git-binding seeding so harness, handler, and worker tests do
// not duplicate exec.Command sequences.
package gittest

import (
	"context"
	"os/exec"
	"testing"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitwork"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

// SkipIfNoGit skips t when the git binary is not on PATH.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only git bootstrap; not part of production trace paths."
func SkipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed; skipping git test")
	}
}

// Init initializes dir as a git repository with an empty initial commit.
// Uses default branch naming and inline user identity for the commit.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only git bootstrap; not part of production trace paths."
func Init(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"-c", "user.email=t@e.local", "-c", "user.name=t", "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// InitOrSkip calls SkipIfNoGit then Init.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only git bootstrap; not part of production trace paths."
func InitOrSkip(t *testing.T, dir string) {
	t.Helper()
	SkipIfNoGit(t)
	Init(t, dir)
}

// InitMain initializes dir on branch main with user config and an empty commit.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only git bootstrap; not part of production trace paths."
func InitMain(t *testing.T, dir string) {
	t.Helper()
	if out, err := exec.Command("git", "init", "-b", "main", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	for _, args := range [][]string{
		{"config", "user.email", "test@test.local"},
		{"config", "user.name", "Test"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if out, err := exec.Command("git", "-C", dir, "commit", "-m", "init", "--allow-empty").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v %s", err, out)
	}
}

// EnsureMain ensures dir is a git repository on main without re-init when
// already present.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only git bootstrap; not part of production trace paths."
func EnsureMain(t *testing.T, dir string) {
	t.Helper()
	if err := exec.Command("git", "-C", dir, "rev-parse", "--git-dir").Run(); err == nil {
		return
	}
	if out, err := exec.Command("git", "init", "-b", "main", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	for _, args := range [][]string{
		{"config", "user.email", "test@test.local"},
		{"config", "user.name", "Test"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	_ = exec.Command("git", "-C", dir, "commit", "-m", "init", "--allow-empty").Run()
}

// SeedWorktree registers repoDir in the store and returns the main worktree id.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only git bootstrap; not part of production trace paths."
func SeedWorktree(t *testing.T, st *store.Store, repoDir string) (worktreeID, branchID string) {
	t.Helper()
	EnsureMain(t, repoDir)
	ctx := context.Background()
	gitSvc := gitwork.New()
	repoRow, err := st.CreateGitRepository(ctx, domain.DefaultProjectID, store.CreateGitRepositoryInput{
		Path: repoDir,
	}, gitSvc)
	if err != nil {
		t.Fatalf("CreateGitRepository: %v", err)
	}
	wts, err := st.ListGitWorktrees(ctx, domain.DefaultProjectID, repoRow.ID)
	if err != nil || len(wts) == 0 {
		t.Fatalf("ListGitWorktrees: %v len=%d", err, len(wts))
	}
	if wts[0].BranchID == "" {
		t.Fatalf("main worktree missing branch_id after CreateGitRepository")
	}
	return wts[0].ID, wts[0].BranchID
}

// SeedWorktreeTemp creates a temp git repo, registers it in the store,
// and returns the worktree id and repo directory path.
//
//funclogmeasure:skip category=tool-required-noop reason="Test-only git bootstrap; not part of production trace paths."
func SeedWorktreeTemp(t *testing.T, st *store.Store) (worktreeID, workDir string) {
	t.Helper()
	dir := t.TempDir()
	InitMain(t, dir)
	wtID, _ := SeedWorktree(t, st, dir)
	return wtID, dir
}

// SeedWorktreeBranch is deprecated: use SeedWorktree. Returns worktreeID twice for legacy call sites.
func SeedWorktreeBranch(t *testing.T, st *store.Store, repoDir string) (worktreeID, branchID, worktreeBranchID string) {
	t.Helper()
	wtID, brID := SeedWorktree(t, st, repoDir)
	return wtID, brID, wtID
}

// SeedWorktreeBranchTemp is deprecated: use SeedWorktreeTemp.
func SeedWorktreeBranchTemp(t *testing.T, st *store.Store) (worktreeBranchID, workDir string) {
	t.Helper()
	wtID, dir := SeedWorktreeTemp(t, st)
	return wtID, dir
}
