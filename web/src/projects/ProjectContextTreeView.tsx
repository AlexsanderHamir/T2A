import { useCallback, useMemo, useRef, type CSSProperties } from "react";
import type { ProjectContextEdge, ProjectContextItem } from "@/types";
import { projectContextKindTone } from "./projectContextKindTone";

type Props = {
  items: ProjectContextItem[];
  edges: ProjectContextEdge[];
  selection?: {
    selectedIds: Set<string>;
    disabled?: boolean;
    onToggle: (item: ProjectContextItem) => void;
  };
};

type TreeEdge = ProjectContextEdge & {
  target: ProjectContextItem;
};

type TreeRoot = {
  item: ProjectContextItem;
};

type ProjectContextForest = TreeRoot[] & {
  childrenBySource: Map<string, TreeEdge[]>;
};

type RelationToneStyle = CSSProperties & {
  "--project-context-relation-hue": string;
};

const RELATION_HUE_RANGES: Record<ProjectContextEdge["relation"], { start: number; span: number }> = {
  blocks: { start: 348, span: 28 },
  depends_on: { start: 218, span: 30 },
  refines: { start: 258, span: 30 },
  related: { start: 184, span: 24 },
  supports: { start: 132, span: 34 },
};

export function ProjectContextTreeView({ items, edges, selection }: Props) {
  const forest = useMemo(() => buildProjectContextForest(items, edges), [edges, items]);

  if (items.length === 0) {
    return null;
  }

  return (
    <div className="project-context-tree-view" aria-label="Project context trees">
      {forest.map((root) => (
        <RootTree
          key={root.item.id}
          root={root}
          childrenBySource={forest.childrenBySource}
          selection={selection}
        />
      ))}
    </div>
  );
}

function RootTree({
  root,
  childrenBySource,
  selection,
}: {
  root: TreeRoot;
  childrenBySource: Map<string, TreeEdge[]>;
  selection?: Props["selection"];
}) {
  const detailsRef = useRef<HTMLDetailsElement>(null);

  const toggleAll = useCallback((event: React.MouseEvent) => {
    event.preventDefault();
    event.stopPropagation();
    const el = detailsRef.current;
    if (!el) return;
    const nested = el.querySelectorAll("details");
    const allOpen = Array.from(nested).every((d) => d.open);
    for (const details of nested) {
      details.open = !allOpen;
    }
    el.open = !allOpen;
  }, []);

  const descendantCount = countDescendants(root.item.id, childrenBySource);

  return (
    <details
      className="project-context-tree"
      ref={detailsRef}
    >
      <summary>
        <TreeRow
          item={root.item}
          childCount={descendantCount}
          onToggleAll={descendantCount > 0 ? toggleAll : undefined}
          selection={selection}
        />
      </summary>
      <TreeNode
        item={root.item}
        childrenBySource={childrenBySource}
        path={[root.item.id]}
        selection={selection}
      />
    </details>
  );
}

function TreeNode({
  item,
  childrenBySource,
  path,
  selection,
}: {
  item: ProjectContextItem;
  childrenBySource: Map<string, TreeEdge[]>;
  path: string[];
  selection?: Props["selection"];
}) {
  const children = childrenBySource.get(item.id) ?? [];
  if (children.length === 0) {
    return null;
  }

  return (
    <ol className="project-context-tree-children">
      {children.map((childEdge) => {
        const loopsBack = path.includes(childEdge.target.id);
        const childCount = loopsBack
          ? 0
          : countDescendants(childEdge.target.id, childrenBySource);
        return (
          <li key={childEdge.id}>
            {loopsBack ? (
              <div className="project-context-tree-leaf project-context-tree-leaf--cycle">
                <TreeRow
                  item={childEdge.target}
                  edge={childEdge}
                  sourceTitle={item.title}
                  isCycle
                  selection={selection}
                />
              </div>
            ) : childCount > 0 ? (
              <details className="project-context-tree-branch" open={path.length < 2}>
                <summary>
                  <TreeRow
                    item={childEdge.target}
                    edge={childEdge}
                    sourceTitle={item.title}
                    childCount={childCount}
                    selection={selection}
                  />
                </summary>
                <TreeNode
                  item={childEdge.target}
                  childrenBySource={childrenBySource}
                  path={[...path, childEdge.target.id]}
                  selection={selection}
                />
              </details>
            ) : (
              <div className="project-context-tree-leaf">
                <TreeRow
                  item={childEdge.target}
                  edge={childEdge}
                  sourceTitle={item.title}
                  selection={selection}
                />
              </div>
            )}
          </li>
        );
      })}
    </ol>
  );
}

