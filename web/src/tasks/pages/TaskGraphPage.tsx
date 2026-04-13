import { useMemo, useRef, useState, type UIEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { getTask } from "@/api";
import type { Priority, Status } from "@/types";
import { TaskGraphPageSkeleton } from "../components/taskLoadingSkeletons";
import { priorityPillClass, statusPillClass } from "../taskPillClasses";
import { taskQueryKeys } from "../queryKeys";

type GraphTaskNode = {
  id: string;
  title: string;
  status: Status;
  priority: Priority;
  children?: GraphTaskNode[];
};

type GraphNode = {
  id: string;
  title: string;
  status: Status;
  priority: Priority;
  parentId: string | null;
  depth: number;
  row: number;
};

const CARD_WIDTH = 240;
const CARD_HEIGHT = 104;
const COL_GAP = 88;
const ROW_GAP = 20;
const PADDING = 24;
const BUFFER_ROWS = 20;

function graphMockUrl(): string {
  return import.meta.env.VITE_TASK_GRAPH_MOCK_URL?.trim() ?? "";
}

function flattenGraphTree(input: GraphTaskNode): GraphNode[] {
  const nodes: GraphNode[] = [];
  const stack: Array<{
    node: typeof input;
    parentId: string | null;
    depth: number;
  }> = [{ node: input, parentId: null, depth: 0 }];
  let row = 0;

  while (stack.length > 0) {
    const current = stack.pop();
    if (!current) continue;
    nodes.push({
      id: current.node.id,
      title: current.node.title,
      status: current.node.status,
      priority: current.node.priority,
      parentId: current.parentId,
      depth: current.depth,
      row,
    });
    row += 1;
    const children = Array.isArray(current.node.children)
      ? current.node.children
      : [];
    for (let i = children.length - 1; i >= 0; i -= 1) {
      const child = children[i];
      if (!child || typeof child !== "object") continue;
      stack.push({
        node: child as typeof input,
        parentId: current.node.id,
        depth: current.depth + 1,
      });
    }
  }

  return nodes;
}

function isGraphTaskNode(value: unknown): value is GraphTaskNode {
  if (!value || typeof value !== "object") return false;
  const rec = value as Record<string, unknown>;
  const validStatuses: Status[] = ["ready", "running", "blocked", "review", "done", "failed"];
  const validPriorities: Priority[] = ["low", "medium", "high", "critical"];
  if (
    typeof rec.id !== "string" ||
    typeof rec.title !== "string" ||
    typeof rec.status !== "string" ||
    !validStatuses.includes(rec.status as Status) ||
    typeof rec.priority !== "string" ||
    !validPriorities.includes(rec.priority as Priority)
  ) {
    return false;
  }
  if (rec.children === undefined) return true;
  if (!Array.isArray(rec.children)) return false;
  return rec.children.every((child) => isGraphTaskNode(child));
}

async function getGraphTask(
  taskId: string,
  options?: { signal?: AbortSignal },
): Promise<GraphTaskNode> {
  const mockUrl = graphMockUrl();
  if (!mockUrl) {
    return getTask(taskId, options) as unknown as GraphTaskNode;
  }
  const res = await fetch(mockUrl, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) {
    throw new Error(`Could not load graph mock from ${mockUrl}`);
  }
  const raw: unknown = await res.json();
  if (!isGraphTaskNode(raw)) {
    throw new Error("Invalid graph mock payload");
  }
  return raw;
}

function clamp(value: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, value));
}

