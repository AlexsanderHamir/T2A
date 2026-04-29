import type { ProjectContextItem, ProjectContextKind } from "@/types";
import { ProjectContextItemEditor } from "./ProjectContextItemEditor";
import { projectContextKindTone } from "./projectContextKindTone";

const NODE_HUES = [
  "248, 63%",
  "160, 60%",
  "330, 55%",
  "38, 75%",
  "200, 65%",
  "280, 50%",
  "15, 65%",
  "175, 55%",
];

type Props = {
  item: ProjectContextItem;
  index: number;
  saving: boolean;
  deleting: boolean;
  canAddConnection?: boolean;
  selected?: boolean;
  selectionDisabled?: boolean;
  onSave: (
    id: string,
    patch: {
      kind: ProjectContextKind;
      title: string;
      body: string;
      pinned: boolean;
    },
  ) => void;
  onDelete: (id: string) => void;
  onAddConnection?: (sourceId: string) => void;
  onToggleSelected?: (item: ProjectContextItem) => void;
};

export function ProjectContextNodeCard({
  item,
  index,
  saving,
  deleting,
  canAddConnection = false,
  selected = false,
  selectionDisabled = false,
  onSave,
  onDelete,
  onAddConnection,
  onToggleSelected,
}: Props) {
  const hue = NODE_HUES[index % NODE_HUES.length];
  const nodeClass = onToggleSelected ? "pc__node pc__node--selectable" : "pc__node";
  return (
    <article
      className={nodeClass}
      style={{ "--pc-hue": hue, animationDelay: `${index * 30}ms` } as React.CSSProperties}
    >
      <div className="pc__node-marker" aria-hidden="true" />
      {onToggleSelected ? (
        <label className="pc__node-select">
          <input
            type="checkbox"
            checked={selected}
            disabled={selectionDisabled}
            onChange={() => onToggleSelected(item)}
          />
          <span className="visually-hidden">Select {item.title}</span>
        </label>
      ) : null}
      <div className="pc__node-body">
        <h5 className="pc__node-title">{item.title}</h5>
        <span className="pc__node-source">
          {item.created_by === "agent" ? "Agent" : "User"}
          {item.pinned ? " · Pinned" : ""}
        </span>
      </div>
      <span
        className="pc__node-kind"
        data-kind-tone={projectContextKindTone(item.kind)}
      >
        {item.kind}
      </span>
      {!onToggleSelected ? (
        <div className="pc__node-actions">
        {canAddConnection ? (
          <button
            type="button"
            className="pc__btn-ghost"
            onClick={() => onAddConnection?.(item.id)}
          >
            Link
          </button>
        ) : null}
        <ProjectContextItemEditor
          item={item}
          saving={saving}
          deleting={deleting}
          onSave={onSave}
          onDelete={onDelete}
        />
        </div>
      ) : null}
    </article>
  );
}
