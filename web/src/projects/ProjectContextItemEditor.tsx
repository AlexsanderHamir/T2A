import type { FormEvent } from "react";
import {
  PROJECT_CONTEXT_KINDS,
  type ProjectContextItem,
  type ProjectContextKind,
} from "@/types";

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

export function ProjectContextItemEditor({
  item,
  saving,
  deleting,
  onSave,
  onDelete,
}: Props) {
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    onSave(item.id, {
      kind: String(form.get("kind") ?? "note") as ProjectContextKind,
      title: String(form.get("title") ?? "").trim(),
      body: String(form.get("body") ?? "").trim(),
      pinned: form.get("pinned") === "on",
    });
  }

  return (
    <details>
      <summary>Edit context item</summary>
      <form className="project-context-item-form" onSubmit={submit}>
        <div className="field grow">
          <label htmlFor={`context-kind-${item.id}`}>Kind</label>
          <select
            id={`context-kind-${item.id}`}
            name="kind"
            defaultValue={item.kind}
          >
            {PROJECT_CONTEXT_KINDS.map((kind) => (
              <option key={kind} value={kind}>
                {kind}
              </option>
            ))}
          </select>
        </div>
        <div className="field grow">
          <label htmlFor={`context-title-${item.id}`}>Title</label>
          <input
            id={`context-title-${item.id}`}
            name="title"
            defaultValue={item.title}
            required
          />
        </div>
        <div className="field grow">
          <label htmlFor={`context-body-${item.id}`}>Body</label>
          <textarea
            id={`context-body-${item.id}`}
            name="body"
            defaultValue={item.body}
            rows={4}
            required
          />
        </div>
        <label className="checkbox-label">
          <input type="checkbox" name="pinned" defaultChecked={item.pinned} />
          <span>Pinned</span>
        </label>
        <div className="row stack-row-actions">
          <button type="submit" disabled={saving}>
            Save item
          </button>
          <button
            type="button"
            className="secondary"
            disabled={deleting}
            onClick={() => onDelete(item.id)}
          >
            Delete
          </button>
        </div>
      </form>
    </details>
  );
}
