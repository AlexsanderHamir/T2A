import type { FormEvent } from "react";
import { STATUSES, type Priority, type Status, type TaskType } from "@/types";
import { FieldLabel, FieldRequirementBadge } from "@/shared/FieldLabel";
import { Modal } from "../../../../shared/Modal";
import { PrioritySelect, TaskTypeSelect } from "../../task-compose";
import { RichPromptEditor } from "../../rich-prompt";

type Props = {
  taskId: string;
  title: string;
  prompt: string;
  priority: Priority;
  taskType: TaskType;
  status: Status;
  checklistInherit: boolean;
  /** When false, the inherit checkbox is disabled (task has no parent). */
  canInheritChecklist: boolean;
  saving: boolean;
  patchPending: boolean;
  /**
   * Inline error from the most recent PATCH attempt (already coerced to a
   * user-presentable string by `useTaskPatchFlow.patchError`). Rendered as
   * a `.err role="alert"` callout above the form actions when non-null.
   *
   * Same backdrop-hides-banner gap as the create / evaluate / subtask /
   * checklist / delete surfaces hardened in sessions #31-#34: while the
   * edit modal is open, the global `<ErrorBanner />` above `<main>` is
   * covered by the modal backdrop so a failed PATCH silently re-enables
   * the Save button with no feedback. The action buttons stay enabled so
   * the user can immediately retry.
   */
  error?: string | null;
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onPriorityChange: (p: Priority) => void;
  onTaskTypeChange: (t: TaskType) => void;
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
  taskType,
  status,
  checklistInherit,
  canInheritChecklist,
  saving,
  patchPending,
  error = null,
  onTitleChange,
  onPromptChange,
  onPriorityChange,
  onTaskTypeChange,
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
      // The spinner still gives in-flight feedback, but the user can
      // step away (Escape / backdrop) from a slow PATCH without losing
      // context. Safe because `useTaskPatchFlow.patchTask.onSuccess`
      // is id-aware: it fires `onPatched(patchedId)` and the parent's
      // `setEditing(prev => prev?.id === patchedId ? null : prev)`
      // refuses to clobber `editing` when the user has dismissed
      // (editing = null) or moved to a different task (editing.id !==
      // patchedId). Server-truth invalidations (`tasks/list`,
      // `task-stats`) still fire so the new field values appear in the
      // list even when the form is already gone.
      dismissibleWhileBusy
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
            <TaskTypeSelect
              id="task-edit-task-type"
              value={taskType}
              onChange={onTaskTypeChange}
              disabled={saving}
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
          {error ? (
            <div className="err task-edit-form-err" role="alert">
              <p>{error}</p>
            </div>
          ) : null}
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
