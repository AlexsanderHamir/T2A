import { TaskCreateModal } from "../components/TaskCreateModal";
import { TaskListSection } from "../components/TaskListSection";
import { useTasksApp } from "../hooks/useTasksApp";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

export function TaskHome({ app }: Props) {
  return (
    <>
      <div className="task-home-toolbar">
        <button
          type="button"
          className="task-home-new-task-btn"
          onClick={app.openCreateModal}
          disabled={app.createModalOpen}
        >
          New task
        </button>
      </div>

      {app.createModalOpen ? (
        <TaskCreateModal
          pending={app.createPending}
          saving={app.saving}
          onClose={app.closeCreateModal}
          title={app.newTitle}
          prompt={app.newPrompt}
          priority={app.newPriority}
          checklistDraft={app.newChecklistDraft}
          checklistItems={app.newChecklistItems}
          parentOptions={app.tasks}
          parentId={app.newParentId}
          checklistInherit={app.newChecklistInherit}
          onTitleChange={app.setNewTitle}
          onPromptChange={app.setNewPrompt}
          onPriorityChange={app.setNewPriority}
          onChecklistDraftChange={app.setNewChecklistDraft}
          onParentIdChange={app.setNewParentId}
          onChecklistInheritChange={app.setNewChecklistInherit}
          onAddChecklistRow={app.addNewChecklistRow}
          onRemoveChecklistRow={app.removeNewChecklistRow}
          pendingSubtasks={app.pendingSubtasks}
          onAddPendingSubtask={app.addPendingSubtask}
          onUpdatePendingSubtask={app.updatePendingSubtask}
          onRemovePendingSubtask={app.removePendingSubtask}
          onSubmit={(e) => void app.submitCreate(e)}
        />
      ) : null}

      <TaskListSection
        tasks={app.tasks}
        rootTasksOnPage={app.rootTasksOnPage}
        loading={app.loading}
        refreshing={app.listRefreshing}
        saving={app.saving}
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
