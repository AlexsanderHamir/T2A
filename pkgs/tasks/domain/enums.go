package domain

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

type TaskType string

const (
	TaskTypeGeneral  TaskType = "general"
	TaskTypeBugFix   TaskType = "bug_fix"
	TaskTypeFeature  TaskType = "feature"
	TaskTypeRefactor TaskType = "refactor"
	TaskTypeDocs     TaskType = "docs"
)

type EventType string

const (
	EventTaskCreated             EventType = "task_created"
	EventStatusChanged           EventType = "status_changed"
	EventPriorityChanged         EventType = "priority_changed"
	EventPromptAppended          EventType = "prompt_appended"
	EventContextAdded            EventType = "context_added"
	EventConstraintAdded         EventType = "constraint_added"
	EventSuccessCriterionAdded   EventType = "success_criterion_added"
	EventNonGoalAdded            EventType = "non_goal_added"
	EventPlanAdded               EventType = "plan_added"
	EventSubtaskAdded            EventType = "subtask_added"
	EventSubtaskRemoved          EventType = "subtask_removed"
	EventChecklistItemAdded      EventType = "checklist_item_added"
	EventChecklistItemToggled    EventType = "checklist_item_toggled"
	EventChecklistItemUpdated    EventType = "checklist_item_updated"
	EventChecklistItemRemoved    EventType = "checklist_item_removed"
	EventChecklistInheritChanged EventType = "checklist_inherit_changed"
	EventMessageAdded            EventType = "message_added"
	EventArtifactAdded           EventType = "artifact_added"
	EventApprovalRequested       EventType = "approval_requested"
	EventApprovalGranted         EventType = "approval_granted"
	EventTaskCompleted           EventType = "task_completed"
	EventTaskFailed              EventType = "task_failed"
	// EventSyncPing is included in the dev ticker rotation (T2A_SSE_TEST) alongside every other EventType.
	EventSyncPing EventType = "sync_ping"
)

type Actor string

const (
	ActorUser  Actor = "user"
	ActorAgent Actor = "agent"
)
