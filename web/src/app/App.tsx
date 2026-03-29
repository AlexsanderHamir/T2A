import { Outlet, Route, Routes } from "react-router-dom";
import { DeleteConfirmDialog } from "../tasks/components/DeleteConfirmDialog";
import { StreamStatusHint } from "../tasks/components/StreamStatusHint";
import { TaskEditForm } from "../tasks/components/TaskEditForm";
import { useTasksApp } from "../tasks/hooks/useTasksApp";
import { TaskDetailPage } from "../tasks/pages/TaskDetailPage";
import { TaskHome } from "../tasks/pages/TaskHome";
import { ErrorBanner } from "../shared/ErrorBanner";
import "./App.css";

function AppShell({ app }: { app: ReturnType<typeof useTasksApp> }) {
  return (
    <div className="app">
      <header className="app-header">
        <div className="app-header-top">
          <h1 className="app-title">Tasks</h1>
          <p className="app-tagline">Capture work. Ship with clarity.</p>
        </div>
        <StreamStatusHint
          connected={app.sseLive}
          listSyncing={app.listRefreshing}
        />
      </header>
      {app.error ? <ErrorBanner message={app.error} /> : null}

      <main>
        <Outlet />

        {app.deleteTarget ? (
          <DeleteConfirmDialog
            taskTitle={app.deleteTarget.title}
            saving={app.saving}
            deletePending={app.deletePending}
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
            patchPending={app.patchPending}
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

export default function App() {
  const app = useTasksApp();

  return (
    <Routes>
      <Route path="/" element={<AppShell app={app} />}>
        <Route index element={<TaskHome app={app} />} />
        <Route path="tasks/:taskId" element={<TaskDetailPage app={app} />} />
      </Route>
    </Routes>
  );
}
