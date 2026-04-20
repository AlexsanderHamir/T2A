import { describe, expect, it } from "vitest";
import { taskQueryKeys } from "./queryKeys";

describe("taskQueryKeys", () => {
  it("builds list root and per-page list keys", () => {
    expect(taskQueryKeys.listRoot()).toEqual(["tasks", "list"]);
    expect(taskQueryKeys.list(0)).toEqual(["tasks", "list", 0]);
    expect(taskQueryKeys.list(3)).toEqual(["tasks", "list", 3]);
  });

  it("scopes detail, checklist, and event detail under the task id", () => {
    expect(taskQueryKeys.detail("t1")).toEqual(["tasks", "detail", "t1"]);
    expect(taskQueryKeys.checklist("t1")).toEqual([
      "tasks",
      "detail",
      "t1",
      "checklist",
    ]);
    expect(taskQueryKeys.eventDetail("t1", 42)).toEqual([
      "tasks",
      "detail",
      "t1",
      "event",
      42,
    ]);
  });

  it("scopes cycles list and per-cycle keys under the task detail", () => {
    expect(taskQueryKeys.cycles("t1")).toEqual([
      "tasks",
      "detail",
      "t1",
      "cycles",
    ]);
    expect(taskQueryKeys.cycle("t1", "cyc-1")).toEqual([
      "tasks",
      "detail",
      "t1",
      "cycles",
      "cyc-1",
    ]);
  });

  it("encodes events cursor variants in the key", () => {
    expect(taskQueryKeys.events("t1", { k: "head" })).toEqual([
      "tasks",
      "detail",
      "t1",
      "events",
      "head",
    ]);
    expect(taskQueryKeys.events("t1", { k: "before", seq: 9 })).toEqual([
      "tasks",
      "detail",
      "t1",
      "events",
      "before",
      9,
    ]);
    expect(taskQueryKeys.events("t1", { k: "after", seq: 10 })).toEqual([
      "tasks",
      "detail",
      "t1",
      "events",
      "after",
      10,
    ]);
  });

  it("defines stats and drafts keys for invalidation outside tasks tree", () => {
    expect(taskQueryKeys.stats()).toEqual(["task-stats"]);
    expect(taskQueryKeys.drafts()).toEqual(["task-drafts"]);
  });
});
