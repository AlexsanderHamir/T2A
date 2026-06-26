package domain

import (
	"encoding/json"
	"time"
)

type Task struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	InitialPrompt string   `json:"initial_prompt"`
	Status        Status   `json:"status"`
	Priority      Priority `json:"priority"`
	ProjectID     *string  `json:"project_id,omitempty"`
	// ProjectContextItemIDs is the user-selected subset of project context to pass to agent runs.
	ProjectContextItemIDs []string  `json:"project_context_item_ids,omitempty"`
	Tags                  []string  `json:"tags,omitempty"`
	Milestone             *string   `json:"milestone,omitempty"`
	Gate                  *TaskGate `json:"gate,omitempty"`
	// DependsOn is hydrated from task_dependencies on read; not a database column.
	DependsOn []DependencyEdge `json:"depends_on,omitempty"`
	// Runner is the agent runner id for this task (e.g. "cursor"). Set at
	// create time from the request or app defaults; must match the worker's
	// configured runner when the task runs.
	Runner string `json:"runner"`
	// CursorModel is forwarded to cursor-agent as --model when non-empty;
	// empty means omit the flag for this task (same semantics as app settings).
	CursorModel string `json:"cursor_model"`
	// RunnerConfig stores per-task runner config overrides as a JSON blob.
	// When non-empty, the worker merges this with the global runner config
	// from app_settings.runner_configs for the matching runner ID.
	RunnerConfig json.RawMessage `json:"runner_config,omitempty"`
	// PickupNotBefore defers agent dequeue until this instant (UTC). NULL means
	// eligible as soon as status is ready (legacy rows and zero-delay creates).
	PickupNotBefore *time.Time `json:"pickup_not_before,omitempty"`
	// CriteriaSatisfiedAt is set when every inherited checklist item has a
	// verified completion row; cleared when any item becomes unchecked.
	// Maintained in checklist completion TX for SQL queue parity.
	CriteriaSatisfiedAt *time.Time `json:"criteria_satisfied_at,omitempty"`
	// PendingRetry holds operator retry intent between POST /retry and worker
	// pickup. Not exposed on the public task API (json:"-").
	PendingRetry *PendingRetry `json:"-"`
	// CreatedAt is hydrated from the seq=1 task_created audit row on read;
	// not a tasks-table column.
	CreatedAt *time.Time `json:"created_at,omitempty"`
	// WorktreeBranchID binds the task to a worktree_branches association
	// ("this branch, in this directory"). Plain indexed nullable column;
	// validated on create from Cycle 4. See ADR-0037.
	WorktreeBranchID *string `json:"worktree_branch_id,omitempty"`
}

// Project is shared context memory for a long-running body of work.
//
// RepositoryID ties a project to exactly one global repository (ADR-0037); the
// repository must exist first. Nullable: the built-in default project is legacy
// with no repository. Plain indexed nullable column (no FK constraint, same
// pattern as Task git-binding columns).
type Project struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Description    string        `json:"description"`
	Status         ProjectStatus `json:"status"`
	ContextSummary string        `json:"context_summary"`
	RepositoryID   *string       `json:"repository_id,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// ProjectContextItem is a human-inspectable memory item attached to a project.
type ProjectContextItem struct {
	ID            string             `json:"id"`
	ProjectID     string             `json:"project_id"`
	Kind          ProjectContextKind `json:"kind"`
	Title         string             `json:"title"`
	Body          string             `json:"body"`
	SourceTaskID  *string            `json:"source_task_id,omitempty"`
	SourceCycleID *string            `json:"source_cycle_id,omitempty"`
	CreatedBy     Actor              `json:"created_by"`
	Pinned        bool               `json:"pinned"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

