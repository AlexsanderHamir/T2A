import type { ProjectContextItem, ProjectContextKind } from "@/types";
import { ProjectContextItemEditor } from "./ProjectContextItemEditor";

type Props = {
  item: ProjectContextItem;
  saving: boolean;
  deleting: boolean;
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
};

export function ProjectContextNodeCard({
  item,
  saving,
  deleting,
  onSave,
  onDelete,
}: Props) {
  return (
    <article className="project-context-node-card">
      <div className="project-context-node-card__body">
        <div className="project-context-node-card__title-row">
          <h5>{item.title}</h5>
          <div className="project-context-node-card__badges">
            <span className="project-context-node-card__kind">{item.kind}</span>
            {item.pinned ? (
              <span className="project-context-node-card__pin">Pinned</span>
            ) : null}
          </div>
        </div>
        <p title={item.body}>{item.body}</p>
      </div>
      <ProjectContextItemEditor
        item={item}
        saving={saving}
        deleting={deleting}
        onSave={onSave}
        onDelete={onDelete}
      />
    </article>
  );
}
