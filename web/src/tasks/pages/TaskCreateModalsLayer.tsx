import {
  ProjectContextPicker,
  ProjectSelect,
  useProjectContextPromptBinding,
  useProjects,
} from "@/projects";
import { useAppTimezone } from "@/shared/time/appTimezone";
import { DraftResumeModal } from "../components/draft-resume";
import { TaskCreateModal } from "../components/task-create-modal";
import type { useTasksApp } from "../hooks/useTasksApp";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

export function TaskCreateModalsLayer({ app }: Props) {
  const appTimezone = useAppTimezone();
  const projects = useProjects({ includeArchived: false, limit: 100 });
  const newPromptProjectContext = useProjectContextPromptBinding({
    projectId: app.createModalOpen ? app.newProjectID : "",
    selectedIds: app.newProjectContextItemIDs,
    onSelectedIdsChange: app.setNewProjectContextItemIDs,
  });

  const assignmentControlsDisabled =
    app.saving || app.createModalAssignmentLocked;

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
          pending={app.createPending}
          saving={app.saving}
          draftSaving={app.draftSavePending}
          draftSaveLabel={app.draftSaveLabel}
          draftSaveError={app.draftSaveError}
          onClose={app.closeCreateModal}
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
          evaluatePending={app.evaluatePending}
          evaluation={app.latestDraftEvaluation}
          taskRunner={app.newTaskRunner}
          taskCursorModel={app.newTaskCursorModel}
          onTaskRunnerChange={app.setNewTaskRunner}
          onTaskCursorModelChange={app.setNewTaskCursorModel}
          projectAssignment={
            <section
              className="task-create-project"
              aria-label="Project assignment"
            >
              <ProjectSelect
                id="task-create-project"
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
                compact
                onChange={app.setNewProjectContextItemIDs}
              />
            </section>
          }
          promptProjectContext={newPromptProjectContext ?? undefined}
          schedule={app.newSchedule}
          onScheduleChange={app.setNewSchedule}
          autonomyEnabled={app.newAutonomyEnabled}
          onAutonomyChange={app.setNewAutonomyEnabled}
          tagsCsv={app.newTagsCsv}
          milestone={app.newMilestone}
          projectId={app.newProjectID}
          dependsOn={app.newDependsOn}
          onTagsCsvChange={app.setNewTagsCsv}
          onMilestoneChange={app.setNewMilestone}
          onDependsOnChange={app.setNewDependsOn}
          appTimezone={appTimezone}
          onSaveDraft={() => void app.saveDraftNow()}
          onEvaluate={() => void app.evaluateDraftBeforeCreate()}
          onSubmit={(e) => void app.submitCreate(e)}
          createError={app.createError}
          createFormError={app.createFormError}
          evaluateError={app.evaluateError}
          onApplyTestScenario={app.applyTestScenario}
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
