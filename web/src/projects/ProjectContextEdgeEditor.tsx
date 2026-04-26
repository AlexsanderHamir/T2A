import { useEffect, useState, type FormEvent } from "react";
import { FieldLabel } from "@/shared/FieldLabel";
import { RichPromptEditor } from "@/tasks/components/rich-prompt";
import { previewTextFromPrompt } from "@/tasks/task-prompt";
import {
  PROJECT_CONTEXT_RELATIONS,
  type ProjectContextEdge,
  type ProjectContextItem,
  type ProjectContextRelation,
} from "@/types";

type Props = {
  edge: ProjectContextEdge;
  items: ProjectContextItem[];
  saving: boolean;
  deleting: boolean;
  onSave: (
    id: string,
    patch: {
      relation: ProjectContextRelation;
      strength: number;
      note: string;
    },
  ) => void;
  onDelete: (id: string) => void;
};

export function ProjectContextEdgeEditor({
  edge,
  items,
  saving,
  deleting,
  onSave,
  onDelete,
}: Props) {
  const source = items.find((item) => item.id === edge.source_context_id);
  const target = items.find((item) => item.id === edge.target_context_id);
  const [note, setNote] = useState(edge.note);
  const notePreview = previewTextFromPrompt(edge.note);

  useEffect(() => {
    setNote(edge.note);
  }, [edge.note]);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    onSave(edge.id, {
      relation: String(
        form.get("relation") ?? "related",
      ) as ProjectContextRelation,
      strength: Number(form.get("strength") ?? edge.strength),
      note: note.trim(),
    });
  }

  return (
    <article className="project-context-edge">
      <div className="project-context-edge__summary">
        <span>{source?.title ?? "Unknown node"}</span>
        <strong>{formatRelation(edge.relation)}</strong>
        <span>{target?.title ?? "Unknown node"}</span>
      </div>
      <p className="muted">
        Strength {edge.strength}/5{notePreview ? ` - ${notePreview}` : ""}
      </p>
      <details>
        <summary>Edit connection</summary>
        <form className="project-context-edge-form" onSubmit={submit}>
          <div className="row">
            <div className="field grow">
              <label htmlFor={`context-edge-relation-${edge.id}`}>Relation</label>
              <select
                id={`context-edge-relation-${edge.id}`}
                name="relation"
                defaultValue={edge.relation}
              >
                {PROJECT_CONTEXT_RELATIONS.map((relation) => (
                  <option key={relation} value={relation}>
                    {formatRelation(relation)}
                  </option>
                ))}
              </select>
            </div>
            <div className="field">
              <label htmlFor={`context-edge-strength-${edge.id}`}>Strength</label>
              <select
                id={`context-edge-strength-${edge.id}`}
                name="strength"
                defaultValue={edge.strength}
              >
                {[1, 2, 3, 4, 5].map((strength) => (
                  <option key={strength} value={strength}>
                    {strength}
                  </option>
                ))}
              </select>
            </div>
          </div>
          <div className="field grow">
            <FieldLabel
              id={`context-edge-note-${edge.id}-label`}
              htmlFor={`context-edge-note-${edge.id}`}
            >
              Note
            </FieldLabel>
            <div className="project-context-editor-shell">
              <RichPromptEditor
                id={`context-edge-note-${edge.id}`}
                value={note}
                onChange={setNote}
                disabled={saving || deleting}
                placeholder="Why are these nodes connected? Type @ to reference a repo file."
              />
            </div>
          </div>
          <div className="row stack-row-actions">
            <button type="submit" disabled={saving}>
              Save connection
            </button>
            <button
              type="button"
              className="secondary"
              disabled={deleting}
              onClick={() => onDelete(edge.id)}
            >
              Delete
            </button>
          </div>
        </form>
      </details>
    </article>
  );
}

function formatRelation(relation: ProjectContextRelation): string {
  return relation.replace("_", " ");
}