// ProjectContextEdge is a user-curated relationship between two context nodes
// owned by the same project.
type ProjectContextEdge struct {
	ID              string                 `json:"id"`
	ProjectID       string                 `json:"project_id"`
	SourceContextID string                 `json:"source_context_id"`
	TargetContextID string                 `json:"target_context_id"`
	Relation        ProjectContextRelation `json:"relation"`
	Strength        int                    `json:"strength"`
	Note            string                 `json:"note"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// TaskContextSnapshot records the exact project context bundle handed to one
// task execution attempt. It is immutable audit data, not canonical project memory.
type TaskContextSnapshot struct {
	ID              string          `json:"id"`
	TaskID          string          `json:"task_id"`
	CycleID         string          `json:"cycle_id"`
	ProjectID       string          `json:"project_id"`
	ContextJSON     json.RawMessage `json:"context_json"`
	RenderedContext string          `json:"rendered_context"`
	TokenEstimate   int             `json:"token_estimate"`
	CreatedAt       time.Time       `json:"created_at"`
}

// TaskDependency is a directed edge: task_id depends on depends_on_task_id completing first.
type TaskDependency struct {
	TaskID          string              `json:"task_id"`
	DependsOnTaskID string              `json:"depends_on_task_id"`
	Satisfies       DependencySatisfies `json:"satisfies"`
	CreatedAt       time.Time           `json:"created_at"`
}

// TaskChecklistItem is a definition row owned by a task.
// Completion is recorded only by the agent worker after verify (verified_by=verify_agent)
// or when execute did not claim done (verified_by=agent_self, failure-only).
type TaskChecklistItem struct {
	ID        string `json:"id"`
	TaskID    string `json:"task_id"`
	SortOrder int    `json:"sort_order"`
	Text      string `json:"text"`
}

// TaskChecklistItemCommand is an optional shell check attached to a
// checklist definition. The worker runs these during verify and writes
// stdout/stderr to temp files under the cycle report dir; the LLM
// verifier interprets the artifacts against ExpectedOutcome.
type TaskChecklistItemCommand struct {
	ID              string `json:"id"`
	ItemID          string `json:"item_id"`
	SortOrder       int    `json:"sort_order"`
	Command         string `json:"command"`
	ExpectedOutcome string `json:"expected_outcome"`
}

// TaskChecklistCompletion records that subject TaskID satisfied checklist item ItemID.
type TaskChecklistCompletion struct {
	TaskID            string       `json:"task_id"`
	ItemID            string       `json:"item_id"`
	At                time.Time    `json:"at"`
	By                Actor        `json:"by"`
	Evidence          string       `json:"evidence"`
	VerifiedBy        VerifierKind `json:"verified_by"`
	VerifierReasoning string       `json:"verifier_reasoning"`
	CycleID           string       `json:"cycle_id"`
}

type TaskEvent struct {
	TaskID string    `json:"task_id"`
	Seq    int64     `json:"seq"`
	At     time.Time `json:"at"`
	Type   EventType `json:"type"`
	By     Actor     `json:"by"`
	Data   json.RawMessage `json:"data"`

	// UserResponse is optional human-supplied text for event types that accept input (see EventTypeAcceptsUserResponse).
	UserResponse *string `json:"user_response,omitempty"`
	// UserResponseAt is set when UserResponse is written or updated (UTC).
	UserResponseAt *time.Time `json:"user_response_at,omitempty"`
	// ResponseThread is an ordered JSON array of ResponseThreadEntry (user ↔ agent messages).
	ResponseThread json.RawMessage `json:"response_thread,omitempty"`
}

// TaskDraft stores a resumable create-task draft payload.
type TaskDraft struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	PayloadJSON json.RawMessage `json:"payload_json"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// TaskTemplate stores a reusable task compose blueprint (not a runnable task).
type TaskTemplate struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	PayloadJSON json.RawMessage `json:"payload_json"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// TaskCycle is one execution attempt for a task. The (TaskID, AttemptSeq) pair
// gives a stable monotonic ordering of attempts. A cycle's lifecycle is enforced
// at the store boundary: at most one Running cycle per task at any time, and
// terminal statuses (Succeeded / Failed / Aborted) are immutable. See
// docs/data-model.md.
type TaskCycle struct {
	ID            string          `json:"id"`
	TaskID        string          `json:"task_id"`
	AttemptSeq    int64           `json:"attempt_seq"`
	Status        CycleStatus     `json:"status"`
	StartedAt     time.Time       `json:"started_at"`
	EndedAt       *time.Time      `json:"ended_at,omitempty"`
	TriggeredBy   Actor           `json:"triggered_by"`
	ParentCycleID *string         `json:"parent_cycle_id,omitempty"`
	MetaJSON      json.RawMessage `json:"meta_json"`
}

// TaskCyclePhase is one phase entry within a cycle. A single cycle can have
// multiple rows for the same Phase value (for example a corrective Verify after
// a second Execute), so PhaseSeq is the monotonic entry-order identity within
// a cycle, while Phase is the phase kind. Lifecycle invariants (one Running
// phase per cycle, terminal status immutable, transitions validated by
// ValidPhaseTransition) live at the store boundary.
type TaskCyclePhase struct {
	ID        string          `json:"id"`
	CycleID   string          `json:"cycle_id"`
	Phase     Phase           `json:"phase"`
	PhaseSeq  int64           `json:"phase_seq"`
	Status    PhaseStatus     `json:"status"`
	StartedAt time.Time       `json:"started_at"`
	EndedAt   *time.Time      `json:"ended_at,omitempty"`
	Summary   *string         `json:"summary,omitempty"`
	DetailsJSON json.RawMessage `json:"details_json"`
	// EventSeq points at the task_events row that mirrors the most recent
	// transition for this phase (set in the same SQL transaction as the mirror
	// insert). Nullable because it is filled in by the store, not by the caller.
	EventSeq *int64 `json:"event_seq,omitempty"`
}

// TaskCycleStreamEvent is a durable, per-attempt record of normalized runner
// progress. It is intentionally separate from TaskEvent so high-volume tool
// streams do not pollute the human-scale task audit timeline.
type TaskCycleStreamEvent struct {
	ID          string          `json:"id"`
	TaskID      string          `json:"task_id"`
	CycleID     string          `json:"cycle_id"`
	PhaseSeq    int64           `json:"phase_seq"`
	StreamSeq   int64           `json:"stream_seq"`
	At          time.Time       `json:"at"`
	Source      string          `json:"source"`
	Kind        string          `json:"kind"`
	Subtype     string          `json:"subtype"`
	Message     string          `json:"message"`
	Tool        string          `json:"tool"`
	PayloadJSON json.RawMessage `json:"payload_json"`
}

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
	ID          string    `json:"id"`
	CycleID     string    `json:"cycle_id"`
	AttemptSeq  int64     `json:"attempt_seq"`
	CriterionID string    `json:"criterion_id"`
	ClaimedDone bool      `json:"claimed_done"`
	Evidence    string    `json:"evidence"`
	WrittenAt   time.Time `json:"written_at"`
}

// TaskCycleVerifyReport is the per-criterion durable record of the
// verify agent's verdict for a single criterion in one retry attempt.
// See TaskCycleCriteriaReport for cascade and idempotency rationale.
//
// VerifierKind is recorded so the SPA can distinguish a deterministic
// check pass (`deterministic_check`) from an LLM verifier pass
// (`verify_agent`) without re-parsing the workflow — same field as
// task_checklist_completions.VerifiedBy.
type TaskCycleVerifyReport struct {
	ID           string       `json:"id"`
	CycleID      string       `json:"cycle_id"`
	AttemptSeq   int64        `json:"attempt_seq"`
	CriterionID  string       `json:"criterion_id"`
	Verified     bool         `json:"verified"`
	VerifierKind VerifierKind `json:"verifier_kind"`
	Reasoning    string       `json:"reasoning"`
	WrittenAt    time.Time    `json:"written_at"`
}

// TaskCycleCommandRun mirrors one verify-phase command execution for a
// criterion attempt. Output bytes live in temp files referenced by
// MetaPath; this row is the durable audit trail for the SPA timeline.
type TaskCycleCommandRun struct {
	ID          string    `json:"id"`
	CycleID     string    `json:"cycle_id"`
	AttemptSeq  int64     `json:"attempt_seq"`
	CriterionID string    `json:"criterion_id"`
	CommandSeq  int64     `json:"command_seq"`
	ExitCode    int       `json:"exit_code"`
	MetaPath    string    `json:"meta_path"`
	WrittenAt   time.Time `json:"written_at"`
}

// TaskCycleCommit is the durable worker-indexed record of one git commit
// declared by the agent in criteria-report.json and validated at execute ingest.
// See docs/domain/cycle-commits.md.
//
// Unique (cycle_id, sha) provides idempotent re-ingest across verify retries.
// cycle_id cascades on delete; task_id is denormalized for list-by-task queries.
type TaskCycleCommit struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"task_id"`
	CycleID     string    `json:"cycle_id"`
	PhaseSeq    int64     `json:"phase_seq"`
	Seq         int64     `json:"seq"`
	Repo        string    `json:"repo"`
	Worktree    string    `json:"worktree"`
	Branch      string    `json:"branch"`
	SHA         string    `json:"sha"`
	CommittedAt time.Time `json:"committed_at"`
	Message     string    `json:"message"`
	RecordedAt  time.Time `json:"recorded_at"`
}

// ExecuteCriteriaReportAttemptSeq is the attempt_seq used when mirroring
// criteria-report.json at execute phase end. Verify attempts use 1..N;
// this sentinel avoids colliding with the verify retry budget.
const ExecuteCriteriaReportAttemptSeq int64 = 1_000_000
