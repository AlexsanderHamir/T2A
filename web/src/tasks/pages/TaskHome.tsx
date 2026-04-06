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

  return (
    <>
      {app.createModalOpen ? (
        <TaskCreateModal
          pending={app.createPending}
          saving={app.saving}
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
