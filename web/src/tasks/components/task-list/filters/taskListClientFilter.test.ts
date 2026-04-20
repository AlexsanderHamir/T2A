import { describe, expect, it } from "vitest";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import type { TaskWithDepth } from "../../../task-tree";
import { filterTasksForListView } from "./taskListClientFilter";

function row(
  partial: Pick<TaskWithDepth, "id" | "title" | "status" | "priority"> &
    Partial<Omit<TaskWithDepth, "id" | "title" | "status" | "priority">>,
): TaskWithDepth {
  return {
    initial_prompt: "",
    checklist_inherit: false,
    depth: 0,
    ...partial,
    runner: partial.runner ?? TASK_TEST_DEFAULTS.runner,
    cursor_model: partial.cursor_model ?? TASK_TEST_DEFAULTS.cursor_model,
  };
}

describe("filterTasksForListView", () => {
  const tasks: TaskWithDepth[] = [
    row({
      id: "1",
      title: "Alpha ready",
      status: "ready",
      priority: "low",
    }),
    row({
      id: "2",
      title: "Beta done",
      status: "done",
      priority: "high",
    }),
    row({
      id: "3",
      title: "Gamma READY",
      status: "ready",
      priority: "high",
    }),
  ];

  it("returns all tasks when filters are all and search empty", () => {
    expect(filterTasksForListView(tasks, "all", "all", "")).toEqual(tasks);
  });

  it("filters by status", () => {
    expect(filterTasksForListView(tasks, "ready", "all", "")).toEqual([
      tasks[0],
      tasks[2],
    ]);
  });

  it("filters by priority", () => {
    expect(filterTasksForListView(tasks, "all", "high", "")).toEqual([
      tasks[1],
      tasks[2],
    ]);
  });

  it("filters by title substring case-insensitively with trim", () => {
    expect(filterTasksForListView(tasks, "all", "all", "  alpha  ")).toEqual([
      tasks[0],
    ]);
    expect(filterTasksForListView(tasks, "all", "all", "ready")).toEqual([
      tasks[0],
      tasks[2],
    ]);
  });

  it("applies status, priority, and title together", () => {
    expect(
      filterTasksForListView(tasks, "ready", "high", "gamma"),
    ).toEqual([tasks[2]]);
  });
});
