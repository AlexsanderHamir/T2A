package model

import (
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// TaskDependency is the GORM persistence shape for domain.TaskDependency (columns only).
type TaskDependency struct {
	TaskID          string                     `gorm:"primaryKey"`
	DependsOnTaskID string                     `gorm:"primaryKey;index"`
	Satisfies       domain.DependencySatisfies `gorm:"not null;default:done;check:chk_task_dependencies_satisfies,satisfies IN ('done')"`
	CreatedAt       time.Time                  `gorm:"not null;index"`

	Task          *Task `gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:CASCADE"`
	DependsOnTask *Task `gorm:"foreignKey:DependsOnTaskID;references:ID;constraint:OnDelete:CASCADE"`
}

// TableName pins the task_dependencies table name.
func (TaskDependency) TableName() string { return "task_dependencies" }
