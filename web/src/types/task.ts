export type Status =
  | "ready"
  | "running"
  | "blocked"
  | "review"
  | "done"
  | "failed";

export type Priority = "low" | "medium" | "high" | "critical";

export type Task = {
  id: string;
  title: string;
  initial_prompt: string;
  status: Status;
  priority: Priority;
};

export type TaskListResponse = {
  tasks: Task[];
  limit: number;
  offset: number;
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
  "message_added",
  "artifact_added",
  "approval_requested",
  "approval_granted",
  "task_completed",
  "task_failed",
] as const;

export type TaskEventType = (typeof TASK_EVENT_TYPES)[number];

export type TaskEvent = {
  seq: number;
  /** ISO 8601 from API */
  at: string;
  type: TaskEventType;
  by: "user" | "agent";
  data: Record<string, unknown>;
};

export type TaskEventsResponse = {
  task_id: string;
  events: TaskEvent[];
};
