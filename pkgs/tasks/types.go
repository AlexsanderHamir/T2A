package tasks

import (
	"time"

	"gorm.io/datatypes"
)

type Status string

const (
	StatusReady   Status = "ready"
	StatusRunning Status = "running"
	StatusBlocked Status = "blocked"
	StatusReview  Status = "review"
	StatusDone    Status = "done"
	StatusFailed  Status = "failed"
)

type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

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

type Actor string

const (
	ActorUser  Actor = "user"
	ActorAgent Actor = "agent"
)

type Task struct {
	ID            string   `gorm:"primaryKey"`
	Title         string   `gorm:"not null"`
	InitialPrompt string   `gorm:"type:text;not null"`
	Status        Status   `gorm:"not null;check:status IN ('ready','running','blocked','review','done','failed'),name:chk_tasks_status"`
	Priority      Priority `gorm:"not null;check:priority IN ('low','medium','high','critical'),name:chk_tasks_priority"`
}

type TaskEvent struct {
	TaskID string    `gorm:"primaryKey;index:task_events_task_id_at,priority:1;index:task_events_task_id_type,priority:1"`
	Seq    int64     `gorm:"primaryKey;check:seq > 0,name:chk_task_events_seq"`
	At     time.Time `gorm:"not null;index:task_events_task_id_at,priority:2"`
	// Avoid GORM CHECK tags on columns named type/by; validate with EventType and Actor in Go instead.
	Type EventType `gorm:"column:type;not null;index:task_events_task_id_type,priority:2"`
	By   Actor     `gorm:"column:by;not null"`
	Data datatypes.JSON `gorm:"column:data_json;type:jsonb;not null;default:'{}'"`

	Task *Task `gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:CASCADE"`
}
