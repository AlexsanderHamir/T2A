import type {
  CompleteTaskCyclePhaseInput,
  StartTaskCycleInput,
  StartTaskCyclePhaseInput,
  TaskCycle,
  TaskCycleDetail,
  TaskCyclePhase,
  TaskCyclesListResponse,
  TerminateTaskCycleInput,
} from "@/types";
import {
  parseTaskCycle,
  parseTaskCycleDetail,
  parseTaskCyclePhase,
  parseTaskCyclesListResponse,
} from "./parseTaskApi";
import { fetchWithTimeout, jsonHeaders, readError } from "./shared";
import {
  assertListIntQuery,
  assertPositiveSeq,
  assertTaskPathId,
} from "./taskRequestBounds";

/**
 * Server-side `maxTaskCyclesListLimit` from `pkgs/tasks/handler/handler_cycles.go`.
 * Mirrors the `400` validation contract documented in `docs/API-HTTP.md`.
 */
export const maxTaskCyclesListLimit = 200;

function actorHeader(actor?: "user" | "agent"): Record<string, string> {
  if (actor === "agent") return { "X-Actor": "agent" };
  return {};
}

export async function listTaskCycles(
  taskId: string,
  options?: { signal?: AbortSignal; limit?: number },
): Promise<TaskCyclesListResponse> {
  const tid = assertTaskPathId(taskId, "task id");
  const q = new URLSearchParams();
  if (options?.limit !== undefined) {
    q.set(
      "limit",
      assertListIntQuery("limit", options.limit, 0, maxTaskCyclesListLimit),
    );
  }
  const qs = q.toString();
  const path =
    qs === ""
      ? `/tasks/${encodeURIComponent(tid)}/cycles`
      : `/tasks/${encodeURIComponent(tid)}/cycles?${qs}`;
  const res = await fetchWithTimeout(path, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseTaskCyclesListResponse(raw);
}

export async function getTaskCycle(
  taskId: string,
  cycleId: string,
  options?: { signal?: AbortSignal },
): Promise<TaskCycleDetail> {
  const tid = assertTaskPathId(taskId, "task id");
  const cid = assertTaskPathId(cycleId, "cycle id");
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/cycles/${encodeURIComponent(cid)}`,
    {
      headers: { Accept: "application/json" },
      signal: options?.signal,
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseTaskCycleDetail(raw);
}

export async function startTaskCycle(
  taskId: string,
  input?: StartTaskCycleInput,
  options?: { actor?: "user" | "agent" },
): Promise<TaskCycle> {
  const tid = assertTaskPathId(taskId, "task id");
  const body: Record<string, unknown> = {};
  if (input?.parent_cycle_id === null) {
    body.parent_cycle_id = null;
  } else if (input?.parent_cycle_id !== undefined) {
    body.parent_cycle_id = assertTaskPathId(
      input.parent_cycle_id,
      "parent_cycle_id",
    );
  }
  if (input?.meta !== undefined) {
    body.meta = input.meta;
  }
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/cycles`,
    {
      method: "POST",
      headers: { ...jsonHeaders, ...actorHeader(options?.actor) },
      body: JSON.stringify(body),
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseTaskCycle(raw);
}

export async function terminateTaskCycle(
  taskId: string,
  cycleId: string,
  input: TerminateTaskCycleInput,
  options?: { actor?: "user" | "agent" },
): Promise<TaskCycle> {
  const tid = assertTaskPathId(taskId, "task id");
  const cid = assertTaskPathId(cycleId, "cycle id");
  const body: Record<string, unknown> = { status: input.status };
  if (input.reason !== undefined) body.reason = input.reason;
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/cycles/${encodeURIComponent(cid)}`,
    {
      method: "PATCH",
      headers: { ...jsonHeaders, ...actorHeader(options?.actor) },
      body: JSON.stringify(body),
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseTaskCycle(raw);
}

export async function startTaskCyclePhase(
  taskId: string,
  cycleId: string,
  input: StartTaskCyclePhaseInput,
  options?: { actor?: "user" | "agent" },
): Promise<TaskCyclePhase> {
  const tid = assertTaskPathId(taskId, "task id");
  const cid = assertTaskPathId(cycleId, "cycle id");
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/cycles/${encodeURIComponent(cid)}/phases`,
    {
      method: "POST",
      headers: { ...jsonHeaders, ...actorHeader(options?.actor) },
      body: JSON.stringify({ phase: input.phase }),
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseTaskCyclePhase(raw);
}

export async function patchTaskCyclePhase(
  taskId: string,
  cycleId: string,
  phaseSeq: number,
  input: CompleteTaskCyclePhaseInput,
  options?: { actor?: "user" | "agent" },
): Promise<TaskCyclePhase> {
  const tid = assertTaskPathId(taskId, "task id");
  const cid = assertTaskPathId(cycleId, "cycle id");
  const seqStr = assertPositiveSeq("phase_seq", phaseSeq);
  const body: Record<string, unknown> = { status: input.status };
  if (input.summary !== undefined) body.summary = input.summary;
  if (input.details !== undefined) body.details = input.details;
  const res = await fetchWithTimeout(
    `/tasks/${encodeURIComponent(tid)}/cycles/${encodeURIComponent(cid)}/phases/${encodeURIComponent(seqStr)}`,
    {
      method: "PATCH",
      headers: { ...jsonHeaders, ...actorHeader(options?.actor) },
      body: JSON.stringify(body),
    },
  );
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseTaskCyclePhase(raw);
}
