package model

import (
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"gorm.io/datatypes"
)

// TaskCycle is the GORM persistence shape for domain.TaskCycle.
type TaskCycle struct {
	ID            string              `gorm:"primaryKey"`
	TaskID        string              `gorm:"not null;index;index:task_cycles_task_id_attempt,unique,priority:1"`
	AttemptSeq    int64               `gorm:"not null;check:chk_task_cycles_attempt_seq,attempt_seq > 0;index:task_cycles_task_id_attempt,unique,priority:2"`
	Status        domain.CycleStatus  `gorm:"not null;index;check:chk_task_cycles_status,status IN ('running','succeeded','failed','aborted')"`
	StartedAt     time.Time           `gorm:"not null"`
	EndedAt       *time.Time          `gorm:""`
	TriggeredBy   domain.Actor        `gorm:"column:triggered_by;not null"`
	ParentCycleID *string             `gorm:"index"`
	MetaJSON      datatypes.JSON      `gorm:"column:meta_json;type:jsonb;not null;default:'{}'"`

	Task *Task `gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:CASCADE"`
}

// TableName pins the task_cycles table name.
func (TaskCycle) TableName() string { return "task_cycles" }
