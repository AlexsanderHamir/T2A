import { parseTask, parseTaskListResponse } from "./parseTaskApi";
import type { Priority, Status, Task, TaskListResponse } from "./types";

const jsonHeaders = {
  "Content-Type": "application/json",
  Accept: "application/json",
};

async function readError(res: Response): Promise<string> {
  const t = await res.text();
  try {
    const j = JSON.parse(t) as { error?: string };
    if (typeof j?.error === "string" && j.error.trim()) {
      return j.error.trim();
    }
  } catch {
    /* plain text */
  }
  return t.trim() || res.statusText;
}

/** File paths under REPO_ROOT matching q, or null if repo is not configured (503). */
export async function searchRepoFiles(
  q: string,
  options?: { signal?: AbortSignal },
): Promise<string[] | null> {
  const params = new URLSearchParams({ q });
  const res = await fetch(`/repo/search?${params}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (res.status === 503) {
    return null;
  }
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  if (
    raw !== null &&
    typeof raw === "object" &&
    "paths" in raw &&
    Array.isArray((raw as { paths: unknown }).paths)
  ) {
    return (raw as { paths: string[] }).paths.filter(
      (p): p is string => typeof p === "string",
    );
  }
  throw new Error("unexpected search response");
}

export type RepoValidateRangeResult = {
  ok: boolean;
  line_count?: number;
  warning?: string;
};

/** Returns null if repo is not configured (503). */
export async function validateRepoRange(
  path: string,
  start: number,
  end: number,
  options?: { signal?: AbortSignal },
): Promise<RepoValidateRangeResult | null> {
  const params = new URLSearchParams({
    path,
    start: String(start),
    end: String(end),
  });
  const res = await fetch(`/repo/validate-range?${params}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (res.status === 503) {
    return null;
  }
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  if (raw !== null && typeof raw === "object" && "ok" in raw) {
    const o = raw as {
      ok: boolean;
      line_count?: number;
      warning?: string;
    };
    return {
      ok: Boolean(o.ok),
      line_count: typeof o.line_count === "number" ? o.line_count : undefined,
      warning: typeof o.warning === "string" ? o.warning : undefined,
    };
  }
  throw new Error("unexpected validate-range response");
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
      status: input.status ?? "",
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
