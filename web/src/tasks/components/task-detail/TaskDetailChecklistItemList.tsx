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
                  disabled={
                    editCriterionPending ||
                    removeItemPending ||
                    addCriterionPending
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
