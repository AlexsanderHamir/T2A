package domain

import (
	"time"

	"gorm.io/datatypes"
)

type Task struct {
	ID            string   `json:"id" gorm:"primaryKey"`
	Title         string   `json:"title" gorm:"not null"`
	InitialPrompt string   `json:"initial_prompt" gorm:"type:text;not null"`
	Status        Status   `json:"status" gorm:"not null;check:chk_tasks_status,status IN ('ready','running','blocked','review','done','failed')"`
	Priority      Priority `json:"priority" gorm:"not null;check:chk_tasks_priority,priority IN ('low','medium','high','critical')"`
}

type TaskEvent struct {
	TaskID string    `gorm:"primaryKey;index:task_events_task_id_at,priority:1;index:task_events_task_id_type,priority:1"`
	Seq    int64     `gorm:"primaryKey;check:chk_task_events_seq,seq > 0"`
	At     time.Time `gorm:"not null;index:task_events_task_id_at,priority:2"`
	// Avoid GORM CHECK tags on columns named type/by; validate with EventType and Actor in Go instead.
	Type EventType      `gorm:"column:type;not null;index:task_events_task_id_type,priority:2"`
	By   Actor          `gorm:"column:by;not null"`
	Data datatypes.JSON `gorm:"column:data_json;type:jsonb;not null;default:'{}'"`

	Task *Task `gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:CASCADE"`
}
