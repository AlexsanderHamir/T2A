import { useRef, useState, type FormEvent, type ReactNode } from "react";
import type { ChecklistItemDraft, PriorityChoice, Status } from "@/types";
import type { RichPromptEditorProjectContextProps } from "../rich-prompt";
import type { TestScenario } from "@/tasks/test-scenarios";
import { Modal } from "../../../shared/Modal";
import { MutationErrorBanner } from "../../../shared/MutationErrorBanner";
import { TaskCreateModalPrimaryFields } from "./fields/TaskCreateModalPrimaryFields";
import { taskCreateModalBusyLabel } from "./taskCreateModalBusyLabel";
import { TaskCreateModalFooterActions } from "./fields/TaskCreateModalFooterActions";
import { TaskCreateModalEditFooterActions } from "./fields/TaskCreateModalEditFooterActions";
import { TaskCreateModalAgentSection } from "./fields/TaskCreateModalAgentSection";
import { TaskCreateModalAutonomyToggle } from "./fields/TaskCreateModalAutonomyToggle";
import { TaskCreateModalSchedulingFields } from "./fields/TaskCreateModalSchedulingFields";
import { TaskCreateModalSection } from "./fields/TaskCreateModalSection";
import { TaskCreateModalStatusField } from "./fields/TaskCreateModalStatusField";
import { TaskCreateModalPickupScheduleField } from "./fields/TaskCreateModalPickupScheduleField";
import { SchedulePicker } from "@/shared/time/SchedulePicker";
import {
  TaskCreateModalEvaluationSummary,
  type TaskCreateModalEvaluation,
} from "./fields/TaskCreateModalEvaluationSummary";
import { TestScenariosTrigger } from "./TestScenariosTrigger";
import { TestScenariosPopover } from "./TestScenariosPopover";
import { advancedSummaryLine } from "./advancedSummaryLine";

const noopOnDependsOnChange = (): void => {};

type Props = {
  /** When set, the modal edits an existing task using the same layout as create. */
  editingTaskId?: string | null;
  editingTaskRunner?: string;
  composeStatus?: Status;
  onComposeStatusChange?: (status: Status) => void;
  /** Edit-mode PATCH in flight (maps to modal `busy`). */
  patchPending?: boolean;
  patchError?: string | null;
  /** Client-side validation (e.g. missing title) in edit mode. */
  formError?: string | null;
  pending: boolean;
  saving: boolean;
  draftSaving: boolean;
  draftSaveLabel: string | null;
  draftSaveError: boolean;
  onClose: () => void;
  title: string;
  prompt: string;
  priority: PriorityChoice;
  checklistItems: ChecklistItemDraft[];
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onPriorityChange: (p: PriorityChoice) => void;
  onAppendChecklistCriterion: (item: ChecklistItemDraft | string) => void;
  onUpdateChecklistRow: (index: number, item: ChecklistItemDraft) => void;
  onRemoveChecklistRow: (index: number) => void;
  evaluatePending: boolean;
  evaluation: TaskCreateModalEvaluation | null;
  taskRunner: string;
  taskCursorModel: string;
  onTaskRunnerChange: (runner: string) => void;
  onTaskCursorModelChange: (v: string) => void;
  projectAssignment?: ReactNode;
  automationAssignment?: ReactNode;
  promptProjectContext?: RichPromptEditorProjectContextProps;
  schedule: string | null;
  onScheduleChange: (next: string | null) => void;
  autonomyEnabled: boolean;
  onAutonomyChange: (enabled: boolean) => void;
  autonomyDisabled?: boolean;
  tagsCsv: string;
  milestone: string;
  projectId: string;
  dependsOn: string[];
  onTagsCsvChange: (value: string) => void;
  onMilestoneChange: (value: string) => void;
  onDependsOnChange: (value: string[]) => void;
  appTimezone: string;
  onSaveDraft: () => void;
  onEvaluate: () => void;
  onSubmit: (e: FormEvent) => void;
  createError?: Error | null;
  createFormError?: string | null;
  evaluateError?: Error | null;
  onApplyTestScenario?: (scenario: TestScenario) => void;
};

