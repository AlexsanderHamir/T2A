package model

import (
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// ProjectContextItem is the GORM persistence shape for domain.ProjectContextItem.
type ProjectContextItem struct {
	ID            string                    `gorm:"primaryKey"`
	ProjectID     string                    `gorm:"not null;index"`
	Kind          domain.ProjectContextKind `gorm:"not null;index;default:note"`
	Title         string                    `gorm:"not null"`
	Body          string                    `gorm:"type:text;not null"`
	SourceTaskID  *string                   `gorm:"index"`
	SourceCycleID *string                   `gorm:"index"`
	CreatedBy     domain.Actor              `gorm:"column:created_by;not null"`
	Pinned        bool                      `gorm:"not null;default:false;index"`
	CreatedAt     time.Time                 `gorm:"not null;index"`
	UpdatedAt     time.Time                 `gorm:"not null;index"`
}

// TableName pins the project_context_items table name.
func (ProjectContextItem) TableName() string { return "project_context_items" }
