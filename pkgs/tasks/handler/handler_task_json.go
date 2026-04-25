package handler

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

type taskCreateJSON struct {
	ID               string          `json:"id"`
	DraftID          string          `json:"draft_id"`
	Title            string          `json:"title"`
	InitialPrompt    string          `json:"initial_prompt"`
	Status           domain.Status   `json:"status"`
	Priority         domain.Priority `json:"priority"`
	TaskType         domain.TaskType `json:"task_type"`
	ParentID         *string         `json:"parent_id"`
	ChecklistInherit *bool           `json:"checklist_inherit"`
	Runner           *string         `json:"runner"`
	CursorModel      *string         `json:"cursor_model"`
	// PickupNotBefore is an optional RFC3339 instant. When provided,
	// the worker will not pick up the task until this time has passed
	// (see docs/SCHEDULING.md). Omitted/null = no schedule = pick up
	// as soon as the global agent_pickup_delay_seconds elapses. The
	// pre-2000 sentinel is rejected to guard against accidental
	// zero-value timestamps.
	PickupNotBefore *string `json:"pickup_not_before,omitempty"`
}

type taskEvaluateJSON struct {
	ID               string                                  `json:"id"`
	Title            string                                  `json:"title"`
	InitialPrompt    string                                  `json:"initial_prompt"`
	Status           domain.Status                           `json:"status"`
	Priority         domain.Priority                         `json:"priority"`
	TaskType         domain.TaskType                         `json:"task_type"`
	ParentID         *string                                 `json:"parent_id"`
	ChecklistInherit *bool                                   `json:"checklist_inherit"`
	ChecklistItems   []store.EvaluateDraftChecklistItemInput `json:"checklist_items"`
}

type taskDraftSaveJSON struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Payload json.RawMessage `json:"payload"`
}

type taskPatchJSON struct {
	Title            *string                   `json:"title"`
	InitialPrompt    *string                   `json:"initial_prompt"`
	Status           *domain.Status            `json:"status"`
	Priority         *domain.Priority          `json:"priority"`
	TaskType         *domain.TaskType          `json:"task_type"`
	ParentID         patchParentField          `json:"parent_id"`
	ChecklistInherit *bool                     `json:"checklist_inherit"`
	PickupNotBefore  patchPickupNotBeforeField `json:"pickup_not_before"`
	// CursorModel sets tasks.cursor_model when the key is present (including
	// the empty string, which clears per-task override). JSON null is decoded
	// as nil and means "no change", same as omitting the key.
	CursorModel *string `json:"cursor_model"`
}

