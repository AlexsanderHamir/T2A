import { describe, expect, it } from "vitest";
import type { TaskCycle } from "@/types/cycle";
import { formatCycleLineageLabel } from "./cycleLineage";

const emptyCycleMeta = {
  runner: "",
  runner_version: "",
  cursor_model: "",
  cursor_model_effective: "",
  prompt_hash: "",
};

function cycle(partial: Partial<TaskCycle> & Pick<TaskCycle, "id" | "attempt_seq">): TaskCycle {
  return {
    task_id: "task-1",
    status: "failed",
    triggered_by: "user",
    started_at: "2026-01-01T00:00:00Z",
    ended_at: "2026-01-01T01:00:00Z",
    meta: {},
    cycle_meta: emptyCycleMeta,
    ...partial,
  };
}

describe("formatCycleLineageLabel", () => {
  it("returns null when there is no parent cycle", () => {
    const c = cycle({ id: "c-2", attempt_seq: 2 });
    expect(formatCycleLineageLabel(c, new Map())).toBeNull();
  });

  it("labels fresh retry lineage", () => {
    const parent = cycle({ id: "c-1", attempt_seq: 2 });
    const child = cycle({
      id: "c-2",
      attempt_seq: 3,
      parent_cycle_id: "c-1",
      meta: { retry_mode: "fresh" },
    });
    const map = new Map([
      ["c-1", parent],
      ["c-2", child],
    ]);
    expect(formatCycleLineageLabel(child, map)).toBe(
      "started over from attempt 2",
    );
  });

  it("labels resume retry lineage", () => {
    const parent = cycle({ id: "c-1", attempt_seq: 1 });
    const child = cycle({
      id: "c-2",
      attempt_seq: 2,
      parent_cycle_id: "c-1",
      meta: { retry_mode: "resume" },
    });
    const map = new Map([
      ["c-1", parent],
      ["c-2", child],
    ]);
    expect(formatCycleLineageLabel(child, map)).toBe(
      "resumed from attempt 1",
    );
  });
});