export function TaskCreateModal({
  editingTaskId = null,
  editingTaskRunner = "",
  composeStatus,
  onComposeStatusChange,
  patchPending = false,
  patchError = null,
  formError = null,
  pending,
  saving,
  draftSaving,
  draftSaveLabel,
  draftSaveError,
  onClose,
  title,
  prompt,
  priority,
  checklistItems,
  onTitleChange,
  onPromptChange,
  onPriorityChange,
  onAppendChecklistCriterion,
  onUpdateChecklistRow,
  onRemoveChecklistRow,
  evaluatePending,
  evaluation,
  taskRunner,
  taskCursorModel,
  onTaskRunnerChange,
  onTaskCursorModelChange,
  projectAssignment,
  automationAssignment,
  promptProjectContext,
  schedule,
  onScheduleChange,
  autonomyEnabled,
  onAutonomyChange,
  autonomyDisabled = false,
  tagsCsv,
  milestone,
  projectId,
  dependsOn,
  onTagsCsvChange,
  onMilestoneChange,
  onDependsOnChange,
  appTimezone,
  onSaveDraft,
  onEvaluate,
  onSubmit,
  createError = null,
  createFormError = null,
  evaluateError = null,
  onApplyTestScenario,
}: Props) {
  const isEdit = editingTaskId != null;
  const disabled = pending || saving;
  const modalBusy = isEdit ? patchPending : pending;
  const modalTitle = isEdit ? "Edit task" : "New task";
  const modalTitleId = isEdit ? "task-edit-modal-title" : "task-create-modal-title";
  const modalDescribedBy = isEdit ? "task-edit-modal-description" : undefined;
  const idsPrefix = isEdit ? "task-edit" : "task-new";
  const status = composeStatus ?? "ready";

  const [scenariosOpen, setScenariosOpen] = useState(false);
  const scenariosTriggerRef = useRef<HTMLButtonElement>(null);

  const handleScenarioPicked = (scenario: TestScenario) => {
    onApplyTestScenario?.(scenario);
    setScenariosOpen(false);
    scenariosTriggerRef.current?.focus();
  };

  const busyLabel = taskCreateModalBusyLabel();

  return (
    <>
      <Modal
        onClose={onClose}
        labelledBy={modalTitleId}
        describedBy={modalDescribedBy}
        size="wide"
        busy={modalBusy}
        busyLabel={isEdit ? undefined : busyLabel}
        dismissibleWhileBusy
      >
        <section className="panel modal-sheet modal-sheet--edit task-create-modal-sheet task-create">
          <header className="task-create-modal-header">
            <div className="task-create-modal-header__top">
              <h2 id={modalTitleId} className="task-create-modal-title">
                {modalTitle}
              </h2>
              {!isEdit && onApplyTestScenario ? (
                <TestScenariosTrigger
                  ref={scenariosTriggerRef}
                  open={scenariosOpen}
                  disabled={disabled}
                  onToggle={() => setScenariosOpen((open) => !open)}
                />
              ) : null}
            </div>
            {isEdit && editingTaskId ? (
              <p
                className="muted stack-tight-zero task-create-modal-task-id"
                id="task-edit-modal-description"
              >
                <code>{editingTaskId}</code>
              </p>
            ) : null}
            {!isEdit && draftSaveLabel ? (
              <p
                className={[
                  "task-create-draft-status",
                  draftSaveError ? "task-create-draft-status--error" : "",
                ]
                  .filter(Boolean)
                  .join(" ")}
                aria-live={draftSaveError ? "assertive" : "polite"}
              >
                {draftSaveLabel}
              </p>
            ) : null}
          </header>

          <form
            className="task-create-modal-form task-create-form"
            onSubmit={onSubmit}
          >
            <div className="task-create-modal-body">
              <TaskCreateModalSection
                variant="essentials"
                title="Essentials"
                lede="What to do, how urgent it is, and how success is judged."
              >
                <TaskCreateModalPrimaryFields
                  idsPrefix={idsPrefix}
                  editorKey={
                    isEdit ? editingTaskId ?? "edit-prompt-modal" : "create-prompt-modal"
                  }
                  disabled={disabled}
                  title={title}
                  onTitleChange={onTitleChange}
                  priority={priority}
                  onPriorityChange={onPriorityChange}
                  prompt={prompt}
                  checklistItems={checklistItems}
                  hideComposeChecklist={false}
                  checklistRequirement={isEdit ? "optional" : "required"}
                  checklistDisabled={isEdit}
                  onPromptChange={onPromptChange}
                  onAppendChecklistCriterion={onAppendChecklistCriterion}
                  onUpdateChecklistRow={onUpdateChecklistRow}
                  onRemoveChecklistRow={onRemoveChecklistRow}
                  projectContext={promptProjectContext}
                />
              </TaskCreateModalSection>

              {projectAssignment ? (
                <TaskCreateModalSection
                  variant="context"
                  title="Project"
                  lede="Scope this task to a project and attach context the agent can reference."
                >
                  {projectAssignment}
                </TaskCreateModalSection>
              ) : null}

              {automationAssignment ? (
                <TaskCreateModalSection
                  variant="context"
                  title="Behaviors"
                  lede="Reusable yes/no instructions injected into the agent prompt at run time."
                >
                  {automationAssignment}
                </TaskCreateModalSection>
              ) : null}

              <TaskCreateModalSection
                variant="execution"
                title="Execution"
                lede="Whether the agent may pick this up and how it runs."
              >
                <TaskCreateModalAutonomyToggle
                  enabled={autonomyEnabled}
                  disabled={disabled || autonomyDisabled}
                  onChange={onAutonomyChange}
                />

                <details className="task-create-advanced">
                  <summary
                    className="task-create-advanced__summary"
                    data-testid="task-create-more-options-toggle"
                  >
                    <span
                      className="task-create-advanced__chevron"
                      aria-hidden="true"
                    />
                    <span className="task-create-advanced__label">
                      More options
                    </span>
                    <span className="task-create-advanced__hint">
                      {advancedSummaryLine({
                        runner: isEdit ? editingTaskRunner : taskRunner,
                        cursorModel: taskCursorModel,
                        schedule,
                        tagsCsv,
                        milestone,
                        dependsOn,
                      })}
                    </span>
                  </summary>
                  <div className="task-create-advanced__body">
                    {isEdit && onComposeStatusChange ? (
                      <TaskCreateModalStatusField
                        id={`${idsPrefix}-status`}
                        status={status}
                        disabled={disabled}
                        onChange={onComposeStatusChange}
                      />
                    ) : null}

                    <TaskCreateModalAgentSection
                      disabled={disabled}
                      variant="createModal"
                      lockRunner={isEdit}
                      runner={isEdit ? editingTaskRunner : taskRunner}
                      cursorModel={taskCursorModel}
                      onRunnerChange={isEdit ? () => {} : onTaskRunnerChange}
                      onCursorModelChange={onTaskCursorModelChange}
                    />

                    {isEdit ? (
                      <TaskCreateModalPickupScheduleField
                        status={status}
                        value={schedule}
                        onChange={onScheduleChange}
                        appTimezone={appTimezone}
                        disabled={disabled}
                        idPrefix={`${idsPrefix}-modal`}
                      />
                    ) : (
                      <SchedulePicker
                        value={schedule}
                        onChange={onScheduleChange}
                        appTimezone={appTimezone}
                        disabled={disabled}
                        idPrefix="task-create-modal"
                      />
                    )}

                    <TaskCreateModalSchedulingFields
                      disabled={disabled}
                      tagsCsv={tagsCsv}
                      milestone={milestone}
                      projectId={projectId}
                      dependsOn={dependsOn}
                      showDependsOn
                      dependsOnDisabled={isEdit}
                      onTagsCsvChange={onTagsCsvChange}
                      onMilestoneChange={onMilestoneChange}
                      onDependsOnChange={
                        isEdit ? noopOnDependsOnChange : onDependsOnChange
                      }
                    />
                  </div>
                </details>
              </TaskCreateModalSection>
            </div>

            {!isEdit ? (
              <TaskCreateModalEvaluationSummary evaluation={evaluation} />
            ) : null}

            {!isEdit ? (
              <MutationErrorBanner
                error={evaluateError}
                fallback="Could not evaluate draft."
                className="task-create-modal-err task-create-modal-err--evaluate"
              />
            ) : null}

            {!isEdit ? (
              <>
                <MutationErrorBanner
                  error={createFormError}
                  className="task-create-modal-err task-create-modal-err--create"
                />

                <MutationErrorBanner
                  error={createError}
                  fallback="Could not create task."
                  className="task-create-modal-err task-create-modal-err--create"
                />
              </>
            ) : (
              <>
                <MutationErrorBanner
                  error={formError}
                  className="task-create-modal-err task-create-modal-err--edit"
                />
                <MutationErrorBanner
                  error={patchError}
                  className="task-edit-form-err task-create-modal-err task-create-modal-err--edit"
                />
              </>
            )}

            {isEdit ? (
              <TaskCreateModalEditFooterActions
                disabled={disabled}
                saveDisabled={!title.trim()}
                onClose={onClose}
              />
            ) : (
              <TaskCreateModalFooterActions
                disabled={disabled}
                draftSaving={draftSaving}
                title={title}
                priority={priority}
                checklistItems={checklistItems}
                evaluatePending={evaluatePending}
                onClose={onClose}
                onSaveDraft={onSaveDraft}
                onEvaluate={onEvaluate}
              />
            )}
          </form>
        </section>
      </Modal>

      {scenariosOpen && onApplyTestScenario && !isEdit ? (
        <TestScenariosPopover
          anchor={scenariosTriggerRef.current}
          onPick={handleScenarioPicked}
          onClose={() => setScenariosOpen(false)}
        />
      ) : null}
    </>
  );
}
