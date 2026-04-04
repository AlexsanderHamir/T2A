import type { Task } from "@/types";

export type TaskWithDepth = Task & { depth: number };

/** Depth-first flattening of GET /tasks root trees for table display. */
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
