import { useMemo, useState } from "react";
import type {
  ProjectContextEdge,
  ProjectContextItem,
  ProjectContextKind,
  ProjectContextRelation,
} from "@/types";
import { ProjectContextEdgeEditor } from "./ProjectContextEdgeEditor";
import { ProjectContextNodeCard } from "./ProjectContextNodeCard";

type Props = {
  items: ProjectContextItem[];
  edges: ProjectContextEdge[];
  nodeSaving: boolean;
  nodeDeleting: boolean;
  edgeSaving: boolean;
  edgeDeleting: boolean;
  onSaveNode: (
    id: string,
    patch: {
      kind: ProjectContextKind;
      title: string;
      body: string;
      pinned: boolean;
    },
  ) => void;
  onDeleteNode: (id: string) => void;
  onSaveEdge: (
    id: string,
    patch: {
      relation: ProjectContextRelation;
      strength: number;
      note: string;
    },
  ) => void;
  onDeleteEdge: (id: string) => void;
};

export function ProjectContextListView({
  items,
  edges,
  nodeSaving,
  nodeDeleting,
  edgeSaving,
  edgeDeleting,
  onSaveNode,
  onDeleteNode,
  onSaveEdge,
  onDeleteEdge,
}: Props) {
  const [nodeQuery, setNodeQuery] = useState("");
  const [connectionQuery, setConnectionQuery] = useState("");
  const itemTitleByID = useMemo(() => {
    return new Map(items.map((item) => [item.id, item.title]));
  }, [items]);
  const filteredItems = useMemo(() => {
    const query = nodeQuery.trim().toLowerCase();
    if (!query) return items;
    return items.filter((item) =>
      [item.title, item.body, item.kind]
        .join(" ")
        .toLowerCase()
        .includes(query),
    );
  }, [items, nodeQuery]);
  const filteredEdges = useMemo(() => {
    const query = connectionQuery.trim().toLowerCase();
    if (!query) return edges;
    return edges.filter((edge) =>
      [
        itemTitleByID.get(edge.source_context_id) ?? "",
        itemTitleByID.get(edge.target_context_id) ?? "",
        edge.relation,
        edge.note,
        String(edge.strength),
      ]
        .join(" ")
        .toLowerCase()
        .includes(query),
    );
  }, [connectionQuery, edges, itemTitleByID]);

  return (
    <div className="project-context-graph">
      <section className="project-context-graph__section">
        <div className="project-context-graph__section-heading">
          <div>
            <h4>Memory nodes</h4>
            <p>
              Durable facts, decisions, constraints, and handoff notes owned by
              this project.
            </p>
          </div>
          <span>{items.length}</span>
        </div>
        <label className="project-context-search">
          <span>Search nodes</span>
          <input
            value={nodeQuery}
            onChange={(event) => setNodeQuery(event.target.value)}
            placeholder="Filter by title, body, or kind"
          />
        </label>
        {filteredItems.length === 0 ? (
          <div className="project-context-empty-card">
            <strong>No matching nodes</strong>
            <p>Try a different search term or clear the filter.</p>
          </div>
        ) : (
          <div className="project-context-node-grid">
            {filteredItems.map((item) => (
              <ProjectContextNodeCard
                key={item.id}
                item={item}
                saving={nodeSaving}
                deleting={nodeDeleting}
                onSave={onSaveNode}
                onDelete={onDeleteNode}
              />
            ))}
          </div>
        )}
      </section>
      <section className="project-context-graph__section">
        <div className="project-context-graph__section-heading">
          <div>
            <h4>Connections</h4>
            <p>Relationships that explain how selected nodes influence each other.</p>
          </div>
          <span>{edges.length}</span>
        </div>
        <label className="project-context-search">
          <span>Search connections</span>
          <input
            value={connectionQuery}
            onChange={(event) => setConnectionQuery(event.target.value)}
            placeholder="Filter by node, relation, note, or strength"
          />
        </label>
        {edges.length === 0 ? (
          <div className="project-context-empty-card">
            <strong>No connections yet</strong>
            <p>
              Add a connection when two nodes support, block, refine, or depend
              on each other.
            </p>
          </div>
        ) : filteredEdges.length === 0 ? (
          <div className="project-context-empty-card">
            <strong>No matching connections</strong>
            <p>Try a different search term or clear the filter.</p>
          </div>
        ) : (
          <div className="project-context-edge-list">
            {filteredEdges.map((edge) => (
              <ProjectContextEdgeEditor
                key={edge.id}
                edge={edge}
                items={items}
                saving={edgeSaving}
                deleting={edgeDeleting}
                onSave={onSaveEdge}
                onDelete={onDeleteEdge}
              />
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
