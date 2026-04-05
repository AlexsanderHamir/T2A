import {
  DEFAULT_NEW_TASK_STATUS,
  type Priority,
  type Status,
  type Task,
  type TaskChecklistResponse,
  type TaskEventDetail,
  type TaskEventsResponse,
  type TaskListResponse,
} from "@/types";
import {
  parseTask,
  parseTaskChecklistResponse,
  parseTaskEventDetail,
  parseTaskEventsResponse,
  parseTaskListResponse,
} from "./parseTaskApi";
import { jsonHeaders, readError } from "./shared";

export async function getTask(
  id: string,
  options?: { signal?: AbortSignal },
): Promise<Task> {
  const res = await fetch(`/tasks/${encodeURIComponent(id)}`, {
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
  const res = await fetch(path, {
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
  const res = await fetch(
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
  const res = await fetch(
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
  const res = await fetch(`/tasks?${q}`, {
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
  id?: string;
  parent_id?: string;
  checklist_inherit?: boolean;
}): Promise<Task> {
  const res = await fetch("/tasks", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({
      title: input.title,
      initial_prompt: input.initial_prompt ?? "",
      status: input.status ?? DEFAULT_NEW_TASK_STATUS,
      priority: input.priority,
      ...(input.id ? { id: input.id } : {}),
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

export async function patchTask(
  id: string,
  patch: {
    title?: string;
    initial_prompt?: string;
    status?: Status;
    priority?: Priority;
    parent_id?: string | null;
    checklist_inherit?: boolean;
  },
): Promise<Task> {
  const body: Record<string, unknown> = {};
  if (patch.title !== undefined) body.title = patch.title;
  if (patch.initial_prompt !== undefined) body.initial_prompt = patch.initial_prompt;
  if (patch.status !== undefined) body.status = patch.status;
  if (patch.priority !== undefined) body.priority = patch.priority;
  if (patch.parent_id !== undefined) body.parent_id = patch.parent_id;
  if (patch.checklist_inherit !== undefined) {
    body.checklist_inherit = patch.checklist_inherit;
  }
  const res = await fetch(`/tasks/${encodeURIComponent(id)}`, {
    method: "PATCH",
    headers: jsonHeaders,
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseTask(raw);
}

export async function deleteTask(id: string): Promise<void> {
  const res = await fetch(`/tasks/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error(await readError(res));
}

export async function listChecklist(
  taskId: string,
  options?: { signal?: AbortSignal },
): Promise<TaskChecklistResponse> {
  const res = await fetch(
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
  const res = await fetch(
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
  const res = await fetch(
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
  const res = await fetch(
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
  const res = await fetch(
    `/tasks/${encodeURIComponent(taskId)}/checklist/items/${encodeURIComponent(itemId)}`,
    { method: "DELETE" },
  );
  if (!res.ok) throw new Error(await readError(res));
}
