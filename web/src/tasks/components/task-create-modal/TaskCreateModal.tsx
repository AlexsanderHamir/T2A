import type { FormEvent } from "react";
import type { PriorityChoice, TaskType } from "@/types";
import type { PendingSubtaskDraft, TaskWithDepth } from "../../task-tree";
import { errorMessage } from "@/lib/errorMessage";
import { Modal } from "../../../shared/Modal";
import { TaskCreateModalPrimaryFields } from "./fields/TaskCreateModalPrimaryFields";
import { TaskCreateModalParentField } from "./fields/TaskCreateModalParentField";
import { TaskCreateModalPendingSubtasksField } from "./fields/TaskCreateModalPendingSubtasksField";
import { TaskCreateModalDraftNameField } from "./fields/TaskCreateModalDraftNameField";
import { taskCreateModalBusyLabel } from "./taskCreateModalBusyLabel";
import { taskCreateModalDmapReady } from "./dmap/taskCreateModalDmapReady";
import { TaskCreateModalInheritChecklistField } from "./fields/TaskCreateModalInheritChecklistField";
import { TaskCreateModalNestedSubtaskModal } from "./nested/TaskCreateModalNestedSubtaskModal";
import { useTaskCreateModalNestedDraft } from "./nested/useTaskCreateModalNestedDraft";
import { TaskCreateModalFooterActions } from "./fields/TaskCreateModalFooterActions";
import {
  TaskCreateModalEvaluationSummary,
  type TaskCreateModalEvaluation,
} from "./fields/TaskCreateModalEvaluationSummary";

type Props = {
  pending: boolean;
  saving: boolean;
  parentOptionsLoading?: boolean;
  draftSaving: boolean;
  draftSaveLabel: string | null;
  draftSaveError: boolean;
  onClose: () => void;
  title: string;
  prompt: string;
  priority: PriorityChoice;
  taskType: TaskType;
  checklistItems: string[];
  parentOptions: TaskWithDepth[];
  parentId: string;
  checklistInherit: boolean;
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onPriorityChange: (p: PriorityChoice) => void;
  onTaskTypeChange: (t: TaskType) => void;
  onParentIdChange: (id: string) => void;
  onChecklistInheritChange: (v: boolean) => void;
  onAppendChecklistCriterion: (text: string) => void;
  onUpdateChecklistRow: (index: number, text: string) => void;
  onRemoveChecklistRow: (index: number) => void;
  pendingSubtasks: PendingSubtaskDraft[];
  onAddPendingSubtask: (d: PendingSubtaskDraft) => void;
  onUpdatePendingSubtask: (index: number, d: PendingSubtaskDraft) => void;
  onRemovePendingSubtask: (index: number) => void;
  evaluatePending: boolean;
  evaluation: TaskCreateModalEvaluation | null;
  draftName: string;
  onDraftNameChange: (name: string) => void;
  dmapCommitLimit: string;
  dmapDomain: string;
  dmapDescription: string;
  onDmapCommitLimitChange: (value: string) => void;
  onDmapDomainChange: (value: string) => void;
  onDmapDescriptionChange: (value: string) => void;
  onSaveDraft: () => void;
  onEvaluate: () => void;
  onSubmit: (e: FormEvent) => void;
  /**
   * Error from the most recent `createMutation` (POST `/tasks`).
   * Surfaced as a `.err role="alert"` callout above the footer
   * actions so the user sees the failure inside the modal — without
   * this, the modal stays open with no feedback after a failed
   * submit because the global `app.error` banner is hidden behind
   * the modal backdrop. Pass `createMutation.error` directly (typed
   * as `Error | null` per react-query v5).
   */
  createError?: Error | null;
  /**
   * Error from the most recent `evaluateDraftMutation`. Same gap as
   * `createError`: the global banner is hidden behind the modal,
   * so without this prop a failed evaluation produces a click that
   * appears to do nothing. Surfaced near the evaluation summary
   * so the user understands which action failed.
   */
  evaluateError?: Error | null;
};

