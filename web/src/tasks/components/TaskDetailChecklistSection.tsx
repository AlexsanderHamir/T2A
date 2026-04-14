import type { UseQueryResult } from "@tanstack/react-query";
import type { FormEvent } from "react";
import type { TaskChecklistResponse } from "@/types";
import { FieldRequirementBadge } from "@/shared/FieldLabel";
import {
  EmptyState,
  EmptyStateChecklistGlyph,
} from "@/shared/EmptyState";
import { ChecklistCriterionModal } from "./ChecklistCriterionModal";
import { TaskDetailChecklistItemList } from "./TaskDetailChecklistItemList";
import { TaskChecklistSkeleton } from "./taskLoadingSkeletons";

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
}: TaskDetailChecklistSectionProps) {
  return (
    <div className="task-detail-section" id="task-detail-checklist">
      <div className="task-detail-checklist-head">
        <div className="field-heading-with-req task-detail-checklist-title-row">
          <h3
            className="task-detail-section-heading"
            id="task-checklist-heading"
          >
            Done criteria
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
                  {checklistQuery.error instanceof Error
                    ? checklistQuery.error.message
                    : "Could not load checklist."}
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
        />
      ) : null}
    </div>
  );
}
