package model

import (
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

// TaskCycleVerifyReport is the GORM persistence shape for domain.TaskCycleVerifyReport.
type TaskCycleVerifyReport struct {
	ID           string              `gorm:"primaryKey"`
	CycleID      string              `gorm:"not null;index;uniqueIndex:idx_cycle_verify_unique,priority:1"`
	AttemptSeq   int64               `gorm:"not null;check:chk_task_cycle_verify_reports_attempt_seq,attempt_seq > 0;uniqueIndex:idx_cycle_verify_unique,priority:2"`
	CriterionID  string              `gorm:"not null;index;uniqueIndex:idx_cycle_verify_unique,priority:3"`
	Verified     bool                `gorm:"not null;default:false"`
	VerifierKind domain.VerifierKind `gorm:"not null;default:''"`
	Reasoning    string              `gorm:"type:text;not null;default:''"`
	WrittenAt    time.Time           `gorm:"not null;index"`

	Cycle     *TaskCycle         `gorm:"foreignKey:CycleID;references:ID;constraint:OnDelete:CASCADE"`
	Criterion *TaskChecklistItem `gorm:"foreignKey:CriterionID;references:ID;constraint:OnDelete:NO ACTION"`
}

// TableName pins the task_cycle_verify_reports table name.
func (TaskCycleVerifyReport) TableName() string { return "task_cycle_verify_reports" }
