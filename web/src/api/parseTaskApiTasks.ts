import { errorMessage } from "@/lib/errorMessage";
import {
  CYCLE_STATUSES,
  PHASE_STATUSES,
  PHASES,
  PRIORITIES,
  STATUSES,
  type CycleFailuresListResponse,
  type CycleStatus,
  type Phase,
  type PhaseStatus,
  type Priority,
  type Status,
  type Task,
  type TaskChecklistItemView,
  type TaskChecklistResponse,
  type TaskListResponse,
  type TaskStatsCycles,
  type TaskStatsPhases,
  type TaskStatsRecentFailure,
  type TaskStatsResponse,
  type TaskStatsRunner,
  type TaskStatsRunnerBucket,
} from "@/types";
import {
  isRecord,
  parseBooleanField,
  parseFiniteNumber,
  parseNonEmptyString,
  parsePriority,
  parseStatus,
  parseString,
  parseTaskType,
} from "./parseTaskApiCore";

/** Validates JSON from GET /tasks before the UI relies on it. */
export function parseTaskListResponse(value: unknown): TaskListResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: list payload must be an object");
  }
  const rawTasks = value.tasks;
  if (rawTasks === null || rawTasks === undefined) {
    return {
      tasks: [],
      limit: parseFiniteNumber(value.limit, "limit"),
      offset: parseFiniteNumber(value.offset, "offset"),
      has_more: parseBooleanField(value.has_more, "has_more"),
    };
  }
  if (!Array.isArray(rawTasks)) {
    throw new Error("Invalid API response: tasks must be an array");
  }
  const tasks = rawTasks.map((item, i) => {
    try {
      return parseTask(item);
    } catch (e) {
      throw new Error(`Invalid API response: tasks[${i}]: ${errorMessage(e)}`);
    }
  });
  return {
    tasks,
    limit: parseFiniteNumber(value.limit, "limit"),
    offset: parseFiniteNumber(value.offset, "offset"),
    has_more: parseBooleanField(value.has_more, "has_more"),
  };
}

/** Validates GET /tasks/stats JSON. */
export function parseTaskStatsResponse(value: unknown): TaskStatsResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: task stats payload must be an object");
  }
  const byStatusRaw = value.by_status;
  if (!isRecord(byStatusRaw)) {
    throw new Error("Invalid API response: by_status must be an object");
  }
  const byPriorityRaw = value.by_priority;
  if (!isRecord(byPriorityRaw)) {
    throw new Error("Invalid API response: by_priority must be an object");
  }
  const byScopeRaw = value.by_scope;
  if (!isRecord(byScopeRaw)) {
    throw new Error("Invalid API response: by_scope must be an object");
  }
  const by_status: Partial<Record<Status, number>> = {};
  for (const [key, rawCount] of Object.entries(byStatusRaw)) {
    if (!(STATUSES as readonly string[]).includes(key)) {
      throw new Error(`Invalid API response: by_status.${key} is not a known status`);
    }
    by_status[key as Status] = parseFiniteNumber(rawCount, `by_status.${key}`);
  }
  const by_priority: Partial<Record<Priority, number>> = {};
  for (const [key, rawCount] of Object.entries(byPriorityRaw)) {
    if (!(PRIORITIES as readonly string[]).includes(key)) {
      throw new Error(`Invalid API response: by_priority.${key} is not a known priority`);
    }
    by_priority[key as Priority] = parseFiniteNumber(rawCount, `by_priority.${key}`);
  }
  if (!("parent" in byScopeRaw)) {
    throw new Error("Invalid API response: by_scope.parent must be present");
  }
  if (!("subtask" in byScopeRaw)) {
    throw new Error("Invalid API response: by_scope.subtask must be present");
  }
  const by_scope = {
    parent: parseFiniteNumber(byScopeRaw.parent, "by_scope.parent"),
    subtask: parseFiniteNumber(byScopeRaw.subtask, "by_scope.subtask"),
  };
  const cycles = parseTaskStatsCycles(value.cycles);
  const phases = parseTaskStatsPhases(value.phases);
  const runner = parseTaskStatsRunner(value.runner);
  const recent_failures = parseTaskStatsRecentFailures(value.recent_failures);
  return {
    total: parseFiniteNumber(value.total, "total"),
    ready: parseFiniteNumber(value.ready, "ready"),
    critical: parseFiniteNumber(value.critical, "critical"),
    scheduled:
      value.scheduled === undefined
        ? 0
        : parseFiniteNumber(value.scheduled, "scheduled"),
    by_status,
    by_priority,
    by_scope,
    cycles,
    phases,
    runner,
    recent_failures,
  };
}

export function parseCycleFailuresListResponse(
  value: unknown,
): CycleFailuresListResponse {
  if (!isRecord(value)) {
    throw new Error(
      "Invalid API response: cycle failures list payload must be an object",
    );
  }
  return {
    total: parseFiniteNumber(value.total, "total"),
    limit: parseFiniteNumber(value.limit, "limit"),
    offset: parseFiniteNumber(value.offset, "offset"),
    sort: parseString(value.sort, "sort"),
    reason_sort_truncated: parseBooleanField(
      value.reason_sort_truncated,
      "reason_sort_truncated",
    ),
    failures: parseTaskStatsRecentFailures(value.failures),
  };
}

