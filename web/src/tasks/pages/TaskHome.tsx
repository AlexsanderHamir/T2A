import { useEffect, useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { Button } from "@/components/ui";
import { TaskListSection } from "../components/task-list";
import { useTasksAppList, useTasksAppModals } from "../app/TasksAppProvider";
import { useProjects } from "@/projects";

export function TaskHome() {
  useDocumentTitle(undefined);
  const list = useTasksAppList();
  const modals = useTasksAppModals();
  const [searchParams, setSearchParams] = useSearchParams();
  const projects = useProjects({ includeArchived: false, limit: 100 });
  const { openCreateModal, createModalOpen } = modals;

  const createIntent = searchParams.get("create");
  const projectIntent = searchParams.get("project")?.trim() ?? "";

  useEffect(() => {
    if (createIntent !== "1" || !projectIntent) return;
    openCreateModal({ projectID: projectIntent });
    setSearchParams({}, { replace: true });
  }, [openCreateModal, createIntent, projectIntent, setSearchParams]);

  /** Row-level busy state for the list only; excludes create so modal typing does not re-render the table. */
  const listSaving = list.patchPending || list.deletePending;

  const listSectionProps = useMemo(
    () => ({
      tasks: list.tasks,
      rootTasksOnPage: list.rootTasksOnPage,
      loading: list.loading,
      refreshing: list.listRefreshing,
      saving: listSaving,
      hideBackgroundRefreshHint: list.sseLive,
      listPage: list.taskListPage,
      listPageSize: list.taskListPageSize,
      projectFilterOptions: projects.data?.projects ?? [],
      onListPageChange: list.setTaskListPage,
      onListFiltersChange: list.resetTaskListPage,
      hasNextPage: list.hasNextTaskPage,
      hasPrevPage: list.hasPrevTaskPage,
      onEdit: modals.openEdit,
      onRequestDelete: modals.requestDelete,
      taskStats: list.taskStats ?? null,
    }),
    [
      list.tasks,
      list.rootTasksOnPage,
      list.loading,
      list.listRefreshing,
      listSaving,
      list.sseLive,
      list.taskListPage,
      list.taskListPageSize,
      projects.data?.projects,
      list.setTaskListPage,
      list.resetTaskListPage,
      list.hasNextTaskPage,
      list.hasPrevTaskPage,
      modals.openEdit,
      modals.requestDelete,
      list.taskStats,
    ],
  );

  const listActions = useMemo(
    () => (
      <Button
        variant="primary"
        className="task-home-new-task-btn"
        onClick={() => openCreateModal()}
        disabled={createModalOpen}
      >
        New task
      </Button>
    ),
    [openCreateModal, createModalOpen],
  );

  return (
    <div className="task-detail-content--enter">
      <TaskListSection {...listSectionProps} actions={listActions} />
    </div>
  );
}
