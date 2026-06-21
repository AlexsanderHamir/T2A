import { describe, expect, it } from "vitest";
import type { Task } from "@/types/task";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import { sortTasksByCreatedDesc } from "./taskListSort";

function makeTask(id: string, created_at: string): Task {
  return {
    id,
    title: id,
    initial_prompt: "",
    status: "ready",
    priority: "medium",
    created_at,
    ...TASK_TEST_DEFAULTS,
  };
}

describe("sortTasksByCreatedDesc", () => {
  it("orders newest created_at first", () => {
    const sorted = sortTasksByCreatedDesc([
      makeTask("old", "2026-01-01T00:00:00Z"),
      makeTask("new", "2026-06-20T12:00:00Z"),
      makeTask("mid", "2026-03-01T00:00:00Z"),
    ]);
    expect(sorted.map((t) => t.id)).toEqual(["new", "mid", "old"]);
  });
});
