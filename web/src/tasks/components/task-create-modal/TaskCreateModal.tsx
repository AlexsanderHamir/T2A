import { useRef, useState, type FormEvent, type ReactNode, type RefObject } from "react";
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
import { isUiFeatureOmitted } from "@/launch/omittedFeatures";
import { SchedulePicker } from "@/shared/time/SchedulePicker";
import { TestScenariosTrigger } from "./TestScenariosTrigger";
import { TestScenariosPopover } from "./TestScenariosPopover";
import { advancedSummaryLine } from "./advancedSummaryLine";

const noopOnDependsOnChange = (): void => {};

type Props = {
  /** When set, the modal edits an existing task using the same layout as create. */
  editingTaskId?: string | null;
  composeTarget?: "task" | "template";
  composeOperation?: "create" | "edit";
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
  onSubmit: (e: FormEvent) => void;
  createError?: Error | null;
  createFormError?: string | null;
  onApplyTestScenario?: (scenario: TestScenario) => void;
};

type TaskCreateModalPresentation = {
  isTaskEdit: boolean;
  isTemplateMode: boolean;
  isEdit: boolean;
  disabled: boolean;
  tagsAndDependenciesUiEnabled: boolean;
  scheduleUiEnabled: boolean;
  modalBusy: boolean;
  modalTitle: string;
  modalTitleId: string;
  modalDescribedBy: string | undefined;
  idsPrefix: string;
  status: Status;
  showTestScenarios: boolean;
  showDraftStatus: boolean;
};

function resolveTaskCreateModalPresentation(input: {
  editingTaskId: string | null;
  composeTarget: "task" | "template";
  composeOperation: "create" | "edit";
  composeStatus: Status | undefined;
  pending: boolean;
  saving: boolean;
  patchPending: boolean;
  draftSaveLabel: string | null;
  onApplyTestScenario?: (scenario: TestScenario) => void;
}): TaskCreateModalPresentation {
  const isTaskEdit = input.editingTaskId != null;
  const isTemplateMode = input.composeTarget === "template";
  const isEdit = isTaskEdit || (isTemplateMode && input.composeOperation === "edit");
  const disabled = input.pending || input.saving;
  const tagsAndDependenciesUiEnabled = !isUiFeatureOmitted("tagsAndDependencies");
  const scheduleUiEnabled = !isUiFeatureOmitted("schedule");
  const modalBusy = isTaskEdit
    ? input.patchPending
    : input.pending || (isTemplateMode && input.saving);
  const modalTitle = isTaskEdit
    ? "Edit task"
    : isTemplateMode
      ? input.composeOperation === "edit"
        ? "Edit template"
        : "New template"
      : "New task";
  const modalTitleId = isEdit
    ? isTemplateMode
      ? "task-template-edit-modal-title"
      : "task-edit-modal-title"
    : isTemplateMode
      ? "task-template-create-modal-title"
      : "task-create-modal-title";
  const modalDescribedBy = isTaskEdit ? "task-edit-modal-description" : undefined;
  const idsPrefix = isEdit
    ? isTemplateMode
      ? "task-template-edit"
      : "task-edit"
    : isTemplateMode
      ? "task-template-new"
      : "task-new";
  const status = input.composeStatus ?? "ready";
  const showTestScenarios = !isEdit && Boolean(input.onApplyTestScenario);
  const showDraftStatus = !isEdit && !isTemplateMode && Boolean(input.draftSaveLabel);

  return {
    isTaskEdit,
    isTemplateMode,
    isEdit,
    disabled,
    tagsAndDependenciesUiEnabled,
    scheduleUiEnabled,
    modalBusy,
    modalTitle,
    modalTitleId,
    modalDescribedBy,
    idsPrefix,
    status,
    showTestScenarios,
    showDraftStatus,
  };
}

