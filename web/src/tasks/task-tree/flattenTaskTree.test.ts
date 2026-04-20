import { describe, expect, it } from "vitest";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import { flattenTaskTree, flattenTaskTreeRoots } from "./flattenTaskTree";

const child = {
  id: "c1",
  title: "Child",
  initial_prompt: "",
  status: "ready" as const,
  priority: "medium" as const,
  checklist_inherit: false as const,
  ...TASK_TEST_DEFAULTS,
};

const root = {
  id: "r1",
  title: "Root",
  initial_prompt: "",
  status: "ready" as const,
  priority: "medium" as const,
  checklist_inherit: false as const,
  ...TASK_TEST_DEFAULTS,
  children: [child],
};

describe("flattenTaskTree", () => {
  it("depth-first includes nested tasks", () => {
    const flat = flattenTaskTree([root]);
    expect(flat).toHaveLength(2);
    expect(flat[0]).toMatchObject({ id: "r1", depth: 0 });
    expect(flat[1]).toMatchObject({ id: "c1", depth: 1 });
    expect(flat[0]).not.toHaveProperty("children");
    expect(flat[1]).not.toHaveProperty("children");
  });

  it("returns an empty array for an empty tree", () => {
    expect(flattenTaskTree([])).toEqual([]);
  });

  it("walks multiple roots and grandchildren depth-first", () => {
    const grand = {
      id: "g1",
      title: "Grand",
      initial_prompt: "",
      status: "ready" as const,
      priority: "medium" as const,
      checklist_inherit: false as const,
      ...TASK_TEST_DEFAULTS,
    };
    const mid = {
      id: "m1",
      title: "Mid",
      initial_prompt: "",
      status: "ready" as const,
      priority: "medium" as const,
      checklist_inherit: false as const,
      ...TASK_TEST_DEFAULTS,
      children: [grand],
    };
    const top = {
      id: "t1",
      title: "Top",
      initial_prompt: "",
      status: "ready" as const,
      priority: "medium" as const,
      checklist_inherit: false as const,
      ...TASK_TEST_DEFAULTS,
      children: [mid],
    };
    const other = {
      id: "t2",
      title: "Other root",
      initial_prompt: "",
      status: "done" as const,
      priority: "low" as const,
      checklist_inherit: false as const,
      ...TASK_TEST_DEFAULTS,
    };
    const flat = flattenTaskTree([top, other]);
    expect(flat.map((t) => ({ id: t.id, depth: t.depth }))).toEqual([
      { id: "t1", depth: 0 },
      { id: "m1", depth: 1 },
      { id: "g1", depth: 2 },
      { id: "t2", depth: 0 },
    ]);
  });
});

describe("flattenTaskTreeRoots", () => {
  it("returns only top-level rows at depth 0", () => {
    const flat = flattenTaskTreeRoots([root]);
    expect(flat).toHaveLength(1);
    expect(flat[0]).toMatchObject({ id: "r1", depth: 0 });
    expect(flat[0]).not.toHaveProperty("children");
  });

  it("returns an empty array for an empty list", () => {
    expect(flattenTaskTreeRoots([])).toEqual([]);
  });

  it("strips children from every root and preserves order", () => {
    const a = {
      id: "a",
      title: "A",
      initial_prompt: "",
      status: "ready" as const,
      priority: "low" as const,
      checklist_inherit: false as const,
      ...TASK_TEST_DEFAULTS,
    };
    const b = {
      id: "b",
      title: "B",
      initial_prompt: "",
      status: "done" as const,
      priority: "high" as const,
      checklist_inherit: false as const,
      ...TASK_TEST_DEFAULTS,
      children: [child],
    };
    const flat = flattenTaskTreeRoots([a, b]);
    expect(flat.map((t) => t.id)).toEqual(["a", "b"]);
    expect(flat.every((t) => t.depth === 0)).toBe(true);
    expect(flat.every((t) => !("children" in t))).toBe(true);
  });
});
