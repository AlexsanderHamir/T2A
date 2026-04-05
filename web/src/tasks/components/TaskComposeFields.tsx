import { useState, type FormEvent } from "react";
import type { PriorityChoice } from "@/types";
import {
  FieldLabel,
  FieldRequirementBadge,
} from "@/shared/FieldLabel";
import { PrioritySelect } from "./PrioritySelect";
import { RichPromptEditor } from "./RichPromptEditor";
import { ChecklistCriterionModal } from "./ChecklistCriterionModal";

export type TaskComposeFieldsProps = {
  /** Prefix for stable `id`s, e.g. `task-new` → `task-new-title`. */
  idsPrefix: string;
  title: string;
  prompt: string;
  priority: PriorityChoice;
  checklistItems: string[];
  /** When true, the done-criteria block is omitted (e.g. subtask inherits a parent checklist). */
  hideChecklist?: boolean;
  disabled: boolean;
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onPriorityChange: (p: PriorityChoice) => void;
  onAppendChecklistCriterion: (text: string) => void;
  onRemoveChecklistRow: (index: number) => void;
  /** Passed to `RichPromptEditor` as `key` so the editor resets when needed. */
  editorKey: string;
};

export function TaskComposeFields({
  idsPrefix,
  title,
  prompt,
  priority,
  checklistItems,
  hideChecklist = false,
  disabled,
  onTitleChange,
  onPromptChange,
  onPriorityChange,
  onAppendChecklistCriterion,
  onRemoveChecklistRow,
  editorKey,
}: TaskComposeFieldsProps) {
  const titleId = `${idsPrefix}-title`;
  const promptId = `${idsPrefix}-prompt`;
  const priorityId = `${idsPrefix}-priority`;
  const checklistHeadingId = `${idsPrefix}-checklist-heading`;

  const [criterionModalOpen, setCriterionModalOpen] = useState(false);
  const [criterionModalText, setCriterionModalText] = useState("");

  const openCriterionModal = () => {
    setCriterionModalText("");
    setCriterionModalOpen(true);
  };

  const closeCriterionModal = () => {
    setCriterionModalOpen(false);
    setCriterionModalText("");
  };

  const submitCriterionModal = (e: FormEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const t = criterionModalText.trim();
    if (!t) return;
    onAppendChecklistCriterion(t);
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
            placeholder="Optional context for an agent… Use the toolbar for headings and bold. Type @ to pick a file from the repo."
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
          <p className="task-create-checklist-hint">
            Optional checklist — all items must be complete before the task can
            be marked done. Use <strong>New criterion</strong> for each line; it
            opens the same short dialog as on the task page. Nothing is saved
            until you submit this form.
          </p>
          {checklistItems.length > 0 ? (
            <ul
              className="task-checklist-list"
              aria-labelledby={checklistHeadingId}
            >
              {checklistItems.map((text, index) => (
                <li key={`${index}-${text}`} className="task-checklist-row">
                  <span className="task-checklist-label">{text}</span>
                  <button
                    type="button"
                    className="task-create-checklist-remove"
                    disabled={disabled}
                    onClick={() => onRemoveChecklistRow(index)}
                  >
                    Remove
                  </button>
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : null}

      {criterionModalOpen ? (
        <ChecklistCriterionModal
          mode="add"
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
