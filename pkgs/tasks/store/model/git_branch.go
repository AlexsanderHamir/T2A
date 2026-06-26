package model

import "time"

// GitBranch is the GORM persistence shape for domain.GitBranch.
type GitBranch struct {
	ID           string    `gorm:"primaryKey"`
	RepositoryID string    `gorm:"not null;index;uniqueIndex:idx_git_branch_repo_name,priority:1"`
	Name         string    `gorm:"not null;uniqueIndex:idx_git_branch_repo_name,priority:2"`
	HeadSHA      string    `gorm:"not null;default:''"`
	CreatedAt    time.Time `gorm:"not null;index"`
}

// TableName pins the git_branches table name.
func (GitBranch) TableName() string { return "git_branches" }
