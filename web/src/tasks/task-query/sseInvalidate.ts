/** SSE `data:` payloads are JSON lines `{ "type": "...", "id": "<uuid>" }` (see docs/API-SSE.md). */

/**
 * Discriminated union for one parsed SSE frame from `GET /events`. Returned by
 * `parseTaskChangeFrame`; `null` is used for blank, malformed, or
 * not-yet-known frames so the stream consumer can fall back to a broad
 * invalidation when needed.
 *
 * Wire shape:
 *   { "type": "task_created" | "task_updated" | "task_deleted",
 *     "id": "<task uuid>" }
 *   { "type": "task_cycle_changed",
 *     "id": "<task uuid>", "cycle_id": "<cycle uuid>" }
 */
export type TaskChangeFrame =
  | { kind: "task"; taskId: string }
  | { kind: "cycle"; taskId: string; cycleId: string }
  | { kind: "settings" }
  | { kind: "agent_run_cancelled" }
  /**
   * Hub-emitted directive that tells the client its reconnect cursor
   * fell out of the SSE ring buffer (or it was forcibly disconnected
   * as a slow consumer). Consumers MUST drop every cached query and
   * refetch from REST. Carries no id/cycle_id. Documented in
   * docs/API-SSE.md (Phase 2 of the realtime smoothness plan).
   */
  | { kind: "resync" };

function readStringId(o: Record<string, unknown>, key: string): string {
  const v = o[key];
  if (typeof v !== "string") {
    return "";
  }
  return v.trim();
}

/**
 * Parses one SSE `data:` line. Cycle frames must include both `id` (task) and
 * `cycle_id`; task frames only need `id`. Unknown event types (including
 * future ones) yield `null` so the caller can fall back to a broad
 * invalidation rather than dropping the frame.
 */
export function parseTaskChangeFrame(data: string): TaskChangeFrame | null {
  const trimmed = data.trim();
  if (trimmed === "") {
    return null;
  }
  let o: Record<string, unknown>;
  try {
    const parsed = JSON.parse(trimmed) as unknown;
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
      return null;
    }
    o = parsed as Record<string, unknown>;
  } catch {
    return null;
  }
  if (o.type === "settings_changed") {
    return { kind: "settings" };
  }
  if (o.type === "agent_run_cancelled") {
    return { kind: "agent_run_cancelled" };
  }
  if (o.type === "resync") {
    return { kind: "resync" };
  }
  const id = readStringId(o, "id");
  if (id === "") {
    return null;
  }
  if (o.type === "task_cycle_changed") {
    const cycleId = readStringId(o, "cycle_id");
    if (cycleId === "") {
      return null;
    }
    return { kind: "cycle", taskId: id, cycleId };
  }
  if (
    o.type === "task_created" ||
    o.type === "task_updated" ||
    o.type === "task_deleted"
  ) {
    return { kind: "task", taskId: id };
  }
  return null;
}

/**
 * Adds the task id from one SSE `data:` payload into `into`. Cycle frames
 * (`task_cycle_changed`) are intentionally skipped so they do not accidentally
 * invalidate the entire task detail subtree; consumers should use
 * `parseTaskChangeFrame` directly when they need to react to cycle changes.
 */
export function collectTaskIdFromSSEData(data: string, into: Set<string>): void {
  const frame = parseTaskChangeFrame(data);
  if (frame === null || frame.kind !== "task") {
    return;
  }
  into.add(frame.taskId);
}
