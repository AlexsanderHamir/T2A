import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { DraftResumeModal } from "../components/DraftResumeModal";
import { TaskCreateModal } from "../components/TaskCreateModal";
import { TaskListSection } from "../components/TaskListSection";
import { useTasksApp } from "../hooks/useTasksApp";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

export function TaskHome({ app }: Props) {
  useDocumentTitle(undefined);
  const handleResumeDraft = (id: string) => {
    void app.resumeDraftByID(id).catch(() => {
      // Error state is exposed by the hook and rendered in the modal.
    });
  };
  const totalTasks = app.taskStats?.total ?? app.tasks.length;
  const readyTasks =
    app.taskStats?.by_status.ready ??
    app.taskStats?.ready ??
    app.tasks.filter((t) => t.status === "ready").length;
  const criticalTasks =
    app.taskStats?.by_priority.critical ??
    app.taskStats?.critical ??
    app.tasks.filter((t) => t.priority === "critical").length;
  const parentTasks =
    app.taskStats?.by_scope.parent ??
    app.tasks.filter((t) => !t.parent_id).length;
  const subtaskTasks =
    app.taskStats?.by_scope.subtask ??
    app.tasks.filter((t) => Boolean(t.parent_id)).length;

  return (
    <>
      {app.createEntryDraftErrorHint ? (
        <div className="row stack-row-actions">
          <p role="alert">
            Saved drafts are unavailable right now, so a fresh task form was opened.
          </p>
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
          parentOptionsLoading={app.loading}
          draftSaving={app.draftSavePending}
          draftSaveLabel={app.draftSaveLabel}
          draftSaveError={app.draftSaveError}
          onClose={app.closeCreateModal}
          title={app.newTitle}
          prompt={app.newPrompt}
          priority={app.newPriority}
          taskType={app.newTaskType}
          checklistItems={app.newChecklistItems}
          parentOptions={app.parentPickerTasks}
          parentId={app.newParentId}
          checklistInherit={app.newChecklistInherit}
          onTitleChange={app.setNewTitle}
          onPromptChange={app.setNewPrompt}
          onPriorityChange={app.setNewPriority}
          onTaskTypeChange={app.setNewTaskType}
          onParentIdChange={app.setNewParentId}
          onChecklistInheritChange={app.setNewChecklistInherit}
          onAppendChecklistCriterion={app.appendNewChecklistCriterion}
          onUpdateChecklistRow={app.updateNewChecklistRow}
          onRemoveChecklistRow={app.removeNewChecklistRow}
          pendingSubtasks={app.pendingSubtasks}
          onAddPendingSubtask={app.addPendingSubtask}
          onUpdatePendingSubtask={app.updatePendingSubtask}
          onRemovePendingSubtask={app.removePendingSubtask}
          evaluatePending={app.evaluatePending}
          evaluation={app.latestDraftEvaluation}
          draftName={app.newDraftName}
          onDraftNameChange={app.setNewDraftName}
          dmapCommitLimit={app.newDmapCommitLimit}
          dmapDomain={app.newDmapDomain}
          dmapDescription={app.newDmapDescription}
          onDmapCommitLimitChange={app.setNewDmapCommitLimit}
          onDmapDomainChange={app.setNewDmapDomain}
          onDmapDescriptionChange={app.setNewDmapDescription}
          onSaveDraft={() => void app.saveDraftNow()}
          onEvaluate={() => void app.evaluateDraftBeforeCreate()}
          onSubmit={(e) => void app.submitCreate(e)}
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
        <section className="task-home-overview" aria-label="Task overview">
          <article className="task-home-kpi-card task-home-kpi-card--total">
            <p className="task-home-kpi-label">Total tasks</p>
            <p className="task-home-kpi-value">{totalTasks}</p>
            <p className="task-home-kpi-meta">
              {parentTasks} parent • {subtaskTasks} subtask
              {subtaskTasks === 1 ? "" : "s"}
            </p>
          </article>
          <article className="task-home-kpi-card task-home-kpi-card--ready">
            <p className="task-home-kpi-label">Ready tasks</p>
            <p className="task-home-kpi-value">{readyTasks}</p>
            <p className="task-home-kpi-meta">ready for agent pickup</p>
          </article>
          <article className="task-home-kpi-card task-home-kpi-card--attention">
            <p className="task-home-kpi-label">Critical</p>
            <p className="task-home-kpi-value">{criticalTasks}</p>
            <p className="task-home-kpi-meta">needs attention</p>
          </article>
        </section>

        <TaskListSection
          actions={
            <button
              type="button"
              className="task-home-new-task-btn"
              onClick={app.openCreateModal}
              disabled={app.createModalOpen}
            >
              New task
            </button>
          }
          tasks={app.tasks}
          rootTasksOnPage={app.rootTasksOnPage}
          loading={app.loading}
          refreshing={app.listRefreshing}
          saving={app.saving}
          hideBackgroundRefreshHint={app.sseLive}
          listPage={app.taskListPage}
          listPageSize={app.taskListPageSize}
          onListPageChange={app.setTaskListPage}
          onListFiltersChange={app.resetTaskListPage}
          hasNextPage={app.hasNextTaskPage}
          hasPrevPage={app.hasPrevTaskPage}
          onEdit={app.openEdit}
          onRequestDelete={app.requestDelete}
        />
      </div>
    </>
  );
}
