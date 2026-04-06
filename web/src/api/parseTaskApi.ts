import {
  PRIORITIES,
  TASK_TYPES,
  STATUSES,
  TASK_EVENT_TYPES,
  type Priority,
  type DraftTaskEvaluation,
  type TaskDraftDetail,
  type TaskDraftSummary,
  type TaskDraftPayload,
  type Status,
  type Task,
  type TaskType,
  type TaskChecklistItemView,
  type TaskChecklistResponse,
  type TaskEvent,
  type TaskEventDetail,
  type TaskEventResponseEntry,
  type TaskEventType,
  type TaskEventsResponse,
  type TaskListResponse,
  type TaskStatsResponse,
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

function parseTaskType(v: unknown): TaskType {
  if (typeof v !== "string" || !(TASK_TYPES as readonly string[]).includes(v)) {
    throw new Error("Invalid API response: task_type must be a known task type");
  }
  return v as TaskType;
}

function parseFiniteNumber(v: unknown, field: string): number {
  if (typeof v !== "number" || !Number.isFinite(v)) {
    throw new Error(`Invalid API response: ${field} must be a number`);
  }
  return v;
}

function parseBooleanField(v: unknown, field: string): boolean {
  if (v === undefined || v === null) {
    return false;
  }
  if (typeof v === "boolean") {
    return v;
  }
  throw new Error(`Invalid API response: ${field} must be a boolean`);
}

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
      const msg = e instanceof Error ? e.message : String(e);
      throw new Error(`Invalid API response: tasks[${i}]: ${msg}`);
    }
  });
  return {
    tasks,
    limit: parseFiniteNumber(value.limit, "limit"),
    offset: parseFiniteNumber(value.offset, "offset"),
    has_more: parseBooleanField(value.has_more, "has_more"),
  };
}

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
  return {
    total: parseFiniteNumber(value.total, "total"),
    ready: parseFiniteNumber(value.ready, "ready"),
    critical: parseFiniteNumber(value.critical, "critical"),
    by_status,
    by_priority,
    by_scope,
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

function parseOptionalUserResponse(
  value: unknown,
  field: string,
): string | undefined {
  if (value === undefined) return undefined;
  if (value === null) return undefined;
  if (typeof value !== "string") {
    throw new Error(
      `Invalid API response: ${field} must be a string, null, or omitted`,
    );
  }
  return value;
}

function parseOptionalISO8601(
  value: unknown,
  field: string,
): string | undefined {
  if (value === undefined) return undefined;
  if (value === null) return undefined;
  if (typeof value !== "string") {
    throw new Error(
      `Invalid API response: ${field} must be a string, null, or omitted`,
    );
  }
  if (Number.isNaN(Date.parse(value))) {
    throw new Error(`Invalid API response: ${field} must be a parseable date`);
  }
  return value;
}

function parseResponseThreadEntries(
  value: unknown,
  fieldPrefix: string,
): TaskEventResponseEntry[] | undefined {
  if (value === undefined) return undefined;
  if (value === null) return undefined;
  if (!Array.isArray(value)) {
    throw new Error(
      `Invalid API response: ${fieldPrefix} must be an array or omitted`,
    );
  }
  const out: TaskEventResponseEntry[] = [];
  for (let i = 0; i < value.length; i++) {
    const row = value[i];
    const p = `${fieldPrefix}[${i}]`;
    if (!isRecord(row)) {
      throw new Error(`Invalid API response: ${p} must be an object`);
    }
    const at = parseString(row.at, `${p}.at`);
    if (Number.isNaN(Date.parse(at))) {
      throw new Error(`Invalid API response: ${p}.at must be a parseable date`);
    }
    out.push({
      at,
      by: parseActor(row.by),
      body: parseString(row.body, `${p}.body`),
    });
  }
  return out.length > 0 ? out : undefined;
}

function parseTaskEventRecord(item: Record<string, unknown>): TaskEvent {
  const at = parseString(item.at, "at");
  if (Number.isNaN(Date.parse(at))) {
    throw new Error("at must be a parseable date");
  }
  const base: TaskEvent = {
    seq: parseFiniteNumber(item.seq, "seq"),
    at,
    type: parseEventType(item.type),
    by: parseActor(item.by),
    data: parseEventData("data" in item ? item.data : {}),
  };
  const ur = parseOptionalUserResponse(item.user_response, "user_response");
  if (ur !== undefined) {
    base.user_response = ur;
  }
  const urAt = parseOptionalISO8601(
    item.user_response_at,
    "user_response_at",
  );
  if (urAt !== undefined) {
    base.user_response_at = urAt;
  }
  const rt = parseResponseThreadEntries(
    item.response_thread,
    "response_thread",
  );
  if (rt !== undefined) {
    base.response_thread = rt;
  }
  const urTrimmed = base.user_response?.trim();
  if (!base.response_thread?.length && urTrimmed) {
    base.response_thread = [
      {
        at: base.user_response_at ?? base.at,
        by: "user",
        body: urTrimmed,
      },
    ];
  }
  return base;
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
    const ev = parseTaskEventRecord(value);
    return { task_id, ...ev };
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    throw new Error(`Invalid API response: event detail: ${msg}`);
  }
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

/**
 * Maximum nesting depth for `children` when parsing task trees. Deeper payloads
 * are rejected to avoid stack overflows on pathological or hostile responses.
 */
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
  };
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
        const msg = e instanceof Error ? e.message : String(e);
        throw new Error(`Invalid API response: children[${i}]: ${msg}`);
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

