import { DeleteConfirmDialog } from "./components/DeleteConfirmDialog";
import { ErrorBanner } from "./components/ErrorBanner";
import { StreamStatusHint } from "./components/StreamStatusHint";
import { TaskCreateForm } from "./components/TaskCreateForm";
import { TaskEditForm } from "./components/TaskEditForm";
import { TaskListSection } from "./components/TaskListSection";
import { useTasksApp } from "./hooks/useTasksApp";
import "./App.css";

export default function App() {
  const app = useTasksApp();

  return (
    <div className="app">
      <h1>Tasks</h1>
      <StreamStatusHint
        connected={app.sseLive}
        listSyncing={app.listRefreshing}
      />
      {app.error ? <ErrorBanner message={app.error} /> : null}

      <TaskCreateForm
        title={app.newTitle}
        prompt={app.newPrompt}
        status={app.newStatus}
        priority={app.newPriority}
        saving={app.saving}
        onTitleChange={app.setNewTitle}
        onPromptChange={app.setNewPrompt}
        onStatusChange={app.setNewStatus}
        onPriorityChange={app.setNewPriority}
        onSubmit={(e) => void app.submitCreate(e)}
      />

      <TaskListSection
        tasks={app.tasks}
        loading={app.loading}
        refreshing={app.listRefreshing}
        saving={app.saving}
        onEdit={app.openEdit}
        onRequestDelete={app.requestDelete}
      />

      {app.deleteTarget ? (
        <DeleteConfirmDialog
          taskTitle={app.deleteTarget.title}
          saving={app.saving}
          onCancel={app.cancelDelete}
          onConfirm={() => void app.confirmDelete()}
        />
      ) : null}

      {app.editing ? (
        <TaskEditForm
          taskId={app.editing.id}
          title={app.editTitle}
          prompt={app.editPrompt}
          status={app.editStatus}
          priority={app.editPriority}
          saving={app.saving}
          onTitleChange={app.setEditTitle}
          onPromptChange={app.setEditPrompt}
          onStatusChange={app.setEditStatus}
          onPriorityChange={app.setEditPriority}
          onSubmit={(e) => void app.submitEdit(e)}
          onCancel={app.closeEdit}
        />
      ) : null}
    </div>
  );
}
