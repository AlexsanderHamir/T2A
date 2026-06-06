import { Suspense, lazy } from "react";
import { Link, Route, Routes, useLocation } from "react-router-dom";
import {
  DeleteConfirmDialog,
  TaskChangeModelModal,
  TaskEditForm,
  TaskDraftsPage,
  TaskCreateModalsLayer,
  TaskHome,
  useTasksApp,
} from "@/tasks";
import { useTaskEventStream } from "@/tasks/hooks/useTaskEventStream";
import { useStickyShellElevation } from "@/lib/useStickyShellElevation";
import {
  ProjectContextPicker,
  ProjectSelect,
  useProjectContextPromptBinding,
  useProjects,
} from "@/projects";

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
const TaskGraphPage = lazy(() =>
  import("@/tasks/pages/TaskGraphPage").then((m) => ({
    default: m.TaskGraphPage,
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

function AppShell({ app }: { app: ReturnType<typeof useTasksApp> }) {
  const location = useLocation();
  // The shell-level project list is consumed only by the edit modal's
  // ProjectSelect. Until the user opens that modal there is no need to
  // hit the network here — TaskHome and the project pages own their
  // own copies for their own surfaces. The bootstrap aggregate seeds
  // this cache key on cold start, so when the edit modal opens the
  // first render hits warm data even before the lazy fetch returns.
  const projects = useProjects({
    includeArchived: false,
    limit: 100,
    enabled: app.editing !== null,
  });
  const editPromptProjectContext = useProjectContextPromptBinding({
    projectId: app.editing ? app.editProjectID : "",
    selectedIds: app.editProjectContextItemIDs,
    onSelectedIdsChange: app.setEditProjectContextItemIDs,
  });
  const homeIsCurrent = location.pathname === "/";
  const draftsIsCurrent = location.pathname.startsWith("/drafts");
  const projectsIsCurrent = location.pathname.startsWith("/projects");
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
              <h1 className="app-title app-title--logo">T2A</h1>
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
                to="/drafts"
                className="app-nav__link"
                {...(draftsIsCurrent
                  ? { "aria-current": "page" as const }
                  : {})}
              >
                Drafts
              </Link>
              <Link
                to="/projects"
                className="app-nav__link"
                {...(projectsIsCurrent
                  ? { "aria-current": "page" as const }
                  : {})}
              >
                Projects
              </Link>
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
        {app.error ? <ErrorBanner message={app.error} /> : null}

        <main id="main-content" tabIndex={-1}>
          <RoutedMainOutlet />
          <TaskCreateModalsLayer app={app} />

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
              taskRunner={app.editing.runner}
              cursorModel={app.editCursorModel}
              onCursorModelChange={app.setEditCursorModel}
              projectAssignment={
                <section
                  className="task-create-project"
                  aria-label="Project assignment"
                >
                  <ProjectSelect
                    id="task-edit-project"
                    value={app.editProjectID}
                    projects={projects.data?.projects ?? []}
                    loading={projects.isLoading}
                    disabled={app.saving}
                    onChange={(projectId) => {
                      app.setEditProjectID(projectId);
                      app.setEditProjectContextItemIDs([]);
                    }}
                  />
                  <ProjectContextPicker
                    projectId={app.editProjectID}
                    selectedIds={app.editProjectContextItemIDs}
                    disabled={app.saving}
                    onChange={app.setEditProjectContextItemIDs}
                  />
                </section>
              }
              promptProjectContext={editPromptProjectContext ?? undefined}
              canInheritChecklist={Boolean(app.editing.parent_id)}
              tagsCsv={app.editTagsCsv}
              milestone={app.editMilestone}
              onTagsCsvChange={app.setEditTagsCsv}
              onMilestoneChange={app.setEditMilestone}
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
  // The task detail / cycle / event / graph routes embed the same
  // shell modals (edit/delete/change-model) that read from the home
  // list cache on close. Keeping data enabled here avoids a flash of
  // empty list state when the user navigates back to "/".
  if (pathname.startsWith("/tasks/")) return true;
  return false;
}

export default function App() {
  // Aggregate bootstrap seeds the TanStack Query cache for settings,
  // root tasks page, stats, projects, and drafts in one round trip so
  // the per-page hooks below skip their cold-start GETs. Silent fallback
  // when /v1/bootstrap is unavailable (older server, stripped build).
  useBootstrap();
  const sseLive = useTaskEventStream();
  const location = useLocation();
  const dataEnabled = routeNeedsHomeListData(location.pathname);
  const app = useTasksApp({ sseLive, dataEnabled });

  // Lazy routes need a Suspense boundary above them. The fallback is
  // intentionally empty: the chunks are small (each page module + its
  // local components) and ship gzipped over a same-origin connection,
  // so a visible loader would flash for ~50ms on most navigations and
  // do more visual harm than good. Pages render their own skeletons
  // for data loading once the chunk arrives.
  return (
    <Suspense fallback={null}>
      <Routes>
        <Route path="/" element={<AppShell app={app} />}>
          <Route index element={<TaskHome app={app} />} />
          <Route path="drafts" element={<TaskDraftsPage app={app} />} />
          <Route path="projects" element={<ProjectListPage />} />
          <Route path="projects/:projectId/context" element={<ProjectContextPage />} />
          <Route path="projects/:projectId" element={<ProjectDetailPage />} />
          <Route path="settings" element={<SettingsPage />} />
          <Route
            path="tasks/:taskId/events/:eventSeq"
            element={<TaskEventDetailPage />}
          />
          <Route
            path="tasks/:taskId/cycles/:cycleId"
            element={<TaskCycleDetailPage />}
          />
          <Route path="tasks/:taskId/graph" element={<TaskGraphPage />} />
          <Route path="tasks/:taskId" element={<TaskDetailPage app={app} />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Routes>
    </Suspense>
  );
}
