package model

import (
	"time"

	"gorm.io/datatypes"
)

// TaskDraft is the GORM persistence shape for domain.TaskDraft.
type TaskDraft struct {
	ID          string         `gorm:"primaryKey"`
	Name        string         `gorm:"not null;index"`
	PayloadJSON datatypes.JSON `gorm:"column:payload_json;type:jsonb;not null;default:'{}'"`
	CreatedAt   time.Time      `gorm:"not null;index"`
	UpdatedAt   time.Time      `gorm:"not null;index"`
}

// TableName pins the task_drafts table name.
func (TaskDraft) TableName() string { return "task_drafts" }
