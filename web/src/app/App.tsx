import { Link, Route, Routes, useLocation } from "react-router-dom";
import { DeleteConfirmDialog, StreamStatusHint } from "../tasks/components/dialogs";
import { TaskEditForm } from "../tasks/components/task-detail";
import { useTasksApp } from "../tasks/hooks/useTasksApp";
import { TaskDetailPage } from "../tasks/pages/TaskDetailPage";
import { TaskEventDetailPage } from "../tasks/pages/TaskEventDetailPage";
import { TaskGraphPage } from "../tasks/pages/TaskGraphPage";
import { TaskDraftsPage } from "../tasks/pages/TaskDraftsPage";
import { TaskHome } from "../tasks/pages/TaskHome";
import { ErrorBanner } from "../shared/ErrorBanner";
import { ModalStackProvider } from "../shared/ModalStackContext";
import { SettingsPage } from "../settings";
import { NotFoundPage } from "./NotFoundPage";
import { RouteAnnouncer } from "./RouteAnnouncer";
import { RoutedMainOutlet } from "./RoutedMainOutlet";
import "./App.css";

function AppShell({ app }: { app: ReturnType<typeof useTasksApp> }) {
  const location = useLocation();
  const homeIsCurrent = location.pathname === "/";

  return (
    <ModalStackProvider>
      <div className="app">
        <a href="#main-content" className="skip-link">
          Skip to main content
        </a>
        <header className="app-header app-header--sticky">
          <div className="app-header-top">
            <div className="app-brand-lockup">
              <nav className="app-header-site-nav" aria-label="Site">
                <Link
                  to="/"
                  className="app-title-link"
                  {...(homeIsCurrent
                    ? { "aria-current": "page" as const }
                    : {})}
                >
                  <h1 className="app-title app-title--logo">T2A</h1>
                </Link>
                <Link to="/drafts" className="app-title-link app-title-link--drafts">
                  Drafts
                </Link>
              </nav>
              <p className="app-tagline term-prompt">
                <span>capture --work --ship-with-clarity</span>
              </p>
            </div>
            <div className="app-header-actions">
              <StreamStatusHint
                connected={app.sseLive}
                listSyncing={app.sseLive ? false : app.listRefreshing}
              />
              <Link
                to="/settings"
                className="app-header-settings-link"
                aria-label="Open settings"
                title="Settings"
              >
                <svg
                  className="app-header-settings-icon"
                  xmlns="http://www.w3.org/2000/svg"
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                  focusable="false"
                >
                  <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
                  <circle cx="12" cy="12" r="3" />
                </svg>
                <span className="visually-hidden">Settings</span>
              </Link>
            </div>
          </div>
        </header>
        {app.error ? <ErrorBanner message={app.error} /> : null}

        <main id="main-content" tabIndex={-1}>
          <RoutedMainOutlet />

          {app.deleteTarget ? (
            <DeleteConfirmDialog
              taskTitle={app.deleteTarget.title}
              subtaskCount={app.deleteTarget.subtaskCount}
              saving={app.saving}
              deletePending={app.deletePending}
              error={app.deleteError}
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
              taskType={app.editTaskType}
              status={app.editStatus}
              checklistInherit={app.editChecklistInherit}
              canInheritChecklist={Boolean(app.editing.parent_id)}
              saving={app.saving}
              patchPending={app.patchPending}
              error={app.patchError}
              onTitleChange={app.setEditTitle}
              onPromptChange={app.setEditPrompt}
              onPriorityChange={app.setEditPriority}
              onTaskTypeChange={app.setEditTaskType}
              onStatusChange={app.setEditStatus}
              onChecklistInheritChange={app.setEditChecklistInherit}
              onSubmit={(e) => void app.submitEdit(e)}
              onCancel={app.closeEdit}
            />
          ) : null}
        </main>
        <RouteAnnouncer />
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
        <Route path="drafts" element={<TaskDraftsPage app={app} />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route
          path="tasks/:taskId/events/:eventSeq"
          element={<TaskEventDetailPage />}
        />
        <Route path="tasks/:taskId/graph" element={<TaskGraphPage />} />
        <Route path="tasks/:taskId" element={<TaskDetailPage app={app} />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}
