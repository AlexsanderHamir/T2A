package model

import "time"

// TaskCycleCriteriaReport is the GORM persistence shape for domain.TaskCycleCriteriaReport.
type TaskCycleCriteriaReport struct {
	ID          string    `gorm:"primaryKey"`
	CycleID     string    `gorm:"not null;index;uniqueIndex:idx_cycle_criteria_unique,priority:1"`
	AttemptSeq  int64     `gorm:"not null;check:chk_task_cycle_criteria_reports_attempt_seq,attempt_seq > 0;uniqueIndex:idx_cycle_criteria_unique,priority:2"`
	CriterionID string    `gorm:"not null;index;uniqueIndex:idx_cycle_criteria_unique,priority:3"`
	ClaimedDone bool      `gorm:"not null;default:false"`
	Evidence    string    `gorm:"type:text;not null;default:''"`
	WrittenAt   time.Time `gorm:"not null;index"`

	Cycle     *TaskCycle         `gorm:"foreignKey:CycleID;references:ID;constraint:OnDelete:CASCADE"`
	Criterion *TaskChecklistItem `gorm:"foreignKey:CriterionID;references:ID;constraint:OnDelete:NO ACTION"`
}

// TableName pins the task_cycle_criteria_reports table name.
func (TaskCycleCriteriaReport) TableName() string { return "task_cycle_criteria_reports" }
