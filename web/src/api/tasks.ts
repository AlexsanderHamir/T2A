import {
  DEFAULT_NEW_TASK_STATUS,
  DEFAULT_NEW_TASK_TYPE,
  type Priority,
  type DraftTaskEvaluation,
  type DraftTaskEvaluationInput,
  type Status,
  type Task,
  type TaskDraftDetail,
  type TaskDraftPayload,
  type TaskDraftSummary,
  type TaskType,
  type TaskChecklistResponse,
  type TaskEventDetail,
  type TaskEventsResponse,
  type TaskListResponse,
  type TaskStatsResponse,
} from "@/types";
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
} from "./parseTaskApi";
import { fetchWithTimeout, jsonHeaders, readError } from "./shared";
import {
  assertAfterId,
  assertListIntQuery,
  assertNonNegativeOffset,
  assertOptionalTaskPathId,
  assertPositiveSeq,
  assertTaskPathId,
} from "./taskRequestBounds";

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
  if (!res.ok) throw new Error(await readError(res));
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
  if (!res.ok) throw new Error(await readError(res));
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
  if (!res.ok) throw new Error(await readError(res));
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
  if (!res.ok) throw new Error(await readError(res));
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
  if (!res.ok) throw new Error(await readError(res));
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
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseTaskStatsResponse(raw);
}

export async function createTask(input: {
  title: string;
  initial_prompt?: string;
  status?: Status;
  priority: Priority;
  task_type?: TaskType;
  id?: string;
  draft_id?: string;
  parent_id?: string;
  checklist_inherit?: boolean;
}): Promise<Task> {
  const body: Record<string, unknown> = {
    title: input.title,
    initial_prompt: input.initial_prompt ?? "",
    status: input.status ?? DEFAULT_NEW_TASK_STATUS,
    priority: input.priority,
    task_type: input.task_type ?? DEFAULT_NEW_TASK_TYPE,
  };
  const cid = assertOptionalTaskPathId(input.id, "id");
  if (cid !== undefined) {
    body.id = cid;
  }
  const draftId = assertOptionalTaskPathId(input.draft_id, "draft_id");
  if (draftId !== undefined) {
    body.draft_id = draftId;
  }
  const parentId = assertOptionalTaskPathId(input.parent_id, "parent_id");
  if (parentId !== undefined) {
    body.parent_id = parentId;
  }
  if (input.checklist_inherit === true) {
    body.checklist_inherit = true;
  }
  const res = await fetchWithTimeout("/tasks", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(await readError(res));
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
  if (input.task_type) {
    payload.task_type = input.task_type;
  }
  const ep = assertOptionalTaskPathId(input.parent_id, "parent_id");
  if (ep !== undefined) {
    payload.parent_id = ep;
  }
  if (input.checklist_inherit !== undefined) {
    payload.checklist_inherit = input.checklist_inherit;
  }
  if (input.checklist_items) {
    payload.checklist_items = input.checklist_items;
  }
  const res = await fetchWithTimeout("/tasks/evaluate", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseDraftTaskEvaluation(raw);
}

export async function listTaskDrafts(
  limit = 50,
  options?: { signal?: AbortSignal },
): Promise<TaskDraftSummary[]> {
  const lim = assertListIntQuery("limit", limit, 0, 100);
  const res = await fetchWithTimeout(`/task-drafts?limit=${encodeURIComponent(lim)}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw new Error(await readError(res));
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
  if (!res.ok) throw new Error(await readError(res));
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
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseTaskDraftDetail(raw);
}

export async function deleteTaskDraft(id: string): Promise<void> {
  const did = assertTaskPathId(id, "draft id");
  const res = await fetchWithTimeout(`/task-drafts/${encodeURIComponent(did)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error(await readError(res));
}

export async function patchTask(
  id: string,
  patch: {
    title?: string;
    initial_prompt?: string;
    status?: Status;
    priority?: Priority;
    task_type?: TaskType;
    parent_id?: string | null;
    checklist_inherit?: boolean;
  },
): Promise<Task> {
  const tid = assertTaskPathId(id);
  const body: Record<string, unknown> = {};
  if (patch.title !== undefined) body.title = patch.title;
  if (patch.initial_prompt !== undefined) body.initial_prompt = patch.initial_prompt;
  if (patch.status !== undefined) body.status = patch.status;
  if (patch.priority !== undefined) body.priority = patch.priority;
  if (patch.task_type !== undefined) body.task_type = patch.task_type;
  if (patch.parent_id !== undefined) {
    body.parent_id =
      patch.parent_id === null
        ? null
        : assertTaskPathId(patch.parent_id, "parent_id");
  }
  if (patch.checklist_inherit !== undefined) {
    body.checklist_inherit = patch.checklist_inherit;
  }
  const res = await fetchWithTimeout(`/tasks/${encodeURIComponent(tid)}`, {
    method: "PATCH",
    headers: jsonHeaders,
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseTask(raw);
}

export async function deleteTask(id: string): Promise<void> {
  const tid = assertTaskPathId(id);
  const res = await fetchWithTimeout(`/tasks/${encodeURIComponent(tid)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error(await readError(res));
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
  if (!res.ok) throw new Error(await readError(res));
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
  if (!res.ok) throw new Error(await readError(res));
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
  if (!res.ok) throw new Error(await readError(res));
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
  if (!res.ok) throw new Error(await readError(res));
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
  if (!res.ok) throw new Error(await readError(res));
}
