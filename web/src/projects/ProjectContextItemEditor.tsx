import { useEffect, useState, type FormEvent } from "react";
import { FieldLabel } from "@/shared/FieldLabel";
import { RichPromptEditor } from "@/tasks/components/rich-prompt";
import { promptHasVisibleContent } from "@/tasks/task-prompt";
import { type ProjectContextItem, type ProjectContextKind } from "@/types";
import { ProjectContextKindPicker } from "./ProjectContextKindPicker";

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
  const [body, setBody] = useState(item.body);

  useEffect(() => {
    setBody(item.body);
  }, [item.body]);

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const nextBody = body.trim();
    if (!promptHasVisibleContent(nextBody)) return;
    onSave(item.id, {
      kind: String(form.get("kind") ?? "note") as ProjectContextKind,
      title: String(form.get("title") ?? "").trim(),
      body: nextBody,
      pinned: false,
    });
  }

  return (
    <details>
      <summary>Edit node</summary>
      <form className="project-context-item-form" onSubmit={submit}>
        <ProjectContextKindPicker
          idPrefix={`context-kind-${item.id}`}
          defaultValue={item.kind}
          disabled={saving || deleting}
        />
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
          <FieldLabel
            id={`context-body-${item.id}-label`}
            htmlFor={`context-body-${item.id}`}
          >
            Body
          </FieldLabel>
          <div className="project-context-editor-shell">
            <RichPromptEditor
              id={`context-body-${item.id}`}
              value={body}
              onChange={setBody}
              disabled={saving || deleting}
              placeholder="Write markdown-style context. Type @ to reference a repo file."
            />
          </div>
        </div>
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
