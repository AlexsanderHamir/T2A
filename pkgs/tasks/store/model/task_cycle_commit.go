package model

import "time"

// TaskCycleCommit is the GORM persistence shape for domain.TaskCycleCommit.
type TaskCycleCommit struct {
	ID          string    `gorm:"primaryKey"`
	TaskID      string    `gorm:"not null;index"`
	CycleID     string    `gorm:"not null;index;uniqueIndex:idx_cycle_commit_sha,priority:1"`
	PhaseSeq    int64     `gorm:"not null;check:chk_task_cycle_commits_phase_seq,phase_seq > 0"`
	Seq         int64     `gorm:"not null;check:chk_task_cycle_commits_seq,seq > 0;index:idx_cycle_commit_order,priority:2"`
	Repo        string    `gorm:"not null;default:'';type:text"`
	Worktree    string    `gorm:"not null;default:'';type:text"`
	Branch      string    `gorm:"not null;default:''"`
	SHA         string    `gorm:"not null;uniqueIndex:idx_cycle_commit_sha,priority:2"`
	CommittedAt time.Time `gorm:"not null;index"`
	Message     string    `gorm:"type:text;not null;default:''"`
	RecordedAt  time.Time `gorm:"not null;index"`
}

// TableName pins the task_cycle_commits table name.
func (TaskCycleCommit) TableName() string { return "task_cycle_commits" }