function parseTaskStatsCycles(value: unknown): TaskStatsCycles {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: cycles must be an object");
  }
  const byStatusRaw = value.by_status;
  if (!isRecord(byStatusRaw)) {
    throw new Error("Invalid API response: cycles.by_status must be an object");
  }
  const byTriggeredByRaw = value.by_triggered_by;
  if (!isRecord(byTriggeredByRaw)) {
    throw new Error("Invalid API response: cycles.by_triggered_by must be an object");
  }
  const by_status: Partial<Record<CycleStatus, number>> = {};
  for (const [key, rawCount] of Object.entries(byStatusRaw)) {
    if (!(CYCLE_STATUSES as readonly string[]).includes(key)) {
      throw new Error(`Invalid API response: cycles.by_status.${key} is not a known cycle status`);
    }
    by_status[key as CycleStatus] = parseFiniteNumber(rawCount, `cycles.by_status.${key}`);
  }
  const by_triggered_by: Partial<Record<"user" | "agent", number>> = {};
  for (const [key, rawCount] of Object.entries(byTriggeredByRaw)) {
    if (key !== "user" && key !== "agent") {
      throw new Error(
        `Invalid API response: cycles.by_triggered_by.${key} must be "user" or "agent"`,
      );
    }
    by_triggered_by[key] = parseFiniteNumber(rawCount, `cycles.by_triggered_by.${key}`);
  }
  return { by_status, by_triggered_by };
}

function parseTaskStatsPhases(value: unknown): TaskStatsPhases {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: phases must be an object");
  }
  const byPhaseStatusRaw = value.by_phase_status;
  if (!isRecord(byPhaseStatusRaw)) {
    throw new Error("Invalid API response: phases.by_phase_status must be an object");
  }
  const by_phase_status = {} as TaskStatsPhases["by_phase_status"];
  for (const phase of PHASES) {
    by_phase_status[phase] = {};
  }
  for (const [phaseKey, inner] of Object.entries(byPhaseStatusRaw)) {
    if (!(PHASES as readonly string[]).includes(phaseKey)) {
      throw new Error(
        `Invalid API response: phases.by_phase_status.${phaseKey} is not a known phase`,
      );
    }
    if (!isRecord(inner)) {
      throw new Error(
        `Invalid API response: phases.by_phase_status.${phaseKey} must be an object`,
      );
    }
    const bucket: Partial<Record<PhaseStatus, number>> = {};
    for (const [statusKey, rawCount] of Object.entries(inner)) {
      if (!(PHASE_STATUSES as readonly string[]).includes(statusKey)) {
        throw new Error(
          `Invalid API response: phases.by_phase_status.${phaseKey}.${statusKey} is not a known phase status`,
        );
      }
      bucket[statusKey as PhaseStatus] = parseFiniteNumber(
        rawCount,
        `phases.by_phase_status.${phaseKey}.${statusKey}`,
      );
    }
    by_phase_status[phaseKey as Phase] = bucket;
  }
  return { by_phase_status };
}

function parseTaskStatsRunner(value: unknown): TaskStatsRunner {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: runner must be an object");
  }
  return {
    by_runner: parseRunnerBucketMap(value.by_runner, "runner.by_runner"),
    by_model: parseRunnerBucketMap(value.by_model, "runner.by_model"),
    by_runner_model: parseRunnerBucketMap(value.by_runner_model, "runner.by_runner_model"),
    by_runner_model_resolved:
      value.by_runner_model_resolved === undefined
        ? {}
        : parseRunnerBucketMap(
            value.by_runner_model_resolved,
            "runner.by_runner_model_resolved",
          ),
  };
}

function parseRunnerBucketMap(
  value: unknown,
  field: string,
): Record<string, TaskStatsRunnerBucket> {
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: ${field} must be an object`);
  }
  const out: Record<string, TaskStatsRunnerBucket> = {};
  for (const [key, raw] of Object.entries(value)) {
    out[key] = parseRunnerBucket(raw, `${field}.${key}`);
  }
  return out;
}

function parseRunnerBucket(value: unknown, field: string): TaskStatsRunnerBucket {
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: ${field} must be an object`);
  }
  const byStatusRaw = value.by_status;
  if (!isRecord(byStatusRaw)) {
    throw new Error(`Invalid API response: ${field}.by_status must be an object`);
  }
  const by_status: Partial<Record<CycleStatus, number>> = {};
  for (const [statusKey, rawCount] of Object.entries(byStatusRaw)) {
    if (!(CYCLE_STATUSES as readonly string[]).includes(statusKey)) {
      throw new Error(
        `Invalid API response: ${field}.by_status.${statusKey} is not a known cycle status`,
      );
    }
    by_status[statusKey as CycleStatus] = parseFiniteNumber(
      rawCount,
      `${field}.by_status.${statusKey}`,
    );
  }
  return {
    by_status,
    succeeded: parseFiniteNumber(value.succeeded, `${field}.succeeded`),
    duration_p50_succeeded_seconds: parseFiniteNumber(
      value.duration_p50_succeeded_seconds,
      `${field}.duration_p50_succeeded_seconds`,
    ),
    duration_p95_succeeded_seconds: parseFiniteNumber(
      value.duration_p95_succeeded_seconds,
      `${field}.duration_p95_succeeded_seconds`,
    ),
  };
}

