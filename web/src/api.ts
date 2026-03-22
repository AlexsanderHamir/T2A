import type { Priority, Status, Task, TaskListResponse } from "./types";

const jsonHeaders = {
  "Content-Type": "application/json",
  Accept: "application/json",
};

async function readError(res: Response): Promise<string> {
  const t = await res.text();
  return t.trim() || res.statusText;
}

export async function listTasks(
  limit = 200,
  offset = 0,
): Promise<TaskListResponse> {
  const q = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  const res = await fetch(`/tasks?${q}`, { headers: { Accept: "application/json" } });
  if (!res.ok) throw new Error(await readError(res));
  return res.json() as Promise<TaskListResponse>;
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
      status: input.status ?? "",
      priority: input.priority ?? "",
      ...(input.id ? { id: input.id } : {}),
    }),
  });
  if (!res.ok) throw new Error(await readError(res));
  return res.json() as Promise<Task>;
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
  return res.json() as Promise<Task>;
}

export async function deleteTask(id: string): Promise<void> {
  const res = await fetch(`/tasks/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw new Error(await readError(res));
}
