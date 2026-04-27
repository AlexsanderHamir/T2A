import { useMemo } from "react";
import { previewTextFromPrompt } from "@/tasks/task-prompt";
import type { ProjectContextEdge, ProjectContextItem } from "@/types";

type Props = {
  items: ProjectContextItem[];
  edges: ProjectContextEdge[];
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

export function ProjectContextTreeView({ items, edges }: Props) {
  const forest = useMemo(() => buildProjectContextForest(items, edges), [edges, items]);

  if (items.length === 0) {
    return null;
  }

  return (
    <div className="project-context-tree-view" aria-label="Project context trees">
      {forest.map((root) => (
        <article className="project-context-tree" key={root.item.id}>
          <TreeNode
            item={root.item}
            childrenBySource={forest.childrenBySource}
            path={[root.item.id]}
          />
        </article>
      ))}
    </div>
  );
}

function TreeNode({
  item,
  childrenBySource,
  path,
  edge,
}: {
  item: ProjectContextItem;
  childrenBySource: Map<string, TreeEdge[]>;
  path: string[];
  edge?: ProjectContextEdge;
}) {
  const children = childrenBySource.get(item.id) ?? [];
  const preview = previewTextFromPrompt(item.body);

  return (
    <div className="project-context-tree-node">
      {edge ? (
        <div className="project-context-tree-edge">
          <span>{formatRelation(edge.relation)}</span>
        </div>
      ) : null}
      <div className="project-context-tree-card">
        <div>
          <strong>{item.title}</strong>
          <p>{preview}</p>
        </div>
        <span className="project-context-node-card__kind">{item.kind}</span>
      </div>
      {children.length > 0 ? (
        <ol className="project-context-tree-children">
          {children.map((childEdge) => {
            const loopsBack = path.includes(childEdge.target.id);
            return (
              <li key={childEdge.id}>
                {loopsBack ? (
                  <div className="project-context-tree-node">
                    <div className="project-context-tree-edge">
                      <span>{formatRelation(childEdge.relation)}</span>
                    </div>
                    <div className="project-context-tree-card project-context-tree-card--cycle">
                      <div>
                        <strong>{childEdge.target.title}</strong>
                        <p>Already appears in this branch.</p>
                      </div>
                      <span className="project-context-node-card__kind">
                        {childEdge.target.kind}
                      </span>
                    </div>
                  </div>
                ) : (
                  <TreeNode
                    item={childEdge.target}
                    childrenBySource={childrenBySource}
                    path={[...path, childEdge.target.id]}
                    edge={childEdge}
                  />
                )}
              </li>
            );
          })}
        </ol>
      ) : null}
    </div>
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

function formatRelation(relation: ProjectContextEdge["relation"]) {
  return relation.replace("_", " ");
}
