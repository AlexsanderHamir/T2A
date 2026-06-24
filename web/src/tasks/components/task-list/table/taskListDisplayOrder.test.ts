import { describe, expect, it } from "vitest";
import type { TaskWithDepth } from "../../../task-tree";
import { makeTaskWithCreatedAt } from "@/test/taskDefaults";
import { computeTaskListDisplayOrder } from "./taskListDisplayOrder";

function makeTask(id: string, created_at: string): TaskWithDepth {
  return { ...makeTaskWithCreatedAt(id, created_at), depth: 0 };
}

/** Documents the pre-fix append-at-bottom bug that bulk template create exposed. */
function computeTaskListDisplayOrderLegacy(
  prevOrder: TaskWithDepth[],
  filteredTasks: TaskWithDepth[],
): TaskWithDepth[] {
  const nextOrder: TaskWithDepth[] = [];
  const filteredById = new Map(filteredTasks.map((t) => [t.id, t]));
  for (const t of prevOrder) {
    const visible = filteredById.get(t.id);
    if (visible) nextOrder.push(visible);
  }
  for (const t of filteredTasks) {
    if (!nextOrder.some((row) => row.id === t.id)) {
      nextOrder.push(t);
    }
  }
  return nextOrder;
}

describe("computeTaskListDisplayOrder", () => {
  it("legacy append-at-bottom order fails the bulk-create scenario", () => {
    const old1 = makeTask("old-1", "2026-01-01T00:00:00Z");
    const old2 = makeTask("old-2", "2026-01-02T00:00:00Z");
    const new1 = makeTask("new-1", "2026-06-20T12:00:00Z");
    const new2 = makeTask("new-2", "2026-06-20T11:00:00Z");

    const legacy = computeTaskListDisplayOrderLegacy([old1, old2], [new1, new2, old1, old2]);
    expect(legacy.map((t) => t.id)).toEqual(["old-1", "old-2", "new-1", "new-2"]);
  });

  it("places newly arrived tasks at their sorted position, not at the bottom", () => {
    const old1 = makeTask("old-1", "2026-01-01T00:00:00Z");
    const old2 = makeTask("old-2", "2026-01-02T00:00:00Z");
    const new1 = makeTask("new-1", "2026-06-20T12:00:00Z");
    const new2 = makeTask("new-2", "2026-06-20T11:00:00Z");

    const prevOrder = [old1, old2];
    const filteredTasks = [new1, new2, old1, old2];

    const order = computeTaskListDisplayOrder(
      prevOrder,
      filteredTasks,
      new Set(),
      new Map(),
    );

    expect(order.map((t) => t.id)).toEqual(["new-1", "new-2", "old-1", "old-2"]);
  });

  it("preserves relative order for rows that were already visible", () => {
    const a = makeTask("a", "2026-06-20T10:00:00Z");
    const b = makeTask("b", "2026-06-20T09:00:00Z");
    const c = makeTask("c", "2026-06-20T08:00:00Z");

    const prevOrder = [a, b, c];
    const filteredTasks = [a, c];

    const order = computeTaskListDisplayOrder(
      prevOrder,
      filteredTasks,
      new Set(),
      new Map(),
    );

    expect(order.map((t) => t.id)).toEqual(["a", "c"]);
  });

  it("keeps filter-exiting rows until their exit animation completes", () => {
    const a = makeTask("a", "2026-06-20T10:00:00Z");
    const b = makeTask("b", "2026-06-20T09:00:00Z");

    const prevOrder = [a, b];
    const filteredTasks = [a];
    const filterExitingIds = new Set(["b"]);
    const filterExitingById = new Map([["b", b]]);

    const order = computeTaskListDisplayOrder(
      prevOrder,
      filteredTasks,
      filterExitingIds,
      filterExitingById,
    );

    expect(order.map((t) => t.id)).toEqual(["a", "b"]);
  });
});
