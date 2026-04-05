import type { UseQueryResult } from "@tanstack/react-query";
import type { FormEvent } from "react";
import type { TaskChecklistResponse } from "@/types";
import { FieldRequirementBadge } from "@/shared/FieldLabel";
import {
  EmptyState,
  EmptyStateChecklistGlyph,
} from "@/shared/EmptyState";
import { ChecklistCriterionModal } from "./ChecklistCriterionModal";
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
      <div className="task-checklist-intro">
        {!checklistInherit ? (
          <p className="task-checklist-intro-lead">
            The agent checks these off as work progresses. All must be complete
            before this task can be marked done.
          </p>
        ) : (
          <p className="task-checklist-intro-lead muted" role="status">
            Inherited for <strong>this</strong> task; criteria are defined
            upstream.
          </p>
        )}
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
      <div
        className="task-detail-checklist-body"
        aria-labelledby="task-checklist-heading"
      >
        {checklistQuery.isError ? (
          <div className="task-checklist-surface">
            <p className="err-inline task-checklist-surface-pad" role="alert">
              {checklistQuery.error instanceof Error
                ? checklistQuery.error.message
                : "Could not load checklist."}
            </p>
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
          <div className="task-checklist-surface">
            <ul className="task-checklist-list task-checklist-list--grouped">
              {(checklistQuery.data?.items ?? []).map((item) => (
                <li key={item.id} className="task-checklist-row">
                  <div className="task-checklist-row-main">
                    <span
                      className={
                        item.done
                          ? "task-checklist-status task-checklist-status--done"
                          : "task-checklist-status task-checklist-status--pending"
                      }
                      role="img"
                      aria-label={
                        item.done ? "Satisfied" : "Not satisfied yet"
                      }
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
                        onClick={() =>
                          onOpenEditCriterionModal(item.id, item.text)
                        }
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
