package domain

import (
	"time"

	"gorm.io/datatypes"
)

type Task struct {
	ID            string   `json:"id" gorm:"primaryKey"`
	Title         string   `json:"title" gorm:"not null"`
	InitialPrompt string   `json:"initial_prompt" gorm:"type:text;not null"`
	Status        Status   `json:"status" gorm:"not null;index;check:chk_tasks_status,status IN ('ready','running','blocked','review','done','failed','on_hold')"`
	Priority      Priority `json:"priority" gorm:"not null;check:chk_tasks_priority,priority IN ('low','medium','high','critical')"`
	ProjectID     *string  `json:"project_id,omitempty" gorm:"index"`
	// ProjectContextItemIDs is the user-selected subset of project context to pass to agent runs.
	ProjectContextItemIDs []string  `json:"project_context_item_ids,omitempty" gorm:"column:project_context_item_ids;serializer:json;type:jsonb;not null;default:'[]'"`
	Tags                  []string  `json:"tags,omitempty" gorm:"column:tags;serializer:json;type:jsonb;not null;default:'[]'"`
	Milestone             *string   `json:"milestone,omitempty" gorm:"index"`
	Gate                  *TaskGate `json:"gate,omitempty" gorm:"column:gate;serializer:json;type:jsonb"`
	// DependsOn is hydrated from task_dependencies on read; not a GORM column.
	DependsOn []DependencyEdge `json:"depends_on,omitempty" gorm:"-"`
	// Runner is the agent runner id for this task (e.g. "cursor"). Set at
	// create time from the request or app defaults; must match the worker's
	// configured runner when the task runs.
	Runner string `json:"runner" gorm:"not null;default:'cursor'"`
	// CursorModel is forwarded to cursor-agent as --model when non-empty;
	// empty means omit the flag for this task (same semantics as app settings).
	CursorModel string `json:"cursor_model" gorm:"not null;default:''"`
	// RunnerConfig stores per-task runner config overrides as a JSON blob.
	// When non-empty, the worker merges this with the global runner config
	// from app_settings.runner_configs for the matching runner ID.
	RunnerConfig datatypes.JSON `json:"runner_config,omitempty" gorm:"column:runner_config;type:jsonb;not null;default:'{}'"`
	// PickupNotBefore defers agent dequeue until this instant (UTC). NULL means
	// eligible as soon as status is ready (legacy rows and zero-delay creates).
	PickupNotBefore *time.Time `json:"pickup_not_before,omitempty" gorm:"index"`
	// CriteriaSatisfiedAt is set when every inherited checklist item has a
	// verified completion row; cleared when any item becomes unchecked.
	// Maintained in checklist completion TX for SQL queue parity.
	CriteriaSatisfiedAt *time.Time `json:"criteria_satisfied_at,omitempty" gorm:"index"`
	// PendingRetry holds operator retry intent between POST /retry and worker
	// pickup. Not exposed on the public task API (json:"-").
	PendingRetry *PendingRetry `json:"-" gorm:"column:pending_retry;serializer:json;type:jsonb"`

	Project *Project `json:"-" gorm:"foreignKey:ProjectID;references:ID;constraint:OnDelete:SET NULL"`
}

// Project is shared context memory for a long-running body of work.
type Project struct {
	ID             string        `json:"id" gorm:"primaryKey"`
	Name           string        `json:"name" gorm:"not null;index"`
	Description    string        `json:"description" gorm:"type:text;not null;default:''"`
	Status         ProjectStatus `json:"status" gorm:"not null;index;default:active;check:chk_projects_status,status IN ('active','archived')"`
	ContextSummary string        `json:"context_summary" gorm:"type:text;not null;default:''"`
	CreatedAt      time.Time     `json:"created_at" gorm:"not null;index"`
	UpdatedAt      time.Time     `json:"updated_at" gorm:"not null;index"`
}

