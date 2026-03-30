import {
  PRIORITIES,
  STATUSES,
  TASK_EVENT_TYPES,
  type Priority,
  type Status,
  type Task,
  type TaskEvent,
  type TaskEventDetail,
  type TaskEventType,
  type TaskEventsResponse,
  type TaskListResponse,
} from "@/types";

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

function parseEventType(v: unknown): TaskEventType {
  if (typeof v !== "string" || !(TASK_EVENT_TYPES as readonly string[]).includes(v)) {
    throw new Error("Invalid API response: event type must be a known value");
  }
  return v as TaskEventType;
}

function parseActor(v: unknown): "user" | "agent" {
  if (v === "user" || v === "agent") return v;
  throw new Error("Invalid API response: event by must be user or agent");
}

function parseEventData(v: unknown): Record<string, unknown> {
  if (v == null) return {};
  if (typeof v !== "object" || Array.isArray(v)) {
    throw new Error("Invalid API response: event data must be an object");
  }
  return v as Record<string, unknown>;
}

function parseTaskEventRecord(item: Record<string, unknown>): TaskEvent {
  const at = parseString(item.at, "at");
  if (Number.isNaN(Date.parse(at))) {
    throw new Error("at must be a parseable date");
  }
  return {
    seq: parseFiniteNumber(item.seq, "seq"),
    at,
    type: parseEventType(item.type),
    by: parseActor(item.by),
    data: parseEventData("data" in item ? item.data : {}),
  };
}

/** Validates GET /tasks/{id}/events JSON. */
export function parseTaskEventsResponse(value: unknown): TaskEventsResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: events payload must be an object");
  }
  const taskID = parseNonEmptyString(value.task_id, "task_id");
  const raw = value.events;
  if (!Array.isArray(raw)) {
    throw new Error("Invalid API response: events must be an array");
  }
  const events: TaskEvent[] = raw.map((item, i) => {
    try {
      if (!isRecord(item)) {
        throw new Error("event must be an object");
      }
      return parseTaskEventRecord(item);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      throw new Error(`Invalid API response: events[${i}]: ${msg}`);
    }
  });
  const approval_pending = value.approval_pending === true;
  const out: TaskEventsResponse = {
    task_id: taskID,
    events,
    approval_pending,
    has_more_newer: value.has_more_newer === true,
    has_more_older: value.has_more_older === true,
  };
  if ("limit" in value && value.limit !== undefined) {
    out.limit = parseFiniteNumber(value.limit, "limit");
  }
  if ("total" in value && value.total !== undefined) {
    out.total = parseFiniteNumber(value.total, "total");
  }
  if ("range_start" in value && value.range_start !== undefined) {
    out.range_start = parseFiniteNumber(value.range_start, "range_start");
  }
  if ("range_end" in value && value.range_end !== undefined) {
    out.range_end = parseFiniteNumber(value.range_end, "range_end");
  }
  return out;
}

/** Validates GET /tasks/{id}/events/{seq} JSON. */
export function parseTaskEventDetail(value: unknown): TaskEventDetail {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: event detail must be an object");
  }
  const task_id = parseNonEmptyString(value.task_id, "task_id");
  try {
    const { seq, at, type, by, data } = parseTaskEventRecord(value);
    return { task_id, seq, at, type, by, data };
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    throw new Error(`Invalid API response: event detail: ${msg}`);
  }
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
