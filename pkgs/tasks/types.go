package tasks

import (
	"time"

	"gorm.io/datatypes"
)

// Status is the current lifecycle state of a task (schema.md: task.status).
type Status string

const (
	StatusReady   Status = "ready"
	StatusRunning Status = "running"
	StatusBlocked Status = "blocked"
	StatusReview  Status = "review"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

// Priority is task urgency (schema.md: task.priority).
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// EventType is task_events.type (schema.md).
type EventType string

const (
	EventTaskCreated            EventType = "task_created"
	EventStatusChanged          EventType = "status_changed"
	EventPriorityChanged        EventType = "priority_changed"
	EventPromptAppended         EventType = "prompt_appended"
	EventContextAdded           EventType = "context_added"
	EventConstraintAdded        EventType = "constraint_added"
	EventSuccessCriterionAdded  EventType = "success_criterion_added"
	EventNonGoalAdded           EventType = "non_goal_added"
	EventPlanAdded              EventType = "plan_added"
	EventSubtaskAdded           EventType = "subtask_added"
	EventMessageAdded           EventType = "message_added"
	EventArtifactAdded          EventType = "artifact_added"
	EventApprovalRequested      EventType = "approval_requested"
	EventApprovalGranted        EventType = "approval_granted"
	EventTaskCompleted          EventType = "task_completed"
	EventTaskFailed             EventType = "task_failed"
)

// Actor is who recorded the event (schema.md: events[].by).
type Actor string

const (
	ActorUser  Actor = "user"
	ActorAgent Actor = "agent"
)

// Task is the current snapshot row (schema.md: task).
type Task struct {
	ID            string   `gorm:"primaryKey"`
	Title         string   `gorm:"not null"`
	InitialPrompt string   `gorm:"type:text;not null"`
	Status        Status   `gorm:"not null;check:status IN ('ready','running','blocked','review','done','failed'),name:chk_tasks_status"`
	Priority      Priority `gorm:"not null;check:priority IN ('low','medium','high','critical'),name:chk_tasks_priority"`
}

// TaskEvent is one append-only log entry (schema.md: events[]).
// Data holds the event-specific JSON object (e.g. status_changed: from/to).
type TaskEvent struct {
	TaskID string    `gorm:"primaryKey;index:task_events_task_id_at,priority:1;index:task_events_task_id_type,priority:1"`
	Seq    int64     `gorm:"primaryKey;check:seq > 0,name:chk_task_events_seq"`
	At     time.Time `gorm:"not null;index:task_events_task_id_at,priority:2"`
	// type / by: reserved-ish in SQL; enum checks live in Go (EventType, Actor). DB still stores text/jsonb.
	Type EventType `gorm:"column:type;not null;index:task_events_task_id_type,priority:2"`
	By   Actor     `gorm:"column:by;not null"`
	Data datatypes.JSON `gorm:"column:data_json;type:jsonb;not null;default:'{}'"`

	Task *Task `gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:CASCADE"`
}
