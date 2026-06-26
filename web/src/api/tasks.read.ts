import {
  type Task,
  type TaskDraftDetail,
  type TaskDraftSummary,
  type TaskChecklistResponse,
  type TaskEventDetail,
  type TaskEventsResponse,
  type TaskListResponse,
  type TaskStatsResponse,
  type CycleFailuresListResponse,
  type TaskDependencyEdge,
} from "@/types";
import { parseNonEmptyString } from "./parseTaskApiCore";
import {
  parseTask,
  parseTaskDraftDetail,
  parseTaskDraftSummaryList,
  parseTaskChecklistResponse,
  parseTaskEventDetail,
  parseTaskEventsResponse,
  parseTaskListResponse,
  parseTaskStatsResponse,
  parseCycleFailuresListResponse,
} from "./parseTaskApi";
import { fetchWithTimeout, apiErrorFromResponse } from "./shared";
import {
  assertAfterId,
  assertListIntQuery,
  assertNonNegativeOffset,
  assertPositiveSeq,
  assertTaskPathId,
} from "./taskRequestBounds";
import { TASK_DRAFTS } from "@/constants/tasks";
import { CYCLE_FAILURE_SORTS, type CycleFailureSort } from "@/constants/api";

function assertCycleFailureSort(sort: string): CycleFailureSort {
  if (!(CYCLE_FAILURE_SORTS as readonly string[]).includes(sort)) {
    throw new Error(`sort must be one of: ${CYCLE_FAILURE_SORTS.join(", ")}`);
  }
  return sort as CycleFailureSort;
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

export function parseDependsOnList(raw: unknown): TaskDependencyEdge[] {
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
