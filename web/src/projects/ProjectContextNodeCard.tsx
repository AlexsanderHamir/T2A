import type { ProjectContextItem, ProjectContextKind } from "@/types";
import { previewTextFromPrompt } from "@/tasks/task-prompt";
import { ProjectContextItemEditor } from "./ProjectContextItemEditor";
import { projectContextKindTone } from "./projectContextKindTone";

type Props = {
  item: ProjectContextItem;
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
  saving,
  deleting,
  canAddConnection = false,
  onSave,
  onDelete,
  onAddConnection,
}: Props) {
  const preview = previewTextFromPrompt(item.body);

  return (
    <article className="project-context-node-card">
      <div className="project-context-node-card__body">
        <div className="project-context-node-card__title-block">
          <div className="project-context-node-card__eyebrow">
            <span>{item.created_by === "agent" ? "Agent memory" : "User memory"}</span>
            {item.pinned ? <span>Pinned</span> : null}
          </div>
          <h5>{item.title}</h5>
          {preview ? <p>{preview}</p> : <p className="project-context-node-card__muted">No details yet</p>}
        </div>
      </div>
      <span
        className="project-context-node-card__kind"
        data-kind-tone={projectContextKindTone(item.kind)}
      >
        {item.kind}
      </span>
      <div className="project-context-node-card__actions">
        {canAddConnection ? (
          <button
            type="button"
            className="project-context-node-card__action-button project-context-node-card__action-button--primary"
            onClick={() => onAddConnection?.(item.id)}
          >
            Add connection
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
