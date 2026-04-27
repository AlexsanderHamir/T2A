import { useEffect, useState, type FormEvent } from "react";
import { FieldLabel } from "@/shared/FieldLabel";
import { Modal } from "@/shared/Modal";
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
  const [isOpen, setIsOpen] = useState(false);

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
    <>
      <button
        type="button"
        className="project-context-node-card__edit"
        onClick={() => setIsOpen(true)}
      >
        Edit node
      </button>
      {isOpen ? (
        <Modal
          onClose={() => setIsOpen(false)}
          labelledBy={`context-edit-title-${item.id}`}
          describedBy={`context-edit-description-${item.id}`}
          size="wide"
          busy={saving || deleting}
          busyLabel={deleting ? "Deleting node..." : "Saving node..."}
          dismissibleWhileBusy
        >
          <form
            className="panel modal-sheet modal-sheet--edit project-context-item-form project-context-item-modal"
            onSubmit={submit}
          >
            <div className="project-context-form__heading">
              <div>
                <h2 id={`context-edit-title-${item.id}`}>Edit node</h2>
                <p id={`context-edit-description-${item.id}`} className="muted">
                  Update this project memory node.
                </p>
              </div>
            </div>
            <ProjectContextKindPicker
              idPrefix={`context-kind-${item.id}`}
              defaultValue={item.kind}
              disabled={saving || deleting}
            />
            <div className="field grow">
              <FieldLabel
                htmlFor={`context-title-${item.id}`}
                requirement="required"
              >
                Title
              </FieldLabel>
              <input
                id={`context-title-${item.id}`}
                name="title"
                defaultValue={item.title}
                required
                aria-required="true"
              />
            </div>
            <div className="field grow">
              <FieldLabel
                id={`context-body-${item.id}-label`}
                htmlFor={`context-body-${item.id}`}
                requirement="required"
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
              <button
                type="button"
                className="secondary"
                disabled={saving || deleting}
                onClick={() => setIsOpen(false)}
              >
                Cancel
              </button>
            </div>
          </form>
        </Modal>
      ) : null}
    </>
  );
}
