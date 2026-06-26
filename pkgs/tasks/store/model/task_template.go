package model

import (
	"time"

	"gorm.io/datatypes"
)

// TaskTemplate is the GORM persistence shape for domain.TaskTemplate.
type TaskTemplate struct {
	ID          string         `gorm:"primaryKey"`
	Name        string         `gorm:"not null;index"`
	PayloadJSON datatypes.JSON `gorm:"column:payload_json;type:jsonb;not null;default:'{}'"`
	CreatedAt   time.Time      `gorm:"not null;index"`
	UpdatedAt   time.Time      `gorm:"not null;index"`
}

// TableName pins the task_templates table name.
func (TaskTemplate) TableName() string { return "task_templates" }
