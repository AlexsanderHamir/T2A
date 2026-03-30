import { TaskCreateForm } from "../components/TaskCreateForm";
import { TaskListSection } from "../components/TaskListSection";
import { useTasksApp } from "../hooks/useTasksApp";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

export function TaskHome({ app }: Props) {
  return (
    <>
      <TaskCreateForm
        title={app.newTitle}
        prompt={app.newPrompt}
        priority={app.newPriority}
        saving={app.saving}
        createPending={app.createPending}
        onTitleChange={app.setNewTitle}
        onPromptChange={app.setNewPrompt}
        onPriorityChange={app.setNewPriority}
        onSubmit={(e) => void app.submitCreate(e)}
      />

      <TaskListSection
        tasks={app.tasks}
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
