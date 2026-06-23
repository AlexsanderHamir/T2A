import { describe, expect, it } from "vitest";
import { SSE_CHANGE_TYPE, SSE_CHANGE_TYPES } from "@/types";
import {
  collectTaskIdFromSSEData,
  parseTaskChangeFrame,
} from "./sseInvalidate";

describe("collectTaskIdFromSSEData", () => {
  it("collects trimmed id from task SSE JSON", () => {
    const s = new Set<string>();
    collectTaskIdFromSSEData(
      '{"type":"task_updated","id":"  abc-1  "}',
      s,
    );
    expect([...s]).toEqual(["abc-1"]);
  });

  it("ignores malformed JSON", () => {
    const s = new Set<string>();
    collectTaskIdFromSSEData("not json", s);
    expect(s.size).toBe(0);
  });

  it("ignores missing or non-string id", () => {
    const s = new Set<string>();
    collectTaskIdFromSSEData('{"type":"task_updated"}', s);
    collectTaskIdFromSSEData('{"id":null}', s);
    expect(s.size).toBe(0);
  });

  it("ignores blank or whitespace-only payloads", () => {
    const s = new Set<string>();
    collectTaskIdFromSSEData("", s);
    collectTaskIdFromSSEData("   \n\t  ", s);
    collectTaskIdFromSSEData('{"id":"   "}', s);
    expect(s.size).toBe(0);
  });

  it("dedupes repeated ids in the same set", () => {
    const s = new Set<string>();
    collectTaskIdFromSSEData('{"type":"task_updated","id":"same"}', s);
    collectTaskIdFromSSEData('{"type":"task_updated","id":"same"}', s);
    expect([...s]).toEqual(["same"]);
  });

  it("does not collect task ids from cycle frames (granular path uses parseTaskChangeFrame)", () => {
    const s = new Set<string>();
    collectTaskIdFromSSEData(
      '{"type":"task_cycle_changed","id":"task-1","cycle_id":"cyc-1"}',
      s,
    );
    expect(s.size).toBe(0);
  });
});

