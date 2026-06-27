package model

import "time"

// GitRepository is the GORM persistence shape for domain.GitRepository.
type GitRepository struct {
	ID            string    `gorm:"primaryKey"`
	Path          string    `gorm:"not null;uniqueIndex:idx_git_repo_path"`
	GitCommonDir  string    `gorm:"not null;default:'';uniqueIndex:idx_git_repo_common_dir"`
	HostPath      string    `gorm:"not null;default:''"`
	DefaultBranch string    `gorm:"not null;default:''"`
	CreatedAt     time.Time `gorm:"not null;index"`
	UpdatedAt     time.Time `gorm:"not null;index"`
}

// TableName pins the git_repositories table name.
func (GitRepository) TableName() string { return "git_repositories" }
