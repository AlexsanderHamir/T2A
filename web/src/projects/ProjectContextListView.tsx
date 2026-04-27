import { useMemo, useState } from "react";
import type {
  ProjectContextItem,
  ProjectContextKind,
} from "@/types";
import { ProjectContextNodeCard } from "./ProjectContextNodeCard";

type Props = {
  items: ProjectContextItem[];
  nodeSaving: boolean;
  nodeDeleting: boolean;
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
  onAddConnection: (sourceId: string) => void;
};

export function ProjectContextListView({
  items,
  nodeSaving,
  nodeDeleting,
  onSaveNode,
  onDeleteNode,
  onAddConnection,
}: Props) {
  const [nodeQuery, setNodeQuery] = useState("");
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
  return (
    <div className="project-context-graph project-context-graph--nodes-only">
      <section className="project-context-graph__section">
        <div className="project-context-graph__section-heading">
          <div>
            <h4>Memory nodes</h4>
          </div>
          <span>{items.length}</span>
        </div>
        <label className="project-context-search">
          <span>Search memory nodes</span>
          <input
            value={nodeQuery}
            onChange={(event) => setNodeQuery(event.target.value)}
            placeholder="Title, body, or kind"
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
                canAddConnection={items.length >= 2}
                onSave={onSaveNode}
                onDelete={onDeleteNode}
                onAddConnection={onAddConnection}
              />
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
