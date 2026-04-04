import type { Task } from "@/types";

export type TaskWithDepth = Task & { depth: number };

/** Depth-first flattening of GET /tasks root trees (e.g. parent-task picker). */
export function flattenTaskTree(nodes: Task[], depth = 0): TaskWithDepth[] {
  const out: TaskWithDepth[] = [];
  for (const n of nodes) {
    const { children, ...rest } = n;
    out.push({ ...rest, depth });
    if (children?.length) {
      out.push(...flattenTaskTree(children, depth + 1));
    }
  }
  return out;
}

/** Top-level tasks only for the home list (no subtask rows). */
export function flattenTaskTreeRoots(nodes: Task[]): TaskWithDepth[] {
  return nodes.map((n) => {
    const { children, ...rest } = n;
    void children;
    return { ...rest, depth: 0 };
  });
}
