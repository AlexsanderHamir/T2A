export type Status =
  | "ready"
  | "running"
  | "blocked"
  | "review"
  | "done"
  | "failed";

export type Priority = "low" | "medium" | "high" | "critical";
export type TaskType =
  | "general"
  | "bug_fix"
  | "feature"
  | "refactor"
  | "docs"
  | "dmap";

/** Empty string means no selection yet (create / draft forms). */
export type PriorityChoice = Priority | "";

export type Task = {
  id: string;
  title: string;
  initial_prompt: string;
  status: Status;
  priority: Priority;
  task_type?: TaskType;
  /** Agent runner id for this task (e.g. `cursor`); chosen at create time. */
  runner: string;
  /**
   * Optional `cursor-agent --model` value for this task. Empty means omit
   * the flag (Cursor default for the account).
   */
  cursor_model: string;
  /**
   * When set (RFC3339), the agent worker will not dequeue this ready task
   * until this instant. Omitted when eligible immediately.
   */
  pickup_not_before?: string;
  /** Present when this task is nested under another (GET /tasks tree). */
  parent_id?: string;
  /** When true, checklist definitions come from the nearest ancestor that does not inherit. */
  checklist_inherit: boolean;
  /** Nested subtasks from GET /tasks or GET /tasks/{id} (tree). */
  children?: Task[];
};

export type TaskListResponse = {
  tasks: Task[];
  limit: number;
  offset: number;
  /** True when the server may have more root tasks (see GET /tasks in docs/API-HTTP.md). */
  has_more: boolean;
};

/**
 * One entry in `recent_failures` from `GET /tasks/stats`. Mirrors the
 * server projection in pkgs/tasks/handler/handler_task_json.go (struct
 * taskStatsFailureJSON). `task_id` + `event_seq` deep-link to
 * `GET /tasks/{task_id}/events/{event_seq}`; `status` is the original
 * terminal cycle status (`failed` or `aborted`) recovered from the
 * cycle_failed payload (the mirror folds aborts into cycle_failed).
 */
export type TaskStatsRecentFailure = {
  task_id: string;
  event_seq: number;
  /** ISO 8601 from API. */
  at: string;
  cycle_id: string;
  attempt_seq: number;
  status: "failed" | "aborted";
  /**
   * Human-readable failure text: prefers `failure_summary` on the
   * cycle_failed mirror (same source as execute phase_failed), then
   * legacy enrichment from a matching phase_failed event, else the
   * mirror reason code (e.g. runner_non_zero_exit).
   */
  reason: string;
};

/** GET /tasks/cycle-failures — paginated cycle_failed list for the failures page. */
export type CycleFailuresListResponse = {
  total: number;
  limit: number;
  offset: number;
  sort: string;
  reason_sort_truncated: boolean;
  failures: TaskStatsRecentFailure[];
};

/**
 * Cycle aggregates from `GET /tasks/stats`. Both maps are always
 * present (`{}` on empty database). Inner enums match
 * `pkgs/tasks/domain` exactly so a future enum change trips the
 * parser, the contract test, and the heatmap in the same PR.
 */
export type TaskStatsCycles = {
  by_status: Partial<Record<import("./cycle").CycleStatus, number>>;
  by_triggered_by: Partial<Record<"user" | "agent", number>>;
};

/**
 * Phase aggregates from `GET /tasks/stats`. The outer map is the four
 * `domain.Phase` values; every key is always present (the inner map is
 * `{}` for phases that have never run). The `(phase x status)` shape
 * is the source of the Observability heatmap.
 */
export type TaskStatsPhases = {
  by_phase_status: Record<
    import("./cycle").Phase,
    Partial<Record<import("./cycle").PhaseStatus, number>>
  >;
};

/**
 * One entry in the runner / model breakdown returned by
 * `GET /tasks/stats` (Phase 2 of the per-task runner+model
 * attribution work). `succeeded` mirrors `by_status.succeeded` so
 * the SPA can branch on the percentile gate without a missing-key
 * check; `duration_p50_succeeded_seconds` /
 * `duration_p95_succeeded_seconds` are 0 when `succeeded === 0`
 * (render "—" rather than "0.00s" in that case).
 */
