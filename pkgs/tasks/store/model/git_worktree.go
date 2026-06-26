package model

import "time"

// GitWorktree is the GORM persistence shape for domain.GitWorktree.
type GitWorktree struct {
	ID             string    `gorm:"primaryKey"`
	RepositoryID   string    `gorm:"not null;index;uniqueIndex:idx_git_worktree_repo_path,priority:1"`
	Path           string    `gorm:"not null;uniqueIndex:idx_git_worktree_repo_path,priority:2"`
	Name           string    `gorm:"not null"`
	IsMain         bool      `gorm:"not null;default:false"`
	ActiveBranchID *string   `gorm:"index"`
	CreatedAt      time.Time `gorm:"not null;index"`
}

// TableName pins the git_worktrees table name.
func (GitWorktree) TableName() string { return "git_worktrees" }
