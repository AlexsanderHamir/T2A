import { Suspense, lazy } from "react";
import { Link, Navigate, Route, Routes, useLocation } from "react-router-dom";
import { isUiFeatureOmitted } from "@/launch/omittedFeatures";
import {
  DeleteConfirmDialog,
  TaskChangeModelModal,
  TaskDraftsPage,
  TaskTemplatesPage,
  TaskCreateModalsLayer,
  TaskHome,
} from "@/tasks";
import { TasksAppProvider, useTasksAppContext, useTasksAppMeta } from "@/tasks/app/TasksAppProvider";
import { useTasksApp } from "@/tasks/hooks/useTasksApp";
import { useTaskEventStream } from "@/tasks/hooks/useTaskEventStream";
import { useStickyShellElevation } from "@/lib/useStickyShellElevation";

// Route-level code splitting. Each lazy() call becomes its own chunk
// so the initial bundle covers only the home/drafts paths a freshly
// landing user actually needs. Vite resolves the deep module paths to
// individual chunks; the barrel re-exports remain but tree-shake out
// of the main chunk because no synchronous consumer imports them.
const TaskDetailPage = lazy(() =>
  import("@/tasks/pages/TaskDetailPage").then((m) => ({
    default: m.TaskDetailPage,
  })),
);
const TaskCycleDetailPage = lazy(() =>
  import("@/tasks/pages/TaskCycleDetailPage").then((m) => ({
    default: m.TaskCycleDetailPage,
  })),
);
const TaskEventDetailPage = lazy(() =>
  import("@/tasks/pages/TaskEventDetailPage").then((m) => ({
    default: m.TaskEventDetailPage,
  })),
);
const TaskCommitDiffPage = lazy(() =>
  import("@/tasks/pages/TaskCommitDiffPage").then((m) => ({
    default: m.TaskCommitDiffPage,
  })),
);
const SettingsPage = lazy(() =>
  import("@/settings/SettingsPage").then((m) => ({
    default: m.SettingsPage,
  })),
);
const ProjectListPage = lazy(() =>
  import("@/projects/ProjectListPage").then((m) => ({
    default: m.ProjectListPage,
  })),
);
const ProjectDetailPage = lazy(() =>
  import("@/projects/ProjectDetailPage").then((m) => ({
    default: m.ProjectDetailPage,
  })),
);
const ProjectContextPage = lazy(() =>
  import("@/projects/ProjectContextPage").then((m) => ({
    default: m.ProjectContextPage,
  })),
);
import { UiTestModeBanner } from "@/dev/UiTestModeBanner";
import { ErrorBanner } from "../shared/ErrorBanner";
import { ModalStackProvider } from "../shared/ModalStackContext";
import { NotFoundPage } from "./NotFoundPage";
import { RouteAnnouncer } from "./RouteAnnouncer";
import { RoutedMainOutlet } from "./RoutedMainOutlet";
import { useBootstrap } from "./hooks/useBootstrap";
import { useSettingsRoutePrefetch } from "./hooks/usePrefetchOnIntent";
import "./App.css";

