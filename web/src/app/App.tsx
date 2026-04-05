import { Outlet, Route, Routes } from "react-router-dom";
import { DeleteConfirmDialog } from "../tasks/components/DeleteConfirmDialog";
import { StreamStatusHint } from "../tasks/components/StreamStatusHint";
import { TaskEditForm } from "../tasks/components/TaskEditForm";
import { useTasksApp } from "../tasks/hooks/useTasksApp";
import { TaskDetailPage } from "../tasks/pages/TaskDetailPage";
import { TaskEventDetailPage } from "../tasks/pages/TaskEventDetailPage";
import { TaskHome } from "../tasks/pages/TaskHome";
import { ErrorBanner } from "../shared/ErrorBanner";
import { ModalStackProvider } from "../shared/ModalStackContext";
import "./App.css";

function AppShell({ app }: { app: ReturnType<typeof useTasksApp> }) {
  return (
    <ModalStackProvider>
      <div className="app">
        <header className="app-header app-header--sticky">
          <div className="app-header-top">
            <h1 className="app-title">Tasks</h1>
            <p className="app-tagline">Capture work. Ship with clarity.</p>
          </div>
          <StreamStatusHint
            connected={app.sseLive}
            listSyncing={app.sseLive ? false : app.listRefreshing}
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
              status={app.editStatus}
              checklistInherit={app.editChecklistInherit}
              canInheritChecklist={Boolean(app.editing.parent_id)}
              saving={app.saving}
              patchPending={app.patchPending}
              onTitleChange={app.setEditTitle}
              onPromptChange={app.setEditPrompt}
              onPriorityChange={app.setEditPriority}
              onStatusChange={app.setEditStatus}
              onChecklistInheritChange={app.setEditChecklistInherit}
              onSubmit={(e) => void app.submitEdit(e)}
              onCancel={app.closeEdit}
            />
          ) : null}
        </main>
      </div>
    </ModalStackProvider>
  );
}

export default function App() {
  const app = useTasksApp();

  return (
    <Routes>
      <Route path="/" element={<AppShell app={app} />}>
        <Route index element={<TaskHome app={app} />} />
        <Route
          path="tasks/:taskId/events/:eventSeq"
          element={<TaskEventDetailPage />}
        />
        <Route path="tasks/:taskId" element={<TaskDetailPage app={app} />} />
      </Route>
    </Routes>
  );
}