function parseTaskStatsRecentFailures(value: unknown): TaskStatsRecentFailure[] {
  if (!Array.isArray(value)) {
    throw new Error("Invalid API response: recent_failures must be an array");
  }
  return value.map((item, i) => parseTaskStatsRecentFailure(item, i));
}

function parseTaskStatsRecentFailure(
  value: unknown,
  index: number,
): TaskStatsRecentFailure {
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: recent_failures[${index}] must be an object`);
  }
  const status = value.status;
  if (status !== "failed" && status !== "aborted") {
    throw new Error(
      `Invalid API response: recent_failures[${index}].status must be "failed" or "aborted"`,
    );
  }
  return {
    task_id: parseNonEmptyString(value.task_id, `recent_failures[${index}].task_id`),
    event_seq: parseFiniteNumber(value.event_seq, `recent_failures[${index}].event_seq`),
    at: parseNonEmptyString(value.at, `recent_failures[${index}].at`),
    cycle_id: parseNonEmptyString(value.cycle_id, `recent_failures[${index}].cycle_id`),
    attempt_seq: parseFiniteNumber(
      value.attempt_seq,
      `recent_failures[${index}].attempt_seq`,
    ),
    status,
    reason: parseString(value.reason, `recent_failures[${index}].reason`),
  };
}

function parseChecklistInherit(v: unknown): boolean {
  return v === true;
}

function parseOptionalParentId(
  value: unknown,
  field: string,
): string | undefined {
  if (value === undefined || value === null) return undefined;
  const s = parseString(value, field);
  return s.trim() === "" ? undefined : s;
}

export const maxTaskParseDepth = 64;

/** Validates a single task object from POST/PATCH responses (recursive `children`). */
export function parseTask(value: unknown): Task {
  return parseTaskAtDepth(value, 0);
}

function parseTaskAtDepth(value: unknown, depth: number): Task {
  if (depth > maxTaskParseDepth) {
    throw new Error("Invalid API response: task tree is too deep");
  }
  if (!isRecord(value)) {
    throw new Error("Invalid API response: task must be an object");
  }
  const initial =
    value.initial_prompt === undefined
      ? ""
      : parseString(value.initial_prompt, "initial_prompt");
  const base: Task = {
    id: parseNonEmptyString(value.id, "id"),
    title: parseString(value.title, "title"),
    initial_prompt: initial,
    status: parseStatus(value.status),
    priority: parsePriority(value.priority),
    checklist_inherit: parseChecklistInherit(value.checklist_inherit),
    runner:
      value.runner !== undefined && value.runner !== null
        ? parseString(value.runner, "runner")
        : "cursor",
    cursor_model:
      value.cursor_model !== undefined && value.cursor_model !== null
        ? parseString(value.cursor_model, "cursor_model")
        : "",
  };
  if (
    value.pickup_not_before !== undefined &&
    value.pickup_not_before !== null
  ) {
    base.pickup_not_before = parseString(
      value.pickup_not_before,
      "pickup_not_before",
    );
  }
  if ("task_type" in value && value.task_type !== undefined) {
    base.task_type = parseTaskType(value.task_type);
  } else {
    base.task_type = "general";
  }
  const pid = parseOptionalParentId(value.parent_id, "parent_id");
  if (pid !== undefined) {
    base.parent_id = pid;
  }
  const rawChildren = value.children;
  if (rawChildren !== undefined) {
    if (!Array.isArray(rawChildren)) {
      throw new Error("Invalid API response: children must be an array");
    }
    base.children = rawChildren.map((item, i) => {
      try {
        return parseTaskAtDepth(item, depth + 1);
      } catch (e) {
        throw new Error(`Invalid API response: children[${i}]: ${errorMessage(e)}`);
      }
    });
  }
  return base;
}

/** Validates GET /tasks/{id}/checklist JSON. */
export function parseTaskChecklistResponse(value: unknown): TaskChecklistResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: checklist payload must be an object");
  }
  const raw = value.items;
  if (!Array.isArray(raw)) {
    throw new Error("Invalid API response: items must be an array");
  }
  const items: TaskChecklistItemView[] = raw.map((row, i) => {
    if (!isRecord(row)) {
      throw new Error(`Invalid API response: items[${i}] must be an object`);
    }
    return {
      id: parseNonEmptyString(row.id, "id"),
      sort_order: parseFiniteNumber(row.sort_order, "sort_order"),
      text: parseString(row.text, "text"),
      done: row.done === true,
    };
  });
  return { items };
}