/** Validates POST /tasks/evaluate JSON. */
export function parseDraftTaskEvaluation(value: unknown): DraftTaskEvaluation {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: draft evaluation must be an object");
  }
  const sectionsRaw = value.sections;
  if (!Array.isArray(sectionsRaw)) {
    throw new Error("Invalid API response: sections must be an array");
  }
  const sections = sectionsRaw.map((row, i) => {
    if (!isRecord(row)) {
      throw new Error(`Invalid API response: sections[${i}] must be an object`);
    }
    const suggestionsRaw = row.suggestions;
    if (!Array.isArray(suggestionsRaw)) {
      throw new Error(
        `Invalid API response: sections[${i}].suggestions must be an array`,
      );
    }
    return {
      key: parseNonEmptyString(row.key, `sections[${i}].key`),
      label: parseString(row.label, `sections[${i}].label`),
      score: parseFiniteNumber(row.score, `sections[${i}].score`),
      summary: parseString(row.summary, `sections[${i}].summary`),
      suggestions: suggestionsRaw.map((s, j) =>
        parseString(s, `sections[${i}].suggestions[${j}]`),
      ),
    };
  });
  const cohesionSuggestionsRaw = value.cohesion_suggestions;
  if (!Array.isArray(cohesionSuggestionsRaw)) {
    throw new Error(
      "Invalid API response: cohesion_suggestions must be an array",
    );
  }
  const createdAt = parseString(value.created_at, "created_at");
  if (Number.isNaN(Date.parse(createdAt))) {
    throw new Error("Invalid API response: created_at must be a parseable date");
  }
  return {
    evaluation_id: parseNonEmptyString(value.evaluation_id, "evaluation_id"),
    created_at: createdAt,
    overall_score: parseFiniteNumber(value.overall_score, "overall_score"),
    overall_summary: parseString(value.overall_summary, "overall_summary"),
    sections,
    cohesion_score: parseFiniteNumber(value.cohesion_score, "cohesion_score"),
    cohesion_summary: parseString(value.cohesion_summary, "cohesion_summary"),
    cohesion_suggestions: cohesionSuggestionsRaw.map((s, i) =>
      parseString(s, `cohesion_suggestions[${i}]`),
    ),
  };
}