describe("parseTaskChangeFrame", () => {
  it("returns a task frame for task_created/updated/deleted", () => {
    expect(
      parseTaskChangeFrame('{"type":"task_updated","id":"task-1"}'),
    ).toEqual({ kind: "task", taskId: "task-1" });
    expect(
      parseTaskChangeFrame('{"type":"task_created","id":"task-2"}'),
    ).toEqual({ kind: "task", taskId: "task-2" });
    expect(
      parseTaskChangeFrame('{"type":"task_deleted","id":"task-3"}'),
    ).toEqual({ kind: "task", taskId: "task-3" });
    expect(
      parseTaskChangeFrame('{"type":"task_gate_changed","id":"task-4"}'),
    ).toEqual({ kind: "task", taskId: "task-4" });
    expect(
      parseTaskChangeFrame('{"type":"task_dependency_changed","id":"task-5"}'),
    ).toEqual({ kind: "task", taskId: "task-5" });
  });

  it("collects task ids from gate and dependency SSE events", () => {
    const s = new Set<string>();
    collectTaskIdFromSSEData('{"type":"task_gate_changed","id":"g-1"}', s);
    collectTaskIdFromSSEData('{"type":"task_dependency_changed","id":"d-1"}', s);
    expect([...s].sort()).toEqual(["d-1", "g-1"]);
  });

  it("returns project frames for project and context changes", () => {
    expect(
      parseTaskChangeFrame('{"type":"project_updated","id":"project-1"}'),
    ).toEqual({ kind: "project", projectId: "project-1" });
    expect(
      parseTaskChangeFrame(
        '{"type":"project_context_changed","id":"project-1"}',
      ),
    ).toEqual({ kind: "project_context", projectId: "project-1" });
  });

  it("returns a cycle frame only when both id and cycle_id are present", () => {
    expect(
      parseTaskChangeFrame(
        '{"type":"task_cycle_changed","id":"task-1","cycle_id":"cyc-1"}',
      ),
    ).toEqual({ kind: "cycle", taskId: "task-1", cycleId: "cyc-1" });
    expect(
      parseTaskChangeFrame('{"type":"task_cycle_changed","id":"task-1"}'),
    ).toBeNull();
    expect(
      parseTaskChangeFrame(
        '{"type":"task_cycle_changed","id":"task-1","cycle_id":"   "}',
      ),
    ).toBeNull();
  });

  it("returns a progress frame without treating it as an invalidation fallback", () => {
    expect(
      parseTaskChangeFrame(
        '{"type":"agent_run_progress","id":"task-1","cycle_id":"cyc-1","phase_seq":2,"progress":{"kind":"tool_call","subtype":"started","tool":"ReadFile","message":"Started ReadFile"}}',
      ),
    ).toEqual({
      kind: "progress",
      taskId: "task-1",
      cycleId: "cyc-1",
      phaseSeq: 2,
      progress: {
        kind: "tool_call",
        subtype: "started",
        tool: "ReadFile",
        message: "Started ReadFile",
      },
    });
  });

  it("returns null for blank, malformed, missing-id, unknown-type, or array payloads", () => {
    expect(parseTaskChangeFrame("")).toBeNull();
    expect(parseTaskChangeFrame("   \n")).toBeNull();
    expect(parseTaskChangeFrame("not json")).toBeNull();
    expect(parseTaskChangeFrame("[1,2,3]")).toBeNull();
    expect(parseTaskChangeFrame('{"type":"task_updated"}')).toBeNull();
    expect(parseTaskChangeFrame('{"type":"sync_ping","id":"x"}')).toBeNull();
  });

  it("returns settings/agent_run_cancelled frames without an id", () => {
    expect(parseTaskChangeFrame('{"type":"settings_changed"}')).toEqual({
      kind: "settings",
    });
    expect(parseTaskChangeFrame('{"type":"agent_run_cancelled"}')).toEqual({
      kind: "agent_run_cancelled",
    });
  });

  // Phase 2 of the realtime smoothness plan: the hub emits this
  // directive when the client's reconnect cursor fell out of the
  // ring buffer (or it was forcibly disconnected as a slow consumer).
  // The frame deliberately carries no id/cycle_id; consumers MUST
  // drop every cached query and refetch from REST. Pinning the
  // parser separately from the consumer (useTaskEventStream) keeps
  // the wire-shape contract explicit even if the hook moves.
  it("returns a resync frame for the hub-emitted resync directive", () => {
    expect(parseTaskChangeFrame('{"type":"resync"}')).toEqual({
      kind: "resync",
    });
  });

  // Enrichment: task_created / task_updated / task_cycle_changed may
  // carry the full entity in `data` so the SPA applies it via
  // setQueryData and skips the follow-up GET. The parser preserves
  // `data` verbatim; deep validation happens at the consumer (parseTask
  // / parseTaskCycleDetail) so a malformed payload falls back to the
  // existing invalidate-and-refetch path instead of dropping the frame.
  it("preserves enriched data on task frames", () => {
    const sample = { id: "task-1", title: "fresh" };
    expect(
      parseTaskChangeFrame(
        JSON.stringify({
          type: "task_updated",
          id: "task-1",
          data: sample,
        }),
      ),
    ).toEqual({ kind: "task", taskId: "task-1", data: sample });
  });

  it("preserves enriched data on task_cycle_changed frames", () => {
    const sample = { id: "cyc-1", cycle_status: "running" };
    expect(
      parseTaskChangeFrame(
        JSON.stringify({
          type: "task_cycle_changed",
          id: "task-1",
          cycle_id: "cyc-1",
          data: sample,
        }),
      ),
    ).toEqual({
      kind: "cycle",
      taskId: "task-1",
      cycleId: "cyc-1",
      data: sample,
    });
  });

  it("omits data field on frames without enrichment", () => {
    expect(
      parseTaskChangeFrame('{"type":"task_updated","id":"task-1"}'),
    ).toEqual({ kind: "task", taskId: "task-1" });
  });

  it("handles every SSE_CHANGE_TYPES wire value (mirrors realtime/wire.go)", () => {
    const samples: Record<(typeof SSE_CHANGE_TYPES)[number], string> = {
      [SSE_CHANGE_TYPE.taskCreated]: '{"type":"task_created","id":"t-1"}',
      [SSE_CHANGE_TYPE.taskUpdated]: '{"type":"task_updated","id":"t-1"}',
      [SSE_CHANGE_TYPE.taskDeleted]: '{"type":"task_deleted","id":"t-1"}',
      [SSE_CHANGE_TYPE.taskGateChanged]: '{"type":"task_gate_changed","id":"t-1"}',
      [SSE_CHANGE_TYPE.taskDependencyChanged]:
        '{"type":"task_dependency_changed","id":"t-1"}',
      [SSE_CHANGE_TYPE.taskCycleChanged]:
        '{"type":"task_cycle_changed","id":"t-1","cycle_id":"c-1"}',
      [SSE_CHANGE_TYPE.agentRunProgress]:
        '{"type":"agent_run_progress","id":"t-1","cycle_id":"c-1","phase_seq":1,"progress":{"kind":"status"}}',
      [SSE_CHANGE_TYPE.projectCreated]: '{"type":"project_created","id":"p-1"}',
      [SSE_CHANGE_TYPE.projectUpdated]: '{"type":"project_updated","id":"p-1"}',
      [SSE_CHANGE_TYPE.projectDeleted]: '{"type":"project_deleted","id":"p-1"}',
      [SSE_CHANGE_TYPE.projectContextChanged]:
        '{"type":"project_context_changed","id":"p-1"}',
      [SSE_CHANGE_TYPE.settingsChanged]: '{"type":"settings_changed"}',
      [SSE_CHANGE_TYPE.agentRunCancelled]: '{"type":"agent_run_cancelled"}',
      [SSE_CHANGE_TYPE.resync]: '{"type":"resync"}',
    };
    for (const wireType of SSE_CHANGE_TYPES) {
      expect(parseTaskChangeFrame(samples[wireType])).not.toBeNull();
    }
  });
});