// ProjectContextItem is a human-inspectable memory item attached to a project.
type ProjectContextItem struct {
	ID            string             `json:"id" gorm:"primaryKey"`
	ProjectID     string             `json:"project_id" gorm:"not null;index"`
	Kind          ProjectContextKind `json:"kind" gorm:"not null;index;default:note"`
	Title         string             `json:"title" gorm:"not null"`
	Body          string             `json:"body" gorm:"type:text;not null"`
	SourceTaskID  *string            `json:"source_task_id,omitempty" gorm:"index"`
	SourceCycleID *string            `json:"source_cycle_id,omitempty" gorm:"index"`
	CreatedBy     Actor              `json:"created_by" gorm:"column:created_by;not null"`
	Pinned        bool               `json:"pinned" gorm:"not null;default:false;index"`
	CreatedAt     time.Time          `json:"created_at" gorm:"not null;index"`
	UpdatedAt     time.Time          `json:"updated_at" gorm:"not null;index"`

	Project     *Project   `json:"-" gorm:"foreignKey:ProjectID;references:ID;constraint:OnDelete:CASCADE"`
	SourceTask  *Task      `json:"-" gorm:"foreignKey:SourceTaskID;references:ID;constraint:OnDelete:SET NULL"`
	SourceCycle *TaskCycle `json:"-" gorm:"foreignKey:SourceCycleID;references:ID;constraint:OnDelete:SET NULL"`
}

// TableName: see TaskChecklistItem.TableName for skip-list rationale.
func (ProjectContextItem) TableName() string { return "project_context_items" }

// ProjectContextEdge is a user-curated relationship between two context nodes
// owned by the same project.
type ProjectContextEdge struct {
	ID              string                 `json:"id" gorm:"primaryKey"`
	ProjectID       string                 `json:"project_id" gorm:"not null;index;uniqueIndex:idx_project_context_edge_unique,priority:1"`
	SourceContextID string                 `json:"source_context_id" gorm:"not null;index;uniqueIndex:idx_project_context_edge_unique,priority:2"`
	TargetContextID string                 `json:"target_context_id" gorm:"not null;index;uniqueIndex:idx_project_context_edge_unique,priority:3"`
	Relation        ProjectContextRelation `json:"relation" gorm:"not null;index;uniqueIndex:idx_project_context_edge_unique,priority:4;check:chk_project_context_relation,relation IN ('supports','blocks','refines','depends_on','related')"`
	Strength        int                    `json:"strength" gorm:"not null;default:3;check:chk_project_context_strength,strength >= 1 AND strength <= 5"`
	Note            string                 `json:"note" gorm:"type:text;not null;default:''"`
	CreatedAt       time.Time              `json:"created_at" gorm:"not null;index"`
	UpdatedAt       time.Time              `json:"updated_at" gorm:"not null;index"`

	Project *Project            `json:"-" gorm:"foreignKey:ProjectID;references:ID;constraint:OnDelete:CASCADE"`
	Source  *ProjectContextItem `json:"-" gorm:"foreignKey:SourceContextID;references:ID;constraint:OnDelete:CASCADE"`
	Target  *ProjectContextItem `json:"-" gorm:"foreignKey:TargetContextID;references:ID;constraint:OnDelete:CASCADE"`
}

// TableName: see TaskChecklistItem.TableName for skip-list rationale.
func (ProjectContextEdge) TableName() string { return "project_context_edges" }

