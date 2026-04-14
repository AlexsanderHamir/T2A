import { useState, type FormEvent } from "react";
import type { PriorityChoice, TaskType } from "@/types";
import { FieldLabel } from "@/shared/FieldLabel";
import { PrioritySelect } from "./PrioritySelect";
import { TaskTypeSelect } from "./TaskTypeSelect";
import { RichPromptEditor } from "./RichPromptEditor";
import { ChecklistCriterionModal } from "./ChecklistCriterionModal";
import { TaskComposeChecklistFields } from "./TaskComposeChecklistFields";

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
        <TaskComposeChecklistFields
          checklistHeadingId={checklistHeadingId}
          checklistItems={checklistItems}
          disabled={disabled}
          onOpenNewCriterion={openCriterionModal}
          onOpenEditCriterion={openEditCriterionModal}
          onRemoveRow={onRemoveChecklistRow}
        />
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
