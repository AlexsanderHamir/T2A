package domain

import (
	"context"
	"log/slog"
	"time"

	"gorm.io/datatypes"
)

type Task struct {
	ID               string   `json:"id" gorm:"primaryKey"`
	Title            string   `json:"title" gorm:"not null"`
	InitialPrompt    string   `json:"initial_prompt" gorm:"type:text;not null"`
	Status           Status   `json:"status" gorm:"not null;index;check:chk_tasks_status,status IN ('ready','running','blocked','review','done','failed')"`
	Priority         Priority `json:"priority" gorm:"not null;check:chk_tasks_priority,priority IN ('low','medium','high','critical')"`
	TaskType         TaskType `json:"task_type" gorm:"not null;default:general;check:chk_tasks_task_type,task_type IN ('general','bug_fix','feature','refactor','docs')"`
	ParentID         *string  `json:"parent_id,omitempty" gorm:"index"`
	ChecklistInherit bool     `json:"checklist_inherit" gorm:"not null;default:false"`
}

// TaskChecklistItem is a definition row owned by a task that does not use checklist_inherit.
type TaskChecklistItem struct {
	ID        string `json:"id" gorm:"primaryKey"`
	TaskID    string `json:"task_id" gorm:"not null;index"`
	SortOrder int    `json:"sort_order" gorm:"not null"`
	Text      string `json:"text" gorm:"not null;type:text"`
}

func (TaskChecklistItem) TableName() string {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.TaskChecklistItem.TableName")
	}
	return "task_checklist_items"
}

// TaskChecklistCompletion records that subject TaskID satisfied checklist item ItemID.
type TaskChecklistCompletion struct {
	TaskID string    `json:"task_id" gorm:"primaryKey"`
	ItemID string    `json:"item_id" gorm:"primaryKey"`
	At     time.Time `json:"at" gorm:"not null"`
	By     Actor     `json:"by" gorm:"column:done_by;not null"`
}

func (TaskChecklistCompletion) TableName() string {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("trace", "operation", "domain.TaskChecklistCompletion.TableName")
	}
	return "task_checklist_completions"
}

type TaskEvent struct {
	TaskID string    `gorm:"primaryKey;index:task_events_task_id_at,priority:1;index:task_events_task_id_type,priority:1"`
	Seq    int64     `gorm:"primaryKey;check:chk_task_events_seq,seq > 0"`
	At     time.Time `gorm:"not null;index:task_events_task_id_at,priority:2"`
	// Avoid GORM CHECK tags on columns named type/by; validate with EventType and Actor in Go instead.
	Type EventType      `gorm:"column:type;not null;index:task_events_task_id_type,priority:2"`
	By   Actor          `gorm:"column:by;not null"`
	Data datatypes.JSON `gorm:"column:data_json;type:jsonb;not null;default:'{}'"`

	// UserResponse is optional human-supplied text for event types that accept input (see EventTypeAcceptsUserResponse).
	UserResponse *string `gorm:"column:user_response;type:text"`
	// UserResponseAt is set when UserResponse is written or updated (UTC).
	UserResponseAt *time.Time `gorm:"column:user_response_at"`
	// ResponseThread is an ordered JSON array of ResponseThreadEntry (user ↔ agent messages).
	ResponseThread datatypes.JSON `gorm:"column:response_thread_json;type:jsonb"`

	Task *Task `gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:CASCADE"`
}

// TaskDraftEvaluation stores one scoring snapshot for a draft task before creation.
type TaskDraftEvaluation struct {
	ID         string         `gorm:"primaryKey"`
	DraftID    *string        `gorm:"index"`
	TaskID     *string        `gorm:"index"`
	By         Actor          `gorm:"column:by;not null"`
	InputJSON  datatypes.JSON `gorm:"column:input_json;type:jsonb;not null;default:'{}'"`
	ResultJSON datatypes.JSON `gorm:"column:result_json;type:jsonb;not null;default:'{}'"`
	CreatedAt  time.Time      `gorm:"not null;index"`
}

// TaskDraft stores a resumable create-task draft payload.
type TaskDraft struct {
	ID          string         `gorm:"primaryKey"`
	Name        string         `gorm:"not null;index"`
	PayloadJSON datatypes.JSON `gorm:"column:payload_json;type:jsonb;not null;default:'{}'"`
	CreatedAt   time.Time      `gorm:"not null;index"`
	UpdatedAt   time.Time      `gorm:"not null;index"`
}