function parseDraftPayload(value: unknown): TaskDraftPayload {
  if (!isRecord(value)) throw new Error("Invalid API response: payload must be object");
  const checklistRaw = value.checklist_items;
  if (!Array.isArray(checklistRaw)) {
    throw new Error("Invalid API response: payload.checklist_items must be array");
  }
  const subtasksRaw = value.pending_subtasks;
  if (!Array.isArray(subtasksRaw)) {
    throw new Error("Invalid API response: payload.pending_subtasks must be array");
  }
  return {
    title: parseString(value.title, "payload.title"),
    initial_prompt: parseString(value.initial_prompt, "payload.initial_prompt"),
    priority: (value.priority as TaskDraftPayload["priority"]) ?? "",
    task_type: parseTaskType(value.task_type ?? "general"),
    parent_id: parseString(value.parent_id ?? "", "payload.parent_id"),
    checklist_inherit: value.checklist_inherit === true,
    checklist_items: checklistRaw.map((s, i) => parseString(s, `payload.checklist_items[${i}]`)),
    pending_subtasks: subtasksRaw.map((s, i) => {
      if (!isRecord(s)) throw new Error(`Invalid API response: payload.pending_subtasks[${i}] must be object`);
      const sChecklist = s.checklist_items;
      if (!Array.isArray(sChecklist)) throw new Error(`Invalid API response: payload.pending_subtasks[${i}].checklist_items must be array`);
      return {
        title: parseString(s.title, `payload.pending_subtasks[${i}].title`),
        initial_prompt: parseString(
          s.initial_prompt,
          `payload.pending_subtasks[${i}].initial_prompt`,
        ),
        priority: parsePriority(s.priority),
        task_type: parseTaskType(s.task_type ?? "general"),
        checklist_items: sChecklist.map((x, j) =>
          parseString(x, `payload.pending_subtasks[${i}].checklist_items[${j}]`),
        ),
        checklist_inherit: s.checklist_inherit === true,
      };
    }),
    ...(isRecord(value.latest_evaluation)
      ? {
          latest_evaluation: {
            overall_score: parseFiniteNumber(
              value.latest_evaluation.overall_score,
              "payload.latest_evaluation.overall_score",
            ),
            overall_summary: parseString(
              value.latest_evaluation.overall_summary,
              "payload.latest_evaluation.overall_summary",
            ),
            sections: Array.isArray(value.latest_evaluation.sections)
              ? value.latest_evaluation.sections
                  .filter((s): s is Record<string, unknown> => isRecord(s))
                  .map((s) => ({
                    key: parseString(s.key, "payload.latest_evaluation.sections[].key"),
                    score: parseFiniteNumber(
                      s.score,
                      "payload.latest_evaluation.sections[].score",
                    ),
                  }))
              : [],
          },
        }
      : {}),
  };
}

export function parseTaskDraftSummaryList(value: unknown): TaskDraftSummary[] {
  if (!isRecord(value)) throw new Error("Invalid API response: draft list must be object");
  const raw = value.drafts;
  if (!Array.isArray(raw)) throw new Error("Invalid API response: drafts must be array");
  return raw.map((item, i) => {
    if (!isRecord(item)) throw new Error(`Invalid API response: drafts[${i}] must be object`);
    const created = parseString(item.created_at, `drafts[${i}].created_at`);
    const updated = parseString(item.updated_at, `drafts[${i}].updated_at`);
    return {
      id: parseNonEmptyString(item.id, `drafts[${i}].id`),
      name: parseString(item.name, `drafts[${i}].name`),
      created_at: created,
      updated_at: updated,
    };
  });
}

export function parseTaskDraftDetail(value: unknown): TaskDraftDetail {
  if (!isRecord(value)) throw new Error("Invalid API response: draft detail must be object");
  return {
    id: parseNonEmptyString(value.id, "id"),
    name: parseString(value.name, "name"),
    created_at: parseString(value.created_at, "created_at"),
    updated_at: parseString(value.updated_at, "updated_at"),
    payload: parseDraftPayload(value.payload),
  };
}