// TaskContextSnapshot records the exact project context bundle handed to one
// task execution attempt. It is immutable audit data, not canonical project memory.
type TaskContextSnapshot struct {
	ID              string         `json:"id" gorm:"primaryKey"`
	TaskID          string         `json:"task_id" gorm:"not null;index"`
	CycleID         string         `json:"cycle_id" gorm:"not null;index;unique"`
	ProjectID       string         `json:"project_id" gorm:"not null;index"`
	ContextJSON     datatypes.JSON `json:"context_json" gorm:"column:context_json;type:jsonb;not null;default:'{}'"`
	RenderedContext string         `json:"rendered_context" gorm:"type:text;not null;default:''"`
	TokenEstimate   int            `json:"token_estimate" gorm:"not null;default:0"`
	CreatedAt       time.Time      `json:"created_at" gorm:"not null;index"`

	Task    *Task      `json:"-" gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:CASCADE"`
	Cycle   *TaskCycle `json:"-" gorm:"foreignKey:CycleID;references:ID;constraint:OnDelete:CASCADE"`
	Project *Project   `json:"-" gorm:"foreignKey:ProjectID;references:ID;constraint:OnDelete:CASCADE"`
}

// TableName: see TaskChecklistItem.TableName for skip-list rationale.
func (TaskContextSnapshot) TableName() string { return "task_context_snapshots" }

// TaskDependency is a directed edge: task_id depends on depends_on_task_id completing first.
type TaskDependency struct {
	TaskID          string              `json:"task_id" gorm:"primaryKey"`
	DependsOnTaskID string              `json:"depends_on_task_id" gorm:"primaryKey;index"`
	Satisfies       DependencySatisfies `json:"satisfies" gorm:"not null;default:done;check:chk_task_dependencies_satisfies,satisfies IN ('done')"`
	CreatedAt       time.Time           `json:"created_at" gorm:"not null;index"`

	Task          *Task `json:"-" gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:CASCADE"`
	DependsOnTask *Task `json:"-" gorm:"foreignKey:DependsOnTaskID;references:ID;constraint:OnDelete:CASCADE"`
}

func (TaskDependency) TableName() string { return "task_dependencies" }

// TaskChecklistItem is a definition row owned by a task.
// Completion is recorded only by the agent worker after verify (verified_by=verify_agent)
// or when execute did not claim done (verified_by=agent_self, failure-only).
type TaskChecklistItem struct {
	ID        string `json:"id" gorm:"primaryKey"`
	TaskID    string `json:"task_id" gorm:"not null;index"`
	SortOrder int    `json:"sort_order" gorm:"not null"`
	Text      string `json:"text" gorm:"not null;type:text"`
}

// TableName returns the GORM table name. Skip-listed in
// cmd/funclogmeasure/analyze.go: pure constant return called at GORM
// reflection time, no decision logic to trace.
func (TaskChecklistItem) TableName() string { return "task_checklist_items" }

// TaskChecklistItemCommand is an optional shell check attached to a
// checklist definition. The worker runs these during verify and writes
// stdout/stderr to temp files under the cycle report dir; the LLM
// verifier interprets the artifacts against ExpectedOutcome.
type TaskChecklistItemCommand struct {
	ID              string `json:"id" gorm:"primaryKey"`
	ItemID          string `json:"item_id" gorm:"not null;index"`
	SortOrder       int    `json:"sort_order" gorm:"not null"`
	Command         string `json:"command" gorm:"not null;type:text"`
	ExpectedOutcome string `json:"expected_outcome" gorm:"not null;default:'';type:text"`

	Item *TaskChecklistItem `json:"-" gorm:"foreignKey:ItemID;references:ID;constraint:OnDelete:CASCADE"`
}

// TableName: see TaskChecklistItem.TableName for skip-list rationale.
func (TaskChecklistItemCommand) TableName() string { return "task_checklist_item_commands" }

// TaskChecklistCompletion records that subject TaskID satisfied checklist item ItemID.
type TaskChecklistCompletion struct {
	TaskID            string       `json:"task_id" gorm:"primaryKey"`
	ItemID            string       `json:"item_id" gorm:"primaryKey"`
	At                time.Time    `json:"at" gorm:"not null"`
	By                Actor        `json:"by" gorm:"column:done_by;not null"`
	Evidence          string       `json:"evidence" gorm:"not null;default:'';type:text"`
	VerifiedBy        VerifierKind `json:"verified_by" gorm:"not null;default:''"`
	VerifierReasoning string       `json:"verifier_reasoning" gorm:"not null;default:'';type:text"`
	CycleID           string       `json:"cycle_id" gorm:"not null;default:''"`
}

