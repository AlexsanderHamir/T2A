package model

import (
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"gorm.io/datatypes"
)

// Task is the GORM persistence shape for domain.Task (columns only).
type Task struct {
	ID                    string               `gorm:"primaryKey"`
	Title                 string               `gorm:"not null"`
	InitialPrompt         string               `gorm:"type:text;not null"`
	Status                domain.Status        `gorm:"not null;index;check:chk_tasks_status,status IN ('ready','running','blocked','review','done','failed','on_hold')"`
	Priority              domain.Priority      `gorm:"not null;check:chk_tasks_priority,priority IN ('low','medium','high','critical')"`
	ProjectID             *string              `gorm:"index"`
	ProjectContextItemIDs []string             `gorm:"column:project_context_item_ids;serializer:json;type:jsonb;not null;default:'[]'"`
	Tags                  []string             `gorm:"column:tags;serializer:json;type:jsonb;not null;default:'[]'"`
	Milestone             *string              `gorm:"index"`
	Gate                  *domain.TaskGate     `gorm:"column:gate;serializer:json;type:jsonb"`
	Runner                string               `gorm:"not null;default:'cursor'"`
	CursorModel           string               `gorm:"not null;default:''"`
	RunnerConfig          datatypes.JSON       `gorm:"column:runner_config;type:jsonb;not null;default:'{}'"`
	PickupNotBefore       *time.Time           `gorm:"index"`
	CriteriaSatisfiedAt   *time.Time           `gorm:"index"`
	PendingRetry          *domain.PendingRetry `gorm:"column:pending_retry;serializer:json;type:jsonb"`
	WorktreeID            *string              `gorm:"index"`
}

// TableName pins the tasks table name.
func (Task) TableName() string { return "tasks" }
