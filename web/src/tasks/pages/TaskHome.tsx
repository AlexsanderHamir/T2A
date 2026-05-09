import { useEffect, useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { TaskListSection } from "../components/task-list";
import { useTasksApp } from "../hooks/useTasksApp";
import { useProjects } from "@/projects";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

export function TaskHome({ app }: Props) {
  useDocumentTitle(undefined);
  const [searchParams, setSearchParams] = useSearchParams();
  const projects = useProjects({ includeArchived: false, limit: 100 });

  const createIntent = searchParams.get("create");
  const projectIntent = searchParams.get("project")?.trim() ?? "";
  const stepIntent = searchParams.get("step")?.trim() ?? "";

  useEffect(() => {
    if (createIntent !== "1" || !projectIntent) return;
    app.openCreateModal(
      stepIntent
        ? { projectID: projectIntent, projectStepID: stepIntent }
        : { projectID: projectIntent },
    );
    setSearchParams({}, { replace: true });
  }, [app.openCreateModal, createIntent, projectIntent, stepIntent, setSearchParams]);

  /** Row-level busy state for the list only; excludes create/evaluate so modal typing does not re-render the table. */
  const listSaving = app.patchPending || app.deletePending;

  const listSectionProps = useMemo(
    () => ({
      tasks: app.tasks,
      rootTasksOnPage: app.rootTasksOnPage,
      loading: app.loading,
      refreshing: app.listRefreshing,
      saving: listSaving,
      hideBackgroundRefreshHint: app.sseLive,
      listPage: app.taskListPage,
      listPageSize: app.taskListPageSize,
      projectFilterOptions: projects.data?.projects ?? [],
      onListPageChange: app.setTaskListPage,
      onListFiltersChange: app.resetTaskListPage,
      hasNextPage: app.hasNextTaskPage,
      hasPrevPage: app.hasPrevTaskPage,
      onEdit: app.openEdit,
      onRequestDelete: app.requestDelete,
      // Quiet inline scoreboard rendered between the heading and the
      // filters when at least one task exists. The strip self-hides on
      // null / 0-total so an unconfigured workspace still reads cleanly.
      taskStats: app.taskStats ?? null,
    }),
    [
      app.tasks,
      app.rootTasksOnPage,
      app.loading,
      app.listRefreshing,
      listSaving,
      app.sseLive,
      app.taskListPage,
      app.taskListPageSize,
      projects.data?.projects,
      app.setTaskListPage,
      app.resetTaskListPage,
      app.hasNextTaskPage,
      app.hasPrevTaskPage,
      app.openEdit,
      app.requestDelete,
      app.taskStats,
    ],
  );

  const listActions = useMemo(
    () => (
      <button
        type="button"
        className="task-home-new-task-btn"
        onClick={() => app.openCreateModal()}
        disabled={app.createModalOpen}
      >
        New task
      </button>
    ),
    [app.openCreateModal, app.createModalOpen],
  );

  return (
    <div className="task-detail-content--enter">
      <TaskListSection {...listSectionProps} actions={listActions} />
    </div>
  );
}
