package model

import (
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"gorm.io/datatypes"
)

// TaskEvent is the GORM persistence shape for domain.TaskEvent (columns only;
// FK association fields are added in the Phase 6 cutover).
type TaskEvent struct {
	TaskID         string           `gorm:"primaryKey;index:task_events_task_id_at,priority:1;index:task_events_task_id_type,priority:1"`
	Seq            int64            `gorm:"primaryKey;check:chk_task_events_seq,seq > 0"`
	At             time.Time        `gorm:"not null;index:task_events_task_id_at,priority:2"`
	Type           domain.EventType `gorm:"column:type;not null;index:task_events_task_id_type,priority:2"`
	By             domain.Actor     `gorm:"column:by;not null"`
	Data           datatypes.JSON   `gorm:"column:data_json;type:jsonb;not null;default:'{}'"`
	UserResponse   *string          `gorm:"column:user_response;type:text"`
	UserResponseAt *time.Time       `gorm:"column:user_response_at"`
	ResponseThread datatypes.JSON   `gorm:"column:response_thread_json;type:jsonb"`
}

// TableName pins the table name (domain relied on GORM name derivation).
func (TaskEvent) TableName() string { return "task_events" }
