import {
  DEFAULT_NEW_TASK_STATUS,
  type Priority,
  type DraftTaskEvaluation,
  type DraftTaskEvaluationInput,
  type Status,
  type Task,
  type TaskDraftDetail,
  type TaskDraftPayload,
  type TaskDraftSummary,
  type TaskChecklistResponse,
  type TaskEventDetail,
  type TaskEventsResponse,
  type TaskListResponse,
  type TaskStatsResponse,
  type CycleFailuresListResponse,
  type TaskDependencyEdge,
  type TaskGate,
} from "@/types";
import { parseNonEmptyString } from "./parseTaskApiCore";
import {
  parseTask,
  parseTaskDraftDetail,
  parseTaskDraftSummaryList,
  parseDraftTaskEvaluation,
  parseTaskChecklistResponse,
  parseTaskEventDetail,
  parseTaskEventsResponse,
  parseTaskListResponse,
  parseTaskStatsResponse,
  parseCycleFailuresListResponse,
} from "./parseTaskApi";
import { fetchWithTimeout, jsonHeaders, apiErrorFromResponse } from "./shared";
import {
  assertAfterId,
  assertListIntQuery,
  assertNonNegativeOffset,
  assertOptionalTaskPathId,
  assertPositiveSeq,
  assertTaskPathId,
} from "./taskRequestBounds";
import { TASK_DRAFTS } from "@/constants/tasks";

/** Matches `GET /tasks/cycle-failures` `sort` query (store cycle failure sorts). */
const CYCLE_FAILURE_SORTS = [
  "at_desc",
  "at_asc",
  "reason_asc",
  "reason_desc",
] as const;

function assertCycleFailureSort(sort: string): (typeof CYCLE_FAILURE_SORTS)[number] {
  if (!(CYCLE_FAILURE_SORTS as readonly string[]).includes(sort)) {
    throw new Error(`sort must be one of: ${CYCLE_FAILURE_SORTS.join(", ")}`);
  }
  return sort as (typeof CYCLE_FAILURE_SORTS)[number];
}

export {
  maxListAfterIDParamBytes,
  maxListIntQueryParamBytes,
  maxTaskPathIDBytes,
  maxTaskSeqPathOrQueryParamBytes,
} from "./taskRequestBounds";

