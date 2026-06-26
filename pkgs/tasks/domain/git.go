package domain

import (
	"errors"
	"time"
)

// Git API error codes returned in JSON {"error","code"} responses.
const (
	GitCodeNotARepository     = "not_a_git_repository"
	GitCodePathExists         = "path_exists"
	GitCodeBranchExists       = "branch_exists"
	GitCodeBranchCheckedOut   = "branch_checked_out"
	GitCodeHasRunningTask     = "has_running_task"
	GitCodeRepositoryNotFound = "repository_not_found"
	GitCodeWorktreeNotFound   = "worktree_not_found"
	GitCodeBranchNotFound     = "branch_not_found"
	GitCodeDuplicate          = "duplicate"
	// GitCodeBranchActiveElsewhere is returned when a branch is the active
	// checkout in another worktree and Hamix rejects binding/running against
	// it (replaces the soft "checked out elsewhere" warning). See ADR-0037.
	GitCodeBranchActiveElsewhere = "branch_active_elsewhere"
	// GitCodeBranchNotAssociated is returned when a task binds a (worktree,
	// branch) pair that has no worktree_branches association row.
	GitCodeBranchNotAssociated = "branch_not_associated"
	// GitCodeProjectRepoMismatch is returned when a task's project belongs to
	// a different repository than its bound worktree.
	GitCodeProjectRepoMismatch = "project_repo_mismatch"
)

// GitErr is a domain error with a stable API code for git entity routes.
type GitErr struct {
	Code string
	Msg  string
}

func (e *GitErr) Error() string { return e.Msg }

// NewGitErr returns an error tagged with a stable git API code.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func NewGitErr(code, msg string) error {
	return &GitErr{Code: code, Msg: msg}
}

// GitErrCode returns the stable code when err wraps *GitErr.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func GitErrCode(err error) string {
	var ge *GitErr
	if errors.As(err, &ge) {
		return ge.Code
	}
	return ""
}

// GitRepository is a registered main git checkout. Globally unique on Path
// (one row per canonical path, shared across projects). See ADR-0037.
type GitRepository struct {
	ID            string    `json:"id"`
	Path          string    `json:"path"`
	HostPath      string    `json:"host_path"`
	DefaultBranch string    `json:"default_branch"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// GitWorktree is a linked working directory for a GitRepository.
//
// ActiveBranchID tracks the branch currently checked out in this directory; at
// most one branch is the active checkout in a given worktree at a time. Plain
// indexed nullable column (no FK constraint) — set by the store/worker, cleared
// on completion. See ADR-0037.
type GitWorktree struct {
	ID             string    `json:"id"`
	RepositoryID   string    `json:"repository_id"`
	Path           string    `json:"path"`
	Name           string    `json:"name"`
	IsMain         bool      `json:"is_main"`
	ActiveBranchID *string   `json:"active_branch_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// GitBranch is a local branch tracked for a GitRepository (repo-level ref).
type GitBranch struct {
	ID           string    `json:"id"`
	RepositoryID string    `json:"repository_id"`
	Name         string    `json:"name"`
	HeadSHA      string    `json:"head_sha"`
	CreatedAt    time.Time `json:"created_at"`
}

// WorktreeBranch associates a repo-level branch with a worktree directory
// ("this branch, in this directory"). It is the precise node a task runs
// against: tasks bind to a WorktreeBranch via Task.WorktreeBranchID. The
// (WorktreeID, BranchID) pair is unique. Both sides must share the same
// repository (enforced at the store boundary). See ADR-0037.
type WorktreeBranch struct {
	ID         string    `json:"id"`
	WorktreeID string    `json:"worktree_id"`
	BranchID   string    `json:"branch_id"`
	CreatedAt  time.Time `json:"created_at"`
}