function TaskCreateModalHeader(props: {
  presentation: TaskCreateModalPresentation;
  editingTaskId: string | null;
  draftSaveLabel: string | null;
  draftSaveError: boolean;
  disabled: boolean;
  scenariosOpen: boolean;
  scenariosTriggerRef: RefObject<HTMLButtonElement | null>;
  onToggleScenarios: () => void;
}) {
  const {
    presentation,
    editingTaskId,
    draftSaveLabel,
    draftSaveError,
    disabled,
    scenariosOpen,
    scenariosTriggerRef,
    onToggleScenarios,
  } = props;

  return (
    <header className="task-create-modal-header">
      <div className="task-create-modal-header__top">
        <h2 id={presentation.modalTitleId} className="task-create-modal-title">
          {presentation.modalTitle}
        </h2>
        {!presentation.showTestScenarios ? null : (
          <TestScenariosTrigger
            ref={scenariosTriggerRef}
            open={scenariosOpen}
            disabled={disabled}
            onToggle={onToggleScenarios}
          />
        )}
      </div>
      {presentation.isTaskEdit && editingTaskId ? (
        <p
          className="muted stack-tight-zero task-create-modal-task-id"
          id="task-edit-modal-description"
        >
          <code>{editingTaskId}</code>
        </p>
      ) : null}
      {presentation.showDraftStatus ? (
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
  );
}

function TaskCreateModalAdvancedOptions(props: {
  presentation: TaskCreateModalPresentation;
  editingTaskRunner: string;
  taskRunner: string;
  taskCursorModel: string;
  onTaskRunnerChange: (runner: string) => void;
  onTaskCursorModelChange: (v: string) => void;
  onComposeStatusChange?: (status: Status) => void;
  schedule: string | null;
  onScheduleChange: (next: string | null) => void;
  appTimezone: string;
  tagsCsv: string;
  milestone: string;
  projectId: string;
  dependsOn: string[];
  onTagsCsvChange: (value: string) => void;
  onMilestoneChange: (value: string) => void;
  onDependsOnChange: (value: string[]) => void;
}) {
  const {
    presentation,
    editingTaskRunner,
    taskRunner,
    taskCursorModel,
    onTaskRunnerChange,
    onTaskCursorModelChange,
    onComposeStatusChange,
    schedule,
    onScheduleChange,
    appTimezone,
    tagsCsv,
    milestone,
    projectId,
    dependsOn,
    onTagsCsvChange,
    onMilestoneChange,
    onDependsOnChange,
  } = props;

  return (
    <details className="task-create-advanced">
      <summary
        className="task-create-advanced__summary"
        data-testid="task-create-more-options-toggle"
      >
        <span className="task-create-advanced__chevron" aria-hidden="true" />
        <span className="task-create-advanced__label">More options</span>
        <span className="task-create-advanced__hint">
          {advancedSummaryLine({
            runner: presentation.isTaskEdit ? editingTaskRunner : taskRunner,
            cursorModel: taskCursorModel,
            schedule,
            tagsCsv,
            milestone,
            dependsOn,
            includeSchedule: presentation.scheduleUiEnabled,
            includeTagsAndDependencies: presentation.tagsAndDependenciesUiEnabled,
          })}
        </span>
      </summary>
      <div className="task-create-advanced__body">
        {presentation.isTaskEdit && onComposeStatusChange ? (
          <TaskCreateModalStatusField
            id={`${presentation.idsPrefix}-status`}
            status={presentation.status}
            disabled={presentation.disabled}
            onChange={onComposeStatusChange}
          />
        ) : null}

        <TaskCreateModalAgentSection
          disabled={presentation.disabled}
          variant="createModal"
          lockRunner={presentation.isTaskEdit}
          runner={presentation.isTaskEdit ? editingTaskRunner : taskRunner}
          cursorModel={taskCursorModel}
          onRunnerChange={presentation.isTaskEdit ? () => {} : onTaskRunnerChange}
          onCursorModelChange={onTaskCursorModelChange}
        />

        {presentation.scheduleUiEnabled ? (
          presentation.isTaskEdit ? (
            <TaskCreateModalPickupScheduleField
              status={presentation.status}
              value={schedule}
              onChange={onScheduleChange}
              appTimezone={appTimezone}
              disabled={presentation.disabled}
              idPrefix={`${presentation.idsPrefix}-modal`}
            />
          ) : (
            <SchedulePicker
              value={schedule}
              onChange={onScheduleChange}
              appTimezone={appTimezone}
              disabled={presentation.disabled}
              idPrefix="task-create-modal"
            />
          )
        ) : null}

        {presentation.tagsAndDependenciesUiEnabled ? (
          <TaskCreateModalSchedulingFields
            disabled={presentation.disabled}
            tagsCsv={tagsCsv}
            milestone={milestone}
            projectId={projectId}
            dependsOn={dependsOn}
            showDependsOn
            dependsOnDisabled={presentation.isTaskEdit}
            onTagsCsvChange={onTagsCsvChange}
            onMilestoneChange={onMilestoneChange}
            onDependsOnChange={
              presentation.isTaskEdit ? noopOnDependsOnChange : onDependsOnChange
            }
          />
        ) : null}
      </div>
    </details>
  );
}

function TaskCreateModalFormBody(props: {
  presentation: TaskCreateModalPresentation;
  editingTaskId: string | null;
  editingTaskRunner: string;
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
  promptProjectContext?: RichPromptEditorProjectContextProps;
  projectAssignment?: ReactNode;
  taskRunner: string;
  taskCursorModel: string;
  onTaskRunnerChange: (runner: string) => void;
  onTaskCursorModelChange: (v: string) => void;
  onComposeStatusChange?: (status: Status) => void;
  autonomyEnabled: boolean;
  autonomyDisabled: boolean;
  onAutonomyChange: (enabled: boolean) => void;
  schedule: string | null;
  onScheduleChange: (next: string | null) => void;
  appTimezone: string;
  tagsCsv: string;
  milestone: string;
  projectId: string;
  dependsOn: string[];
  onTagsCsvChange: (value: string) => void;
  onMilestoneChange: (value: string) => void;
  onDependsOnChange: (value: string[]) => void;
}) {
  const {
    presentation,
    editingTaskId,
    editingTaskRunner,
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
    promptProjectContext,
    projectAssignment,
    taskRunner,
    taskCursorModel,
    onTaskRunnerChange,
    onTaskCursorModelChange,
    onComposeStatusChange,
    autonomyEnabled,
    autonomyDisabled,
    onAutonomyChange,
    schedule,
    onScheduleChange,
    appTimezone,
    tagsCsv,
    milestone,
    projectId,
    dependsOn,
    onTagsCsvChange,
    onMilestoneChange,
    onDependsOnChange,
  } = props;

  return (
    <div className="task-create-modal-body">
      <TaskCreateModalSection
        variant="essentials"
        title="Essentials"
        lede="What to do, how urgent it is, and how success is judged."
      >
        <TaskCreateModalPrimaryFields
          idsPrefix={presentation.idsPrefix}
          editorKey={
            presentation.isTaskEdit
              ? editingTaskId ?? "edit-prompt-modal"
              : presentation.isTemplateMode
                ? "template-prompt-modal"
                : "create-prompt-modal"
          }
          disabled={presentation.disabled}
          title={title}
          onTitleChange={onTitleChange}
          priority={priority}
          onPriorityChange={onPriorityChange}
          prompt={prompt}
          checklistItems={checklistItems}
          hideComposeChecklist={false}
          checklistRequirement={presentation.isTaskEdit ? "optional" : "required"}
          checklistDisabled={presentation.isTaskEdit}
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
        lede="Choose whether the agent should start on its own and which runner to use."
      >
        <TaskCreateModalAutonomyToggle
          enabled={autonomyEnabled}
          disabled={presentation.disabled || autonomyDisabled}
          onChange={onAutonomyChange}
        />

        <TaskCreateModalAdvancedOptions
          presentation={presentation}
          editingTaskRunner={editingTaskRunner}
          taskRunner={taskRunner}
          taskCursorModel={taskCursorModel}
          onTaskRunnerChange={onTaskRunnerChange}
          onTaskCursorModelChange={onTaskCursorModelChange}
          onComposeStatusChange={onComposeStatusChange}
          schedule={schedule}
          onScheduleChange={onScheduleChange}
          appTimezone={appTimezone}
          tagsCsv={tagsCsv}
          milestone={milestone}
          projectId={projectId}
          dependsOn={dependsOn}
          onTagsCsvChange={onTagsCsvChange}
          onMilestoneChange={onMilestoneChange}
          onDependsOnChange={onDependsOnChange}
        />
      </TaskCreateModalSection>
    </div>
  );
}

function TaskCreateModalMutationErrors(props: {
  isTaskEdit: boolean;
  createFormError?: string | null;
  createError?: Error | null;
  formError?: string | null;
  patchError?: string | null;
}) {
  const { isTaskEdit, createFormError, createError, formError, patchError } = props;

  if (!isTaskEdit) {
    return (
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
    );
  }

  return (
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
  );
}

function TaskCreateModalActionFooter(props: {
  presentation: TaskCreateModalPresentation;
  title: string;
  priority: PriorityChoice;
  checklistItems: ChecklistItemDraft[];
  draftSaving: boolean;
  onClose: () => void;
  onSaveDraft: () => void;
}) {
  const {
    presentation,
    title,
    priority,
    checklistItems,
    draftSaving,
    onClose,
    onSaveDraft,
  } = props;

  if (presentation.isTaskEdit) {
    return (
      <TaskCreateModalEditFooterActions
        disabled={presentation.disabled}
        saveDisabled={!title.trim()}
        onClose={onClose}
      />
    );
  }

  if (presentation.isTemplateMode && presentation.isEdit) {
    return (
      <TaskCreateModalEditFooterActions
        disabled={presentation.disabled}
        saveDisabled={!title.trim()}
        onClose={onClose}
      />
    );
  }

  return (
    <TaskCreateModalFooterActions
      variant={presentation.isTemplateMode ? "template" : "task-create"}
      disabled={presentation.disabled}
      draftSaving={draftSaving}
      title={title}
      priority={priority}
      checklistItems={checklistItems}
      onClose={onClose}
      onSaveDraft={presentation.isTemplateMode ? undefined : onSaveDraft}
    />
  );
}

export function TaskCreateModal({
  editingTaskId = null,
  composeTarget = "task",
  composeOperation = "create",
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
  onSubmit,
  createError = null,
  createFormError = null,
  onApplyTestScenario,
}: Props) {
  const presentation = resolveTaskCreateModalPresentation({
    editingTaskId,
    composeTarget,
    composeOperation,
    composeStatus,
    pending,
    saving,
    patchPending,
    draftSaveLabel,
    onApplyTestScenario,
  });

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
        labelledBy={presentation.modalTitleId}
        describedBy={presentation.modalDescribedBy}
        size="wide"
        busy={presentation.modalBusy}
        busyLabel={presentation.isEdit ? undefined : busyLabel}
        dismissibleWhileBusy
      >
        <section className="panel modal-sheet modal-sheet--edit task-create-modal-sheet task-create">
          <TaskCreateModalHeader
            presentation={presentation}
            editingTaskId={editingTaskId}
            draftSaveLabel={draftSaveLabel}
            draftSaveError={draftSaveError}
            disabled={presentation.disabled}
            scenariosOpen={scenariosOpen}
            scenariosTriggerRef={scenariosTriggerRef}
            onToggleScenarios={() => setScenariosOpen((open) => !open)}
          />

          <form
            className="task-create-modal-form task-create-form"
            onSubmit={onSubmit}
          >
            <TaskCreateModalFormBody
              presentation={presentation}
              editingTaskId={editingTaskId}
              editingTaskRunner={editingTaskRunner}
              title={title}
              prompt={prompt}
              priority={priority}
              checklistItems={checklistItems}
              onTitleChange={onTitleChange}
              onPromptChange={onPromptChange}
              onPriorityChange={onPriorityChange}
              onAppendChecklistCriterion={onAppendChecklistCriterion}
              onUpdateChecklistRow={onUpdateChecklistRow}
              onRemoveChecklistRow={onRemoveChecklistRow}
              promptProjectContext={promptProjectContext}
              projectAssignment={projectAssignment}
              taskRunner={taskRunner}
              taskCursorModel={taskCursorModel}
              onTaskRunnerChange={onTaskRunnerChange}
              onTaskCursorModelChange={onTaskCursorModelChange}
              onComposeStatusChange={onComposeStatusChange}
              autonomyEnabled={autonomyEnabled}
              autonomyDisabled={autonomyDisabled}
              onAutonomyChange={onAutonomyChange}
              schedule={schedule}
              onScheduleChange={onScheduleChange}
              appTimezone={appTimezone}
              tagsCsv={tagsCsv}
              milestone={milestone}
              projectId={projectId}
              dependsOn={dependsOn}
              onTagsCsvChange={onTagsCsvChange}
              onMilestoneChange={onMilestoneChange}
              onDependsOnChange={onDependsOnChange}
            />

            <TaskCreateModalMutationErrors
              isTaskEdit={presentation.isTaskEdit}
              createFormError={createFormError}
              createError={createError}
              formError={formError}
              patchError={patchError}
            />

            <TaskCreateModalActionFooter
              presentation={presentation}
              title={title}
              priority={priority}
              checklistItems={checklistItems}
              draftSaving={draftSaving}
              onClose={onClose}
              onSaveDraft={onSaveDraft}
            />
          </form>
        </section>
      </Modal>

      {scenariosOpen && presentation.showTestScenarios ? (
        <TestScenariosPopover
          anchor={scenariosTriggerRef.current}
          onPick={handleScenarioPicked}
          onClose={() => setScenariosOpen(false)}
        />
      ) : null}
    </>
  );
}
