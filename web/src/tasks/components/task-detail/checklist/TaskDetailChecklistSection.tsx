import type { UseQueryResult } from "@tanstack/react-query";
import type { FormEvent } from "react";
import type { TaskChecklistResponse } from "@/types";
import { errorMessage } from "@/lib/errorMessage";
import { FieldRequirementBadge } from "@/shared/FieldLabel";
import {
  EmptyState,
  EmptyStateChecklistGlyph,
} from "@/shared/EmptyState";
import { ChecklistCriterionModal } from "../../task-compose";
import { TaskDetailChecklistItemList } from "./TaskDetailChecklistItemList";
import { TaskChecklistSkeleton } from "../../skeletons";

export type TaskDetailChecklistSectionProps = {
  saving: boolean;
  checklistQuery: UseQueryResult<TaskChecklistResponse, Error>;
  doneCount: number;
  totalCount: number;
  modalOpen: boolean;
  newCriterionText: string;
  onNewCriterionTextChange: (v: string) => void;
  onOpenAddModal: () => void;
  onCloseAddModal: () => void;
  onSubmitNewCriterion: (e: FormEvent) => void;
  addCriterionPending: boolean;
  editModalOpen: boolean;
  editingItemId: string | null;
  editCriterionText: string;
  onEditCriterionTextChange: (v: string) => void;
  onOpenEditCriterionModal: (itemId: string, text: string) => void;
  onCloseEditCriterionModal: () => void;
  onSubmitEditCriterion: (e: FormEvent) => void;
  editCriterionPending: boolean;
  onRemoveChecklistItem: (itemId: string) => void;
  removeItemPending: boolean;
  /**
   * Most recent error from the underlying mutations. Surfaced inline
   * so users get visible feedback when a write fails (the modals stay
   * open & the buttons re-enable, but without this the user has no
   * idea anything went wrong). `null` for happy path / idle / pending.
   */
  addCriterionError?: unknown;
  editCriterionError?: unknown;
  removeItemError?: unknown;
  /** False when the task is running or done — criteria are locked after agent pickup. */
  canAddCriterion?: boolean;
};

