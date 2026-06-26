package model

import (
	"time"

	"gorm.io/datatypes"
)

// TaskCycleStreamEvent is the GORM persistence shape for domain.TaskCycleStreamEvent.
type TaskCycleStreamEvent struct {
	ID          string         `gorm:"primaryKey"`
	TaskID      string         `gorm:"not null;index:task_cycle_stream_events_task_cycle_seq,priority:1"`
	CycleID     string         `gorm:"not null;index:task_cycle_stream_events_task_cycle_seq,priority:2;index:task_cycle_stream_events_cycle_seq,unique,priority:1"`
	PhaseSeq    int64          `gorm:"not null;check:chk_task_cycle_stream_events_phase_seq,phase_seq > 0"`
	StreamSeq   int64          `gorm:"not null;check:chk_task_cycle_stream_events_stream_seq,stream_seq > 0;index:task_cycle_stream_events_task_cycle_seq,priority:3;index:task_cycle_stream_events_cycle_seq,unique,priority:2"`
	At          time.Time      `gorm:"not null;index"`
	Source      string         `gorm:"not null;index"`
	Kind        string         `gorm:"not null;index"`
	Subtype     string         `gorm:"not null;default:''"`
	Message     string         `gorm:"type:text;not null;default:''"`
	Tool        string         `gorm:"not null;default:''"`
	PayloadJSON datatypes.JSON `gorm:"column:payload_json;type:jsonb;not null;default:'{}'"`
}

// TableName pins the task_cycle_stream_events table name.
func (TaskCycleStreamEvent) TableName() string { return "task_cycle_stream_events" }
