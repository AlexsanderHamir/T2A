import { errorMessage } from "@/lib/errorMessage";
import {
  type CycleMeta,
  type TaskCycle,
  type TaskCycleDetail,
  type TaskCyclePhase,
  type TaskCycleStreamEvent,
  type TaskCycleStreamResponse,
  type TaskCyclesListResponse,
} from "@/types";
import {
  isRecord,
  parseActor,
  parseBooleanField,
  parseCycleStatus,
  parseFiniteNumber,
  parseISO8601Required,
  parseNonEmptyString,
  parseObjectField,
  parseOptionalNonEmptyId,
  parseOptionalParseableDate,
  parsePhase,
  parsePhaseStatus,
  parseString,
} from "./parseTaskApiCore";

/**
 * Validates the typed `cycle_meta` projection introduced in Phase 1b of
 * the per-task runner/model attribution plan. The server always emits
 * the full object (zero-value when the underlying `meta` predates the
 * projection keys), so the parser tolerates a missing `cycle_meta`
 * field — falling back to `meta` when present, then to all-empty
 * strings — for forward and backward compatibility with older API
 * snapshots embedded in tests / fixtures.
 *
 * Empty strings are SEMANTIC and propagated verbatim; see
 * {@link CycleMeta} for the rendering contract.
 */
function parseCycleMeta(value: unknown, meta: Record<string, unknown>): CycleMeta {
  const source = isRecord(value) ? value : meta;
  const stringField = (raw: unknown): string => (typeof raw === "string" ? raw : "");
  return {
    runner: stringField(source.runner),
    runner_version: stringField(source.runner_version),
    cursor_model: stringField(source.cursor_model),
    cursor_model_effective: stringField(source.cursor_model_effective),
    prompt_hash: stringField(source.prompt_hash),
  };
}

/** Validates one cycle row from `GET /tasks/{id}/cycles[*]` (also used by detail). */
export function parseTaskCycle(value: unknown): TaskCycle {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: cycle must be an object");
  }
  const meta = parseObjectField(value.meta, "meta");
  const out: TaskCycle = {
    id: parseNonEmptyString(value.id, "id"),
    task_id: parseNonEmptyString(value.task_id, "task_id"),
    attempt_seq: parseFiniteNumber(value.attempt_seq, "attempt_seq"),
    status: parseCycleStatus(value.status),
    started_at: parseISO8601Required(value.started_at, "started_at"),
    triggered_by: parseActor(value.triggered_by),
    meta,
    cycle_meta: parseCycleMeta(value.cycle_meta, meta),
  };
  const ended = parseOptionalParseableDate(value.ended_at, "ended_at");
  if (ended !== undefined) out.ended_at = ended;
  const parent = parseOptionalNonEmptyId(value.parent_cycle_id, "parent_cycle_id");
  if (parent !== undefined) out.parent_cycle_id = parent;
  return out;
}

/** Validates one phase row inside a cycle detail or PATCH/POST phase response. */
export function parseTaskCyclePhase(value: unknown): TaskCyclePhase {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: phase must be an object");
  }
  const out: TaskCyclePhase = {
    id: parseNonEmptyString(value.id, "id"),
    cycle_id: parseNonEmptyString(value.cycle_id, "cycle_id"),
    phase: parsePhase(value.phase),
    phase_seq: parseFiniteNumber(value.phase_seq, "phase_seq"),
    status: parsePhaseStatus(value.status),
    started_at: parseISO8601Required(value.started_at, "started_at"),
    details: parseObjectField(value.details, "details"),
  };
  const ended = parseOptionalParseableDate(value.ended_at, "ended_at");
  if (ended !== undefined) out.ended_at = ended;
  if (value.summary !== undefined && value.summary !== null) {
    out.summary = parseString(value.summary, "summary");
  }
  if (value.event_seq !== undefined && value.event_seq !== null) {
    out.event_seq = parseFiniteNumber(value.event_seq, "event_seq");
  }
  return out;
}

/** Validates `GET /tasks/{id}/cycles` envelope. */
export function parseTaskCyclesListResponse(
  value: unknown,
): TaskCyclesListResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: cycles payload must be an object");
  }
  const raw = value.cycles;
  if (!Array.isArray(raw)) {
    throw new Error("Invalid API response: cycles must be an array");
  }
  const cycles = raw.map((item, i) => {
    try {
      return parseTaskCycle(item);
    } catch (e) {
      throw new Error(`Invalid API response: cycles[${i}]: ${errorMessage(e)}`);
    }
  });
  return {
    task_id: parseNonEmptyString(value.task_id, "task_id"),
    cycles,
    limit: parseFiniteNumber(value.limit, "limit"),
    has_more: parseBooleanField(value.has_more, "has_more"),
  };
}

/** Validates `GET /tasks/{id}/cycles/{cycleId}` envelope (cycle row + ordered phases). */
export function parseTaskCycleDetail(value: unknown): TaskCycleDetail {
  const cycle = parseTaskCycle(value);
  if (!isRecord(value)) {
    throw new Error("Invalid API response: cycle detail must be an object");
  }
  const raw = value.phases;
  if (!Array.isArray(raw)) {
    throw new Error("Invalid API response: phases must be an array");
  }
  const phases = raw.map((item, i) => {
    try {
      return parseTaskCyclePhase(item);
    } catch (e) {
      throw new Error(`Invalid API response: phases[${i}]: ${errorMessage(e)}`);
    }
  });
  return { ...cycle, phases };
}

export function parseTaskCycleStreamEvent(value: unknown): TaskCycleStreamEvent {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: stream event must be an object");
  }
  const out: TaskCycleStreamEvent = {
    id: parseNonEmptyString(value.id, "id"),
    task_id: parseNonEmptyString(value.task_id, "task_id"),
    cycle_id: parseNonEmptyString(value.cycle_id, "cycle_id"),
    phase_seq: parseFiniteNumber(value.phase_seq, "phase_seq"),
    stream_seq: parseFiniteNumber(value.stream_seq, "stream_seq"),
    at: parseISO8601Required(value.at, "at"),
    source: parseNonEmptyString(value.source, "source"),
    kind: parseNonEmptyString(value.kind, "kind"),
    payload: parseObjectField(value.payload, "payload"),
  };
  if (value.subtype !== undefined && value.subtype !== null) {
    out.subtype = parseString(value.subtype, "subtype");
  }
  if (value.message !== undefined && value.message !== null) {
    out.message = parseString(value.message, "message");
  }
  if (value.tool !== undefined && value.tool !== null) {
    out.tool = parseString(value.tool, "tool");
  }
  return out;
}

export function parseTaskCycleStreamResponse(
  value: unknown,
): TaskCycleStreamResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: cycle stream payload must be an object");
  }
  const raw = value.events;
  if (!Array.isArray(raw)) {
    throw new Error("Invalid API response: events must be an array");
  }
  const events = raw.map((item, i) => {
    try {
      return parseTaskCycleStreamEvent(item);
    } catch (e) {
      throw new Error(`Invalid API response: events[${i}]: ${errorMessage(e)}`);
    }
  });
  const out: TaskCycleStreamResponse = {
    task_id: parseNonEmptyString(value.task_id, "task_id"),
    cycle_id: parseNonEmptyString(value.cycle_id, "cycle_id"),
    events,
    limit: parseFiniteNumber(value.limit, "limit"),
    has_more: parseBooleanField(value.has_more, "has_more"),
  };
  if (value.next_after_seq !== undefined && value.next_after_seq !== null) {
    out.next_after_seq = parseFiniteNumber(value.next_after_seq, "next_after_seq");
  }
  return out;
}
