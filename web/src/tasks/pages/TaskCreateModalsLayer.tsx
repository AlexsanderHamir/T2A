import {
  ProjectContextPicker,
  ProjectSelect,
  useProjectContextPromptBinding,
  useProjects,
} from "@/projects";
import { isUiFeatureOmitted } from "@/launch/omittedFeatures";
import { useAppTimezone } from "@/shared/time/appTimezone";
import { DraftResumeModal } from "../components/draft-resume";
import { TaskCreateModal } from "../components/task-create-modal";
import { useTasksAppContext } from "../app/TasksAppProvider";

export function TaskCreateModalsLayer() {
  const app = useTasksAppContext();
  const appTimezone = useAppTimezone();
  const projectsUiEnabled = !isUiFeatureOmitted("projects");
  const projects = useProjects({
    includeArchived: false,
    limit: 100,
    enabled: projectsUiEnabled && app.createModalOpen,
  });
  const promptProjectContext = useProjectContextPromptBinding({
    projectId:
      projectsUiEnabled && app.createModalOpen ? app.newProjectID : "",
    selectedIds: app.newProjectContextItemIDs,
    onSelectedIdsChange: app.setNewProjectContextItemIDs,
  });

  const assignmentControlsDisabled =
    app.saving || app.createModalAssignmentLocked;
  const isEditing = app.editingTaskId != null;
  const isTemplateMode = app.composeTarget === "template";
  const isTemplateEdit = isTemplateMode && app.composeOperation === "edit";

  const handleResumeDraft = (id: string) => {
    void app.resumeDraftByID(id).catch(() => {
      // Error state is exposed by the hook and rendered in the modal.
    });
  };

  return (
    <>
      {app.createEntryDraftErrorHint ? (
        <div className="err error-banner" role="alert">
          <span className="error-banner__text">
            Saved drafts are unavailable right now, so a fresh task form was opened.
          </span>
          <button
            type="button"
            className="secondary"
            onClick={() => {
              void app.retryCreateEntryDraftLoad();
            }}
          >
            Retry loading drafts
          </button>
        </div>
      ) : null}
      {app.createModalOpen ? (
        <TaskCreateModal
          editingTaskId={app.editingTaskId}
          composeTarget={app.composeTarget}
          composeOperation={app.composeOperation}
          editingTaskRunner={app.editingTaskRunner}
          composeStatus={app.composeStatus}
          onComposeStatusChange={app.setComposeStatus}
          patchPending={app.patchPending}
          patchError={app.patchError}
          formError={app.editFormError}
          pending={isTemplateMode ? app.templateSavePending : app.createPending}
          saving={app.saving}
          draftSaving={isEditing || isTemplateMode ? false : app.draftSavePending}
          draftSaveLabel={isEditing || isTemplateMode ? null : app.draftSaveLabel}
          draftSaveError={isEditing || isTemplateMode ? false : app.draftSaveError}
          onClose={app.closeEdit}
          title={app.newTitle}
          prompt={app.newPrompt}
          priority={app.newPriority}
          checklistItems={app.newChecklistItems}
          onTitleChange={app.setNewTitle}
          onPromptChange={app.setNewPrompt}
          onPriorityChange={app.setNewPriority}
          onAppendChecklistCriterion={app.appendNewChecklistCriterion}
          onUpdateChecklistRow={app.updateNewChecklistRow}
          onRemoveChecklistRow={app.removeNewChecklistRow}
          taskRunner={isEditing ? app.editingTaskRunner : app.newTaskRunner}
          taskCursorModel={app.newTaskCursorModel}
          onTaskRunnerChange={app.setNewTaskRunner}
          onTaskCursorModelChange={app.setNewTaskCursorModel}
          projectAssignment={
            projectsUiEnabled ? (
              <section
                className="task-create-project"
                aria-label="Project assignment"
              >
                <ProjectSelect
                  id={isEditing ? "task-edit-project" : "task-create-project"}
                  value={app.newProjectID}
                  projects={projects.data?.projects ?? []}
                  loading={projects.isLoading}
                  disabled={assignmentControlsDisabled}
                  onChange={(projectId) => {
                    app.setNewProjectID(projectId);
                    app.setNewProjectContextItemIDs([]);
                  }}
                />
                <ProjectContextPicker
                  projectId={app.newProjectID}
                  selectedIds={app.newProjectContextItemIDs}
                  disabled={app.saving}
                  compact={!isEditing}
                  onChange={app.setNewProjectContextItemIDs}
                />
              </section>
            ) : undefined
          }
          promptProjectContext={
            projectsUiEnabled ? (promptProjectContext ?? undefined) : undefined
          }
          schedule={app.newSchedule}
          onScheduleChange={app.setNewSchedule}
          autonomyEnabled={
            isEditing ? app.composeStatus === "ready" : app.newAutonomyEnabled
          }
          onAutonomyChange={app.setNewAutonomyEnabled}
          autonomyDisabled={isEditing}
          tagsCsv={app.newTagsCsv}
          milestone={app.newMilestone}
          projectId={app.newProjectID}
          worktreeId={app.newWorktreeID}
          branchId={app.newBranchID}
          onWorktreeChange={app.setNewWorktreeID}
          onBranchChange={app.setNewBranchID}
          dependsOn={app.newDependsOn}
          onTagsCsvChange={app.setNewTagsCsv}
          onMilestoneChange={app.setNewMilestone}
          onDependsOnChange={app.setNewDependsOn}
          appTimezone={appTimezone}
          onSaveDraft={() => {
            if (!isEditing) void app.saveDraftNow();
          }}
          onSubmit={(e) => void app.submitComposeModal(e)}
          createError={
            isEditing ? null : isTemplateMode ? app.templateSaveError : app.createError
          }
          createFormError={isEditing ? null : app.createFormError}
          onApplyTestScenario={
            isEditing || isTemplateEdit ? undefined : app.applyTestScenario
          }
        />
      ) : null}
      {app.draftPickerOpen ? (
        <DraftResumeModal
          drafts={app.taskDrafts}
          onClose={() => app.setDraftPickerOpen(false)}
          onStartFresh={() => void app.startFreshDraft()}
          onResume={handleResumeDraft}
          loading={app.draftListLoading}
          loadError={app.draftListError}
          onRetryLoad={() => {
            void app.retryDraftList();
          }}
          resumePending={app.resumeDraftPending}
          resumeError={app.resumeDraftError}
        />
      ) : null}
    </>
  );
}
