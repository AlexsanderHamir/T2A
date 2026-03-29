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
