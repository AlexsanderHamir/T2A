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
	// GitCodeBranchBoundToWorktree is returned when a branch is already assigned
	// to a different worktree at register/create time. See ADR-0039.
	GitCodeBranchBoundToWorktree = "branch_bound_to_worktree"
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
// and GitCommonDir (one row per git object database). See ADR-0037.
type GitRepository struct {
	ID            string    `json:"id" gorm:"primaryKey"`
	Path          string    `json:"path" gorm:"not null;uniqueIndex:idx_git_repo_path"`
	GitCommonDir  string    `json:"git_common_dir" gorm:"not null;default:'';uniqueIndex:idx_git_repo_common_dir"`
	HostPath      string    `json:"host_path" gorm:"not null;default:''"`
	DefaultBranch string    `json:"default_branch" gorm:"not null;default:''"`
	CreatedAt     time.Time `json:"created_at" gorm:"not null;index"`
	UpdatedAt     time.Time `json:"updated_at" gorm:"not null;index"`
}

// GitWorktree is a linked working directory for a GitRepository with a fixed
// branch assignment. See ADR-0039.
type GitWorktree struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	RepositoryID string    `json:"repository_id" gorm:"not null;index;uniqueIndex:idx_git_worktree_repo_path,priority:1"`
	Path         string    `json:"path" gorm:"not null;uniqueIndex:idx_git_worktree_repo_path,priority:2"`
	Name         string    `json:"name" gorm:"not null"`
	IsMain       bool      `json:"is_main" gorm:"not null;default:false"`
	BranchID     string    `json:"branch_id" gorm:"not null;index;uniqueIndex:idx_git_worktree_branch_unique"`
	CreatedAt    time.Time `json:"created_at" gorm:"not null;index"`
}

// GitBranch is a local branch tracked for a GitRepository (repo-level ref).
type GitBranch struct {
	ID           string    `json:"id" gorm:"primaryKey"`
	RepositoryID string    `json:"repository_id" gorm:"not null;index;uniqueIndex:idx_git_branch_repo_name,priority:1"`
	Name         string    `json:"name" gorm:"not null;uniqueIndex:idx_git_branch_repo_name,priority:2"`
	HeadSHA      string    `json:"head_sha" gorm:"not null;default:''"`
	CreatedAt    time.Time `json:"created_at" gorm:"not null;index"`
}