export function TaskGraphPage() {
  const { taskId = "" } = useParams<{ taskId: string }>();
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const [viewport, setViewport] = useState({
    scrollLeft: 0,
    scrollTop: 0,
    width: 1200,
    height: 680,
  });

  const taskQuery = useQuery({
    queryKey: taskQueryKeys.detail(taskId),
    queryFn: ({ signal }) => getGraphTask(taskId, { signal }),
    enabled: Boolean(taskId),
  });

  const layout = useMemo(() => {
    if (!taskQuery.data) {
      return {
        nodes: [] as GraphNode[],
        nodeById: new Map<string, GraphNode>(),
        maxDepth: 0,
        height: 0,
        width: 0,
      };
    }
    const nodes = flattenGraphTree(taskQuery.data);
    let maxDepth = 0;
    const nodeById = new Map<string, GraphNode>();
    for (const node of nodes) {
      maxDepth = Math.max(maxDepth, node.depth);
      nodeById.set(node.id, node);
    }
    const width = PADDING * 2 + (maxDepth + 1) * CARD_WIDTH + maxDepth * COL_GAP;
    const height =
      PADDING * 2 + nodes.length * CARD_HEIGHT + Math.max(0, nodes.length - 1) * ROW_GAP;
    return { nodes, nodeById, maxDepth, width, height };
  }, [taskQuery.data]);

  const onScroll = (event: UIEvent<HTMLDivElement>) => {
    const target = event.currentTarget;
    setViewport({
      scrollLeft: target.scrollLeft,
      scrollTop: target.scrollTop,
      width: target.clientWidth,
      height: target.clientHeight,
    });
  };

  const rowHeight = CARD_HEIGHT + ROW_GAP;
  const startRow = clamp(
    Math.floor((viewport.scrollTop - PADDING) / rowHeight) - BUFFER_ROWS,
    0,
    Math.max(0, layout.nodes.length - 1),
  );
  const endRow = clamp(
    Math.ceil((viewport.scrollTop + viewport.height - PADDING) / rowHeight) + BUFFER_ROWS,
    0,
    Math.max(0, layout.nodes.length - 1),
  );
  const visibleNodes = layout.nodes.slice(startRow, endRow + 1);
  const leftBound = viewport.scrollLeft - CARD_WIDTH;
  const rightBound = viewport.scrollLeft + viewport.width + CARD_WIDTH;

  const visibleGraphNodes = visibleNodes.filter((node) => {
    const x = PADDING + node.depth * (CARD_WIDTH + COL_GAP);
    return x + CARD_WIDTH >= leftBound && x <= rightBound;
  });

  const visibleEdges = visibleGraphNodes
    .map((node) => {
      if (!node.parentId) return null;
      const parent = layout.nodeById.get(node.parentId);
      if (!parent) return null;
      const x1 = PADDING + parent.depth * (CARD_WIDTH + COL_GAP) + CARD_WIDTH;
      const y1 = PADDING + parent.row * rowHeight + CARD_HEIGHT / 2;
      const x2 = PADDING + node.depth * (CARD_WIDTH + COL_GAP);
      const y2 = PADDING + node.row * rowHeight + CARD_HEIGHT / 2;
      return { key: `${parent.id}-${node.id}`, x1, y1, x2, y2 };
    })
    .filter(Boolean) as Array<{
    key: string;
    x1: number;
    y1: number;
    x2: number;
    y2: number;
  }>;

  if (!taskId) {
    return <p className="muted">Missing task id.</p>;
  }

  if (taskQuery.isPending) {
    return <TaskGraphPageSkeleton />;
  }

  if (taskQuery.isError || !taskQuery.data) {
    const message =
      taskQuery.isError && taskQuery.error instanceof Error
        ? taskQuery.error.message
        : "Could not load task graph.";
    return (
      <section className="panel task-graph-page">
        <div role="alert">
          <p className="err-inline">{message}</p>
          <div className="task-graph-error-actions">
            <button
              type="button"
              className="secondary"
              onClick={() => void taskQuery.refetch()}
            >
              Try again
            </button>
            <Link to={`/tasks/${encodeURIComponent(taskId)}`}>Back to task detail</Link>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section className="panel task-graph-page task-graph-content--enter">
      <nav className="task-detail-nav" aria-label="Task graph navigation">
        <Link to={`/tasks/${encodeURIComponent(taskId)}`} className="task-detail-back">
          ← Back to task detail
        </Link>
      </nav>
      <header className="task-graph-header">
        <h2 className="task-graph-title">Task graph</h2>
        <p className="muted task-graph-meta">
          {layout.nodes.length.toLocaleString()} nodes rendered with viewport virtualization
        </p>
      </header>
      <div
        className="task-graph-viewport"
        ref={viewportRef}
        onScroll={onScroll}
        role="region"
        aria-label="Virtualized task graph canvas"
      >
        <div
          className="task-graph-canvas"
          style={{ width: `${layout.width}px`, height: `${layout.height}px` }}
        >
          <svg
            className="task-graph-edges"
            viewBox={`0 0 ${layout.width} ${layout.height}`}
            preserveAspectRatio="none"
            aria-hidden="true"
          >
            {visibleEdges.map((edge) => (
              <path
                key={edge.key}
                d={`M ${edge.x1} ${edge.y1} C ${edge.x1 + COL_GAP * 0.5} ${edge.y1}, ${edge.x2 - COL_GAP * 0.5} ${edge.y2}, ${edge.x2} ${edge.y2}`}
              />
            ))}
          </svg>
          {visibleGraphNodes.map((node) => {
            const x = PADDING + node.depth * (CARD_WIDTH + COL_GAP);
            const y = PADDING + node.row * rowHeight;
            return (
              <article
                key={node.id}
                className="task-graph-node"
                style={{ left: `${x}px`, top: `${y}px` }}
              >
                <Link className="task-graph-node-link" to={`/tasks/${node.id}`}>
                  {node.title}
                </Link>
                <div className="task-graph-node-meta">
                  <span className={priorityPillClass(node.priority)}>
                    {node.priority}
                  </span>
                  <span className={statusPillClass(node.status)}>
                    {node.status}
                  </span>
                </div>
              </article>
            );
          })}
        </div>
      </div>
    </section>
  );
}
