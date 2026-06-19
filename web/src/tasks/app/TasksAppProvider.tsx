import {
  createContext,
  useContext,
  useMemo,
  type ReactNode,
} from "react";
import type { useTasksApp } from "../hooks/useTasksApp";

export type TasksAppContextValue = ReturnType<typeof useTasksApp>;

const TasksAppContext = createContext<TasksAppContextValue | null>(null);

export function TasksAppProvider({
  value,
  children,
}: {
  value: TasksAppContextValue;
  children: ReactNode;
}) {
  return (
    <TasksAppContext.Provider value={value}>{children}</TasksAppContext.Provider>
  );
}

export function useTasksAppContext(): TasksAppContextValue {
  const ctx = useContext(TasksAppContext);
  if (!ctx) {
    throw new Error("useTasksAppContext must be used within TasksAppProvider");
  }
  return ctx;
}

export function useTasksAppList() {
  const app = useTasksAppContext();
  return useMemo(
    () => ({
      tasks: app.tasks,
      rootTasksOnPage: app.rootTasksOnPage,
      loading: app.loading,
      listRefreshing: app.listRefreshing,
      patchPending: app.patchPending,
      deletePending: app.deletePending,
      sseLive: app.sseLive,
      taskListPage: app.taskListPage,
      setTaskListPage: app.setTaskListPage,
      resetTaskListPage: app.resetTaskListPage,
      taskListPageSize: app.taskListPageSize,
      hasNextTaskPage: app.hasNextTaskPage,
      hasPrevTaskPage: app.hasPrevTaskPage,
      openEdit: app.openEdit,
      requestDelete: app.requestDelete,
      taskStats: app.taskStats,
      taskStatsLoading: app.taskStatsLoading,
    }),
    [
      app.tasks,
      app.rootTasksOnPage,
      app.loading,
      app.listRefreshing,
      app.patchPending,
      app.deletePending,
      app.sseLive,
      app.taskListPage,
      app.setTaskListPage,
      app.resetTaskListPage,
      app.taskListPageSize,
      app.hasNextTaskPage,
      app.hasPrevTaskPage,
      app.openEdit,
      app.requestDelete,
      app.taskStats,
      app.taskStatsLoading,
    ],
  );
}

export function useTasksAppModals() {
  const app = useTasksAppContext();
  return useMemo(
    () => ({
      deleteTarget: app.deleteTarget,
      requestDelete: app.requestDelete,
      cancelDelete: app.cancelDelete,
      confirmDelete: app.confirmDelete,
      deletePending: app.deletePending,
      deleteError: app.deleteError,
      deleteSuccess: app.deleteSuccess,
      deleteVariables: app.deleteVariables,
      changeModelTask: app.changeModelTask,
      changeModelDraft: app.changeModelDraft,
      setChangeModelDraft: app.setChangeModelDraft,
      openChangeModel: app.openChangeModel,
      closeChangeModel: app.closeChangeModel,
      submitChangeModel: app.submitChangeModel,
      patchPending: app.patchPending,
      patchError: app.patchError,
      openCreateModal: app.openCreateModal,
      createModalOpen: app.createModalOpen,
      openEdit: app.openEdit,
      closeEdit: app.closeEdit,
      closeCreateModal: app.closeCreateModal,
    }),
    [
      app.deleteTarget,
      app.requestDelete,
      app.cancelDelete,
      app.confirmDelete,
      app.deletePending,
      app.deleteError,
      app.deleteSuccess,
      app.deleteVariables,
      app.changeModelTask,
      app.changeModelDraft,
      app.setChangeModelDraft,
      app.openChangeModel,
      app.closeChangeModel,
      app.submitChangeModel,
      app.patchPending,
      app.patchError,
      app.openCreateModal,
      app.createModalOpen,
      app.openEdit,
      app.closeEdit,
      app.closeCreateModal,
    ],
  );
}

export function useTasksAppMeta() {
  const app = useTasksAppContext();
  return useMemo(
    () => ({
      error: app.error,
      saving: app.saving,
      sseLive: app.sseLive,
    }),
    [app.error, app.saving, app.sseLive],
  );
}
