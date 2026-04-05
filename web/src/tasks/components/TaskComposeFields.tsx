import type { PriorityChoice } from "@/types";
import {
  FieldLabel,
  FieldRequirementBadge,
} from "@/shared/FieldLabel";
import { PrioritySelect } from "./PrioritySelect";
import { RichPromptEditor } from "./RichPromptEditor";

export type TaskComposeFieldsProps = {
  /** Prefix for stable `id`s, e.g. `task-new` → `task-new-title`. */
  idsPrefix: string;
  title: string;
  prompt: string;
  priority: PriorityChoice;
  checklistDraft: string;
  checklistItems: string[];
  /** When true, the done-criteria block is omitted (e.g. subtask inherits a parent checklist). */
  hideChecklist?: boolean;
  disabled: boolean;
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onPriorityChange: (p: PriorityChoice) => void;
  onChecklistDraftChange: (v: string) => void;
  onAddChecklistRow: () => void;
  onRemoveChecklistRow: (index: number) => void;
  /** Passed to `RichPromptEditor` as `key` so the editor resets when needed. */
  editorKey: string;
};

export function TaskComposeFields({
  idsPrefix,
  title,
  prompt,
  priority,
  checklistDraft,
  checklistItems,
  hideChecklist = false,
  disabled,
  onTitleChange,
  onPromptChange,
  onPriorityChange,
  onChecklistDraftChange,
  onAddChecklistRow,
  onRemoveChecklistRow,
  editorKey,
}: TaskComposeFieldsProps) {
  const titleId = `${idsPrefix}-title`;
  const promptId = `${idsPrefix}-prompt`;
  const priorityId = `${idsPrefix}-priority`;
  const checklistHeadingId = `${idsPrefix}-checklist-heading`;
  const checklistDraftId = `${idsPrefix}-checklist-draft`;

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
          <div className="field-heading-with-req task-create-checklist-heading-row">
            <h3
              className="task-create-checklist-heading"
              id={checklistHeadingId}
            >
              Done criteria
            </h3>
            <FieldRequirementBadge requirement="optional" />
          </div>
          <p className="task-create-checklist-hint">
            Optional checklist — all items must be complete before the task can
            be marked done.
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
          <div className="task-checklist-add-form task-create-checklist-add">
            <div className="field grow">
              <FieldLabel htmlFor={checklistDraftId} requirement="optional">
                Add criterion
              </FieldLabel>
              <input
                id={checklistDraftId}
                value={checklistDraft}
                onChange={(ev) => onChecklistDraftChange(ev.target.value)}
                onKeyDown={(ev) => {
                  if (ev.key !== "Enter") return;
                  ev.preventDefault();
                  if (!checklistDraft.trim() || disabled) return;
                  onAddChecklistRow();
                }}
                placeholder="Describe what must be true to mark done"
                disabled={disabled}
              />
            </div>
            <button
              type="button"
              className="task-create-checklist-add-btn"
              aria-label="Add checklist criterion"
              disabled={!checklistDraft.trim() || disabled}
              onClick={onAddChecklistRow}
            >
              Add
            </button>
          </div>
        </div>
      ) : null}
    </>
  );
}
