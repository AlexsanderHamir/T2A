import { describe, expect, it } from "vitest";
import { flattenTaskTree, flattenTaskTreeRoots } from "./flattenTaskTree";

const child = {
  id: "c1",
  title: "Child",
  initial_prompt: "",
  status: "ready" as const,
  priority: "medium" as const,
  checklist_inherit: false as const,
};

const root = {
  id: "r1",
  title: "Root",
  initial_prompt: "",
  status: "ready" as const,
  priority: "medium" as const,
  checklist_inherit: false as const,
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
});

describe("flattenTaskTreeRoots", () => {
  it("returns only top-level rows at depth 0", () => {
    const flat = flattenTaskTreeRoots([root]);
    expect(flat).toHaveLength(1);
    expect(flat[0]).toMatchObject({ id: "r1", depth: 0 });
    expect(flat[0]).not.toHaveProperty("children");
  });
});