// TableName: see TaskChecklistItem.TableName for skip-list rationale.
func (TaskChecklistCompletion) TableName() string { return "task_checklist_completions" }

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

// TaskDraft stores a resumable create-task draft payload.
type TaskDraft struct {
	ID          string         `gorm:"primaryKey"`
	Name        string         `gorm:"not null;index"`
	PayloadJSON datatypes.JSON `gorm:"column:payload_json;type:jsonb;not null;default:'{}'"`
	CreatedAt   time.Time      `gorm:"not null;index"`
	UpdatedAt   time.Time      `gorm:"not null;index"`
}

// TaskTemplate stores a reusable task compose blueprint (not a runnable task).
type TaskTemplate struct {
	ID          string         `gorm:"primaryKey"`
	Name        string         `gorm:"not null;index"`
	PayloadJSON datatypes.JSON `gorm:"column:payload_json;type:jsonb;not null;default:'{}'"`
	CreatedAt   time.Time      `gorm:"not null;index"`
	UpdatedAt   time.Time      `gorm:"not null;index"`
}

// TaskCycle is one execution attempt for a task. The (TaskID, AttemptSeq) pair
// gives a stable monotonic ordering of attempts. A cycle's lifecycle is enforced
// at the store boundary: at most one Running cycle per task at any time, and
// terminal statuses (Succeeded / Failed / Aborted) are immutable. See
// docs/data-model.md.
type TaskCycle struct {
	ID            string      `gorm:"primaryKey"`
	TaskID        string      `gorm:"not null;index;index:task_cycles_task_id_attempt,unique,priority:1"`
	AttemptSeq    int64       `gorm:"not null;check:chk_task_cycles_attempt_seq,attempt_seq > 0;index:task_cycles_task_id_attempt,unique,priority:2"`
	Status        CycleStatus `gorm:"not null;index;check:chk_task_cycles_status,status IN ('running','succeeded','failed','aborted')"`
	StartedAt     time.Time   `gorm:"not null"`
	EndedAt       *time.Time  `gorm:""`
	TriggeredBy   Actor       `gorm:"column:triggered_by;not null"`
	ParentCycleID *string     `gorm:"index"`
	// MetaJSON is small free-form runner metadata such as {"runner":"cursor-cli","prompt_hash":"..."}.
	MetaJSON datatypes.JSON `gorm:"column:meta_json;type:jsonb;not null;default:'{}'"`

	Task *Task `gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:CASCADE"`
}

// TableName: see TaskChecklistItem.TableName for skip-list rationale.
func (TaskCycle) TableName() string { return "task_cycles" }

// TaskCyclePhase is one phase entry within a cycle. A single cycle can have
// multiple rows for the same Phase value (for example a corrective Verify after
// a second Execute), so PhaseSeq is the monotonic entry-order identity within
// a cycle, while Phase is the phase kind. Lifecycle invariants (one Running
// phase per cycle, terminal status immutable, transitions validated by
// ValidPhaseTransition) live at the store boundary.
type TaskCyclePhase struct {
	ID        string      `gorm:"primaryKey"`
	CycleID   string      `gorm:"not null;index;index:task_cycle_phases_cycle_id_seq,unique,priority:1"`
	Phase     Phase       `gorm:"column:phase;not null;check:chk_task_cycle_phases_phase,phase IN ('execute','verify')"`
	PhaseSeq  int64       `gorm:"not null;check:chk_task_cycle_phases_phase_seq,phase_seq > 0;index:task_cycle_phases_cycle_id_seq,unique,priority:2"`
	Status    PhaseStatus `gorm:"not null;index;check:chk_task_cycle_phases_status,status IN ('running','succeeded','failed','skipped')"`
	StartedAt time.Time   `gorm:"not null"`
	EndedAt   *time.Time  `gorm:""`
	Summary   *string     `gorm:"type:text"`
	// DetailsJSON is structured per-phase output (verify check results, persist artifact ids, etc.).
	DetailsJSON datatypes.JSON `gorm:"column:details_json;type:jsonb;not null;default:'{}'"`
	// EventSeq points at the task_events row that mirrors the most recent
	// transition for this phase (set in the same SQL transaction as the mirror
	// insert). Nullable because it is filled in by the store, not by the caller.
	EventSeq *int64 `gorm:"column:event_seq"`

	Cycle *TaskCycle `gorm:"foreignKey:CycleID;references:ID;constraint:OnDelete:CASCADE"`
}

