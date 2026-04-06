import type { FormEvent } from "react";
import type { PriorityChoice } from "@/types";
import { FieldRequirementBadge } from "@/shared/FieldLabel";
import { Modal } from "../../shared/Modal";
import { TaskComposeFields } from "./TaskComposeFields";

type Props = {
  taskId: string;
  pending: boolean;
  saving: boolean;
  onClose: () => void;
  title: string;
  prompt: string;
  priority: PriorityChoice;
  checklistItems: string[];
  checklistInherit: boolean;
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onPriorityChange: (p: PriorityChoice) => void;
  onAppendChecklistCriterion: (text: string) => void;
  onUpdateChecklistRow: (index: number, text: string) => void;
  onRemoveChecklistRow: (index: number) => void;
  onChecklistInheritChange: (v: boolean) => void;
  onSubmit: (e: FormEvent) => void;
};

export function SubtaskCreateModal({
  taskId,
  pending,
  saving,
  onClose,
  title,
  prompt,
  priority,
  checklistItems,
  checklistInherit,
  onTitleChange,
  onPromptChange,
  onPriorityChange,
  onAppendChecklistCriterion,
  onUpdateChecklistRow,
  onRemoveChecklistRow,
  onChecklistInheritChange,
  onSubmit,
}: Props) {
  const disabled = pending || saving;

  return (
    <Modal
      onClose={onClose}
      labelledBy="subtask-create-title"
      size="wide"
      busy={pending}
      busyLabel="Creating subtask…"
    >
      <section className="panel modal-sheet modal-sheet--edit task-subtask-modal-sheet">
        <h2 id="subtask-create-title">New subtask</h2>
        <form
          className="task-subtask-modal-form task-create-form"
          onSubmit={onSubmit}
        >
          <TaskComposeFields
            idsPrefix="task-subtask-modal"
            editorKey={`subtask-modal-${taskId}`}
            title={title}
            prompt={prompt}
            priority={priority}
            checklistItems={checklistItems}
            hideChecklist={checklistInherit}
            disabled={disabled}
            onTitleChange={onTitleChange}
            onPromptChange={onPromptChange}
            onPriorityChange={onPriorityChange}
            onAppendChecklistCriterion={onAppendChecklistCriterion}
            onUpdateChecklistRow={onUpdateChecklistRow}
            onRemoveChecklistRow={onRemoveChecklistRow}
          />
          <label className="checkbox-label task-subtask-inherit">
            <input
              type="checkbox"
              checked={checklistInherit}
              onChange={(ev) => onChecklistInheritChange(ev.target.checked)}
              disabled={disabled}
            />
            <span className="checkbox-label-body">
              <span>Inherit this task&apos;s checklist criteria</span>
              <FieldRequirementBadge requirement="optional" />
            </span>
          </label>
          <div className="row stack-row-actions task-subtask-modal-actions">
            <button
              type="button"
              className="secondary"
              disabled={disabled}
              onClick={onClose}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="task-create-submit"
              disabled={!title.trim() || !priority || disabled}
            >
              Add subtask
            </button>
          </div>
        </form>
      </section>
    </Modal>
  );
}
