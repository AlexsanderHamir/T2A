package model

import "time"

// TaskCycleCommandRun is the GORM persistence shape for domain.TaskCycleCommandRun.
type TaskCycleCommandRun struct {
	ID          string    `gorm:"primaryKey"`
	CycleID     string    `gorm:"not null;index;uniqueIndex:idx_cycle_command_run_unique,priority:1"`
	AttemptSeq  int64     `gorm:"not null;check:chk_task_cycle_command_runs_attempt_seq,attempt_seq > 0;uniqueIndex:idx_cycle_command_run_unique,priority:2"`
	CriterionID string    `gorm:"not null;index;uniqueIndex:idx_cycle_command_run_unique,priority:3"`
	CommandSeq  int64     `gorm:"not null;check:chk_task_cycle_command_runs_command_seq,command_seq >= 0;uniqueIndex:idx_cycle_command_run_unique,priority:4"`
	ExitCode    int       `gorm:"not null;default:-1"`
	MetaPath    string    `gorm:"not null;default:'';type:text"`
	WrittenAt   time.Time `gorm:"not null;index"`

	Cycle     *TaskCycle         `gorm:"foreignKey:CycleID;references:ID;constraint:OnDelete:CASCADE"`
	Criterion *TaskChecklistItem `gorm:"foreignKey:CriterionID;references:ID;constraint:OnDelete:NO ACTION"`
}

// TableName pins the task_cycle_command_runs table name.
func (TaskCycleCommandRun) TableName() string { return "task_cycle_command_runs" }