type listResponse struct {
	Tasks   []store.TaskNode `json:"tasks"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	HasMore bool             `json:"has_more"`
}

type taskStatsResponse struct {
	Total    int64 `json:"total"`
	Ready    int64 `json:"ready"`
	Critical int64 `json:"critical"`
	// Scheduled mirrors store.TaskStats.Scheduled — the count of
	// status='ready' tasks deferred via pickup_not_before > now.
	// Always present (0 on a fresh database) so the wire shape
	// stays stable; the SPA distinguishes "0 ready, 12 scheduled"
	// (intentionally deferred — agent worker is correctly idle)
	// from "0 ready, 0 scheduled" (truly idle, nothing to do).
	Scheduled      int64                     `json:"scheduled"`
	ByStatus       map[domain.Status]int64   `json:"by_status"`
	ByPriority     map[domain.Priority]int64 `json:"by_priority"`
	ByScope        map[string]int64          `json:"by_scope"`
	Cycles         taskStatsCyclesJSON       `json:"cycles"`
	Phases         taskStatsPhasesJSON       `json:"phases"`
	Runner         taskStatsRunnerJSON       `json:"runner"`
	RecentFailures []taskStatsFailureJSON    `json:"recent_failures"`
}

// taskStatsCyclesJSON is the always-present `cycles` block of
// /tasks/stats. Both maps are non-nil (`{}` on empty database) per the
// store-layer invariant in stats.Get.
type taskStatsCyclesJSON struct {
	ByStatus      map[domain.CycleStatus]int64 `json:"by_status"`
	ByTriggeredBy map[domain.Actor]int64       `json:"by_triggered_by"`
}

// taskStatsPhasesJSON is the always-present `phases` block. The outer
// map carries every domain.Phase enum value (4 keys); inner maps are
// non-nil but only carry enum keys with nonzero count. The (phase x
// status) shape is pinned so clients can render every phase/status cell.
type taskStatsPhasesJSON struct {
	ByPhaseStatus map[domain.Phase]map[domain.PhaseStatus]int64 `json:"by_phase_status"`
}

// taskStatsRunnerJSON is the always-present `runner` block of
// /tasks/stats added in Phase 2 of the per-task runner/model
// attribution plan. All three maps are non-nil ({} on empty database)
// per the store-layer invariant in stats.Get. Bucket keys are:
//
//   - by_runner:        Runner.Name() (verbatim from cycle_meta.runner).
//     The literal "unknown" key (RunnerUnknownKey)
//     holds pre-feature cycles whose meta_json never
//     carried the runner key.
//   - by_model:         Runner.EffectiveModel resolution (verbatim
//     from cycle_meta.cursor_model_effective). The
//     empty-string "" key is the explicit "default
//     model" bucket — the SPA renders it as such.
//   - by_runner_model:  pipe-delimited "<runner>|<model>" composite
//     key. The frontend splits on "|" to render
//     the two-level table.
//   - by_runner_model_resolved:
//     pipe-delimited "<runner>|<effective>|<resolved>"
//     triple-composite key. Only populated for
//     cycles whose execute-phase details_json
//     surfaced a non-empty resolved_model — the
//     cursor adapter lifts that value from
//     cursor-agent's stream-json `system.init.model`
//     event, which is the only surface that reveals
//     what model `auto` actually routed to. The
//     SPA uses this map to render "Cursor CLI ·
//     Auto → Claude 4 Sonnet" style sub-rows only
//     when there is a real observation, so
//     pre-feature / non-cursor cycles don't get
//     phantom entries.
//
// Each bucket carries the by-status counter (mirrors the global
// cycles.by_status shape) plus succeeded-only p50/p95 durations
// (decision D3): failed/aborted runs do not skew the latency cells.
type taskStatsRunnerJSON struct {
	ByRunner              map[string]taskStatsRunnerBucketJSON `json:"by_runner"`
	ByModel               map[string]taskStatsRunnerBucketJSON `json:"by_model"`
	ByRunnerModel         map[string]taskStatsRunnerBucketJSON `json:"by_runner_model"`
	ByRunnerModelResolved map[string]taskStatsRunnerBucketJSON `json:"by_runner_model_resolved"`
}

// taskStatsRunnerBucketJSON is the per-bucket payload. Empty-bucket
// p50/p95 values are 0 (NOT null/omitted) — the SPA decides whether
// to render "—" instead of "0.00s" by gating on succeeded > 0.
type taskStatsRunnerBucketJSON struct {
	ByStatus                    map[domain.CycleStatus]int64 `json:"by_status"`
	Succeeded                   int64                        `json:"succeeded"`
	DurationP50SucceededSeconds float64                      `json:"duration_p50_succeeded_seconds"`
	DurationP95SucceededSeconds float64                      `json:"duration_p95_succeeded_seconds"`
}

// taskStatsFailureJSON is one row in the `recent_failures` array. The
// frontend deep-links to `/tasks/{task_id}/events/{event_seq}` so both
// fields are mandatory; reason / cycle_id / attempt_seq / status round
// out the "what happened" summary card.
//
// Reason prefers failure_summary on the cycle_failed mirror (written at
// terminate time from the failed execute phase), then legacy enrichment
// from a matching phase_failed event, else the mirror reason code (see
// stats.scan_failures and cycles.Terminate).
type taskStatsFailureJSON struct {
	TaskID     string    `json:"task_id"`
	EventSeq   int64     `json:"event_seq"`
	At         time.Time `json:"at"`
	CycleID    string    `json:"cycle_id"`
	AttemptSeq int64     `json:"attempt_seq"`
	Status     string    `json:"status"`
	Reason     string    `json:"reason"`
}

// cycleFailuresResponse is GET /tasks/cycle-failures (paginated list).
type cycleFailuresResponse struct {
	Total               int64                  `json:"total"`
	Limit               int                    `json:"limit"`
	Offset              int                    `json:"offset"`
	Sort                string                 `json:"sort"`
	ReasonSortTruncated bool                   `json:"reason_sort_truncated"`
	Failures            []taskStatsFailureJSON `json:"failures"`
}

// taskStatsResponseFromStore projects the store-level TaskStats onto
// the wire envelope. The store guarantees every map is non-nil and
// RecentFailures is a non-nil slice; this projector preserves both
// invariants so JSON encoding never emits `null` for those fields.
func recentFailuresToJSON(failures []store.RecentFailure) []taskStatsFailureJSON {
	out := make([]taskStatsFailureJSON, 0, len(failures))
	for _, f := range failures {
		out = append(out, taskStatsFailureJSON{
			TaskID:     f.TaskID,
			EventSeq:   f.EventSeq,
			At:         f.At,
			CycleID:    f.CycleID,
			AttemptSeq: f.AttemptSeq,
			Status:     f.Status,
			Reason:     f.Reason,
		})
	}
	return out
}

func taskStatsResponseFromStore(s store.TaskStats) taskStatsResponse {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.taskStatsResponseFromStore")
	failures := recentFailuresToJSON(s.RecentFailures)
	return taskStatsResponse{
		Total:      s.Total,
		Ready:      s.Ready,
		Critical:   s.Critical,
		Scheduled:  s.Scheduled,
		ByStatus:   s.ByStatus,
		ByPriority: s.ByPriority,
		ByScope:    s.ByScope,
		Cycles: taskStatsCyclesJSON{
			ByStatus:      s.Cycles.ByStatus,
			ByTriggeredBy: s.Cycles.ByTriggeredBy,
		},
		Phases: taskStatsPhasesJSON{
			ByPhaseStatus: s.Phases.ByPhaseStatus,
		},
		Runner:         taskStatsRunnerFromStore(s.Runner),
		RecentFailures: failures,
	}
}

// taskStatsRunnerFromStore preserves the always-non-nil-map invariant
// (every map is {} on a fresh database, never null on the wire) and
// projects each per-bucket payload onto the JSON shape pinned by
// taskStatsRunnerBucketJSON.
func taskStatsRunnerFromStore(s store.RunnerStats) taskStatsRunnerJSON {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.taskStatsRunnerFromStore")
	out := taskStatsRunnerJSON{
		ByRunner:              make(map[string]taskStatsRunnerBucketJSON, len(s.ByRunner)),
		ByModel:               make(map[string]taskStatsRunnerBucketJSON, len(s.ByModel)),
		ByRunnerModel:         make(map[string]taskStatsRunnerBucketJSON, len(s.ByRunnerModel)),
		ByRunnerModelResolved: make(map[string]taskStatsRunnerBucketJSON, len(s.ByRunnerModelResolved)),
	}
	for k, b := range s.ByRunner {
		out.ByRunner[k] = bucketJSONFromStore(b)
	}
	for k, b := range s.ByModel {
		out.ByModel[k] = bucketJSONFromStore(b)
	}
	for k, b := range s.ByRunnerModel {
		out.ByRunnerModel[k] = bucketJSONFromStore(b)
	}
	for k, b := range s.ByRunnerModelResolved {
		out.ByRunnerModelResolved[k] = bucketJSONFromStore(b)
	}
	return out
}

func bucketJSONFromStore(b store.RunnerBucket) taskStatsRunnerBucketJSON {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.bucketJSONFromStore")
	return taskStatsRunnerBucketJSON{
		ByStatus:                    b.ByStatus,
		Succeeded:                   b.Succeeded,
		DurationP50SucceededSeconds: b.DurationP50SucceededSeconds,
		DurationP95SucceededSeconds: b.DurationP95SucceededSeconds,
	}
}

type taskEventLine struct {
	Seq            int64                        `json:"seq"`
	At             time.Time                    `json:"at"`
	Type           domain.EventType             `json:"type"`
	By             domain.Actor                 `json:"by"`
	Data           json.RawMessage              `json:"data"`
	UserResponse   *string                      `json:"user_response,omitempty"`
	UserResponseAt *time.Time                   `json:"user_response_at,omitempty"`
	ResponseThread []domain.ResponseThreadEntry `json:"response_thread,omitempty"`
}

type taskEventsResponse struct {
	TaskID          string          `json:"task_id"`
	Events          []taskEventLine `json:"events"`
	Limit           *int            `json:"limit,omitempty"`
	Total           *int64          `json:"total,omitempty"`
	RangeStart      *int64          `json:"range_start,omitempty"`
	RangeEnd        *int64          `json:"range_end,omitempty"`
	HasMoreNewer    bool            `json:"has_more_newer"`
	HasMoreOlder    bool            `json:"has_more_older"`
	ApprovalPending bool            `json:"approval_pending"`
}

type taskEventDetailResponse struct {
	TaskID         string                       `json:"task_id"`
	Seq            int64                        `json:"seq"`
	At             time.Time                    `json:"at"`
	Type           domain.EventType             `json:"type"`
	By             domain.Actor                 `json:"by"`
	Data           json.RawMessage              `json:"data"`
	UserResponse   *string                      `json:"user_response,omitempty"`
	UserResponseAt *time.Time                   `json:"user_response_at,omitempty"`
	ResponseThread []domain.ResponseThreadEntry `json:"response_thread,omitempty"`
}

type taskEventUserResponseJSON struct {
	UserResponse string `json:"user_response"`
}
