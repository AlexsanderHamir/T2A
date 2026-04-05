import type { FormEvent } from "react";
import { STATUSES, type Priority, type Status } from "@/types";
import { FieldLabel, FieldRequirementBadge } from "@/shared/FieldLabel";
import { Modal } from "../../shared/Modal";
import { PrioritySelect } from "./PrioritySelect";
import { RichPromptEditor } from "./RichPromptEditor";

type Props = {
  taskId: string;
  title: string;
  prompt: string;
  priority: Priority;
  status: Status;
  checklistInherit: boolean;
  /** When false, the inherit checkbox is disabled (task has no parent). */
  canInheritChecklist: boolean;
  saving: boolean;
  patchPending: boolean;
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onPriorityChange: (p: Priority) => void;
  onStatusChange: (s: Status) => void;
  onChecklistInheritChange: (v: boolean) => void;
  onSubmit: (e: FormEvent) => void;
  onCancel: () => void;
};

export function TaskEditForm({
  taskId,
  title,
  prompt,
  priority,
  status,
  checklistInherit,
  canInheritChecklist,
  saving,
  patchPending,
  onTitleChange,
  onPromptChange,
  onPriorityChange,
  onStatusChange,
  onChecklistInheritChange,
  onSubmit,
  onCancel,
}: Props) {
  return (
    <Modal
      onClose={onCancel}
      labelledBy="edit-dialog-title"
      describedBy="edit-dialog-description"
      size="wide"
      busy={patchPending}
    >
      <section className="panel modal-sheet modal-sheet--edit">
        <h2 id="edit-dialog-title">Edit task</h2>
        <form onSubmit={(e) => void onSubmit(e)}>
          <p className="muted stack-tight-zero" id="edit-dialog-description">
            <code>{taskId}</code>
          </p>
          <div className="row">
            <div className="field grow">
              <FieldLabel htmlFor="task-edit-title" requirement="required">
                Title
              </FieldLabel>
              <input
                id="task-edit-title"
                value={title}
                onChange={(ev) => onTitleChange(ev.target.value)}
                required
                aria-required="true"
              />
            </div>
            <PrioritySelect
              id="task-edit-priority"
              value={priority}
              allowUnset={false}
              onChange={(p) => {
                if (p !== "") onPriorityChange(p);
              }}
            />
          </div>
          <div className="field grow">
            <FieldLabel htmlFor="task-edit-status" requirement="required">
              Status
            </FieldLabel>
            <select
              aria-required="true"
              id="task-edit-status"
              value={status}
              onChange={(ev) => onStatusChange(ev.target.value as Status)}
            >
              {STATUSES.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </div>
          <div className="field grow stack-tight checkbox-field">
            <label className="checkbox-label">
              <input
                type="checkbox"
                checked={checklistInherit}
                disabled={!canInheritChecklist || saving}
                onChange={(ev) => onChecklistInheritChange(ev.target.checked)}
              />
              <span className="checkbox-label-body">
                <span>Use parent&apos;s checklist (inherit completion criteria)</span>
                <FieldRequirementBadge requirement="optional" />
              </span>
            </label>
            {!canInheritChecklist ? (
              <p className="muted stack-tight-zero">
                Only tasks with a parent can inherit its checklist.
              </p>
            ) : null}
          </div>
          <div className="field grow stack-tight prompt-field-full">
            <FieldLabel
              id="task-edit-prompt-label"
              htmlFor="task-edit-prompt"
              requirement="optional"
            >
              Initial prompt
            </FieldLabel>
            <RichPromptEditor
              key={taskId}
              id="task-edit-prompt"
              value={prompt}
              onChange={onPromptChange}
              disabled={saving}
              placeholder="Use the toolbar for headings and bold. Type @ to pick a file from the repo."
            />
          </div>
          <div className="row stack-row-actions">
            <button type="submit" disabled={saving}>
              Save
            </button>
            <button
              type="button"
              className="secondary"
              disabled={saving}
              onClick={onCancel}
            >
              Cancel
            </button>
          </div>
        </form>
      </section>
    </Modal>
  );
}
