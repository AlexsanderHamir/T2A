import { useState, type FormEvent } from "react";
import type { PriorityChoice, TaskType } from "@/types";
import {
  FieldLabel,
  FieldRequirementBadge,
} from "@/shared/FieldLabel";
import { PrioritySelect } from "./PrioritySelect";
import { TaskTypeSelect } from "./TaskTypeSelect";
import { RichPromptEditor } from "./RichPromptEditor";
import { ChecklistCriterionModal } from "./ChecklistCriterionModal";

export type TaskComposeFieldsProps = {
  /** Prefix for stable `id`s, e.g. `task-new` → `task-new-title`. */
  idsPrefix: string;
  title: string;
  prompt: string;
  priority: PriorityChoice;
  taskType: TaskType;
  checklistItems: string[];
  /** When true, the done-criteria block is omitted (e.g. subtask inherits a parent checklist). */
  hideChecklist?: boolean;
  disabled: boolean;
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onPriorityChange: (p: PriorityChoice) => void;
  onTaskTypeChange: (t: TaskType) => void;
  onAppendChecklistCriterion: (text: string) => void;
  onUpdateChecklistRow: (index: number, text: string) => void;
  onRemoveChecklistRow: (index: number) => void;
  /** Passed to `RichPromptEditor` as `key` so the editor resets when needed. */
  editorKey: string;
};

export function TaskComposeFields({
  idsPrefix,
  title,
  prompt,
  priority,
  taskType,
  checklistItems,
  hideChecklist = false,
  disabled,
  onTitleChange,
  onPromptChange,
  onPriorityChange,
  onTaskTypeChange,
  onAppendChecklistCriterion,
  onUpdateChecklistRow,
  onRemoveChecklistRow,
  editorKey,
}: TaskComposeFieldsProps) {
  const titleId = `${idsPrefix}-title`;
  const promptId = `${idsPrefix}-prompt`;
  const priorityId = `${idsPrefix}-priority`;
  const taskTypeId = `${idsPrefix}-task-type`;
  const checklistHeadingId = `${idsPrefix}-checklist-heading`;

  const [criterionModalOpen, setCriterionModalOpen] = useState(false);
  const [criterionModalText, setCriterionModalText] = useState("");
  const [criterionEditIndex, setCriterionEditIndex] = useState<number | null>(null);

  const openCriterionModal = () => {
    setCriterionEditIndex(null);
    setCriterionModalText("");
    setCriterionModalOpen(true);
  };

  const openEditCriterionModal = (index: number, text: string) => {
    setCriterionEditIndex(index);
    setCriterionModalText(text);
    setCriterionModalOpen(true);
  };

  const closeCriterionModal = () => {
    setCriterionModalOpen(false);
    setCriterionEditIndex(null);
    setCriterionModalText("");
  };

  const submitCriterionModal = (e: FormEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const t = criterionModalText.trim();
    if (!t) return;
    if (criterionEditIndex === null) {
      onAppendChecklistCriterion(t);
    } else {
      onUpdateChecklistRow(criterionEditIndex, t);
    }
    closeCriterionModal();
  };

  return (
    <>
      <div className="task-create-title-row">
        <div className="field grow">
          <FieldLabel htmlFor={titleId} requirement="required">
            Title
          </FieldLabel>
          <input
            id={titleId}
            value={title}
            onChange={(ev) => onTitleChange(ev.target.value)}
            placeholder="What should get done?"
            required
            aria-required="true"
            disabled={disabled}
          />
        </div>
        <PrioritySelect
          id={priorityId}
          value={priority}
          compact
          onChange={onPriorityChange}
        />
        <TaskTypeSelect
          id={taskTypeId}
          value={taskType}
          onChange={onTaskTypeChange}
          disabled={disabled}
        />
      </div>

      <div className="field grow stack-tight prompt-field-full task-create-prompt">
        <FieldLabel
          id={`${promptId}-label`}
          htmlFor={promptId}
          requirement="optional"
        >
          Initial prompt
        </FieldLabel>
        <div className="task-create-editor-shell">
          <RichPromptEditor
            key={editorKey}
            id={promptId}
            value={prompt}
            onChange={onPromptChange}
            disabled={disabled}
            placeholder="Optional context. Toolbar for headings and bold; type @ to mention a repo file."
          />
        </div>
      </div>

      {!hideChecklist ? (
        <div className="task-create-checklist">
          <div className="task-create-checklist-head">
            <div className="field-heading-with-req task-create-checklist-title-row">
              <h3
                className="task-create-checklist-heading"
                id={checklistHeadingId}
              >
                Done criteria
              </h3>
              <FieldRequirementBadge requirement="optional" />
            </div>
            <button
              type="button"
              className="task-detail-add-checklist-btn"
              disabled={disabled}
              onClick={openCriterionModal}
            >
              New criterion
            </button>
          </div>
          <p className="task-create-checklist-hint muted">
            All items must be satisfied before the task is done.{" "}
            <strong>New criterion</strong> adds one; saved when you click{" "}
            <strong>Create</strong>.
          </p>
          {checklistItems.length > 0 ? (
            <div className="task-checklist-surface">
              <ul
                className="task-checklist-list task-checklist-list--grouped"
                aria-labelledby={checklistHeadingId}
              >
                {checklistItems.map((text, index) => (
                  <li key={`${index}-${text}`} className="task-checklist-row">
                    <div className="task-checklist-row-main">
                      <span className="task-checklist-text">{text}</span>
                    </div>
                    <div className="task-checklist-row-actions">
                      <button
                        type="button"
                        className="task-detail-checklist-edit"
                        disabled={disabled}
                        onClick={() => openEditCriterionModal(index, text)}
                      >
                        Edit
                      </button>
                      <button
                        type="button"
                        className="task-detail-checklist-remove"
                        disabled={disabled}
                        onClick={() => onRemoveChecklistRow(index)}
                      >
                        Remove
                      </button>
                    </div>
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
        </div>
      ) : null}

      {criterionModalOpen ? (
        <ChecklistCriterionModal
          mode={criterionEditIndex === null ? "add" : "edit"}
          pending={false}
          saving={false}
          onClose={closeCriterionModal}
          text={criterionModalText}
          onTextChange={setCriterionModalText}
          onSubmit={submitCriterionModal}
          modalStack="nested"
          lockBodyScroll={false}
        />
      ) : null}
    </>
  );
}
