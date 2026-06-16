import { useState } from "react";
import type { TaskChecklistItemView } from "@/types";
import { ChecklistStatusIcon } from "./ChecklistStatusIcon";
import { ChecklistVerifyBadge } from "./ChecklistVerifyBadge";
import { ChecklistVerificationModal } from "./ChecklistVerificationModal";

type Props = {
  items: TaskChecklistItemView[];
  editCriterionPending: boolean;
  removeItemPending: boolean;
  addCriterionPending: boolean;
  onOpenEditCriterionModal: (
    itemId: string,
    text: string,
    verifyCommands?: import("@/types").ChecklistVerifyCommandInput[],
  ) => void;
  onRemoveChecklistItem: (itemId: string) => void;
};

export function TaskDetailChecklistItemList({
  items,
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
          const verifyCommandCount = item.verify_commands?.length ?? 0;
          const hasVerificationDetail =
            item.done &&
            ((typeof item.evidence === "string" && item.evidence.length > 0) ||
              (typeof item.verifier_reasoning === "string" &&
                item.verifier_reasoning.length > 0));
          const showRowMeta = hasVerificationDetail;
          const canEditRow =
            !item.done &&
            !editCriterionPending &&
            !removeItemPending &&
            !addCriterionPending;
          const canViewRow =
            item.done &&
            !editCriterionPending &&
            !removeItemPending &&
            !addCriterionPending;
          const canOpenRow = canEditRow || canViewRow;
          return (
            <li
              key={item.id}
              className={
                item.done
                  ? canViewRow
                    ? "task-checklist-row task-checklist-row--done task-checklist-row--interactive"
                    : "task-checklist-row task-checklist-row--done"
                  : canEditRow
                    ? "task-checklist-row task-checklist-row--pending task-checklist-row--interactive"
                    : "task-checklist-row task-checklist-row--pending"
              }
              onClick={(event) => {
                if (!canOpenRow) return;
                if ((event.target as HTMLElement).closest("button")) return;
                onOpenEditCriterionModal(
                  item.id,
                  item.text,
                  item.verify_commands,
                );
              }}
            >
            <div className="task-checklist-row-primary">
              <ChecklistStatusIcon done={item.done} />
              <span className="task-checklist-text" title={item.text}>
                {item.text}
              </span>
              <div className="task-checklist-row-trailing">
                {verifyCommandCount > 0 ? (
                  <ChecklistVerifyBadge count={verifyCommandCount} />
                ) : null}
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
                        ? "Edit (locked: already marked done)"
                        : undefined
                    }
                    onClick={() =>
                      onOpenEditCriterionModal(
                        item.id,
                        item.text,
                        item.verify_commands,
                      )
                    }
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
              </div>
            </div>
            {showRowMeta ? (
              <div className="task-checklist-row-meta">
                <button
                  type="button"
                  className="task-checklist-verification-trigger"
                  onClick={() => setOpenVerificationId(item.id)}
                  aria-label={`View verification details for: ${item.text}`}
                >
                  <span className="task-checklist-verification-trigger-label">
                    View verification
                  </span>
                  <span
                    className="task-checklist-verification-trigger-arrow"
                    aria-hidden="true"
                  >
                    &rarr;
                  </span>
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
          evidence={openItem.evidence}
          verifierReasoning={openItem.verifier_reasoning}
          onClose={() => setOpenVerificationId(null)}
        />
      ) : null}
    </div>
  );
}