export type TaskStatsRunnerBucket = {
  by_status: Partial<Record<import("./cycle").CycleStatus, number>>;
  succeeded: number;
  duration_p50_succeeded_seconds: number;
  duration_p95_succeeded_seconds: number;
};

/**
 * Per-runner / per-model / per-(runner,model) aggregates on
 * `GET /tasks/stats`. All three maps are always present (`{}` on
 * empty database). Bucket keys are verbatim from cycle meta:
 *  - `by_runner` is keyed by `runner.Name()` ("unknown" for cycles
 *    whose meta predates the V2 keys)
 *  - `by_model` is keyed by the resolved effective model; the
 *    empty-string key is preserved (means "no model configured")
 *  - `by_runner_model` is the (runner, model) pair joined by `|`
 *  - `by_runner_model_resolved` is the (runner, effective model,
 *    resolved model) triple joined by `|`. Only populated for cycles
 *    whose execute-phase details surfaced a concrete resolved model
 *    (today: cursor-agent's `system.init.model` event under
 *    `--output-format stream-json`). Cycles without a resolved model
 *    are intentionally absent so the SPA only renders a "→ actual
 *    model" sub-row when we have a real observation, not a guess.
 */
export type TaskStatsRunner = {
  by_runner: Record<string, TaskStatsRunnerBucket>;
  by_model: Record<string, TaskStatsRunnerBucket>;
  by_runner_model: Record<string, TaskStatsRunnerBucket>;
  by_runner_model_resolved: Record<string, TaskStatsRunnerBucket>;
};

export type TaskStatsResponse = {
  total: number;
  ready: number;
  critical: number;
  /**
   * Count of `status='ready'` tasks intentionally deferred via
   * `pickup_not_before > now()`. Always present (`0` on a fresh
   * database). The Observability page uses this to distinguish
   * "0 ready, 12 scheduled" (intentionally deferred — agent worker is
   * correctly idle) from "0 ready, 0 scheduled" (truly idle, nothing
   * to do). Defaults to `0` when an older backend omits the key
   * (parser sets it explicitly so callers can rely on a number).
   */
  scheduled: number;
  by_status: Partial<Record<Status, number>>;
  by_priority: Partial<Record<Priority, number>>;
  by_scope: {
    parent: number;
    subtask: number;
  };
  cycles: TaskStatsCycles;
  phases: TaskStatsPhases;
  runner: TaskStatsRunner;
  /** Newest first; capped server-side at 25. Always an array (never null). */
  recent_failures: TaskStatsRecentFailure[];
};

export type TaskChangeType =
  | "task_created"
  | "task_updated"
  | "task_deleted"
  | "task_cycle_changed";

/**
 * Wire shape of a single SSE frame on `GET /events`.
 *
 * `cycle_id` is only present on `task_cycle_changed` (omitted for the other
 * three types so the existing wire shape stays byte-identical).
 */
export type TaskChangeEvent = {
  type: TaskChangeType;
  id: string;
  cycle_id?: string;
};

export const STATUSES: Status[] = [
  "ready",
  "running",
  "blocked",
  "review",
  "done",
  "failed",
];

export const PRIORITIES: Priority[] = [
  "low",
  "medium",
  "high",
  "critical",
];

/** New tasks start here; status is not user-selectable in the UI. */
export const DEFAULT_NEW_TASK_STATUS: Status = "ready";

/** Mirrors server `domain.EventType` (audit trail). */
export const TASK_EVENT_TYPES = [
  "task_created",
  "status_changed",
  "priority_changed",
  "prompt_appended",
  "context_added",
  "constraint_added",
  "success_criterion_added",
  "non_goal_added",
  "plan_added",
  "subtask_added",
  "subtask_removed",
  "checklist_item_added",
  "checklist_item_toggled",
  "checklist_item_updated",
  "checklist_item_removed",
  "checklist_inherit_changed",
  "message_added",
  "artifact_added",
  "approval_requested",
  "approval_granted",
  "task_completed",
  "task_failed",
  // Execution-cycle audit mirrors. The backend writes these in the same
  // SQL transaction as task_cycles / task_cycle_phases rows so GET
  // /tasks/{id}/events is a complete witness of cycle activity (see
  // pkgs/tasks/domain/enums.go and docs/EXECUTION-CYCLES.md). They land
  // on the timeline as soon as the agent worker dispatches a real task,
  // so omitting them from this allow-list makes parseTaskApi reject the
  // entire /events response with "event type must be a known value" and
  // collapses the Updates section into an error banner.
  "cycle_started",
  "cycle_completed",
  "cycle_failed",
  "phase_started",
  "phase_completed",
  "phase_failed",
  "phase_skipped",
  "sync_ping",
] as const;

