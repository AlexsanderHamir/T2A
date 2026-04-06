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
} from "./parseTaskApi";
import { fetchWithTimeout, jsonHeaders, readError } from "./shared";

export async function getTask(
  id: string,
  options?: { signal?: AbortSignal },
): Promise<Task> {
  const res = await fetchWithTimeout(`/tasks/${encodeURIComponent(id)}`, {
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
  const q = new URLSearchParams();
  if (options?.limit !== undefined) q.set("limit", String(options.limit));
  if (options?.beforeSeq !== undefined) {
    q.set("before_seq", String(options.beforeSeq));
  }
  if (options?.afterSeq !== undefined) {
    q.set("after_seq", String(options.afterSeq));
  }
  const qs = q.toString();
  const path =
    qs === ""
      ? `/tasks/${encodeURIComponent(id)}/events`
      : `/tasks/${encodeURIComponent(id)}/events?${qs}`;
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
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(taskId)}/events/${encodeURIComponent(String(seq))}`,
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
  const headers: Record<string, string> = { ...jsonHeaders };
  if (options?.actor === "agent") {
    headers["X-Actor"] = "agent";
  }
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(taskId)}/events/${encodeURIComponent(String(seq))}`,
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
  const q = new URLSearchParams({ limit: String(limit) });
  if (options?.afterId) {
    q.set("after_id", options.afterId);
  } else {
    q.set("offset", String(offset));
  }
  const res = await fetchWithTimeout(`/tasks?${q}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseTaskListResponse(raw);
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
  const res = await fetchWithTimeout("/tasks", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({
      title: input.title,
      initial_prompt: input.initial_prompt ?? "",
      status: input.status ?? DEFAULT_NEW_TASK_STATUS,
      priority: input.priority,
      task_type: input.task_type ?? DEFAULT_NEW_TASK_TYPE,
      ...(input.id ? { id: input.id } : {}),
      ...(input.draft_id ? { draft_id: input.draft_id } : {}),
      ...(input.parent_id ? { parent_id: input.parent_id } : {}),
      ...(input.checklist_inherit === true
        ? { checklist_inherit: true }
        : {}),
    }),
  });
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseTask(raw);
}

export async function evaluateDraftTask(
  input: DraftTaskEvaluationInput,
): Promise<DraftTaskEvaluation> {
  const res = await fetchWithTimeout("/tasks/evaluate", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({
      id: input.id,
      title: input.title,
      initial_prompt: input.initial_prompt ?? "",
      ...(input.status ? { status: input.status } : {}),
      ...(input.priority ? { priority: input.priority } : {}),
      ...(input.task_type ? { task_type: input.task_type } : {}),
      ...(input.parent_id ? { parent_id: input.parent_id } : {}),
      ...(input.checklist_inherit !== undefined
        ? { checklist_inherit: input.checklist_inherit }
        : {}),
      ...(input.checklist_items ? { checklist_items: input.checklist_items } : {}),
    }),
  });
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseDraftTaskEvaluation(raw);
}

export async function listTaskDrafts(
  limit = 50,
  options?: { signal?: AbortSignal },
): Promise<TaskDraftSummary[]> {
  const res = await fetchWithTimeout(`/task-drafts?limit=${encodeURIComponent(String(limit))}`, {
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
  const res = await fetchWithTimeout("/task-drafts", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
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
  const res = await fetchWithTimeout(`/task-drafts/${encodeURIComponent(id)}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseTaskDraftDetail(raw);
}

export async function deleteTaskDraft(id: string): Promise<void> {
  const res = await fetchWithTimeout(`/task-drafts/${encodeURIComponent(id)}`, {
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
  const body: Record<string, unknown> = {};
  if (patch.title !== undefined) body.title = patch.title;
  if (patch.initial_prompt !== undefined) body.initial_prompt = patch.initial_prompt;
  if (patch.status !== undefined) body.status = patch.status;
  if (patch.priority !== undefined) body.priority = patch.priority;
  if (patch.task_type !== undefined) body.task_type = patch.task_type;
  if (patch.parent_id !== undefined) body.parent_id = patch.parent_id;
  if (patch.checklist_inherit !== undefined) {
    body.checklist_inherit = patch.checklist_inherit;
  }
  const res = await fetchWithTimeout(`/tasks/${encodeURIComponent(id)}`, {
    method: "PATCH",
    headers: jsonHeaders,
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseTask(raw);
}

export async function deleteTask(id: string): Promise<void> {
  const res = await fetchWithTimeout(`/tasks/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error(await readError(res));
}

export async function listChecklist(
  taskId: string,
  options?: { signal?: AbortSignal },
): Promise<TaskChecklistResponse> {
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(taskId)}/checklist`,
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
  const headers: Record<string, string> = { ...jsonHeaders };
  if (options?.actor === "agent") {
    headers["X-Actor"] = "agent";
  }
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(taskId)}/checklist/items/${encodeURIComponent(itemId)}`,
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
  const headers: Record<string, string> = { ...jsonHeaders };
  if (options?.actor === "agent") {
    headers["X-Actor"] = "agent";
  }
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(taskId)}/checklist/items/${encodeURIComponent(itemId)}`,
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
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(taskId)}/checklist/items/${encodeURIComponent(itemId)}`,
    { method: "DELETE" },
  );
  if (!res.ok) throw new Error(await readError(res));
}
