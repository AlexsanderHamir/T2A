import { describe, expect, it } from "vitest";
import { taskListPagerSummary } from "./taskListPagerSummary";

describe("taskListPagerSummary", () => {
  it("describes an empty page", () => {
    expect(
      taskListPagerSummary({
        tasksLength: 0,
        listPage: 0,
        listPageSize: 20,
        rootTasksOnPage: 0,
        hasNextPage: false,
      }),
    ).toBe("Page 1 (no tasks on this page)");
  });

  it("uses 1-based page number when empty on a later page", () => {
    expect(
      taskListPagerSummary({
        tasksLength: 0,
        listPage: 2,
        listPageSize: 20,
        rootTasksOnPage: 0,
        hasNextPage: false,
      }),
    ).toBe("Page 3 (no tasks on this page)");
  });

  it("shows inclusive range on first page", () => {
    expect(
      taskListPagerSummary({
        tasksLength: 5,
        listPage: 0,
        listPageSize: 20,
        rootTasksOnPage: 5,
        hasNextPage: false,
      }),
    ).toBe("1–5");
  });

  it("shows range on second page", () => {
    expect(
      taskListPagerSummary({
        tasksLength: 25,
        listPage: 1,
        listPageSize: 20,
        rootTasksOnPage: 5,
        hasNextPage: false,
      }),
    ).toBe("21–25");
  });

  it("appends plus when more pages may exist", () => {
    expect(
      taskListPagerSummary({
        tasksLength: 20,
        listPage: 0,
        listPageSize: 20,
        rootTasksOnPage: 20,
        hasNextPage: true,
      }),
    ).toBe("1–20+");
  });
});
