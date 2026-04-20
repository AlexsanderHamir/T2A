import { useMemo } from "react";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { DraftResumeModal } from "../components/draft-resume";
import { TaskCreateModal } from "../components/task-create-modal";
import { TaskListSection } from "../components/task-list";
import { useTasksApp } from "../hooks/useTasksApp";
import { useAppTimezone } from "@/shared/time/appTimezone";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

export function TaskHome({ app }: Props) {
  useDocumentTitle(undefined);
  const appTimezone = useAppTimezone();

  /** Row-level busy state for the list only; excludes create/evaluate so modal typing does not re-render the table. */
  const listSaving = app.patchPending || app.deletePending;

  const listSectionProps = useMemo(
    () => ({
      tasks: app.tasks,
      rootTasksOnPage: app.rootTasksOnPage,
      loading: app.loading,
      refreshing: app.listRefreshing,
      saving: listSaving,
      hideBackgroundRefreshHint: app.sseLive,
      listPage: app.taskListPage,
      listPageSize: app.taskListPageSize,
      onListPageChange: app.setTaskListPage,
      onListFiltersChange: app.resetTaskListPage,
      hasNextPage: app.hasNextTaskPage,
      hasPrevPage: app.hasPrevTaskPage,
      onEdit: app.openEdit,
      onRequestDelete: app.requestDelete,
    }),
    [
      app.tasks,
      app.rootTasksOnPage,
      app.loading,
      app.listRefreshing,
      listSaving,
      app.sseLive,
      app.taskListPage,
      app.taskListPageSize,
      app.setTaskListPage,
      app.resetTaskListPage,
      app.hasNextTaskPage,
      app.hasPrevTaskPage,
      app.openEdit,
      app.requestDelete,
    ],
  );

  const listActions = useMemo(
    () => (
      <button
        type="button"
        className="task-home-new-task-btn"
        onClick={app.openCreateModal}
        disabled={app.createModalOpen}
      >
        New task
      </button>
    ),
    [app.openCreateModal, app.createModalOpen],
  );

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
          taskType={app.newTaskType}
          checklistItems={app.newChecklistItems}
          onTitleChange={app.setNewTitle}
          onPromptChange={app.setNewPrompt}
          onPriorityChange={app.setNewPriority}
          onTaskTypeChange={app.setNewTaskType}
          onAppendChecklistCriterion={app.appendNewChecklistCriterion}
          onUpdateChecklistRow={app.updateNewChecklistRow}
          onRemoveChecklistRow={app.removeNewChecklistRow}
          pendingSubtasks={app.pendingSubtasks}
          onAddPendingSubtask={app.addPendingSubtask}
          onUpdatePendingSubtask={app.updatePendingSubtask}
          onRemovePendingSubtask={app.removePendingSubtask}
          evaluatePending={app.evaluatePending}
          evaluation={app.latestDraftEvaluation}
          dmapCommitLimit={app.newDmapCommitLimit}
          dmapDomain={app.newDmapDomain}
          dmapDescription={app.newDmapDescription}
          onDmapCommitLimitChange={app.setNewDmapCommitLimit}
          onDmapDomainChange={app.setNewDmapDomain}
          onDmapDescriptionChange={app.setNewDmapDescription}
          taskRunner={app.newTaskRunner}
          taskCursorModel={app.newTaskCursorModel}
          onTaskRunnerChange={app.setNewTaskRunner}
          onTaskCursorModelChange={app.setNewTaskCursorModel}
          schedule={app.newSchedule}
          onScheduleChange={app.setNewSchedule}
          appTimezone={appTimezone}
          onSaveDraft={() => void app.saveDraftNow()}
          onEvaluate={() => void app.evaluateDraftBeforeCreate()}
          onSubmit={(e) => void app.submitCreate(e)}
          createError={app.createError}
          evaluateError={app.evaluateError}
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

      <div className="task-detail-content--enter">
        <TaskListSection {...listSectionProps} actions={listActions} />
      </div>
    </>
  );
}
