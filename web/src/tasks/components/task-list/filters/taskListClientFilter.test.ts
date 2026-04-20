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

  describe("scheduled bucket", () => {
    const NOW = Date.UTC(2026, 3, 19, 12, 0, 0);
    const future = (offsetMin: number) =>
      new Date(NOW + offsetMin * 60_000).toISOString();
    const sched = (
      partial: Pick<TaskWithDepth, "id" | "title" | "status"> &
        Partial<Omit<TaskWithDepth, "id" | "title" | "status">>,
    ) =>
      row({
        priority: "medium",
        ...partial,
      });

    const inOneHour = sched({
      id: "f1",
      title: "Future ready",
      status: "ready",
      pickup_not_before: future(60),
    });
    const inThePast = sched({
      id: "p1",
      title: "Past ready",
      status: "ready",
      pickup_not_before: future(-60),
    });
    const noSchedule = sched({
      id: "n1",
      title: "Plain ready",
      status: "ready",
    });
    const inFlight = sched({
      id: "if1",
      title: "Future running (excluded)",
      status: "running",
      pickup_not_before: future(60),
    });
    const malformed = sched({
      id: "m1",
      title: "Bad sched",
      status: "ready",
      pickup_not_before: "not a date",
    });

    it("matches only ready+future tasks", () => {
      expect(
        filterTasksForListView(
          [inOneHour, inThePast, noSchedule, inFlight, malformed],
          "scheduled",
          "all",
          "",
          NOW,
        ),
      ).toEqual([inOneHour]);
    });

    it("excludes tasks whose pickup time is exactly now", () => {
      const exactlyNow = sched({
        id: "n2",
        title: "Now",
        status: "ready",
        pickup_not_before: new Date(NOW).toISOString(),
      });
      expect(
        filterTasksForListView([exactlyNow], "scheduled", "all", "", NOW),
      ).toEqual([]);
    });

    it("composes with priority and title filters", () => {
      const matching = sched({
        id: "fa",
        title: "Friday review",
        status: "ready",
        priority: "high",
        pickup_not_before: future(60),
      });
      const nonMatching = sched({
        id: "fb",
        title: "Monday triage",
        status: "ready",
        priority: "high",
        pickup_not_before: future(60),
      });
      expect(
        filterTasksForListView(
          [matching, nonMatching],
          "scheduled",
          "high",
          "friday",
          NOW,
        ),
      ).toEqual([matching]);
    });
  });
});
