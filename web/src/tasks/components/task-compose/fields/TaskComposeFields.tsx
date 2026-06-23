import { useState, type FormEvent, type ReactNode } from "react";
import type { PriorityChoice, ChecklistItemDraft } from "@/types";
import { normalizeVerifyCommands } from "@/tasks/task-compose/checklistRequirement";
import { FieldLabel } from "@/shared/FieldLabel";
import { PrioritySelect } from "./PrioritySelect";
import {
  RichPromptEditor,
  type RichPromptEditorProjectContextProps,
} from "../../rich-prompt";
import { ChecklistCriterionModal } from "../modals/ChecklistCriterionModal";
import { TaskComposeChecklistFields } from "./TaskComposeChecklistFields";

export type TaskComposeFieldsProps = {
  /** Prefix for stable `id`s, e.g. `task-new` → `task-new-title`. */
  idsPrefix: string;
  title: string;
  prompt: string;
  priority: PriorityChoice;
  checklistItems: ChecklistItemDraft[];
  /** When true, the done-criteria block is omitted (e.g. subtask inherits a parent checklist). */
  hideChecklist?: boolean;
  /** When `required`, at least one done criterion is required on create. */
  checklistRequirement?: "optional" | "required";
  /** When true, checklist add/edit/remove controls are disabled. */
  checklistDisabled?: boolean;
  disabled: boolean;
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onPriorityChange: (p: PriorityChoice) => void;
  onAppendChecklistCriterion: (item: ChecklistItemDraft) => void;
  onUpdateChecklistRow: (index: number, item: ChecklistItemDraft) => void;
  onRemoveChecklistRow: (index: number) => void;
  /** Passed to `RichPromptEditor` as `key` so the editor resets when needed. */
  editorKey: string;
  /**
   * When provided, the prompt editor wires the `#` project context
   * suggestion plugin and renders the read-only REFERENCES block above the
   * editable area. Pass `undefined` for surfaces where project context
   * isn't applicable (e.g. nested subtask drafts that inherit from parent).
   */
  projectContext?: RichPromptEditorProjectContextProps;
  /** Rendered between the title row and the prompt editor (e.g. git binding). */
  betweenTitleAndPrompt?: ReactNode;
  /** When set, @-mentions resolve against this worktree. */
  worktreeId?: string;
};

export function TaskComposeFields({
  idsPrefix,
  title,
  prompt,
  priority,
  checklistItems,
  hideChecklist = false,
  checklistRequirement = "optional",
  checklistDisabled = false,
  disabled,
  onTitleChange,
  onPromptChange,
  onPriorityChange,
  onAppendChecklistCriterion,
  onUpdateChecklistRow,
  onRemoveChecklistRow,
  editorKey,
  projectContext,
  betweenTitleAndPrompt,
  worktreeId,
}: TaskComposeFieldsProps) {
  const titleId = `${idsPrefix}-title`;
  const promptId = `${idsPrefix}-prompt`;
  const priorityId = `${idsPrefix}-priority`;
  const checklistHeadingId = `${idsPrefix}-checklist-heading`;

  const [criterionModalOpen, setCriterionModalOpen] = useState(false);
  const [criterionModalText, setCriterionModalText] = useState("");
  const [criterionModalCommands, setCriterionModalCommands] = useState<
    ChecklistItemDraft["verify_commands"]
  >([]);
  const [criterionEditIndex, setCriterionEditIndex] = useState<number | null>(null);

  const openCriterionModal = () => {
    setCriterionEditIndex(null);
    setCriterionModalText("");
    setCriterionModalCommands([]);
    setCriterionModalOpen(true);
  };

  const openEditCriterionModal = (index: number, item: ChecklistItemDraft) => {
    setCriterionEditIndex(index);
    setCriterionModalText(item.text);
    setCriterionModalCommands(item.verify_commands ?? []);
    setCriterionModalOpen(true);
  };

  const closeCriterionModal = () => {
    setCriterionModalOpen(false);
    setCriterionEditIndex(null);
    setCriterionModalText("");
    setCriterionModalCommands([]);
  };

  const submitCriterionModal = (e: FormEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const t = criterionModalText.trim();
    if (!t) return;
    const item: ChecklistItemDraft = {
      text: t,
      verify_commands: normalizeVerifyCommands(criterionModalCommands ?? []),
    };
    if (criterionEditIndex === null) {
      onAppendChecklistCriterion(item);
    } else {
      onUpdateChecklistRow(criterionEditIndex, item);
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
            className="task-create-title-input"
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

      {betweenTitleAndPrompt}

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
            placeholder={
              projectContext
                ? "Optional context. Toolbar for headings and bold; type @ to mention a repo file or # to reference project context."
                : "Optional context. Toolbar for headings and bold; type @ to mention a repo file."
            }
            projectContext={projectContext}
            worktreeId={worktreeId}
          />
        </div>
      </div>

      {!hideChecklist ? (
        <TaskComposeChecklistFields
          checklistHeadingId={checklistHeadingId}
          checklistItems={checklistItems}
          checklistRequirement={checklistRequirement}
          disabled={disabled || checklistDisabled}
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
          verifyCommands={criterionModalCommands ?? []}
          onVerifyCommandsChange={setCriterionModalCommands}
          onSubmit={submitCriterionModal}
          modalStack="nested"
          lockBodyScroll={false}
        />
      ) : null}
    </>
  );
}
