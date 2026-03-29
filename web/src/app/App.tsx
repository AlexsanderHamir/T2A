import { DeleteConfirmDialog } from "../tasks/components/DeleteConfirmDialog";
import { StreamStatusHint } from "../tasks/components/StreamStatusHint";
import { TaskCreateForm } from "../tasks/components/TaskCreateForm";
import { TaskEditForm } from "../tasks/components/TaskEditForm";
import { TaskListSection } from "../tasks/components/TaskListSection";
import { useTasksApp } from "../tasks/hooks/useTasksApp";
import { ErrorBanner } from "../shared/ErrorBanner";
import "./App.css";

export default function App() {
  const app = useTasksApp();

  return (
    <div className="app">
      <header className="app-header">
        <h1>Tasks</h1>
        <StreamStatusHint
          connected={app.sseLive}
          listSyncing={app.listRefreshing}
        />
      </header>
      {app.error ? <ErrorBanner message={app.error} /> : null}

      <main>
        <TaskCreateForm
          title={app.newTitle}
          prompt={app.newPrompt}
          priority={app.newPriority}
          saving={app.saving}
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
            priority={app.editPriority}
            saving={app.saving}
            onTitleChange={app.setEditTitle}
            onPromptChange={app.setEditPrompt}
            onPriorityChange={app.setEditPriority}
            onSubmit={(e) => void app.submitEdit(e)}
            onCancel={app.closeEdit}
          />
        ) : null}
      </main>
    </div>
  );
}
