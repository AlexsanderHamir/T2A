import { useRef, useState, type FormEvent, type ReactNode } from "react";
import type { PriorityChoice, TaskType } from "@/types";
import type { RichPromptEditorProjectContextProps } from "../rich-prompt";
import type { TestScenario } from "@/tasks/test-scenarios";
import { Modal } from "../../../shared/Modal";
import { MutationErrorBanner } from "../../../shared/MutationErrorBanner";
import { TaskCreateModalPrimaryFields } from "./fields/TaskCreateModalPrimaryFields";
import { taskCreateModalBusyLabel } from "./taskCreateModalBusyLabel";
import { taskCreateModalDmapReady } from "./dmap/taskCreateModalDmapReady";
import { TaskCreateModalFooterActions } from "./fields/TaskCreateModalFooterActions";
import { TaskCreateModalAgentSection } from "./fields/TaskCreateModalAgentSection";
import { TaskCreateModalAutonomyToggle } from "./fields/TaskCreateModalAutonomyToggle";
import { TaskCreateModalSchedulingFields } from "./fields/TaskCreateModalSchedulingFields";
import { SchedulePicker } from "@/shared/time/SchedulePicker";
import {
  TaskCreateModalEvaluationSummary,
  type TaskCreateModalEvaluation,
} from "./fields/TaskCreateModalEvaluationSummary";
import { TestScenariosTrigger } from "./TestScenariosTrigger";
import { TestScenariosPopover } from "./TestScenariosPopover";
import { advancedSummaryLine } from "./advancedSummaryLine";

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
  projectAssignment?: ReactNode;
  promptProjectContext?: RichPromptEditorProjectContextProps;
  schedule: string | null;
  onScheduleChange: (next: string | null) => void;
  autonomyEnabled: boolean;
  onAutonomyChange: (enabled: boolean) => void;
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
  projectAssignment,
  promptProjectContext,
  schedule,
  onScheduleChange,
  autonomyEnabled,
  onAutonomyChange,
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
  const disabled = pending || saving;
  const dmapMode = taskType === "dmap";
  const dmapReady = taskCreateModalDmapReady(
    dmapMode,
    dmapCommitLimit,
    dmapDomain,
  );

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
        labelledBy="task-create-modal-title"
        size="wide"
        busy={pending}
        busyLabel={busyLabel}
        dismissibleWhileBusy
      >
        <section className="panel modal-sheet modal-sheet--edit task-create-modal-sheet task-create">
          <header className="task-create-modal-header">
            <div className="task-create-modal-header__text">
              <div className="task-create-modal-header__title-row">
                <h2
                  id="task-create-modal-title"
                  className="task-create-modal-title"
                >
                  New task
                </h2>
                {draftSaveLabel ? (
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
              </div>
              <p className="task-create-modal-subtitle">
                Title and prompt are enough to start. Everything else is
                optional.
              </p>
            </div>
            {onApplyTestScenario ? (
              <TestScenariosTrigger
                ref={scenariosTriggerRef}
                open={scenariosOpen}
                disabled={disabled}
                onToggle={() => setScenariosOpen((open) => !open)}
              />
            ) : null}
          </header>

          <form
            className="task-create-modal-form task-create-form"
            onSubmit={onSubmit}
          >
            <div className="task-create-modal-section task-create-modal-section--essentials">
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
                checklistRequirement="optional"
                onPromptChange={onPromptChange}
                onAppendChecklistCriterion={onAppendChecklistCriterion}
                onUpdateChecklistRow={onUpdateChecklistRow}
                onRemoveChecklistRow={onRemoveChecklistRow}
                projectContext={promptProjectContext}
              />

              {projectAssignment}
            </div>

            <div className="task-create-modal-section task-create-modal-section--execution">
              <TaskCreateModalAutonomyToggle
                enabled={autonomyEnabled}
                disabled={disabled}
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
                  <span className="task-create-advanced__label">More options</span>
                  <span className="task-create-advanced__hint">
                    {advancedSummaryLine({
                      runner: taskRunner,
                      cursorModel: taskCursorModel,
                      schedule,
                      tagsCsv,
                      milestone,
                      dependsOn,
                    })}
                  </span>
                </summary>
                <div className="task-create-advanced__body">
                  <TaskCreateModalAgentSection
                    disabled={disabled}
                    variant="createModal"
                    runner={taskRunner}
                    cursorModel={taskCursorModel}
                    onRunnerChange={onTaskRunnerChange}
                    onCursorModelChange={onTaskCursorModelChange}
                  />

                  <SchedulePicker
                    value={schedule}
                    onChange={onScheduleChange}
                    appTimezone={appTimezone}
                    disabled={disabled}
                    idPrefix="task-create-modal"
                  />

                  <TaskCreateModalSchedulingFields
                    disabled={disabled}
                    tagsCsv={tagsCsv}
                    milestone={milestone}
                    projectId={projectId}
                    dependsOn={dependsOn}
                    onTagsCsvChange={onTagsCsvChange}
                    onMilestoneChange={onMilestoneChange}
                    onDependsOnChange={onDependsOnChange}
                  />
                </div>
              </details>
            </div>

            <TaskCreateModalEvaluationSummary evaluation={evaluation} />

            <MutationErrorBanner
              error={evaluateError}
              fallback="Could not evaluate draft."
              className="task-create-modal-err task-create-modal-err--evaluate"
            />

            <MutationErrorBanner
              error={createFormError}
              className="task-create-modal-err task-create-modal-err--create"
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

      {scenariosOpen && onApplyTestScenario ? (
        <TestScenariosPopover
          anchor={scenariosTriggerRef.current}
          onPick={handleScenarioPicked}
          onClose={() => setScenariosOpen(false)}
        />
      ) : null}
    </>
  );
}
