import type { ProjectContextItem, ProjectContextKind } from "@/types";
import { previewTextFromPrompt } from "@/tasks/task-prompt";
import { ProjectContextItemEditor } from "./ProjectContextItemEditor";

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
        <div className="project-context-node-card__title-row">
          <h5>{item.title}</h5>
          <div className="project-context-node-card__badges">
            <span className="project-context-node-card__kind">{item.kind}</span>
          </div>
        </div>
        <p title={preview}>{preview}</p>
      </div>
      <div className="project-context-node-card__actions">
        {canAddConnection ? (
          <button
            type="button"
            className="project-context-node-card__edit"
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
