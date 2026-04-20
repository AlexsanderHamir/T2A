import type { FormEvent } from "react";
import type { PriorityChoice, TaskType } from "@/types";
import type { PendingSubtaskDraft } from "../../task-tree";
import { Modal } from "../../../shared/Modal";
import { MutationErrorBanner } from "../../../shared/MutationErrorBanner";
import { TaskCreateModalPrimaryFields } from "./fields/TaskCreateModalPrimaryFields";
import { TaskCreateModalPendingSubtasksField } from "./fields/TaskCreateModalPendingSubtasksField";
import { taskCreateModalBusyLabel } from "./taskCreateModalBusyLabel";
import { taskCreateModalDmapReady } from "./dmap/taskCreateModalDmapReady";
import { TaskCreateModalNestedSubtaskModal } from "./nested/TaskCreateModalNestedSubtaskModal";
import { useTaskCreateModalNestedDraft } from "./nested/useTaskCreateModalNestedDraft";
import { TaskCreateModalFooterActions } from "./fields/TaskCreateModalFooterActions";
import { TaskCreateModalAgentSection } from "./fields/TaskCreateModalAgentSection";
import { SchedulePicker } from "@/shared/time/SchedulePicker";
import {
  TaskCreateModalEvaluationSummary,
  type TaskCreateModalEvaluation,
} from "./fields/TaskCreateModalEvaluationSummary";

type Props = {
  pending: boolean;
  saving: boolean;
  draftSaving: boolean;
  draftSaveLabel: string | null;
  draftSaveError: boolean;
  onClose: () => void;
  title: string;
  prompt: string;
  priority: PriorityChoice;
  taskType: TaskType;
  checklistItems: string[];
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onPriorityChange: (p: PriorityChoice) => void;
  onTaskTypeChange: (t: TaskType) => void;
  onAppendChecklistCriterion: (text: string) => void;
  onUpdateChecklistRow: (index: number, text: string) => void;
  onRemoveChecklistRow: (index: number) => void;
  pendingSubtasks: PendingSubtaskDraft[];
  onAddPendingSubtask: (d: PendingSubtaskDraft) => void;
  onUpdatePendingSubtask: (index: number, d: PendingSubtaskDraft) => void;
  onRemovePendingSubtask: (index: number) => void;
  evaluatePending: boolean;
  evaluation: TaskCreateModalEvaluation | null;
  dmapCommitLimit: string;
  dmapDomain: string;
  dmapDescription: string;
  onDmapCommitLimitChange: (value: string) => void;
  onDmapDomainChange: (value: string) => void;
  onDmapDescriptionChange: (value: string) => void;
  taskRunner: string;
  taskCursorModel: string;
  onTaskRunnerChange: (runner: string) => void;
  onTaskCursorModelChange: (v: string) => void;
  /**
   * Future pickup time as an RFC3339 UTC ISO string, or `null` when
   * the operator wants the task picked up immediately. Plumbed
   * straight into the `SchedulePicker` rendered inside the modal.
   * The modal owns no schedule state of its own — `useTasksApp`
   * holds the canonical source so the same value survives a
   * close+reopen of the same draft and resets cleanly on a fresh
   * draft.
   */
  schedule: string | null;
  onScheduleChange: (next: string | null) => void;
  /**
   * IANA timezone the picker should render its wall-clock literal
   * + caption in. Forwarded as a prop (rather than read from a hook
   * inside the picker) so the modal owns the "look up the operator
   * timezone" decision once and the picker stays trivially testable.
   */
  appTimezone: string;
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
  draftSaving,
  draftSaveLabel,
  draftSaveError,
  onClose,
  title,
  prompt,
  priority,
  taskType,
  checklistItems,
  onTitleChange,
  onPromptChange,
  onPriorityChange,
  onTaskTypeChange,
  onAppendChecklistCriterion,
  onUpdateChecklistRow,
  onRemoveChecklistRow,
  pendingSubtasks,
  onAddPendingSubtask,
  onUpdatePendingSubtask,
  onRemovePendingSubtask,
  evaluatePending,
  evaluation,
  dmapCommitLimit,
  dmapDomain,
  dmapDescription,
  onDmapCommitLimitChange,
  onDmapDomainChange,
  onDmapDescriptionChange,
  taskRunner,
  taskCursorModel,
  onTaskRunnerChange,
  onTaskCursorModelChange,
  schedule,
  onScheduleChange,
  appTimezone,
  onSaveDraft,
  onEvaluate,
  onSubmit,
  createError = null,
  evaluateError = null,
}: Props) {
  const disabled = pending || saving;
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

  const busyLabel = taskCreateModalBusyLabel(pendingSubtasks.length);

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
          <h2 id="task-create-modal-title" className="term-arrow">
            <span>New task</span>
          </h2>
          {draftSaveLabel ? (
            <p
              className={[
                "task-create-draft-status",
                draftSaveError ? "task-create-draft-status--error" : "muted",
              ]
                .filter(Boolean)
                .join(" ")}
              aria-live={draftSaveError ? "assertive" : "polite"}
            >
              {draftSaveLabel}
            </p>
          ) : null}
          <form
            className="task-create-modal-form task-create-form"
            onSubmit={onSubmit}
          >
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
              hideComposeChecklist={false}
              onPromptChange={onPromptChange}
              onAppendChecklistCriterion={onAppendChecklistCriterion}
              onUpdateChecklistRow={onUpdateChecklistRow}
              onRemoveChecklistRow={onRemoveChecklistRow}
            />

            <TaskCreateModalAgentSection
              disabled={disabled}
              runner={taskRunner}
              cursorModel={taskCursorModel}
              onRunnerChange={onTaskRunnerChange}
              onCursorModelChange={onTaskCursorModelChange}
            />

            {!dmapMode ? (
              <TaskCreateModalPendingSubtasksField
                pendingSubtasks={pendingSubtasks}
                disabled={disabled}
                onOpenNestedNew={openNestedNew}
                onOpenNestedEdit={openNestedEdit}
                onRemovePendingSubtask={onRemovePendingSubtask}
              />
            ) : null}

            <SchedulePicker
              value={schedule}
              onChange={onScheduleChange}
              appTimezone={appTimezone}
              disabled={disabled}
              idPrefix="task-create-modal"
            />

            <TaskCreateModalEvaluationSummary evaluation={evaluation} />

            <MutationErrorBanner
              error={evaluateError}
              fallback="Could not evaluate draft."
              className="task-create-modal-err task-create-modal-err--evaluate"
            />

            <MutationErrorBanner
              error={createError}
              fallback="Could not create task."
              className="task-create-modal-err task-create-modal-err--create"
            />

            <TaskCreateModalFooterActions
              disabled={disabled}
              draftSaving={draftSaving}
              title={title}
              priority={priority}
              dmapReady={dmapReady}
              evaluatePending={evaluatePending}
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