function AppShell() {
  const app = useTasksAppContext();
  const { error } = useTasksAppMeta();
  const location = useLocation();
  const homeIsCurrent = location.pathname === "/";
  const draftsIsCurrent = location.pathname.startsWith("/drafts");
  const templatesIsCurrent = location.pathname.startsWith("/templates");
  const projectsUiEnabled = !isUiFeatureOmitted("projects");
  const projectsIsCurrent = projectsUiEnabled && location.pathname.startsWith("/projects");
  const headerElevated = useStickyShellElevation();
  // Settings chunk is small but the icon is the most prominent
  // header affordance; prefetching on hover is a free win.
  const settingsIntent = useSettingsRoutePrefetch();

  return (
    <ModalStackProvider>
      <div className="app">
        <a href="#main-content" className="skip-link">
          Skip to main content
        </a>
        <header
          className="app-header app-header--sticky"
          data-elevated={headerElevated ? "true" : "false"}
        >
          <div className="app-header-top">
            {/* Brand sits OUTSIDE <nav> so the wordmark is not announced
                as a navigation destination peer to Tasks/Drafts/etc.
                It still links home (with aria-current on /) so keyboard
                + click affordance stays. */}
            <Link
              to="/"
              className="app-brand"
              {...(homeIsCurrent
                ? { "aria-current": "page" as const }
                : {})}
            >
              <img
                src="/Hamix_wordmark.png"
                alt="Hamix"
                className="app-title app-title--logo app-title--wordmark"
              />
            </Link>
            <nav className="app-nav" aria-label="Primary">
              <Link
                to="/"
                className="app-nav__link"
                {...(homeIsCurrent
                  ? { "aria-current": "page" as const }
                  : {})}
              >
                Tasks
              </Link>
              <Link
                to="/templates"
                className="app-nav__link"
                {...(templatesIsCurrent
                  ? { "aria-current": "page" as const }
                  : {})}
              >
                Templates
              </Link>
              <Link
                to="/drafts"
                className="app-nav__link"
                {...(draftsIsCurrent
                  ? { "aria-current": "page" as const }
                  : {})}
              >
                Drafts
              </Link>
              {projectsUiEnabled ? (
                <Link
                  to="/projects"
                  className="app-nav__link"
                  {...(projectsIsCurrent
                    ? { "aria-current": "page" as const }
                    : {})}
                >
                  Projects
                </Link>
              ) : null}
            </nav>
            <div className="app-header-actions">
              <Link
                to="/settings"
                className="app-header-settings-link"
                aria-label="Open settings"
                title="Settings"
                onPointerEnter={settingsIntent.onPointerEnter}
                onFocus={settingsIntent.onFocus}
                {...(location.pathname.startsWith("/settings")
                  ? { "aria-current": "page" as const }
                  : {})}
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
        <UiTestModeBanner />
        {error ? <ErrorBanner message={error} /> : null}

        <main id="main-content" tabIndex={-1}>
          <RoutedMainOutlet />
          <TaskCreateModalsLayer />

          {app.deleteTarget ? (
            <DeleteConfirmDialog
              taskTitle={app.deleteTarget.title}
              saving={app.saving}
              deletePending={app.deletePending}
              error={app.deleteError}
              onCancel={app.cancelDelete}
              onConfirm={() => void app.confirmDelete()}
            />
          ) : null}

          {app.changeModelTask ? (
            <TaskChangeModelModal
              task={app.changeModelTask}
              cursorModel={app.changeModelDraft}
              onCursorModelChange={app.setChangeModelDraft}
              saving={app.saving}
              patchPending={app.patchPending}
              error={app.patchError}
              onSubmit={(e) => void app.submitChangeModel(e)}
              onCancel={app.closeChangeModel}
            />
          ) : null}
        </main>
        <RouteAnnouncer />
      </div>
    </ModalStackProvider>
  );
}

/**
 * Routes that actually consume `app.tasks` / `app.taskStats`. When the
 * user navigates to any other route (settings, project pages, the
 * standalone task detail subroutes), we suspend the home-list / stats
 * queries so they stop firing GET requests for views nobody is
 * rendering. The matcher is path-prefix-based to stay zero-cost and to
 * handle nested deep-link URLs without an explicit allowlist per
 * sub-route.
 */
function routeNeedsHomeListData(pathname: string): boolean {
  if (pathname === "/" || pathname === "") return true;
  if (pathname.startsWith("/drafts")) return true;
  if (pathname.startsWith("/templates")) return true;
  // The task detail / cycle / event / graph routes embed the same
  // shell modals (edit/delete/change-model) that read from the home
  // list cache on close. Keeping data enabled here avoids a flash of
  // empty list state when the user navigates back to "/".
  if (pathname.startsWith("/tasks/")) return true;
  return false;
}

export default function App() {
  useBootstrap();
  const sseLive = useTaskEventStream();
  const location = useLocation();
  const dataEnabled = routeNeedsHomeListData(location.pathname);
  const app = useTasksApp({ sseLive, dataEnabled });
  const projectsUiEnabled = !isUiFeatureOmitted("projects");

  return (
    <TasksAppProvider value={app}>
      <Suspense fallback={null}>
        <Routes>
          <Route path="/" element={<AppShell />}>
            <Route index element={<TaskHome />} />
            <Route path="drafts" element={<TaskDraftsPage />} />
            <Route path="templates" element={<TaskTemplatesPage />} />
          {projectsUiEnabled ? (
            <>
              <Route path="projects" element={<ProjectListPage />} />
              <Route
                path="projects/:projectId/context"
                element={<ProjectContextPage />}
              />
              <Route path="projects/:projectId" element={<ProjectDetailPage />} />
            </>
          ) : (
            <Route path="projects/*" element={<Navigate to="/" replace />} />
          )}
          <Route path="settings" element={<SettingsPage />} />
          <Route
            path="tasks/:taskId/events/:eventSeq"
            element={<TaskEventDetailPage />}
          />
          <Route
            path="tasks/:taskId/commits/:sha"
            element={<TaskCommitDiffPage />}
          />
          <Route
            path="tasks/:taskId/cycles/:cycleId"
            element={<TaskCycleDetailPage />}
          />
          <Route path="tasks/:taskId" element={<TaskDetailPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Routes>
    </Suspense>
    </TasksAppProvider>
  );
}