export function TaskCreateModal({
  pending,
  saving,
  parentOptionsLoading = false,
  draftSaving,
  draftSaveLabel,
  draftSaveError,
  onClose,
  title,
  prompt,
  priority,
  taskType,
  checklistItems,
  parentOptions,
  parentId,
  checklistInherit,
  onTitleChange,
  onPromptChange,
  onPriorityChange,
  onTaskTypeChange,
  onParentIdChange,
  onChecklistInheritChange,
  onAppendChecklistCriterion,
  onUpdateChecklistRow,
  onRemoveChecklistRow,
  pendingSubtasks,
  onAddPendingSubtask,
  onUpdatePendingSubtask,
  onRemovePendingSubtask,
  evaluatePending,
  evaluation,
  draftName,
  onDraftNameChange,
  dmapCommitLimit,
  dmapDomain,
  dmapDescription,
  onDmapCommitLimitChange,
  onDmapDomainChange,
  onDmapDescriptionChange,
  onSaveDraft,
  onEvaluate,
  onSubmit,
  createError = null,
  evaluateError = null,
}: Props) {
  const disabled = pending || saving;
  const hasParent = Boolean(parentId.trim());
  const hideComposeChecklist = hasParent && checklistInherit;
  const dmapMode = taskType === "dmap";
  const dmapReady = taskCreateModalDmapReady(
    dmapMode,
    dmapCommitLimit,
    dmapDomain,
  );

  const {
    nestedOpen,
    nestedInstanceKey,
    nestedInitial,
    openNestedNew,
    openNestedEdit,
    handleNestedClose,
    handleNestedSave,
  } = useTaskCreateModalNestedDraft({
    pendingSubtasks,
    onAddPendingSubtask,
    onUpdatePendingSubtask,
  });

  const busyLabel = taskCreateModalBusyLabel(hasParent, pendingSubtasks.length);

  return (
    <>
      <Modal
        onClose={onClose}
        labelledBy="task-create-modal-title"
        size="wide"
        busy={pending}
        busyLabel={busyLabel}
        // While `createMutation` is pending the spinner overlay still
        // gives in-flight feedback, but the user can step away from
        // the modal (Escape / backdrop) without losing context. Safe
        // because `useTasksApp.createMutation.onSuccess` gates its
        // `closeCreateModal()` call on `newDraftIDRef.current ===
        // variables.draft_id` and `resetNewTaskForm` clears
        // `requestedResumeRef`, so a stale create resolution can no
        // longer slam shut a draft the user has switched to. Server-
        // truth invalidations (`taskQueryKeys.all`, `task-stats`,
        // `task-drafts`) still fire so the new task appears in the
        // list even if the modal was already closed by the user.
        dismissibleWhileBusy
      >
        <section className="panel modal-sheet modal-sheet--edit task-create-modal-sheet task-create">
          <h2 id="task-create-modal-title">
            {hasParent ? "New subtask" : "New task"}
          </h2>
          <form
            className="task-create-modal-form task-create-form"
            onSubmit={onSubmit}
          >
            <TaskCreateModalDraftNameField
              draftName={draftName}
              onDraftNameChange={onDraftNameChange}
              disabled={disabled}
              draftSaveLabel={draftSaveLabel}
              draftSaveError={draftSaveError}
            />
            <TaskCreateModalParentField
              parentOptionsLoading={parentOptionsLoading}
              parentId={parentId}
              parentOptions={parentOptions}
              onParentIdChange={onParentIdChange}
              disabled={disabled}
              hasParent={hasParent}
            />

            <TaskCreateModalPrimaryFields
              dmapMode={dmapMode}
              disabled={disabled}
              title={title}
              onTitleChange={onTitleChange}
              priority={priority}
              onPriorityChange={onPriorityChange}
              taskType={taskType}
              onTaskTypeChange={onTaskTypeChange}
              dmapCommitLimit={dmapCommitLimit}
              dmapDomain={dmapDomain}
              dmapDescription={dmapDescription}
              onDmapCommitLimitChange={onDmapCommitLimitChange}
              onDmapDomainChange={onDmapDomainChange}
              onDmapDescriptionChange={onDmapDescriptionChange}
              prompt={prompt}
              checklistItems={checklistItems}
              hideComposeChecklist={hideComposeChecklist}
              onPromptChange={onPromptChange}
              onAppendChecklistCriterion={onAppendChecklistCriterion}
              onUpdateChecklistRow={onUpdateChecklistRow}
              onRemoveChecklistRow={onRemoveChecklistRow}
            />

            {hasParent && !dmapMode ? (
              <TaskCreateModalInheritChecklistField
                checklistInherit={checklistInherit}
                disabled={disabled}
                onChecklistInheritChange={onChecklistInheritChange}
              />
            ) : null}

            {!hasParent && !dmapMode ? (
              <TaskCreateModalPendingSubtasksField
                pendingSubtasks={pendingSubtasks}
                disabled={disabled}
                onOpenNestedNew={openNestedNew}
                onOpenNestedEdit={openNestedEdit}
                onRemovePendingSubtask={onRemovePendingSubtask}
              />
            ) : null}

            <TaskCreateModalEvaluationSummary evaluation={evaluation} />

            {evaluateError ? (
              <div
                className="err task-create-modal-err task-create-modal-err--evaluate"
                role="alert"
              >
                {errorMessage(evaluateError, "Could not evaluate draft.")}
              </div>
            ) : null}

            {createError ? (
              <div
                className="err task-create-modal-err task-create-modal-err--create"
                role="alert"
              >
                {errorMessage(createError, "Could not create task.")}
              </div>
            ) : null}

            <TaskCreateModalFooterActions
              disabled={disabled}
              draftSaving={draftSaving}
              title={title}
              priority={priority}
              dmapReady={dmapReady}
              evaluatePending={evaluatePending}
              hasParent={hasParent}
              onClose={onClose}
              onSaveDraft={onSaveDraft}
              onEvaluate={onEvaluate}
            />
          </form>
        </section>
      </Modal>

      <TaskCreateModalNestedSubtaskModal
        open={nestedOpen}
        instanceKey={nestedInstanceKey}
        initialDraft={nestedInitial}
        onClose={handleNestedClose}
        onSave={handleNestedSave}
      />
    </>
  );
}
