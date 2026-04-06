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
      {app.createModalOpen ? (
        <TaskCreateModal
          pending={app.createPending}
          saving={app.saving}
          draftSaving={app.draftSavePending}
          draftSaveLabel={app.draftSaveLabel}
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
          onResume={(id) => void app.resumeDraftByID(id)}
        />
      ) : null}

      <section className="task-home-overview" aria-label="Task overview">
        <article className="task-home-kpi-card">
          <p className="task-home-kpi-label">Total tasks</p>
          <p className="task-home-kpi-value">{totalTasks}</p>
          <p className="task-home-kpi-meta">
            {parentTasks} parent • {subtaskTasks} subtask
            {subtaskTasks === 1 ? "" : "s"}
          </p>
        </article>
        <article className="task-home-kpi-card">
          <p className="task-home-kpi-label">Ready</p>
          <p className="task-home-kpi-value">{readyTasks}</p>
          <p className="task-home-kpi-meta">awaiting action</p>
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
    </>
  );
}
