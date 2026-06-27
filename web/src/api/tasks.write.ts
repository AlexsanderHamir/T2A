import {
  DEFAULT_NEW_TASK_STATUS,
  type Priority,
  type Status,
  type Task,
  type TaskDraftPayload,
  type TaskChecklistResponse,
  type TaskEventDetail,
  type ChecklistVerifyCommandInput,
  type TaskDependencyEdge,
  type TaskGate,
} from "@/types";
import {
  parseTask,
  parseTaskChecklistResponse,
  parseTaskEventDetail,
} from "./parseTaskApi";
import { fetchWithTimeout, jsonHeaders, apiErrorFromResponse } from "./shared";
import {
  assertOptionalTaskPathId,
  assertPositiveSeq,
  assertTaskPathId,
} from "./taskRequestBounds";
import { parseDependsOnList } from "./tasks.read";

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
  worktree_id?: string;
  checklist_items: Array<{ text: string; verify_commands?: ChecklistVerifyCommandInput[] }>;
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
  const worktreeId = assertOptionalTaskPathId(input.worktree_id, "worktree_id");
  if (worktreeId !== undefined) {
    body.worktree_id = worktreeId;
  }
  body.checklist_items = input.checklist_items;
  const res = await fetchWithTimeout("/tasks", {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(body),
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTask(raw);
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

export type TaskRetryMode = "fresh" | "resume";

export async function retryTask(
  id: string,
  input: { mode: TaskRetryMode; parent_cycle_id?: string },
): Promise<Task> {
  const tid = assertTaskPathId(id);
  const body: Record<string, unknown> = { mode: input.mode };
  if (input.parent_cycle_id !== undefined) {
    body.parent_cycle_id = input.parent_cycle_id;
  }
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/retry`,
    {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify(body),
    },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTask(raw);
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

export async function addChecklistItem(
  taskId: string,
  text: string,
  options?: {
    actor?: "user" | "agent";
    verify_commands?: ChecklistVerifyCommandInput[];
  },
): Promise<void> {
  const headers: Record<string, string> = { ...jsonHeaders };
  if (options?.actor === "agent") {
    headers["X-Actor"] = "agent";
  }
  const body: Record<string, unknown> = { text };
  if (options?.verify_commands && options.verify_commands.length > 0) {
    body.verify_commands = options.verify_commands;
  }
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(taskId)}/checklist/items`,
    {
      method: "POST",
      headers,
      body: JSON.stringify(body),
    },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
}

export async function patchChecklistItemVerifyCommands(
  taskId: string,
  itemId: string,
  verify_commands: ChecklistVerifyCommandInput[],
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
      body: JSON.stringify({ verify_commands }),
    },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseTaskChecklistResponse(raw);
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
