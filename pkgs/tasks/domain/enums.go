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

// ProjectStatus is the lifecycle state of a long-lived project context.
type ProjectStatus string

const (
	ProjectStatusActive   ProjectStatus = "active"
	ProjectStatusArchived ProjectStatus = "archived"
)

// ProjectContextKind identifies the role a context item plays in project memory.
type ProjectContextKind string

const (
	ProjectContextKindNote       ProjectContextKind = "note"
	ProjectContextKindDecision   ProjectContextKind = "decision"
	ProjectContextKindConstraint ProjectContextKind = "constraint"
	ProjectContextKindHandoff    ProjectContextKind = "handoff"
)

// ProjectContextRelation identifies how one project context node relates to another.
type ProjectContextRelation string

const (
	ProjectContextRelationSupports  ProjectContextRelation = "supports"
	ProjectContextRelationBlocks    ProjectContextRelation = "blocks"
	ProjectContextRelationRefines   ProjectContextRelation = "refines"
	ProjectContextRelationDependsOn ProjectContextRelation = "depends_on"
	ProjectContextRelationRelated   ProjectContextRelation = "related"
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
	// Execution-cycle audit mirrors. Emitted in the same SQL transaction as writes to
	// task_cycles / task_cycle_phases so GET /tasks/{id}/events stays a complete witness
	// of cycle activity. See docs/EXECUTION-CYCLES.md.
	EventCycleStarted   EventType = "cycle_started"
	EventCycleCompleted EventType = "cycle_completed"
	EventCycleFailed    EventType = "cycle_failed"
	EventPhaseStarted   EventType = "phase_started"
	EventPhaseCompleted EventType = "phase_completed"
	EventPhaseFailed    EventType = "phase_failed"
	EventPhaseSkipped   EventType = "phase_skipped"
	// EventSyncPing is included in the dev ticker rotation (T2A_SSE_TEST) alongside every other EventType.
	EventSyncPing EventType = "sync_ping"
)

// Phase is one entry in a task execution cycle. The four values match the
// "diagnose -> execute -> verify -> persist" loop from moat.md.
type Phase string

const (
	PhaseDiagnose Phase = "diagnose"
	PhaseExecute  Phase = "execute"
	PhaseVerify   Phase = "verify"
	PhasePersist  Phase = "persist"
)

// CycleStatus is the lifecycle state of a single task_cycles row.
type CycleStatus string

const (
	CycleStatusRunning   CycleStatus = "running"
	CycleStatusSucceeded CycleStatus = "succeeded"
	CycleStatusFailed    CycleStatus = "failed"
	CycleStatusAborted   CycleStatus = "aborted"
)

// PhaseStatus is the lifecycle state of a single task_cycle_phases row.
type PhaseStatus string

const (
	PhaseStatusRunning   PhaseStatus = "running"
	PhaseStatusSucceeded PhaseStatus = "succeeded"
	PhaseStatusFailed    PhaseStatus = "failed"
	PhaseStatusSkipped   PhaseStatus = "skipped"
)

type Actor string

const (
	ActorUser  Actor = "user"
	ActorAgent Actor = "agent"
)
