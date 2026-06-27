// Package gitwork wraps fixed git subprocess operations for worktree and branch
// management. Callers use Service; handlers and the worker must not invoke git
// directly for these operations.
package gitwork

import (
	"context"
	"errors"
)

// Repository is an opened git repository (main worktree + common git dir).
type Repository struct {
	Root      string // absolute path of the main worktree
	CommonDir string // absolute path of the shared git directory
}

// Worktree is one linked working directory for a repository.
type Worktree struct {
	Path     string
	Branch   string // empty when detached HEAD
	IsMain   bool
	Locked   bool
	Prunable bool
}

// Branch is a local branch ref in a repository.
type Branch struct {
	Name      string
	HeadSHA   string
	IsCurrent bool
	Upstream  string // empty when no upstream tracking branch
}

// AddWorktreeOptions configures git worktree add.
type AddWorktreeOptions struct {
	Branch       string // existing branch, or new branch name when CreateBranch is true
	CreateBranch bool
	StartPoint   string // optional ref to start a new branch from
}

// Service performs git worktree and branch operations.
type Service interface {
	OpenRepository(ctx context.Context, path string) (*Repository, error)

	ListWorktrees(ctx context.Context, repo *Repository) ([]Worktree, error)
	AddWorktree(ctx context.Context, repo *Repository, path string, opts AddWorktreeOptions) (*Worktree, error)
	RemoveWorktree(ctx context.Context, repo *Repository, path string, force bool) error

	ListBranches(ctx context.Context, repo *Repository) ([]Branch, error)
	CreateBranch(ctx context.Context, repo *Repository, name, startPoint string) (*Branch, error)
	DeleteBranch(ctx context.Context, repo *Repository, name string, force bool) error
	WorktreeCurrentBranch(ctx context.Context, worktreePath string) (string, error)
	Checkout(ctx context.Context, worktreePath, branch string) error
}

var (
	// ErrNotARepository is returned when the path is not inside a git repository.
	ErrNotARepository = errors.New("gitwork: not a git repository")
	// ErrWorktreeExists is returned when the worktree path already exists.
	ErrWorktreeExists = errors.New("gitwork: worktree path already exists")
	// ErrBranchExists is returned when creating a branch that already exists.
	ErrBranchExists = errors.New("gitwork: branch already exists")
	// ErrBranchCheckedOut is returned when a branch is checked out elsewhere.
	ErrBranchCheckedOut = errors.New("gitwork: branch is checked out in another worktree")
	// ErrDirty is returned when uncommitted changes block checkout or removal.
	ErrDirty = errors.New("gitwork: worktree has uncommitted changes")
	// ErrGitMissing is returned when the git binary is not on PATH.
	ErrGitMissing = errors.New("gitwork: git binary not found on PATH")
)