export async function getTask(
  id: string,
  options?: { signal?: AbortSignal },
): Promise<Task> {
  const tid = assertTaskPathId(id);
  const res = await fetchWithTimeout(`/tasks/${encodeURIComponent(tid)}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTask(raw);
}

export async function listTaskEvents(
  id: string,
  options?: {
    signal?: AbortSignal;
    limit?: number;
    beforeSeq?: number;
    afterSeq?: number;
  },
): Promise<TaskEventsResponse> {
  const tid = assertTaskPathId(id);
  const q = new URLSearchParams();
  if (options?.limit !== undefined) {
    q.set("limit", assertListIntQuery("limit", options.limit, 0, 200));
  }
  if (options?.beforeSeq !== undefined) {
    q.set("before_seq", assertPositiveSeq("before_seq", options.beforeSeq));
  }
  if (options?.afterSeq !== undefined) {
    q.set("after_seq", assertPositiveSeq("after_seq", options.afterSeq));
  }
  const qs = q.toString();
  const path =
    qs === ""
      ? `/tasks/${encodeURIComponent(tid)}/events`
      : `/tasks/${encodeURIComponent(tid)}/events?${qs}`;
  const res = await fetchWithTimeout(path, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTaskEventsResponse(raw);
}

export async function getTaskEvent(
  taskId: string,
  seq: number,
  options?: { signal?: AbortSignal },
): Promise<TaskEventDetail> {
  const tid = assertTaskPathId(taskId, "task id");
  const seqStr = assertPositiveSeq("seq", seq);
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/events/${encodeURIComponent(seqStr)}`,
    {
      headers: { Accept: "application/json" },
      signal: options?.signal,
    },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTaskEventDetail(raw);
}

export async function patchTaskEventUserResponse(
  taskId: string,
  seq: number,
  userResponse: string,
  options?: { actor?: "user" | "agent" },
): Promise<TaskEventDetail> {
  const tid = assertTaskPathId(taskId, "task id");
  const seqStr = assertPositiveSeq("seq", seq);
  const headers: Record<string, string> = { ...jsonHeaders };
  if (options?.actor === "agent") {
    headers["X-Actor"] = "agent";
  }
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/events/${encodeURIComponent(seqStr)}`,
    {
      method: "PATCH",
      headers,
      body: JSON.stringify({ user_response: userResponse }),
    },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTaskEventDetail(raw);
}

export async function listTasks(
  limit = 200,
  offset = 0,
  options?: { signal?: AbortSignal; afterId?: string },
): Promise<TaskListResponse> {
  const lim = assertListIntQuery("limit", limit, 0, 200);
  const q = new URLSearchParams({ limit: lim });
  if (options?.afterId) {
    q.set("after_id", assertAfterId(options.afterId));
  } else {
    q.set("offset", assertNonNegativeOffset("offset", offset));
  }
  const res = await fetchWithTimeout(`/tasks?${q}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTaskListResponse(raw);
}

export async function getTaskStats(
  options?: { signal?: AbortSignal },
): Promise<TaskStatsResponse> {
  const res = await fetchWithTimeout("/tasks/stats", {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTaskStatsResponse(raw);
}

/**
 * Paginated `cycle_failed` events (`GET /tasks/cycle-failures`).
 * Default server behaviour is newest-first (`at_desc`, limit 50).
 */
export async function getCycleFailures(options: {
  signal?: AbortSignal;
  limit?: number;
  offset?: number;
  sort?: string;
}): Promise<CycleFailuresListResponse> {
  const limitStr =
    options.limit === undefined
      ? "50"
      : assertListIntQuery("limit", options.limit, 1, 200);
  const offsetStr =
    options.offset === undefined
      ? "0"
      : assertNonNegativeOffset("offset", options.offset);
  const sort =
    options.sort === undefined || options.sort === ""
      ? "at_desc"
      : assertCycleFailureSort(options.sort.trim());
  const q = new URLSearchParams({
    limit: limitStr,
    offset: offsetStr,
    sort,
  });
  const res = await fetchWithTimeout(`/tasks/cycle-failures?${q}`, {
    headers: { Accept: "application/json" },
    signal: options.signal,
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseCycleFailuresListResponse(raw);
}

export async function createTask(input: {
  title: string;
  initial_prompt?: string;
  status?: Status;
  priority: Priority;
  id?: string;
  draft_id?: string;
  project_id?: string;
  project_context_item_ids?: string[];
  runner?: string;
  cursor_model?: string;
  /**
   * Optional RFC3339 UTC instant for the agent worker's earliest pickup.
   * Omit (or pass `undefined`) to keep the existing global-delay
   * behaviour. The server rejects empty strings on create — to "no
   * schedule" the task, just omit the field.
   * See docs/data-model.md.
   */
  pickup_not_before?: string;
  tags?: string[];
  milestone?: string;
  gate?: TaskGate;
  depends_on?: TaskDependencyEdge[];
}): Promise<Task> {
  const body: Record<string, unknown> = {
    title: input.title,
    initial_prompt: input.initial_prompt ?? "",
    status: input.status ?? DEFAULT_NEW_TASK_STATUS,
    priority: input.priority,
  };
  if (input.runner !== undefined) {
    body.runner = input.runner;
  }
  if (input.cursor_model !== undefined) {
    body.cursor_model = input.cursor_model;
  }
  const cid = assertOptionalTaskPathId(input.id, "id");
  if (cid !== undefined) {
    body.id = cid;
  }
  const draftId = assertOptionalTaskPathId(input.draft_id, "draft_id");
  if (draftId !== undefined) {
    body.draft_id = draftId;
  }
  const projectId = assertOptionalTaskPathId(input.project_id, "project_id");
  if (projectId !== undefined) {
    body.project_id = projectId;
  }
  if (input.project_context_item_ids !== undefined) {
    body.project_context_item_ids = input.project_context_item_ids;
  }
  if (input.pickup_not_before !== undefined) {
    body.pickup_not_before = input.pickup_not_before;
  }
  if (input.tags !== undefined) {
    body.tags = input.tags;
  }
  if (input.milestone !== undefined) {
    body.milestone = input.milestone;
  }
  if (input.gate !== undefined) {
    body.gate = input.gate;
  }
  if (input.depends_on !== undefined) {
    body.depends_on = input.depends_on;
  }
  const res = await fetchWithTimeout("/tasks", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(body),
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTask(raw);
}

export async function evaluateDraftTask(
  input: DraftTaskEvaluationInput,
): Promise<DraftTaskEvaluation> {
  const payload: Record<string, unknown> = {
    title: input.title,
    initial_prompt: input.initial_prompt ?? "",
  };
  const eid = assertOptionalTaskPathId(input.id, "id");
  if (eid !== undefined) {
    payload.id = eid;
  }
  if (input.status) {
    payload.status = input.status;
  }
  if (input.priority) {
    payload.priority = input.priority;
  }
  if (input.checklist_items) {
    payload.checklist_items = input.checklist_items;
  }
  const res = await fetchWithTimeout("/tasks/evaluate", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseDraftTaskEvaluation(raw);
}

export async function listTaskDrafts(
  limit = TASK_DRAFTS.draftsPageDefaultLimit,
  options?: { signal?: AbortSignal },
): Promise<TaskDraftSummary[]> {
  const lim = assertListIntQuery("limit", limit, 0, 100);
  const res = await fetchWithTimeout(`/task-drafts?limit=${encodeURIComponent(lim)}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTaskDraftSummaryList(raw);
}

export async function saveTaskDraft(input: {
  id?: string;
  name: string;
  payload: TaskDraftPayload;
}): Promise<{ id: string; name: string }> {
  const sid = assertOptionalTaskPathId(input.id, "id");
  const res = await fetchWithTimeout("/task-drafts", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({
      name: input.name,
      payload: input.payload,
      ...(sid !== undefined ? { id: sid } : {}),
    }),
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw = (await res.json()) as { id: string; name: string };
  if (!raw?.id || !raw?.name) throw new Error("Invalid API response: draft save payload");
  return raw;
}

export async function getTaskDraft(
  id: string,
  options?: { signal?: AbortSignal },
): Promise<TaskDraftDetail> {
  const did = assertTaskPathId(id, "draft id");
  const res = await fetchWithTimeout(`/task-drafts/${encodeURIComponent(did)}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTaskDraftDetail(raw);
}

export async function deleteTaskDraft(id: string): Promise<void> {
  const did = assertTaskPathId(id, "draft id");
  const res = await fetchWithTimeout(`/task-drafts/${encodeURIComponent(did)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
}

export async function patchTask(
  id: string,
  patch: {
    title?: string;
    initial_prompt?: string;
    status?: Status;
    priority?: Priority;
    project_id?: string | null;
    project_context_item_ids?: string[];
    /**
     * Schedule wire encoding (see docs/data-model.md):
     *  - omit/undefined: do not touch the column on PATCH.
     *  - `null`: clear the schedule (server treats `null` and the
     *    explicit empty string symmetrically).
     *  - RFC3339 UTC string: set the schedule to that instant. The
     *    server rejects pre-2000 sentinel timestamps and non-RFC3339
     *    strings with a 400.
     */
    pickup_not_before?: string | null;
    /** Per-task Cursor CLI model; empty string clears stored override. */
    cursor_model?: string;
    tags?: string[];
    milestone?: string | null;
    gate?: TaskGate | null;
    depends_on?: TaskDependencyEdge[];
  },
): Promise<Task> {
  const tid = assertTaskPathId(id);
  const body: Record<string, unknown> = {};
  if (patch.title !== undefined) body.title = patch.title;
  if (patch.initial_prompt !== undefined) body.initial_prompt = patch.initial_prompt;
  if (patch.status !== undefined) body.status = patch.status;
  if (patch.priority !== undefined) body.priority = patch.priority;
  if (patch.project_id !== undefined) {
    body.project_id =
      patch.project_id === null
        ? null
        : assertTaskPathId(patch.project_id, "project_id");
  }
  if (patch.project_context_item_ids !== undefined) {
    body.project_context_item_ids = patch.project_context_item_ids;
  }
  if (patch.pickup_not_before !== undefined) {
    body.pickup_not_before = patch.pickup_not_before;
  }
  if (patch.cursor_model !== undefined) {
    body.cursor_model = patch.cursor_model;
  }
  if (patch.tags !== undefined) {
    body.tags = patch.tags;
  }
  if (patch.milestone !== undefined) {
    body.milestone = patch.milestone;
  }
  if (patch.gate !== undefined) {
    body.gate = patch.gate;
  }
  if (patch.depends_on !== undefined) {
    body.depends_on = patch.depends_on;
  }
  const res = await fetchWithTimeout(`/tasks/${encodeURIComponent(tid)}`, {
    method: "PATCH",
    headers: jsonHeaders,
    body: JSON.stringify(body),
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTask(raw);
}

function parseDependsOnList(raw: unknown): TaskDependencyEdge[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw.map((edge, i) => {
    if (typeof edge === "string") {
      return { task_id: parseNonEmptyString(edge, `depends_on[${i}]`), satisfies: "done" as const };
    }
    if (edge !== null && typeof edge === "object" && !Array.isArray(edge)) {
      const obj = edge as Record<string, unknown>;
      const satisfies = "done" as const;
      return {
        task_id: parseNonEmptyString(obj.task_id, `depends_on[${i}].task_id`),
        satisfies,
      };
    }
    throw new Error(`Invalid API response: depends_on[${i}] must be string or object`);
  });
}

export async function listTaskDependencies(
  taskId: string,
  options?: { signal?: AbortSignal },
): Promise<TaskDependencyEdge[]> {
  const tid = assertTaskPathId(taskId);
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/dependencies`,
    { headers: { Accept: "application/json" }, signal: options?.signal },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw = (await res.json()) as { depends_on?: unknown };
  return parseDependsOnList(raw.depends_on);
}

export async function addTaskDependency(
  taskId: string,
  dependsOnTaskId: string,
  satisfies: TaskDependencyEdge["satisfies"] = "done",
): Promise<TaskDependencyEdge[]> {
  const tid = assertTaskPathId(taskId);
  const dep = assertTaskPathId(dependsOnTaskId, "depends_on_task_id");
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/dependencies`,
    {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify({ depends_on_task_id: dep, satisfies: satisfies ?? "done" }),
    },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw = (await res.json()) as { depends_on?: unknown };
  return parseDependsOnList(raw.depends_on);
}

export async function removeTaskDependency(
  taskId: string,
  dependsOnTaskId: string,
): Promise<void> {
  const tid = assertTaskPathId(taskId);
  const dep = assertTaskPathId(dependsOnTaskId, "depends_on_task_id");
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/dependencies/${encodeURIComponent(dep)}`,
    { method: "DELETE" },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
}

export async function patchTaskGate(
  taskId: string,
  action: "release" | "hold" | "clear_hold",
): Promise<Task> {
  const tid = assertTaskPathId(taskId);
  const res = await fetchWithTimeout(`/tasks/${encodeURIComponent(tid)}/gate`, {
    method: "PATCH",
    headers: jsonHeaders,
    body: JSON.stringify({ action }),
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTask(raw);
}

export async function deleteTask(id: string): Promise<void> {
  const tid = assertTaskPathId(id);
  const res = await fetchWithTimeout(`/tasks/${encodeURIComponent(tid)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
}

export async function listChecklist(
  taskId: string,
  options?: { signal?: AbortSignal },
): Promise<TaskChecklistResponse> {
  const tid = assertTaskPathId(taskId, "task id");
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/checklist`,
    {
      headers: { Accept: "application/json" },
      signal: options?.signal,
    },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTaskChecklistResponse(raw);
}

export async function addChecklistItem(
  taskId: string,
  text: string,
  options?: { actor?: "user" | "agent" },
): Promise<void> {
  const headers: Record<string, string> = { ...jsonHeaders };
  if (options?.actor === "agent") {
    headers["X-Actor"] = "agent";
  }
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(taskId)}/checklist/items`,
    {
      method: "POST",
      headers,
      body: JSON.stringify({ text }),
    },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
}

export async function patchChecklistItemText(
  taskId: string,
  itemId: string,
  text: string,
  options?: { actor?: "user" | "agent" },
): Promise<TaskChecklistResponse> {
  const tid = assertTaskPathId(taskId, "task id");
  const iid = assertTaskPathId(itemId, "item id");
  const headers: Record<string, string> = { ...jsonHeaders };
  if (options?.actor === "agent") {
    headers["X-Actor"] = "agent";
  }
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/checklist/items/${encodeURIComponent(iid)}`,
    {
      method: "PATCH",
      headers,
      body: JSON.stringify({ text }),
    },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTaskChecklistResponse(raw);
}

/** Agent integrations only: the API rejects this call unless `actor: "agent"` (X-Actor header). */
export async function patchChecklistItemDone(
  taskId: string,
  itemId: string,
  done: boolean,
  options?: { actor?: "user" | "agent" },
): Promise<TaskChecklistResponse> {
  const tid = assertTaskPathId(taskId, "task id");
  const iid = assertTaskPathId(itemId, "item id");
  const headers: Record<string, string> = { ...jsonHeaders };
  if (options?.actor === "agent") {
    headers["X-Actor"] = "agent";
  }
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/checklist/items/${encodeURIComponent(iid)}`,
    {
      method: "PATCH",
      headers,
      body: JSON.stringify({ done }),
    },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTaskChecklistResponse(raw);
}

export async function deleteChecklistItem(
  taskId: string,
  itemId: string,
): Promise<void> {
  const tid = assertTaskPathId(taskId, "task id");
  const iid = assertTaskPathId(itemId, "item id");
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/checklist/items/${encodeURIComponent(iid)}`,
    { method: "DELETE" },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
}