// TableName: see TaskChecklistItem.TableName for skip-list rationale.
func (TaskCyclePhase) TableName() string { return "task_cycle_phases" }

// TaskCycleStreamEvent is a durable, per-attempt record of normalized runner
// progress. It is intentionally separate from TaskEvent so high-volume tool
// streams do not pollute the human-scale task audit timeline.
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

	Task  *Task      `gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:CASCADE"`
	Cycle *TaskCycle `gorm:"foreignKey:CycleID;references:ID;constraint:OnDelete:CASCADE"`
}

// TableName: see TaskChecklistItem.TableName for skip-list rationale.
func (TaskCycleStreamEvent) TableName() string { return "task_cycle_stream_events" }

// TaskCycleCriteriaReport is the per-criterion durable record of what
// the execute agent claimed about a single criterion in one retry
// attempt. Mirrors the `criteria-report.json` side-channel file the
// agent CLI writes (see docs/data-model.md "Report file contracts")
// into the database so verdict evidence survives the cycle. The file
// is still produced and parsed by the worker — it is the agent ↔
// worker wire format — but the file is GC'd at cycle terminate; this
// row is the audit trail.
//
// (CycleID, AttemptSeq, CriterionID) is the natural read key and the
// idempotency key for the worker's bulk upsert: re-parsing the same
// report after a transient store error is safe.
//
// Cascade semantics:
//   - cycle_id: ON DELETE CASCADE — verdicts disappear with their cycle.
//   - criterion_id: ON DELETE NO ACTION — when an operator deletes a
//     checklist item, prior verdicts for it stay so historical cycles
//     remain readable. The handler returns the row even if the FK is
//     stale; the SPA renders the criterion id verbatim in that case.
type TaskCycleCriteriaReport struct {
	ID          string    `gorm:"primaryKey"`
	CycleID     string    `gorm:"not null;index;uniqueIndex:idx_cycle_criteria_unique,priority:1"`
	AttemptSeq  int64     `gorm:"not null;check:chk_task_cycle_criteria_reports_attempt_seq,attempt_seq > 0;uniqueIndex:idx_cycle_criteria_unique,priority:2"`
	CriterionID string    `gorm:"not null;index;uniqueIndex:idx_cycle_criteria_unique,priority:3"`
	ClaimedDone bool      `gorm:"not null;default:false"`
	Evidence    string    `gorm:"type:text;not null;default:''"`
	WrittenAt   time.Time `gorm:"not null;index"`

	Cycle     *TaskCycle         `gorm:"foreignKey:CycleID;references:ID;constraint:OnDelete:CASCADE"`
	Criterion *TaskChecklistItem `gorm:"foreignKey:CriterionID;references:ID;constraint:OnDelete:NO ACTION"`
}

// TableName: see TaskChecklistItem.TableName for skip-list rationale.
func (TaskCycleCriteriaReport) TableName() string { return "task_cycle_criteria_reports" }

