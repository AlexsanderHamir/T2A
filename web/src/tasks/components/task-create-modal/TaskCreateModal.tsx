import { useRef, useState, type FormEvent, type ReactNode } from "react";
import type { PriorityChoice } from "@/types";
import type { RichPromptEditorProjectContextProps } from "../rich-prompt";
import type { TestScenario } from "@/tasks/test-scenarios";
import { Modal } from "../../../shared/Modal";
import { MutationErrorBanner } from "../../../shared/MutationErrorBanner";
import { TaskCreateModalPrimaryFields } from "./fields/TaskCreateModalPrimaryFields";
import { taskCreateModalBusyLabel } from "./taskCreateModalBusyLabel";
import { TaskCreateModalFooterActions } from "./fields/TaskCreateModalFooterActions";
import { TaskCreateModalAgentSection } from "./fields/TaskCreateModalAgentSection";
import { TaskCreateModalAutonomyToggle } from "./fields/TaskCreateModalAutonomyToggle";
import { TaskCreateModalSchedulingFields } from "./fields/TaskCreateModalSchedulingFields";
import { TaskCreateModalSection } from "./fields/TaskCreateModalSection";
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
  checklistItems: string[];
  onTitleChange: (v: string) => void;
  onPromptChange: (v: string) => void;
  onPriorityChange: (p: PriorityChoice) => void;
  onAppendChecklistCriterion: (text: string) => void;
  onUpdateChecklistRow: (index: number, text: string) => void;
  onRemoveChecklistRow: (index: number) => void;
  evaluatePending: boolean;
  evaluation: TaskCreateModalEvaluation | null;
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
            <div className="task-create-modal-header__top">
              <h2
                id="task-create-modal-title"
                className="task-create-modal-title"
              >
                New task
              </h2>
              {onApplyTestScenario ? (
                <TestScenariosTrigger
                  ref={scenariosTriggerRef}
                  open={scenariosOpen}
                  disabled={disabled}
                  onToggle={() => setScenariosOpen((open) => !open)}
                />
              ) : null}
            </div>
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
          </header>

          <form
            className="task-create-modal-form task-create-form"
            onSubmit={onSubmit}
          >
            <div className="task-create-modal-form__scroll">
            <div className="task-create-modal-body">
              <TaskCreateModalSection
                variant="essentials"
                title="Essentials"
                lede="What to do, how urgent it is, and how success is judged."
              >
                <TaskCreateModalPrimaryFields
                  disabled={disabled}
                  title={title}
                  onTitleChange={onTitleChange}
                  priority={priority}
                  onPriorityChange={onPriorityChange}
                  prompt={prompt}
                  checklistItems={checklistItems}
                  hideComposeChecklist={false}
                  checklistRequirement="required"
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

              <TaskCreateModalSection
                variant="execution"
                title="Execution"
                lede="Whether the agent may pick this up and how it runs."
              >
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
                    <span className="task-create-advanced__label">
                      More options
                    </span>
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
              </TaskCreateModalSection>
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
            </div>

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
