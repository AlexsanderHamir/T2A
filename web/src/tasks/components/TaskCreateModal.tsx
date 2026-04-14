import type { FormEvent } from "react";
import type { PriorityChoice, TaskType } from "@/types";
import type { TaskWithDepth } from "../flattenTaskTree";
import type { PendingSubtaskDraft } from "../pendingSubtaskDraft";
import { Modal } from "../../shared/Modal";
import { TaskCreateModalPrimaryFields } from "./TaskCreateModalPrimaryFields";
import { TaskCreateModalParentField } from "./TaskCreateModalParentField";
import { TaskCreateModalPendingSubtasksField } from "./TaskCreateModalPendingSubtasksField";
import { TaskCreateModalDraftNameField } from "./TaskCreateModalDraftNameField";
import { taskCreateModalBusyLabel } from "./taskCreateModalBusyLabel";
import { taskCreateModalDmapReady } from "./taskCreateModalDmapReady";
import { TaskCreateModalInheritChecklistField } from "./TaskCreateModalInheritChecklistField";
import { TaskCreateModalNestedSubtaskModal } from "./TaskCreateModalNestedSubtaskModal";
import { useTaskCreateModalNestedDraft } from "./useTaskCreateModalNestedDraft";
import { TaskCreateModalFooterActions } from "./TaskCreateModalFooterActions";
import {
  TaskCreateModalEvaluationSummary,
  type TaskCreateModalEvaluation,
} from "./TaskCreateModalEvaluationSummary";

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
