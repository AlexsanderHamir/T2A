import { useState } from "react";
import type { TaskChecklistItemView } from "@/types";
import { ChecklistVerificationModal } from "./ChecklistVerificationModal";

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
  // Identity of the criterion whose verification detail is currently
  // open in the popup. Single-open is intentional — verification detail
  // can be long, and stacking multiple sheets would defeat the goal of
  // keeping the checklist row scannable.
  const [openVerificationId, setOpenVerificationId] = useState<string | null>(
    null,
  );

  const openItem = openVerificationId
    ? items.find((item) => item.id === openVerificationId) ?? null
    : null;

  return (
    <div className="task-checklist-surface">
      <ul className="task-checklist-list task-checklist-list--grouped">
        {items.map((item) => {
          const hasVerificationDetail =
            item.done &&
            ((typeof item.evidence === "string" && item.evidence.length > 0) ||
              (typeof item.verifier_reasoning === "string" &&
                item.verifier_reasoning.length > 0));
          return (
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
              <div className="task-checklist-text-block">
                <span className="task-checklist-text">{item.text}</span>
                {item.check ? (
                  <code className="task-checklist-check">{item.check}</code>
                ) : null}
                <div className="task-checklist-row-meta">
                  {item.done && item.verified_by ? (
                    <span className="task-checklist-verified-badge">
                      Verified
                      <span className="task-checklist-verified-badge-by">
                        {" "}
                        ({item.verified_by})
                      </span>
                    </span>
                  ) : null}
                  {hasVerificationDetail ? (
                    <button
                      type="button"
                      className="task-checklist-verification-trigger"
                      onClick={() => setOpenVerificationId(item.id)}
                      aria-label={`View verification details for: ${item.text}`}
                    >
                      View verification
                      <span
                        className="task-checklist-verification-trigger-arrow"
                        aria-hidden="true"
                      >
                        &rarr;
                      </span>
                    </button>
                  ) : null}
                </div>
              </div>
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
                      ? "Already marked done — cannot edit a satisfied criterion."
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
                  // Symmetric with the Edit lock above: removing a done
                  // criterion would orphan the persisted
                  // checklist_item_toggled (done=true) audit row and
                  // erase the historical fact that the task ever
                  // satisfied this requirement. The backend rejects
                  // this with ErrInvalidInput; disabling here keeps the
                  // affordance honest so users don't trigger a bogus
                  // 400 round-trip.
                  disabled={item.done || removeItemPending}
                  title={
                    item.done
                      ? "Already marked done — cannot remove a satisfied criterion."
                      : undefined
                  }
                  aria-label={
                    item.done
                      ? `Remove criterion (locked: already marked done): ${item.text}`
                      : undefined
                  }
                  onClick={() => onRemoveChecklistItem(item.id)}
                >
                  Remove
                </button>
              </div>
            ) : null}
          </li>
          );
        })}
      </ul>
      {openItem ? (
        <ChecklistVerificationModal
          criterionText={openItem.text}
          verifiedBy={openItem.verified_by}
          evidence={openItem.evidence}
          verifierReasoning={openItem.verifier_reasoning}
          onClose={() => setOpenVerificationId(null)}
        />
      ) : null}
    </div>
  );
}
