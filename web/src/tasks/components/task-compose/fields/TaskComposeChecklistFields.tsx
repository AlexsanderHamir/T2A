import { FieldRequirementBadge } from "@/shared/FieldLabel";

type Props = {
  checklistHeadingId: string;
  checklistItems: string[];
  /** When `required`, shows the required badge (e.g. parent tasks with subtasks). */
  checklistRequirement?: "optional" | "required";
  disabled: boolean;
  onOpenNewCriterion: () => void;
  onOpenEditCriterion: (index: number, text: string) => void;
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
                    onClick={() => onOpenEditCriterion(index, text)}
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
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}
