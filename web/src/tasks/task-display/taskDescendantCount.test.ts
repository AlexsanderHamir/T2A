import { describe, expect, it } from "vitest";
import type { Task } from "@/types/task";
import { taskDescendantCount } from "./taskDescendantCount";

const leaf = (id: string, children?: Task[]): Task => ({
  id,
  title: id,
  initial_prompt: "",
  status: "ready",
  priority: "medium",
  task_type: "general",
  runner: "cursor",
  cursor_model: "",
  checklist_inherit: false,
  ...(children ? { children } : {}),
});

describe("taskDescendantCount", () => {
  it("returns 0 for a leaf with no children field", () => {
    expect(taskDescendantCount(leaf("a"))).toBe(0);
  });

  it("returns 0 for an empty children array", () => {
    expect(taskDescendantCount({ children: [] })).toBe(0);
  });

  it("counts immediate children", () => {
    expect(
      taskDescendantCount(leaf("p", [leaf("c1"), leaf("c2")])),
    ).toBe(2);
  });

  it("counts grandchildren transitively (BFS-equivalent total)", () => {
    const tree = leaf("p", [
      leaf("c1", [leaf("g1"), leaf("g2")]),
      leaf("c2"),
    ]);
    expect(taskDescendantCount(tree)).toBe(4);
  });
});
