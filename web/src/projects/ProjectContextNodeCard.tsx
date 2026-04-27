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
};

export function ProjectContextNodeCard({
  item,
  index,
  saving,
  deleting,
  canAddConnection = false,
  onSave,
  onDelete,
  onAddConnection,
}: Props) {
  const hue = NODE_HUES[index % NODE_HUES.length];
  return (
    <article
      className="pc__node"
      style={{ "--pc-hue": hue, animationDelay: `${index * 30}ms` } as React.CSSProperties}
    >
      <div className="pc__node-marker" aria-hidden="true" />
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
    </article>
  );
}
