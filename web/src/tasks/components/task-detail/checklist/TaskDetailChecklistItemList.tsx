import type { TaskChecklistItemView } from "@/types";

type Props = {
  items: TaskChecklistItemView[];
  checklistInherit: boolean;
  editCriterionPending: boolean;
  removeItemPending: boolean;
  addCriterionPending: boolean;
  onOpenEditCriterionModal: (itemId: string, text: string) => void;
  onRemoveChecklistItem: (itemId: string) => void;
};

export function TaskDetailChecklistItemList({
  items,
  checklistInherit,
  editCriterionPending,
  removeItemPending,
  addCriterionPending,
  onOpenEditCriterionModal,
  onRemoveChecklistItem,
}: Props) {
  return (
    <div className="task-checklist-surface">
      <ul className="task-checklist-list task-checklist-list--grouped">
        {items.map((item) => (
          <li key={item.id} className="task-checklist-row">
            <div className="task-checklist-row-main">
              <span
                className={
                  item.done
                    ? "task-checklist-status task-checklist-status--done"
                    : "task-checklist-status task-checklist-status--pending"
                }
                role="img"
                aria-label={item.done ? "Satisfied" : "Not satisfied yet"}
              >
                {item.done ? "✓" : null}
              </span>
              <span className="task-checklist-text">{item.text}</span>
            </div>
            {!checklistInherit ? (
              <div className="task-checklist-row-actions">
                <button
                  type="button"
                  className="task-detail-checklist-edit"
                  // Done criteria are locked: editing the text after the
                  // agent has accepted the criterion as satisfied would
                  // silently rewrite history (the
                  // checklist_item_toggled audit row would now point at
                  // text that didn't exist at completion time). The
                  // backend rejects this with ErrInvalidInput as well —
                  // disabling here just keeps the affordance honest.
                  disabled={
                    item.done ||
                    editCriterionPending ||
                    removeItemPending ||
                    addCriterionPending
                  }
                  title={
                    item.done
                      ? "Already marked done — cannot edit. Remove it if it was wrong."
                      : undefined
                  }
                  aria-label={
                    item.done
                      ? `Edit criterion (locked: already marked done): ${item.text}`
                      : undefined
                  }
                  onClick={() => onOpenEditCriterionModal(item.id, item.text)}
                >
                  Edit
                </button>
                <button
                  type="button"
                  className="task-detail-checklist-remove"
                  disabled={removeItemPending}
                  onClick={() => onRemoveChecklistItem(item.id)}
                >
                  Remove
                </button>
              </div>
            ) : null}
          </li>
        ))}
      </ul>
    </div>
  );
}
