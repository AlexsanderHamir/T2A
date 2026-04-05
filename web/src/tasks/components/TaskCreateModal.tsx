import { useCallback, useState, type FormEvent } from "react";
import type { PriorityChoice } from "@/types";
import { FieldLabel, FieldRequirementBadge } from "@/shared/FieldLabel";
import type { TaskWithDepth } from "../flattenTaskTree";
import type { PendingSubtaskDraft } from "../pendingSubtaskDraft";
import { Modal } from "../../shared/Modal";
import { NestedSubtaskDraftModal } from "./NestedSubtaskDraftModal";
import { TaskComposeFields } from "./TaskComposeFields";

type Props = {
  pending: boolean;
  saving: boolean;
  onClose: () => void;
  title: string;
  prompt: string;
  priority: PriorityChoice;
  checklistItems: string[];
  parentOptions: TaskWithDepth[];
  parentId: string;
  checklistInherit: boolean;
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onPriorityChange: (p: PriorityChoice) => void;
  onParentIdChange: (id: string) => void;
  onChecklistInheritChange: (v: boolean) => void;
  onAppendChecklistCriterion: (text: string) => void;
  onRemoveChecklistRow: (index: number) => void;
  pendingSubtasks: PendingSubtaskDraft[];
  onAddPendingSubtask: (d: PendingSubtaskDraft) => void;
  onUpdatePendingSubtask: (index: number, d: PendingSubtaskDraft) => void;
  onRemovePendingSubtask: (index: number) => void;
  onSubmit: (e: FormEvent) => void;
};

