package model

import (
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"gorm.io/datatypes"
)

// TaskCyclePhase is the GORM persistence shape for domain.TaskCyclePhase.
type TaskCyclePhase struct {
	ID          string              `gorm:"primaryKey"`
	CycleID     string              `gorm:"not null;index;index:task_cycle_phases_cycle_id_seq,unique,priority:1"`
	Phase       domain.Phase        `gorm:"column:phase;not null;check:chk_task_cycle_phases_phase,phase IN ('execute','verify')"`
	PhaseSeq    int64               `gorm:"not null;check:chk_task_cycle_phases_phase_seq,phase_seq > 0;index:task_cycle_phases_cycle_id_seq,unique,priority:2"`
	Status      domain.PhaseStatus  `gorm:"not null;index;check:chk_task_cycle_phases_status,status IN ('running','succeeded','failed','skipped')"`
	StartedAt   time.Time           `gorm:"not null"`
	EndedAt     *time.Time          `gorm:""`
	Summary     *string             `gorm:"type:text"`
	DetailsJSON datatypes.JSON      `gorm:"column:details_json;type:jsonb;not null;default:'{}'"`
	EventSeq    *int64              `gorm:"column:event_seq"`
}

// TableName pins the task_cycle_phases table name.
func (TaskCyclePhase) TableName() string { return "task_cycle_phases" }
