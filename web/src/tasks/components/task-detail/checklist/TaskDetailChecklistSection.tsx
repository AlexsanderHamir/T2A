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
  checklistInherit: boolean;
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
};

export function TaskDetailChecklistSection({
  checklistInherit,
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
}: TaskDetailChecklistSectionProps) {
  return (
    <div className="task-detail-section" id="task-detail-checklist">
      <div className="task-detail-checklist-head">
        <div className="field-heading-with-req task-detail-checklist-title-row">
          <h3
            className="task-detail-section-heading term-prompt"
            id="task-checklist-heading"
          >
            <span>Done criteria</span>
          </h3>
          <FieldRequirementBadge
            requirement={checklistInherit ? "none" : "optional"}
          />
        </div>
        {!checklistInherit ? (
          <button
            type="button"
            className="task-detail-add-checklist-btn"
            onClick={onOpenAddModal}
            disabled={saving}
          >
            Add criterion
          </button>
        ) : null}
      </div>
      {checklistInherit ||
      (!checklistQuery.isPending &&
        !checklistQuery.isError &&
        totalCount > 0) ? (
        <div className="task-checklist-intro">
          {checklistInherit ? (
            <p className="task-checklist-intro-lead muted" role="status">
              Inherited for <strong>this</strong> task; criteria are defined
              upstream.
            </p>
          ) : null}
          {!checklistQuery.isPending &&
          !checklistQuery.isError &&
          totalCount > 0 ? (
            <p
              className="task-checklist-progress muted"
              role="status"
              aria-label={
                totalCount === 1
                  ? `Checklist progress: ${doneCount} of 1 requirement satisfied`
                  : `Checklist progress: ${doneCount} of ${totalCount} requirements satisfied`
              }
            >
              <strong className="task-checklist-progress-strong">
                {doneCount} of {totalCount}
              </strong>{" "}
              {totalCount === 1
                ? "requirement satisfied"
                : "requirements satisfied"}
            </p>
          ) : null}
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
            description={
              <>
                Use <strong>Add criterion</strong> above to describe what must
                be true before this task can be marked done.
              </>
            }
          />
        ) : (
          <TaskDetailChecklistItemList
            items={checklistQuery.data?.items ?? []}
            checklistInherit={checklistInherit}
            editCriterionPending={editCriterionPending}
            removeItemPending={removeItemPending}
            addCriterionPending={addCriterionPending}
            onOpenEditCriterionModal={onOpenEditCriterionModal}
            onRemoveChecklistItem={onRemoveChecklistItem}
          />
        )}
      </div>
      {modalOpen && !checklistInherit ? (
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
      {editModalOpen && !checklistInherit && editingItemId ? (
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
