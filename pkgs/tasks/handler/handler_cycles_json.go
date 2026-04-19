package handler

import (
	"encoding/json"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// cycleStartJSON is the request body for POST /tasks/{id}/cycles.
//
// triggered_by and the X-Actor request header carry overlapping intent;
// to avoid two ways to express the same thing the cycle handler ignores
// any body actor field and always derives from X-Actor (default: user),
// matching the task and checklist routes.
type cycleStartJSON struct {
	ParentCycleID *string         `json:"parent_cycle_id,omitempty"`
	Meta          json.RawMessage `json:"meta,omitempty"`
}

// cycleTerminateJSON is the request body for PATCH /tasks/{id}/cycles/{cycleId}.
type cycleTerminateJSON struct {
	Status domain.CycleStatus `json:"status"`
	Reason string             `json:"reason,omitempty"`
}

// phaseStartJSON is the request body for POST /tasks/{id}/cycles/{cycleId}/phases.
type phaseStartJSON struct {
	Phase domain.Phase `json:"phase"`
}

// phasePatchJSON is the request body for
// PATCH /tasks/{id}/cycles/{cycleId}/phases/{phaseSeq}. status is required;
// summary and details are optional. A nil summary leaves the column unchanged.
type phasePatchJSON struct {
	Status  domain.PhaseStatus `json:"status"`
	Summary *string            `json:"summary,omitempty"`
	Details json.RawMessage    `json:"details,omitempty"`
}

// taskCycleResponse is the JSON shape for a single cycle row. Mirrors
// domain.TaskCycle but uses snake_case keys consistent with the rest of
// taskapi and exposes meta as raw JSON so the client never sees a quoted
// string. meta is always present (defaulted to "{}" by the store).
type taskCycleResponse struct {
	ID            string             `json:"id"`
	TaskID        string             `json:"task_id"`
	AttemptSeq    int64              `json:"attempt_seq"`
	Status        domain.CycleStatus `json:"status"`
	StartedAt     time.Time          `json:"started_at"`
	EndedAt       *time.Time         `json:"ended_at,omitempty"`
	TriggeredBy   domain.Actor       `json:"triggered_by"`
	ParentCycleID *string            `json:"parent_cycle_id,omitempty"`
	Meta          json.RawMessage    `json:"meta"`
}

// taskCyclePhaseResponse is the JSON shape for a single phase row.
// details is always present (defaulted to "{}" by the store).
type taskCyclePhaseResponse struct {
	ID        string             `json:"id"`
	CycleID   string             `json:"cycle_id"`
	Phase     domain.Phase       `json:"phase"`
	PhaseSeq  int64              `json:"phase_seq"`
	Status    domain.PhaseStatus `json:"status"`
	StartedAt time.Time          `json:"started_at"`
	EndedAt   *time.Time         `json:"ended_at,omitempty"`
	Summary   *string            `json:"summary,omitempty"`
	Details   json.RawMessage    `json:"details"`
	EventSeq  *int64             `json:"event_seq,omitempty"`
}

// taskCyclesListResponse is the JSON envelope for GET /tasks/{id}/cycles.
// cycles is always a JSON array (never null). has_more is detected by
// fetching limit+1 rows from the store; the extra row is dropped.
//
// next_before_attempt_seq is the cursor for the next (older) page when
// has_more is true. It carries the attempt_seq of the last (lowest) row
// in this response, suitable for the caller to pass back as
// ?before_attempt_seq= on the next GET. Omitted via omitempty when no
// next page exists so clients can use absence as the end-of-stream
// signal (matches the /events `next_after_seq` / `next_before_seq`
// convention).
type taskCyclesListResponse struct {
	TaskID               string              `json:"task_id"`
	Cycles               []taskCycleResponse `json:"cycles"`
	Limit                int                 `json:"limit"`
	HasMore              bool                `json:"has_more"`
	NextBeforeAttemptSeq *int64              `json:"next_before_attempt_seq,omitempty"`
}

// taskCycleDetailResponse is the JSON envelope for GET /tasks/{id}/cycles/{cycleId}.
// phases is always a JSON array (never null) ordered by phase_seq ASC.
type taskCycleDetailResponse struct {
	ID            string                   `json:"id"`
	TaskID        string                   `json:"task_id"`
	AttemptSeq    int64                    `json:"attempt_seq"`
	Status        domain.CycleStatus       `json:"status"`
	StartedAt     time.Time                `json:"started_at"`
	EndedAt       *time.Time               `json:"ended_at,omitempty"`
	TriggeredBy   domain.Actor             `json:"triggered_by"`
	ParentCycleID *string                  `json:"parent_cycle_id,omitempty"`
	Meta          json.RawMessage          `json:"meta"`
	Phases        []taskCyclePhaseResponse `json:"phases"`
}
