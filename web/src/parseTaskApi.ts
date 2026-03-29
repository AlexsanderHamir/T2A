import { PRIORITIES, STATUSES, type Priority, type Status, type Task, type TaskListResponse } from "./types";

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function parseNonEmptyString(v: unknown, field: string): string {
  if (typeof v !== "string" || !v.trim()) {
    throw new Error(`Invalid API response: ${field} must be a non-empty string`);
  }
  return v;
}

function parseString(v: unknown, field: string): string {
  if (typeof v !== "string") {
    throw new Error(`Invalid API response: ${field} must be a string`);
  }
  return v;
}

function parseStatus(v: unknown): Status {
  if (typeof v !== "string" || !(STATUSES as readonly string[]).includes(v)) {
    throw new Error("Invalid API response: status must be a known task status");
  }
  return v as Status;
}

function parsePriority(v: unknown): Priority {
  if (typeof v !== "string" || !(PRIORITIES as readonly string[]).includes(v)) {
    throw new Error("Invalid API response: priority must be a known task priority");
  }
  return v as Priority;
}

function parseFiniteNumber(v: unknown, field: string): number {
  if (typeof v !== "number" || !Number.isFinite(v)) {
    throw new Error(`Invalid API response: ${field} must be a number`);
  }
  return v;
}

/** Validates JSON from GET /tasks before the UI relies on it. */
export function parseTaskListResponse(value: unknown): TaskListResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: list payload must be an object");
  }
  const rawTasks = value.tasks;
  if (!Array.isArray(rawTasks)) {
    throw new Error("Invalid API response: tasks must be an array");
  }
  const tasks = rawTasks.map((item, i) => {
    try {
      return parseTask(item);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      throw new Error(`Invalid API response: tasks[${i}]: ${msg}`);
    }
  });
  return {
    tasks,
    limit: parseFiniteNumber(value.limit, "limit"),
    offset: parseFiniteNumber(value.offset, "offset"),
  };
}

/** Validates a single task object from POST/PATCH responses. */
export function parseTask(value: unknown): Task {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: task must be an object");
  }
  const initial =
    value.initial_prompt === undefined
      ? ""
      : parseString(value.initial_prompt, "initial_prompt");
  return {
    id: parseNonEmptyString(value.id, "id"),
    title: parseString(value.title, "title"),
    initial_prompt: initial,
    status: parseStatus(value.status),
    priority: parsePriority(value.priority),
  };
}
