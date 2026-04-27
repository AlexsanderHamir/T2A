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
  const resultLabel =
    nodeQuery.trim().length > 0
      ? `${filteredItems.length} of ${items.length}`
      : `${items.length}`;

  return (
    <div className="pc__list-view">
      <div className="pc__list-bar">
        <label className="pc__search">
          <span className="visually-hidden">Search memory nodes</span>
          <svg className="pc__search-icon" width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
            <circle cx="6" cy="6" r="4.5" stroke="currentColor" strokeWidth="1.2" />
            <path d="M9.5 9.5L13 13" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round" />
          </svg>
          <input
            value={nodeQuery}
            onChange={(event) => setNodeQuery(event.target.value)}
            placeholder="Search by title, body, or kind..."
          />
        </label>
        <span className="pc__count">{resultLabel}</span>
      </div>

      {filteredItems.length === 0 ? (
        <div className="pc__empty">
          <p>No matching nodes</p>
          <span>Try a different search term or clear the filter.</span>
        </div>
      ) : (
        <div className="pc__node-grid">
          {filteredItems.map((item, i) => (
            <ProjectContextNodeCard
              key={item.id}
              item={item}
              index={i}
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
    </div>
  );
}