// TaskCycleVerifyReport is the per-criterion durable record of the
// verify agent's verdict for a single criterion in one retry attempt.
// See TaskCycleCriteriaReport for cascade and idempotency rationale.
//
// VerifierKind is recorded so the SPA can distinguish a deterministic
// check pass (`deterministic_check`) from an LLM verifier pass
// (`verify_agent`) without re-parsing the workflow — same field as
// task_checklist_completions.VerifiedBy.
type TaskCycleVerifyReport struct {
	ID           string       `gorm:"primaryKey"`
	CycleID      string       `gorm:"not null;index;uniqueIndex:idx_cycle_verify_unique,priority:1"`
	AttemptSeq   int64        `gorm:"not null;check:chk_task_cycle_verify_reports_attempt_seq,attempt_seq > 0;uniqueIndex:idx_cycle_verify_unique,priority:2"`
	CriterionID  string       `gorm:"not null;index;uniqueIndex:idx_cycle_verify_unique,priority:3"`
	Verified     bool         `gorm:"not null;default:false"`
	VerifierKind VerifierKind `gorm:"not null;default:''"`
	Reasoning    string       `gorm:"type:text;not null;default:''"`
	WrittenAt    time.Time    `gorm:"not null;index"`

	Cycle     *TaskCycle         `gorm:"foreignKey:CycleID;references:ID;constraint:OnDelete:CASCADE"`
	Criterion *TaskChecklistItem `gorm:"foreignKey:CriterionID;references:ID;constraint:OnDelete:NO ACTION"`
}

// TableName: see TaskChecklistItem.TableName for skip-list rationale.
func (TaskCycleVerifyReport) TableName() string { return "task_cycle_verify_reports" }

// TaskCycleCommandRun mirrors one verify-phase command execution for a
// criterion attempt. Output bytes live in temp files referenced by
// MetaPath; this row is the durable audit trail for the SPA timeline.
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

// TableName: see TaskChecklistItem.TableName for skip-list rationale.
func (TaskCycleCommandRun) TableName() string { return "task_cycle_command_runs" }

// TaskCycleCommit is the durable worker-indexed record of one git commit
// declared by the agent in criteria-report.json and validated at execute ingest.
// See docs/domain/cycle-commits.md.
//
// Unique (cycle_id, sha) provides idempotent re-ingest across verify retries.
// cycle_id cascades on delete; task_id is denormalized for list-by-task queries.
type TaskCycleCommit struct {
	ID          string     `gorm:"primaryKey"`
	TaskID      string     `gorm:"not null;index"`
	CycleID     string     `gorm:"not null;index;uniqueIndex:idx_cycle_commit_sha,priority:1"`
	PhaseSeq    int64      `gorm:"not null;check:chk_task_cycle_commits_phase_seq,phase_seq > 0"`
	Seq         int64      `gorm:"not null;check:chk_task_cycle_commits_seq,seq > 0;index:idx_cycle_commit_order,priority:2"`
	Repo        string     `gorm:"not null;default:'';type:text"`
	Worktree    string     `gorm:"not null;default:'';type:text"`
	Branch      string     `gorm:"not null;default:''"`
	SHA         string     `gorm:"not null;uniqueIndex:idx_cycle_commit_sha,priority:2"`
	CommittedAt time.Time  `gorm:"not null;index"`
	Message     string     `gorm:"type:text;not null;default:''"`
	RecordedAt  time.Time  `gorm:"not null;index"`
	Cycle       *TaskCycle `gorm:"foreignKey:CycleID;references:ID;constraint:OnDelete:CASCADE"`
	Task        *Task      `gorm:"foreignKey:TaskID;references:ID;constraint:OnDelete:CASCADE"`
}

// TableName: see TaskChecklistItem.TableName for skip-list rationale.
func (TaskCycleCommit) TableName() string { return "task_cycle_commits" }

// ExecuteCriteriaReportAttemptSeq is the attempt_seq used when mirroring
// criteria-report.json at execute phase end. Verify attempts use 1..N;
// this sentinel avoids colliding with the verify retry budget.
const ExecuteCriteriaReportAttemptSeq int64 = 1_000_000
