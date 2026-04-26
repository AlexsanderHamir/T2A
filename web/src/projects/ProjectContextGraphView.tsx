import { useMemo } from "react";
import { previewTextFromPrompt } from "@/tasks/task-prompt";
import { GRAPH_LAYOUT_PX } from "@/tasks/task-graph";
import type { ProjectContextEdge, ProjectContextItem } from "@/types";

type Props = {
  items: ProjectContextItem[];
  edges: ProjectContextEdge[];
};

type LayoutNode = {
  item: ProjectContextItem;
  depth: number;
  row: number;
  x: number;
  y: number;
};

const {
  CARD_WIDTH,
  CARD_HEIGHT,
  COL_GAP,
  ROW_GAP,
  PADDING,
} = GRAPH_LAYOUT_PX;

export function ProjectContextGraphView({ items, edges }: Props) {
  const layout = useMemo(() => buildProjectContextGraphLayout(items, edges), [
    edges,
    items,
  ]);

  if (items.length === 0) {
    return null;
  }

  return (
    <div
      className="task-graph-viewport project-context-graph-viewport"
      role="region"
      aria-label="Project context graph canvas"
    >
      <div
        className="task-graph-canvas"
        style={{ width: `${layout.width}px`, height: `${layout.height}px` }}
      >
        <svg
          className="task-graph-edges project-context-graph-edges"
          viewBox={`0 0 ${layout.width} ${layout.height}`}
          preserveAspectRatio="none"
          aria-hidden="true"
        >
          {layout.edges.map((edge) => (
            <path
              key={edge.key}
              d={`M ${edge.x1} ${edge.y1} C ${edge.cx1} ${edge.y1}, ${edge.cx2} ${edge.y2}, ${edge.x2} ${edge.y2}`}
            />
          ))}
        </svg>
        {layout.nodes.map((node) => {
          const preview = previewTextFromPrompt(node.item.body);
          return (
            <article
              key={node.item.id}
              className="task-graph-node project-context-graph-node"
              style={{ left: `${node.x}px`, top: `${node.y}px` }}
            >
              <div className="project-context-graph-node__title">
                {node.item.title}
              </div>
              <p>{preview}</p>
              <div className="task-graph-node-meta">
                <span className="project-context-node-card__kind">
                  {node.item.kind}
                </span>
              </div>
            </article>
          );
        })}
      </div>
    </div>
  );
}

function buildProjectContextGraphLayout(
  items: ProjectContextItem[],
  edges: ProjectContextEdge[],
) {
  const itemByID = new Map(items.map((item) => [item.id, item]));
  const incoming = new Map<string, string[]>();
  for (const edge of edges) {
    if (!itemByID.has(edge.source_context_id) || !itemByID.has(edge.target_context_id)) {
      continue;
    }
    const list = incoming.get(edge.target_context_id) ?? [];
    list.push(edge.source_context_id);
    incoming.set(edge.target_context_id, list);
  }

  const memo = new Map<string, number>();
  const visiting = new Set<string>();
  const depthFor = (id: string): number => {
    const cached = memo.get(id);
    if (cached !== undefined) return cached;
    if (visiting.has(id)) return 0;
    visiting.add(id);
    const parents = incoming.get(id) ?? [];
    const depth = parents.reduce((max, parentID) => {
      return Math.max(max, depthFor(parentID) + 1);
    }, 0);
    visiting.delete(id);
    memo.set(id, depth);
    return depth;
  };

  const columns = new Map<number, ProjectContextItem[]>();
  let maxDepth = 0;
  for (const item of items) {
    const depth = depthFor(item.id);
    maxDepth = Math.max(maxDepth, depth);
    const column = columns.get(depth) ?? [];
    column.push(item);
    columns.set(depth, column);
  }

  const nodes: LayoutNode[] = [];
  const nodeByID = new Map<string, LayoutNode>();
  for (const [depth, columnItems] of columns) {
    columnItems.forEach((item, row) => {
      const node = {
        item,
        depth,
        row,
        x: PADDING + depth * (CARD_WIDTH + COL_GAP),
        y: PADDING + row * (CARD_HEIGHT + ROW_GAP),
      };
      nodes.push(node);
      nodeByID.set(item.id, node);
    });
  }

  const maxRows = Math.max(1, ...Array.from(columns.values()).map((column) => column.length));
  const width = PADDING * 2 + (maxDepth + 1) * CARD_WIDTH + maxDepth * COL_GAP;
  const height =
    PADDING * 2 + maxRows * CARD_HEIGHT + Math.max(0, maxRows - 1) * ROW_GAP;

  const graphEdges = edges
    .map((edge) => {
      const source = nodeByID.get(edge.source_context_id);
      const target = nodeByID.get(edge.target_context_id);
      if (!source || !target) return null;
      const x1 = source.x + CARD_WIDTH;
      const y1 = source.y + CARD_HEIGHT / 2;
      const x2 = target.x;
      const y2 = target.y + CARD_HEIGHT / 2;
      const curve = Math.max(COL_GAP * 0.45, Math.abs(x2 - x1) * 0.45);
      return {
        key: edge.id,
        x1,
        y1,
        x2,
        y2,
        cx1: x1 + curve,
        cx2: x2 - curve,
      };
    })
    .filter(Boolean) as Array<{
    key: string;
    x1: number;
    y1: number;
    x2: number;
    y2: number;
    cx1: number;
    cx2: number;
  }>;

  return { nodes, edges: graphEdges, width, height };
}
