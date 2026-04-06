export type Status =
  | "ready"
  | "running"
  | "blocked"
  | "review"
  | "done"
  | "failed";

export type Priority = "low" | "medium" | "high" | "critical";
export type TaskType = "general" | "bug_fix" | "feature" | "refactor" | "docs";

/** Empty string means no selection yet (create / draft forms). */
export type PriorityChoice = Priority | "";

export type Task = {
  id: string;
  title: string;
  initial_prompt: string;
  status: Status;
  priority: Priority;
  task_type?: TaskType;
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
  /** True when the server may have more root tasks (see GET /tasks in docs/DESIGN.md). */
  has_more: boolean;
};

export type TaskChangeType =
  | "task_created"
  | "task_updated"
  | "task_deleted";

export type TaskChangeEvent = {
  type: TaskChangeType;
  id: string;
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