export function TaskCreateModal({
  pending,
  saving,
  onClose,
  title,
  prompt,
  priority,
  checklistItems,
  parentOptions,
  parentId,
  checklistInherit,
  onTitleChange,
  onPromptChange,
  onPriorityChange,
  onParentIdChange,
  onChecklistInheritChange,
  onAppendChecklistCriterion,
  onRemoveChecklistRow,
  pendingSubtasks,
  onAddPendingSubtask,
  onUpdatePendingSubtask,
  onRemovePendingSubtask,
  onSubmit,
}: Props) {
  const disabled = pending || saving;
  const hasParent = Boolean(parentId.trim());
  const hideComposeChecklist = hasParent && checklistInherit;
  const subtasksHeadingId = "task-new-subtasks-heading";

  const [nestedOpen, setNestedOpen] = useState(false);
  const [nestedEditIndex, setNestedEditIndex] = useState<number | null>(null);
  const [nestedInstanceKey, setNestedInstanceKey] = useState(0);
  const [nestedInitial, setNestedInitial] = useState<PendingSubtaskDraft | null>(
    null,
  );

  const openNestedNew = useCallback(() => {
    setNestedEditIndex(null);
    setNestedInitial(null);
    setNestedInstanceKey((k) => k + 1);
    setNestedOpen(true);
  }, []);

  const openNestedEdit = useCallback(
    (index: number) => {
      const d = pendingSubtasks[index];
      setNestedEditIndex(index);
      setNestedInitial({
        title: d.title,
        initial_prompt: d.initial_prompt,
        priority: d.priority,
        checklistItems: [...d.checklistItems],
        checklist_inherit: d.checklist_inherit,
      });
      setNestedInstanceKey((k) => k + 1);
      setNestedOpen(true);
    },
    [pendingSubtasks],
  );

  const handleNestedClose = useCallback(() => {
    setNestedOpen(false);
  }, []);

  const handleNestedSave = useCallback(
    (d: PendingSubtaskDraft) => {
      if (nestedEditIndex !== null) {
        onUpdatePendingSubtask(nestedEditIndex, d);
      } else {
        onAddPendingSubtask(d);
      }
      setNestedOpen(false);
    },
    [nestedEditIndex, onAddPendingSubtask, onUpdatePendingSubtask],
  );

  const busyLabel = hasParent
    ? "Creating subtask…"
    : pendingSubtasks.length > 0
      ? "Creating task and subtasks…"
      : "Creating task…";

  return (
    <>
      <Modal
        onClose={onClose}
        labelledBy="task-create-modal-title"
        size="wide"
        busy={pending}
        busyLabel={busyLabel}
      >
        <section className="panel modal-sheet modal-sheet--edit task-create-modal-sheet task-create">
          <h2 id="task-create-modal-title">
            {hasParent ? "New subtask" : "New task"}
          </h2>
          <form
            className="task-create-modal-form task-create-form"
            onSubmit={onSubmit}
          >
            <div className="field grow task-create-parent-field">
              <FieldLabel htmlFor="task-new-parent" requirement="optional">
                Parent task
              </FieldLabel>
              <select
                id="task-new-parent"
                value={parentId}
                onChange={(ev) => onParentIdChange(ev.target.value)}
                disabled={disabled}
              >
                <option value="">None — top-level task</option>
                {parentOptions.map((t) => (
                  <option key={t.id} value={t.id}>
                    {"— ".repeat(t.depth)}
                    {t.title}
                  </option>
                ))}
              </select>
              <p className="task-create-parent-hint muted">
                {hasParent ? (
                  <>
                    Full details for this subtask — prompt, priority, and
                    optional done criteria (or inherit from the parent&apos;s
                    checklist).
                  </>
                ) : (
                  <>
                    Leave as top-level for a root task (full form below).
                    Choose a parent to create a <strong>subtask</strong>. The
                    list includes subtasks so you can nest under any task on
                    this page (the home table shows top-level tasks only).
                  </>
                )}
              </p>
            </div>

            <TaskComposeFields
              idsPrefix="task-new"
              editorKey="create-prompt-modal"
              title={title}
              prompt={prompt}
              priority={priority}
              checklistItems={checklistItems}
              hideChecklist={hideComposeChecklist}
              disabled={disabled}
              onTitleChange={onTitleChange}
              onPromptChange={onPromptChange}
              onPriorityChange={onPriorityChange}
              onAppendChecklistCriterion={onAppendChecklistCriterion}
              onRemoveChecklistRow={onRemoveChecklistRow}
            />

            {hasParent ? (
              <label className="checkbox-label task-create-inherit-field">
                <input
                  type="checkbox"
                  checked={checklistInherit}
                  onChange={(ev) =>
                    onChecklistInheritChange(ev.target.checked)
                  }
                  disabled={disabled}
                />
                <span className="checkbox-label-body">
                  <span>Inherit parent&apos;s checklist criteria</span>
                  <FieldRequirementBadge requirement="optional" />
                </span>
              </label>
            ) : null}

            {!hasParent ? (
              <div className="task-create-subtasks">
                <div className="field-heading-with-req task-create-subtasks-heading-row">
                  <h3
                    className="task-create-subtasks-heading"
                    id={subtasksHeadingId}
                  >
                    Subtasks
                  </h3>
                  <FieldRequirementBadge requirement="optional" />
                </div>
                <p className="task-create-subtasks-hint">
                  Optional — use <strong>New subtask</strong> for each child; it
                  opens the same detailed form in a second window. Nothing is
                  saved until you choose Create here.
                </p>
                {pendingSubtasks.length > 0 ? (
                  <ul
                    className="task-checklist-list"
                    aria-labelledby={subtasksHeadingId}
                  >
                    {pendingSubtasks.map((d, index) => (
                      <li
                        key={`${index}-${d.title}`}
                        className="task-checklist-row task-create-pending-subtask-row"
                      >
                        <span className="task-checklist-label">{d.title}</span>
                        <div className="task-create-pending-subtask-actions">
                          <button
                            type="button"
                            className="task-detail-checklist-add-btn"
                            disabled={disabled}
                            onClick={() => openNestedEdit(index)}
                          >
                            Edit
                          </button>
                          <button
                            type="button"
                            className="task-create-checklist-remove"
                            disabled={disabled}
                            onClick={() => onRemovePendingSubtask(index)}
                          >
                            Remove
                          </button>
                        </div>
                      </li>
                    ))}
                  </ul>
                ) : null}
                <button
                  type="button"
                  className="task-detail-add-subtask-btn task-create-open-nested-subtask"
                  disabled={disabled}
                  aria-label="Open form to add a subtask"
                  onClick={openNestedNew}
                >
                  New subtask
                </button>
              </div>
            ) : null}

            <div className="row stack-row-actions task-create-modal-actions">
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
                {hasParent ? "Add subtask" : "Create"}
              </button>
            </div>
          </form>
        </section>
      </Modal>

      {nestedOpen ? (
        <NestedSubtaskDraftModal
          instanceKey={nestedInstanceKey}
          initialDraft={nestedInitial}
          onClose={handleNestedClose}
          onSave={handleNestedSave}
        />
      ) : null}
    </>
  );
}