function TreeRow({
  item,
  edge,
  sourceTitle,
  childCount,
  isCycle = false,
  onToggleAll,
  selection,
}: {
  item: ProjectContextItem;
  edge?: ProjectContextEdge;
  sourceTitle?: string;
  childCount?: number;
  isCycle?: boolean;
  onToggleAll?: (event: React.MouseEvent) => void;
  selection?: Props["selection"];
}) {
  const resolvedCount = childCount ?? 0;
  const childLabel = resolvedCount === 1 ? "1 child" : `${resolvedCount} children`;

  return (
    <span
      className={
        edge
          ? "project-context-tree-row project-context-tree-row--child"
          : "project-context-tree-row"
      }
    >
      <span className="project-context-tree-row__marker" aria-hidden="true" />
      {selection ? (
        <label
          className="project-context-tree-select"
          onClick={(event) => event.stopPropagation()}
        >
          <input
            type="checkbox"
            checked={selection.selectedIds.has(item.id)}
            disabled={selection.disabled}
            onChange={() => selection.onToggle(item)}
          />
          <span className="visually-hidden">Select {item.title}</span>
        </label>
      ) : null}
      <span className="project-context-tree-row__main">
        <strong>{item.title}</strong>
        {edge && sourceTitle ? (
          <span
            className="project-context-tree-relationship"
            aria-label={`${sourceTitle} ${formatRelation(edge.relation)} ${item.title}`}
            style={relationToneStyle(edge)}
          >
            <span className="project-context-tree-relationship__source">
              <span>From</span>
              <strong>{sourceTitle}</strong>
            </span>
            <span className="project-context-tree-relationship__relation">
              {formatRelation(edge.relation)}
            </span>
            <span className="project-context-tree-relationship__target">
              <span>To</span>
              <strong>{item.title}</strong>
            </span>
          </span>
        ) : (
          null
        )}
        {isCycle ? <small>Already appears in this branch.</small> : null}
      </span>
      <span className="project-context-tree-row__chips">
        {onToggleAll ? (
          <button
            type="button"
            className="project-context-tree-chip project-context-tree-chip--toggle"
            onClick={onToggleAll}
          >
            Expand all
          </button>
        ) : null}
        <span
          className="project-context-tree-chip project-context-tree-chip--kind"
          data-kind-tone={projectContextKindTone(item.kind)}
        >
          {item.kind}
        </span>
        <span className="project-context-tree-chip project-context-tree-chip--count">
          {childLabel}
        </span>
      </span>
    </span>
  );
}

function buildProjectContextForest(
  items: ProjectContextItem[],
  edges: ProjectContextEdge[],
): ProjectContextForest {
  const itemByID = new Map(items.map((item) => [item.id, item]));
  const targetIDs = new Set<string>();
  const childrenBySource = new Map<string, TreeEdge[]>();

  for (const edge of edges) {
    const target = itemByID.get(edge.target_context_id);
    if (!target || !itemByID.has(edge.source_context_id)) {
      continue;
    }
    targetIDs.add(edge.target_context_id);
    const children = childrenBySource.get(edge.source_context_id) ?? [];
    children.push({ ...edge, target });
    childrenBySource.set(edge.source_context_id, children);
  }

  for (const children of childrenBySource.values()) {
    children.sort((left, right) => left.target.title.localeCompare(right.target.title));
  }

  const roots = items.filter((item) => !targetIDs.has(item.id));
  const reachable = new Set<string>();
  for (const root of roots) {
    markReachable(root.id, childrenBySource, reachable);
  }

  const cycleRoots = items.filter((item) => !reachable.has(item.id));
  const orderedRoots = [...roots, ...cycleRoots].map((item) => ({ item }));

  return Object.assign(orderedRoots, { childrenBySource });
}

function markReachable(
  itemID: string,
  childrenBySource: Map<string, TreeEdge[]>,
  reachable: Set<string>,
) {
  if (reachable.has(itemID)) return;
  reachable.add(itemID);
  for (const edge of childrenBySource.get(itemID) ?? []) {
    markReachable(edge.target.id, childrenBySource, reachable);
  }
}

function countDescendants(
  itemID: string,
  childrenBySource: Map<string, TreeEdge[]>,
  seen = new Set<string>(),
): number {
  if (seen.has(itemID)) return 0;
  seen.add(itemID);
  let count = 0;
  for (const edge of childrenBySource.get(itemID) ?? []) {
    count += 1 + countDescendants(edge.target.id, childrenBySource, new Set(seen));
  }
  return count;
}

function formatRelation(relation: ProjectContextEdge["relation"]) {
  const labels: Record<ProjectContextEdge["relation"], string> = {
    blocks: "Blocks",
    depends_on: "Depends on",
    refines: "Refines",
    related: "Related",
    supports: "Supports",
  };
  return labels[relation];
}

function relationToneStyle(edge: ProjectContextEdge): RelationToneStyle {
  const range = RELATION_HUE_RANGES[edge.relation];
  const offset = hashString(`${edge.id}:${edge.source_context_id}:${edge.target_context_id}`) % range.span;
  return {
    "--project-context-relation-hue": `${(range.start + offset) % 360}deg`,
  };
}

function hashString(value: string): number {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash;
}
