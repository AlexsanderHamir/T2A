import {
  CYCLE_STATUSES,
  LEGACY_PHASES,
  PHASE_STATUSES,
  PHASES,
  PRIORITIES,
  STATUSES,
  TASK_EVENT_TYPES,
  type CycleStatus,
  type Phase,
  type PhaseStatus,
  type Priority,
  type Status,
  type TaskEventType,
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

// Accept legacy phase values on read so historical cycle rows that
// predate the diagnose/persist trim still render in the SPA. The
// write-side enum (PHASES) is unchanged — only execute/verify can be
// started by new POSTs.
const READABLE_PHASES: readonly string[] = [
  ...(PHASES as readonly string[]),
  ...(LEGACY_PHASES as readonly string[]),
];

export function parsePhase(v: unknown): Phase {
  if (typeof v !== "string" || !READABLE_PHASES.includes(v)) {
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
  if (typeof v === "string" && !v.trim()) return undefined;
  return parseNonEmptyString(v, field);
}

export type NamedEntitySummary = {
  id: string;
  name: string;
  created_at: string;
  updated_at: string;
};

/** Validates list responses shaped like `{ drafts: [...] }` or `{ templates: [...] }`. */
export function parseNamedEntitySummaryList(
  value: unknown,
  arrayKey: string,
  entityLabel: string,
): NamedEntitySummary[] {
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: ${entityLabel} list must be object`);
  }
  const raw = value[arrayKey];
  if (!Array.isArray(raw)) {
    throw new Error(`Invalid API response: ${arrayKey} must be array`);
  }
  return raw.map((item, i) => {
    if (!isRecord(item)) {
      throw new Error(`Invalid API response: ${arrayKey}[${i}] must be object`);
    }
    return {
      id: parseNonEmptyString(item.id, `${arrayKey}[${i}].id`),
      name: parseString(item.name, `${arrayKey}[${i}].name`),
      created_at: parseString(item.created_at, `${arrayKey}[${i}].created_at`),
      updated_at: parseString(item.updated_at, `${arrayKey}[${i}].updated_at`),
    };
  });
}
