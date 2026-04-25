import {
  CYCLE_STATUSES,
  PHASE_STATUSES,
  PHASES,
  PRIORITIES,
  STATUSES,
  TASK_EVENT_TYPES,
  TASK_TYPES,
  type CycleStatus,
  type Phase,
  type PhaseStatus,
  type Priority,
  type Status,
  type TaskEventType,
  type TaskType,
} from "@/types";

export function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

export function parseNonEmptyString(v: unknown, field: string): string {
  if (typeof v !== "string" || !v.trim()) {
    throw new Error(`Invalid API response: ${field} must be a non-empty string`);
  }
  return v;
}

export function parseString(v: unknown, field: string): string {
  if (typeof v !== "string") {
    throw new Error(`Invalid API response: ${field} must be a string`);
  }
  return v;
}

export function parseStatus(v: unknown): Status {
  if (typeof v !== "string" || !(STATUSES as readonly string[]).includes(v)) {
    throw new Error("Invalid API response: status must be a known task status");
  }
  return v as Status;
}

export function parsePriority(v: unknown): Priority {
  if (typeof v !== "string" || !(PRIORITIES as readonly string[]).includes(v)) {
    throw new Error("Invalid API response: priority must be a known task priority");
  }
  return v as Priority;
}

export function parsePriorityChoice(v: unknown): Priority | "" {
  if (v === undefined || v === null || v === "") return "";
  return parsePriority(v);
}

export function parseTaskType(v: unknown): TaskType {
  if (typeof v !== "string" || !(TASK_TYPES as readonly string[]).includes(v)) {
    throw new Error("Invalid API response: task_type must be a known task type");
  }
  return v as TaskType;
}

export function parseFiniteNumber(v: unknown, field: string): number {
  if (typeof v !== "number" || !Number.isFinite(v)) {
    throw new Error(`Invalid API response: ${field} must be a number`);
  }
  return v;
}

export function parseBooleanField(v: unknown, field: string): boolean {
  if (v === undefined || v === null) {
    return false;
  }
  if (typeof v === "boolean") {
    return v;
  }
  throw new Error(`Invalid API response: ${field} must be a boolean`);
}

export function parseCycleStatus(v: unknown): CycleStatus {
  if (typeof v !== "string" || !(CYCLE_STATUSES as readonly string[]).includes(v)) {
    throw new Error("Invalid API response: status must be a known cycle status");
  }
  return v as CycleStatus;
}

export function parsePhase(v: unknown): Phase {
  if (typeof v !== "string" || !(PHASES as readonly string[]).includes(v)) {
    throw new Error("Invalid API response: phase must be a known phase");
  }
  return v as Phase;
}

export function parsePhaseStatus(v: unknown): PhaseStatus {
  if (typeof v !== "string" || !(PHASE_STATUSES as readonly string[]).includes(v)) {
    throw new Error("Invalid API response: status must be a known phase status");
  }
  return v as PhaseStatus;
}

export function parseActor(v: unknown): "user" | "agent" {
  if (v === "user" || v === "agent") return v;
  throw new Error("Invalid API response: event by must be user or agent");
}

export function parseEventType(v: unknown): TaskEventType {
  if (typeof v !== "string" || !(TASK_EVENT_TYPES as readonly string[]).includes(v)) {
    throw new Error("Invalid API response: event type must be a known value");
  }
  return v as TaskEventType;
}

export function parseObjectField(v: unknown, field: string): Record<string, unknown> {
  if (v === undefined || v === null) return {};
  if (!isRecord(v)) {
    throw new Error(`Invalid API response: ${field} must be an object`);
  }
  return v;
}

export function parseISO8601Required(v: unknown, field: string): string {
  const s = parseString(v, field);
  if (Number.isNaN(Date.parse(s))) {
    throw new Error(`Invalid API response: ${field} must be a parseable date`);
  }
  return s;
}

export function parseOptionalParseableDate(
  v: unknown,
  field: string,
): string | undefined {
  if (v === undefined || v === null) return undefined;
  return parseISO8601Required(v, field);
}

export function parseOptionalNonEmptyId(
  v: unknown,
  field: string,
): string | undefined {
  if (v === undefined || v === null) return undefined;
  return parseNonEmptyString(v, field);
}