export type TaskEventType = (typeof TASK_EVENT_TYPES)[number];

/** One message in the user ↔ agent thread on an event (`response_thread` in API). */
export type TaskEventResponseEntry = {
  /** ISO 8601 from API */
  at: string;
  by: "user" | "agent";
  body: string;
};

export type TaskEvent = {
  seq: number;
  /** ISO 8601 from API */
  at: string;
  type: TaskEventType;
  by: "user" | "agent";
  data: Record<string, unknown>;
  /** Human-submitted text for event types that accept input (`PATCH .../events/{seq}`). */
  user_response?: string;
  /** ISO 8601 when `user_response` was last saved; omitted for legacy rows. */
  user_response_at?: string;
  /** Ordered messages on this event (user and agent); legacy rows may be synthesized server-side. */
  response_thread?: TaskEventResponseEntry[];
};

export type TaskEventsResponse = {
  task_id: string;
  events: TaskEvent[];
  /** From server when using paged `GET /tasks/{id}/events`; omitted on legacy full list. */
  limit?: number;
  total?: number;
  /** 1-based inclusive ranks in newest-first ordering (paged responses). */
  range_start?: number;
  range_end?: number;
  /** False when omitted in JSON (unpaged full list). */
  has_more_newer?: boolean;
  has_more_older?: boolean;
  /** Latest approval request still open (server-computed; not limited to the current page). */
  approval_pending: boolean;
};

/** Single row from `GET /tasks/{id}/events/{seq}` (same shape as one list element plus `task_id`). */
export type TaskEventDetail = TaskEvent & {
  task_id: string;
};

/** One checklist row from GET /tasks/{id}/checklist. */
export type TaskChecklistItemView = {
  id: string;
  sort_order: number;
  text: string;
  done: boolean;
};

export type TaskChecklistResponse = {
  items: TaskChecklistItemView[];
};

export type DraftTaskEvaluationInput = {
  id?: string;
  title: string;
  initial_prompt?: string;
  status?: Status;
  priority?: Priority;
  task_type?: TaskType;
  parent_id?: string;
  checklist_inherit?: boolean;
  checklist_items?: Array<{ text: string }>;
};

export const TASK_TYPES: TaskType[] = [
  "general",
  "bug_fix",
  "feature",
  "refactor",
  "docs",
  "dmap",
];

export const DEFAULT_NEW_TASK_TYPE: TaskType = "general";

export type DraftTaskEvaluationSection = {
  key: string;
  label: string;
  score: number;
  summary: string;
  suggestions: string[];
};

export type DraftTaskEvaluation = {
  evaluation_id: string;
  created_at: string;
  overall_score: number;
  overall_summary: string;
  sections: DraftTaskEvaluationSection[];
  cohesion_score: number;
  cohesion_summary: string;
  cohesion_suggestions: string[];
};

export type TaskDraftPayload = {
  title: string;
  initial_prompt: string;
  priority: PriorityChoice;
  task_type: TaskType;
  /** Omitted in older drafts; defaults from app settings when missing. */
  runner?: string;
  cursor_model?: string;
  parent_id: string;
  checklist_inherit: boolean;
  checklist_items: string[];
  pending_subtasks: Array<{
    title: string;
    initial_prompt: string;
    priority: Priority;
    task_type: TaskType;
    checklist_items: string[];
    checklist_inherit: boolean;
  }>;
  latest_evaluation?: {
    overall_score: number;
    overall_summary: string;
    sections: Array<{ key: string; score: number }>;
  };
  dmap_config?: {
    commit_limit: number;
    domain: string;
    description: string;
  };
};

export type TaskDraftSummary = {
  id: string;
  name: string;
  created_at: string;
  updated_at: string;
};

export type TaskDraftDetail = TaskDraftSummary & {
  payload: TaskDraftPayload;
};
