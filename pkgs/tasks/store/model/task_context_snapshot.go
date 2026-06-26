package model

import (
	"time"

	"gorm.io/datatypes"
)

// TaskContextSnapshot is the GORM persistence shape for domain.TaskContextSnapshot.
type TaskContextSnapshot struct {
	ID              string         `gorm:"primaryKey"`
	TaskID          string         `gorm:"not null;index"`
	CycleID         string         `gorm:"not null;index;unique"`
	ProjectID       string         `gorm:"not null;index"`
	ContextJSON     datatypes.JSON `gorm:"column:context_json;type:jsonb;not null;default:'{}'"`
	RenderedContext string         `gorm:"type:text;not null;default:''"`
	TokenEstimate   int            `gorm:"not null;default:0"`
	CreatedAt       time.Time      `gorm:"not null;index"`
}

// TableName pins the task_context_snapshots table name.
func (TaskContextSnapshot) TableName() string { return "task_context_snapshots" }
