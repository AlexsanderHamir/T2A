package domain

import (
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
	// Runner is the agent runner id for this task (e.g. "cursor"). Set at
	// create time from the request or app defaults; must match the worker's
	// configured runner when the task runs.
	Runner string `json:"runner" gorm:"not null;default:'cursor'"`
	// CursorModel is forwarded to cursor-agent as --model when non-empty;
	// empty means omit the flag for this task (same semantics as app settings).
	CursorModel string `json:"cursor_model" gorm:"not null;default:''"`
	// PickupNotBefore defers agent dequeue until this instant (UTC). NULL means
	// eligible as soon as status is ready (legacy rows and zero-delay creates).
	PickupNotBefore *time.Time `json:"pickup_not_before,omitempty" gorm:"index"`
}

// TaskChecklistItem is a definition row owned by a task that does not use checklist_inherit.
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

// TaskChecklistCompletion records that subject TaskID satisfied checklist item ItemID.
type TaskChecklistCompletion struct {
	TaskID string    `json:"task_id" gorm:"primaryKey"`
	ItemID string    `json:"item_id" gorm:"primaryKey"`
	At     time.Time `json:"at" gorm:"not null"`
	By     Actor     `json:"by" gorm:"column:done_by;not null"`
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

// TaskCycle is one execution attempt for a task. The (TaskID, AttemptSeq) pair
// gives a stable monotonic ordering of attempts. A cycle's lifecycle is enforced
// at the store boundary: at most one Running cycle per task at any time, and
// terminal statuses (Succeeded / Failed / Aborted) are immutable. See
// docs/EXECUTION-CYCLES.md.
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
	Phase     Phase       `gorm:"column:phase;not null;check:chk_task_cycle_phases_phase,phase IN ('diagnose','execute','verify','persist')"`
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