export function TaskDetailChecklistSection({
  saving,
  checklistQuery,
  doneCount,
  totalCount,
  modalOpen,
  newCriterionText,
  onNewCriterionTextChange,
  onOpenAddModal,
  onCloseAddModal,
  onSubmitNewCriterion,
  addCriterionPending,
  editModalOpen,
  editingItemId,
  editCriterionText,
  onEditCriterionTextChange,
  onOpenEditCriterionModal,
  onCloseEditCriterionModal,
  onSubmitEditCriterion,
  editCriterionPending,
  onRemoveChecklistItem,
  removeItemPending,
  addCriterionError = null,
  editCriterionError = null,
  removeItemError = null,
  canAddCriterion = true,
}: TaskDetailChecklistSectionProps) {
  const showProgress =
    !checklistQuery.isPending &&
    !checklistQuery.isError &&
    totalCount > 0;
  const allSatisfied = showProgress && doneCount === totalCount;
  const progressPct =
    showProgress && totalCount > 0
      ? Math.round((doneCount / totalCount) * 100)
      : 0;

  return (
    <div className="task-detail-section" id="task-detail-checklist">
      <div className="task-detail-checklist-head">
        <div className="field-heading-with-req task-detail-checklist-title-row">
          <h3
            className="task-detail-section-heading"
            id="task-checklist-heading"
          >
            <span>Done criteria</span>
          </h3>
          <FieldRequirementBadge requirement="optional" />
        </div>
        <button
          type="button"
          className="task-detail-add-checklist-btn"
          onClick={onOpenAddModal}
          disabled={saving || !canAddCriterion}
          title={
            canAddCriterion
              ? undefined
              : "Criteria cannot be added while the agent is working on this task or after it is done."
          }
        >
          Add criterion
        </button>
      </div>
      {showProgress ? (
        <div
          className={
            allSatisfied
              ? "task-checklist-progress-card task-checklist-progress-card--complete"
              : "task-checklist-progress-card"
          }
          role="status"
          aria-label={
            totalCount === 1
              ? `Checklist progress: ${doneCount} of 1 requirement satisfied`
              : `Checklist progress: ${doneCount} of ${totalCount} requirements satisfied`
          }
        >
          <div className="task-checklist-progress-head">
            <p className="task-checklist-progress-fraction">
              <span className="task-checklist-progress-done">{doneCount}</span>
              <span className="task-checklist-progress-sep" aria-hidden="true">
                /
              </span>
              <span className="task-checklist-progress-total">{totalCount}</span>
            </p>
            {allSatisfied ? null : (
              <p className="task-checklist-progress-label">
                {totalCount - doneCount === 1
                  ? "1 remaining"
                  : `${totalCount - doneCount} remaining`}
              </p>
            )}
          </div>
          <div
            className="task-checklist-progress-track"
            aria-hidden="true"
          >
            <div
              className="task-checklist-progress-fill"
              style={{ width: `${progressPct}%` }}
            />
          </div>
        </div>
      ) : null}
      <div
        className="task-detail-checklist-body"
        aria-labelledby="task-checklist-heading"
      >
        {checklistQuery.isError ? (
          <div className="task-checklist-surface">
            <div className="task-checklist-surface-pad">
              <div className="err task-checklist-fetch-err" role="alert">
                <p>
                  {errorMessage(
                    checklistQuery.error,
                    "Could not load checklist.",
                  )}
                </p>
                <div className="task-detail-error-actions">
                  <button
                    type="button"
                    className="secondary"
                    onClick={() => {
                      void checklistQuery.refetch();
                    }}
                  >
                    Try again
                  </button>
                </div>
              </div>
            </div>
          </div>
        ) : checklistQuery.isPending ? (
          <div className="task-checklist-surface">
            <TaskChecklistSkeleton />
          </div>
        ) : (checklistQuery.data?.items.length ?? 0) === 0 ? (
          <EmptyState
            density="compact"
            className="task-detail-section-empty"
            icon={<EmptyStateChecklistGlyph />}
            title="No criteria yet"
            description="Use Add criterion to define what must be true before this task is done."
          />
        ) : (
          <TaskDetailChecklistItemList
            items={checklistQuery.data?.items ?? []}
            editCriterionPending={editCriterionPending}
            removeItemPending={removeItemPending}
            addCriterionPending={addCriterionPending}
            onOpenEditCriterionModal={onOpenEditCriterionModal}
            onRemoveChecklistItem={onRemoveChecklistItem}
          />
        )}
      </div>
      {modalOpen ? (
        <ChecklistCriterionModal
          mode="add"
          pending={addCriterionPending}
          saving={saving}
          onClose={onCloseAddModal}
          text={newCriterionText}
          onTextChange={onNewCriterionTextChange}
          onSubmit={onSubmitNewCriterion}
          // Safe because `useTaskDetailChecklist.addChecklistMutation`
          // is race-hardened: `onSuccess` only fires the form-clear +
          // modal-close branch when its captured `submissionToken`
          // still matches `addSubmissionTokenRef.current`. Server-truth
          // invalidations (`taskQueryKeys.checklist(taskId)`,
          // `taskQueryKeys.detail(taskId)`) still fire so the new
          // criterion appears in the list even when the modal is
          // already gone. See `.agent/frontend-improvement-agent.log`
          // Session 30.
          dismissibleWhileBusy
          error={addCriterionError}
          errorFallback="Could not add criterion."
        />
      ) : null}
      {editModalOpen && editingItemId ? (
        <ChecklistCriterionModal
          mode="edit"
          pending={editCriterionPending}
          saving={saving}
          onClose={onCloseEditCriterionModal}
          text={editCriterionText}
          onTextChange={onEditCriterionTextChange}
          onSubmit={onSubmitEditCriterion}
          // Safe because `useTaskDetailChecklist.updateChecklistTextMutation`
          // is race-hardened: `onSuccess` only fires
          // `closeEditCriterionModal()` when its captured
          // `variables.itemId` still matches `editingChecklistItemId`.
          // Server-truth invalidations still fire so the persisted
          // text appears in the list even when the modal is already
          // gone. See `.agent/frontend-improvement-agent.log`
          // Session 30.
          dismissibleWhileBusy
          error={editCriterionError}
          errorFallback="Could not save changes."
        />
      ) : null}
      {removeItemError ? (
        <div
          className="err task-checklist-remove-err"
          role="alert"
        >
          {errorMessage(removeItemError, "Could not remove criterion.")}
        </div>
      ) : null}
    </div>
  );
}
