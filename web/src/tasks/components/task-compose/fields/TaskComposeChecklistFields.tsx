import { FieldRequirementBadge } from "@/shared/FieldLabel";
import type { ChecklistItemDraft } from "@/types";
import { CREATE_CHECKLIST_REQUIRED_MSG } from "@/tasks/task-compose/checklistRequirement";

type Props = {
  checklistHeadingId: string;
  checklistItems: ChecklistItemDraft[];
  /** When `required`, shows the required badge and create-time helper copy. */
  checklistRequirement?: "optional" | "required";
  disabled: boolean;
  onOpenNewCriterion: () => void;
  onOpenEditCriterion: (index: number, item: ChecklistItemDraft) => void;
  onRemoveRow: (index: number) => void;
};

export function TaskComposeChecklistFields({
  checklistHeadingId,
  checklistItems,
  checklistRequirement = "optional",
  disabled,
  onOpenNewCriterion,
  onOpenEditCriterion,
  onRemoveRow,
}: Props) {
  const isEmpty = checklistItems.length === 0;

  return (
    <div className="task-create-checklist">
      <div className="task-create-checklist-head">
        <div className="field-heading-with-req task-create-checklist-title-row">
          <h3 className="task-create-checklist-heading" id={checklistHeadingId}>
            Done criteria
          </h3>
          <FieldRequirementBadge requirement={checklistRequirement} />
        </div>
        <button
          type="button"
          className="task-detail-add-checklist-btn"
          disabled={disabled}
          onClick={onOpenNewCriterion}
        >
          New criterion
        </button>
      </div>

      {isEmpty ? (
        <div
          className="task-create-checklist-empty"
          aria-labelledby={checklistHeadingId}
        >
          <p className="task-create-checklist-empty__text">
            {checklistRequirement === "required"
              ? CREATE_CHECKLIST_REQUIRED_MSG
              : "No criteria yet."}
          </p>
        </div>
      ) : (
        <div className="task-checklist-surface">
          <ul
            className="task-checklist-list task-checklist-list--grouped"
            aria-labelledby={checklistHeadingId}
          >
            {checklistItems.map((item, index) => {
              const commandCount = item.verify_commands?.length ?? 0;
              return (
                <li key={`${index}-${item.text}`} className="task-checklist-row">
                  <div className="task-checklist-row-main">
                    <span className="task-checklist-text">{item.text}</span>
                    {commandCount > 0 ? (
                      <span className="task-checklist-verify-badge">
                        {commandCount} verify
                        {commandCount === 1 ? "" : " cmds"}
                      </span>
                    ) : null}
                  </div>
                  <div className="task-checklist-row-actions">
                    <button
                      type="button"
                      className="task-detail-checklist-edit"
                      disabled={disabled}
                      onClick={() => onOpenEditCriterion(index, item)}
                    >
                      Edit
                    </button>
                    <button
                      type="button"
                      className="task-detail-checklist-remove"
                      disabled={disabled}
                      onClick={() => onRemoveRow(index)}
                    >
                      Remove
                    </button>
                  </div>
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </div>
  );
}
