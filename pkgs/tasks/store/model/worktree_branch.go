package model

import "time"

// WorktreeBranch is the GORM persistence shape for domain.WorktreeBranch
// (columns only — no GORM association fields).
type WorktreeBranch struct {
	ID         string    `gorm:"primaryKey"`
	WorktreeID string    `gorm:"not null;index;uniqueIndex:idx_worktree_branch_unique,priority:1"`
	BranchID   string    `gorm:"not null;index;uniqueIndex:idx_worktree_branch_unique,priority:2"`
	CreatedAt  time.Time `gorm:"not null;index"`
}

// TableName pins the worktree_branches table name.
func (WorktreeBranch) TableName() string { return "worktree_branches" }
