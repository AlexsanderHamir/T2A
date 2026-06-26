import { describe, expect, it } from "vitest";
import { projectQueryKeys } from "@/projects/queryKeys";
import { settingsQueryKeys } from "@/settings/settingsQueryKeys";
import { taskQueryKeys } from "../task-query";
import { decideSyncFrame } from "./decideSyncFrame";

describe("decideSyncFrame", () => {
  const noSuppress = () => false;
  const alwaysSuppress = () => true;

  it("schedules debounce for malformed frames", () => {
    const decision = decideSyncFrame({ frame: null, shouldSuppressTaskEcho: noSuppress });
    expect(decision.schedule).toBe("debounce");
    expect(decision.effects).toEqual([]);
    expect(decision.pendingDelta).toEqual({});
  });

  it("suppresses task frames during in-flight mutation", () => {
    const decision = decideSyncFrame({
      frame: { kind: "task", taskId: "t1" },
      shouldSuppressTaskEcho: alwaysSuppress,
    });
    expect(decision.schedule).toBe("immediate");
    expect(decision.pendingDelta).toEqual({});
    expect(decision.effects).toEqual([]);
  });

  it("queues task id and patch effect when enriched", () => {
    const data = { id: "t1", title: "x" };
    const decision = decideSyncFrame({
      frame: { kind: "task", taskId: "t1", data },
      shouldSuppressTaskEcho: noSuppress,
    });
    expect(decision.schedule).toBe("debounce");
    expect(decision.pendingDelta.addTaskId).toBe("t1");
    expect(decision.effects).toContainEqual({
      kind: "patch_task_detail",
      taskId: "t1",
      data,
    });
  });

  it("invalidates project queries immediately", () => {
    const decision = decideSyncFrame({
      frame: { kind: "project", projectId: "p1" },
      shouldSuppressTaskEcho: noSuppress,
    });
    expect(decision.schedule).toBe("immediate");
    expect(decision.effects).toContainEqual({
      kind: "invalidate",
      queryKey: projectQueryKeys.all,
    });
    expect(decision.effects).toContainEqual({
      kind: "invalidate",
      queryKey: taskQueryKeys.listRoot(),
    });
  });

  it("resync clears pending and invalidates broadly", () => {
    const decision = decideSyncFrame({
      frame: { kind: "resync" },
      shouldSuppressTaskEcho: noSuppress,
    });
    expect(decision.schedule).toBe("resync");
    expect(decision.pendingDelta.clearAllPending).toBe(true);
    expect(decision.effects.some((e) => e.kind === "rum_sse_resync")).toBe(true);
    expect(decision.effects).toContainEqual({
      kind: "invalidate",
      queryKey: settingsQueryKeys.app(),
    });
  });

  it("queues cycle frames with optional patch", () => {
    const data = { cycle: {}, phases: [] };
    const decision = decideSyncFrame({
      frame: { kind: "cycle", taskId: "t1", cycleId: "c1", data },
      shouldSuppressTaskEcho: noSuppress,
    });
    expect(decision.schedule).toBe("debounce");
    expect(decision.pendingDelta.addCycle).toEqual({ taskId: "t1", cycleId: "c1" });
    expect(decision.effects).toContainEqual({
      kind: "patch_cycle_detail",
      taskId: "t1",
      cycleId: "c1",
      data,
    });
  });
});
