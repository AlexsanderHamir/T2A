import {
  DEFAULT_NEW_TASK_STATUS,
  type Priority,
  type Status,
  type Task,
  type TaskEventDetail,
  type TaskEventsResponse,
  type TaskListResponse,
} from "@/types";
import {
  parseTask,
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

export async function listTasks(
  limit = 200,
  offset = 0,
  options?: { signal?: AbortSignal },
): Promise<TaskListResponse> {
  const q = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
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
  priority?: Priority;
  id?: string;
}): Promise<Task> {
  const res = await fetch("/tasks", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify({
      title: input.title,
      initial_prompt: input.initial_prompt ?? "",
      status: input.status ?? DEFAULT_NEW_TASK_STATUS,
      priority: input.priority ?? "",
      ...(input.id ? { id: input.id } : {}),
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
  },
): Promise<Task> {
  const body: Record<string, unknown> = {};
  if (patch.title !== undefined) body.title = patch.title;
  if (patch.initial_prompt !== undefined) body.initial_prompt = patch.initial_prompt;
  if (patch.status !== undefined) body.status = patch.status;
  if (patch.priority !== undefined) body.priority = patch.priority;
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
