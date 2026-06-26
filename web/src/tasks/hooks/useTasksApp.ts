import { useQuery } from "@tanstack/react-query";
import { fetchAppSettings } from "@/api/settings";
import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { getTaskStats, listTasks } from "../../api";
import { flattenTaskTreeRoots } from "../task-tree";
import { TASK_LIST_PAGE_SIZE } from "../task-paging";
import { settingsQueryKeys } from "@/settings/settingsQueryKeys";
import { taskQueryKeys } from "../task-query";
import { errorMessage } from "@/lib/errorMessage";
import {
  type Priority,
  type Task,
} from "@/types";
import { useHysteresisBoolean } from "@/lib/useHysteresisBoolean";
import { TASK_TIMINGS } from "@/constants/tasks";
import { useTaskDeleteFlow } from "./useTaskDeleteFlow";
import { useTaskPatchFlow } from "./useTaskPatchFlow";
import { useTaskCreateFlow } from "./useTaskCreateFlow";
import { canEditTaskPickupSchedule } from "../task-pickup/canEditTaskPickupSchedule";
import { canEditTask } from "../task-display/canEditTask";
import { QUERY_POLICY } from "../queryPolicy";

/** Background refetches (SSE invalidate, focus) are short; avoid UI flicker. */
const LIST_REFRESH_SHOW_MS = TASK_TIMINGS.listRefreshShowMs;
const LIST_REFRESH_HIDE_MS = TASK_TIMINGS.listRefreshHideMs;

export type UseTasksAppOptions = {
  /** Whether the task change SSE stream is connected; owned by `App` via `useTaskEventStream`. */
  sseLive: boolean;
  /**
   * Whether the home-list / stats queries should be active. When the
   * user is on a route that does not consume `app.tasks` / `app.taskStats`
   * (e.g. `/settings`), passing `false` suspends the queries — the
   * cache remains populated for the next visit but no new GETs fire
   * while the route is mounted. Defaults to `true` so callers that
   * do not pass this stay on the historical eager-fetch behaviour.
   */
  dataEnabled?: boolean;
};

export function useTasksApp({ sseLive, dataEnabled = true }: UseTasksAppOptions) {
  const {
    createFlowError,
    editingTaskId,
    closeCreateModal,
    newTitle,
    newPrompt,
    newPriority,
    newProjectID,
    newProjectContextItemIDs,
    newTagsCsv,
    newMilestone,
    newTaskCursorModel,
    newSchedule,
    composeStatus,
    ...createFlow
  } = useTaskCreateFlow();

  const editingTaskIdRef = useRef<string | null>(null);
  editingTaskIdRef.current = editingTaskId;

  /** Quick-edit modal for `cursor_model` only (e.g. task detail model configuration row). */
  const [changeModelTask, setChangeModelTask] = useState<Task | null>(null);
  const [changeModelDraft, setChangeModelDraft] = useState("");

  const {
    deleteTarget,
    requestDelete,
    cancelDelete,
    confirmDelete,
    deletePending,
    deleteError,
    deleteSuccess,
    deleteVariables,
    resetError: resetDeleteError,
  } = useTaskDeleteFlow({
    onDeleted: (deletedId) => {
      if (editingTaskIdRef.current === deletedId) {
        closeCreateModal();
      }
    },
  });

  /** Client-side validation for edit save (shown after server errors when applicable). */
  const [editTitleRequiredError, setEditTitleRequiredError] = useState<
    string | null
  >(null);

  const [taskListPage, setTaskListPage] = useState(0);

  useQuery({
    queryKey: settingsQueryKeys.app(),
    queryFn: ({ signal }) => fetchAppSettings({ signal }),
  });

  const tasksQuery = useQuery({
    queryKey: taskQueryKeys.list({
      limit: TASK_LIST_PAGE_SIZE,
      offset: taskListPage * TASK_LIST_PAGE_SIZE,
    }),
    queryFn: ({ signal }) =>
      listTasks(
        TASK_LIST_PAGE_SIZE,
        taskListPage * TASK_LIST_PAGE_SIZE,
        { signal },
      ),
    enabled: dataEnabled,
    staleTime: QUERY_POLICY.listStaleTimeMs,
  });
  const taskStatsQuery = useQuery({
    queryKey: taskQueryKeys.stats(),
    queryFn: async ({ signal }) => {
      try {
        return await getTaskStats({ signal });
      } catch {
        return null;
      }
    },
    enabled: dataEnabled,
    staleTime: QUERY_POLICY.listStaleTimeMs,
  });

  const resetTaskListPage = useCallback(() => {
    setTaskListPage(0);
  }, []);

  const rootTaskTrees = useMemo(
    () => tasksQuery.data?.tasks ?? [],
    [tasksQuery.data?.tasks],
  );
  const tasks = useMemo(
    () => flattenTaskTreeRoots(rootTaskTrees),
    [rootTaskTrees],
  );

  const loading = tasksQuery.isPending;
  const rawListRefreshing =
    tasksQuery.isFetching && !tasksQuery.isPending;
  const listRefreshing = useHysteresisBoolean(
    rawListRefreshing,
    LIST_REFRESH_SHOW_MS,
    LIST_REFRESH_HIDE_MS,
  );

  const {
    patchTask: runPatch,
    patchPending,
    patchError,
    resetError: resetPatchError,
  } = useTaskPatchFlow({
    onPatched: (patchedId) => {
      if (editingTaskIdRef.current === patchedId) {
        closeCreateModal();
      }
      setChangeModelTask((prev) => (prev?.id === patchedId ? null : prev));
    },
  });

  useEffect(() => {
    if (!createFlow.createModalOpen && !changeModelTask) resetPatchError();
  }, [createFlow.createModalOpen, changeModelTask, resetPatchError]);

  useEffect(() => {
    if (!deleteTarget) resetDeleteError();
  }, [deleteTarget, resetDeleteError]);

  const saving =
    createFlow.createPending ||
    createFlow.templateSavePending ||
    patchPending ||
    deletePending;

  const error = useMemo(() => {
    if (tasksQuery.isError) return errorMessage(tasksQuery.error);
    if (createFlowError) return createFlowError;
    if (patchError) return patchError;
    if (deleteError) return deleteError;
    return editTitleRequiredError;
  }, [
    tasksQuery.isError,
    tasksQuery.error,
    createFlowError,
    patchError,
    deleteError,
    editTitleRequiredError,
  ]);

  useEffect(() => {
    if (editTitleRequiredError && newTitle.trim()) {
      setEditTitleRequiredError(null);
    }
  }, [newTitle, editTitleRequiredError]);

  function openEdit(t: Task) {
    if (!canEditTask(t.status)) {
      return;
    }
    setChangeModelTask(null);
    setEditTitleRequiredError(null);
    void createFlow.beginEditSession(t);
  }

  function closeEdit() {
    closeCreateModal();
    setEditTitleRequiredError(null);
  }

  function openChangeModel(t: Task) {
    if (editingTaskId) {
      closeCreateModal();
    }
    setEditTitleRequiredError(null);
    setChangeModelTask(t);
    setChangeModelDraft(t.cursor_model ?? "");
  }

  function closeChangeModel() {
    setChangeModelTask(null);
  }

  function submitChangeModel(e: FormEvent) {
    e.preventDefault();
    const t = changeModelTask;
    if (!t) return;
    runPatch({
      id: t.id,
      title: t.title.trim(),
      initial_prompt: t.initial_prompt,
      status: t.status,
      priority: t.priority,
      project_id: t.project_id ?? null,
      project_context_item_ids: t.project_context_item_ids ?? [],
      cursor_model: changeModelDraft.trim(),
    });
  }

  function submitEdit(e: FormEvent) {
    e.preventDefault();
    if (!editingTaskId || !newPriority) return;
    if (!newTitle.trim()) {
      setEditTitleRequiredError("Title is required.");
      return;
    }
    setEditTitleRequiredError(null);
    runPatch({
      id: editingTaskId,
      title: newTitle.trim(),
      initial_prompt: newPrompt,
      status: composeStatus,
      priority: newPriority as Priority,
      project_id: newProjectID.trim() || null,
      project_context_item_ids: newProjectContextItemIDs,
      tags: newTagsCsv
        .split(/[,;\n]+/)
        .map((t) => t.trim())
        .filter(Boolean),
      milestone: newMilestone.trim() || null,
      cursor_model: newTaskCursorModel.trim(),
      ...(canEditTaskPickupSchedule(composeStatus)
        ? { pickup_not_before: newSchedule }
        : {}),
    });
  }

  function submitComposeModal(e: FormEvent) {
    if (editingTaskId) {
      submitEdit(e);
      return;
    }
    if (createFlow.composeTarget === "template") {
      void createFlow.submitTemplate(e);
      return;
    }
    void createFlow.submitCreate(e);
  }

  useEffect(() => {
    if (!tasksQuery.isPending && rootTaskTrees.length === 0 && taskListPage > 0) {
      setTaskListPage(0);
    }
  }, [tasksQuery.isPending, rootTaskTrees.length, taskListPage]);

  const hasNextTaskPage = rootTaskTrees.length === TASK_LIST_PAGE_SIZE;
  const hasPrevTaskPage = taskListPage > 0;

  return {
    ...createFlow,
    closeCreateModal,
    editingTaskId,
    composeStatus,
    newTitle,
    newPrompt,
    newPriority,
    newProjectID,
    newProjectContextItemIDs,
    newTagsCsv,
    newMilestone,
    newTaskCursorModel,
    newSchedule,
    tasks,
    rootTasksOnPage: rootTaskTrees.length,
    loading,
    listRefreshing,
    saving,
    patchPending,
    patchError,
    deletePending,
    deleteSuccess,
    deleteVariables,
    error,
    sseLive,
    taskStats: taskStatsQuery.data,
    taskStatsLoading: taskStatsQuery.isPending,
    changeModelTask,
    changeModelDraft,
    setChangeModelDraft,
    openChangeModel,
    closeChangeModel,
    submitChangeModel,
    openEdit,
    closeEdit,
    submitEdit,
    submitComposeModal,
    editFormError: editTitleRequiredError,
    deleteTarget,
    requestDelete,
    cancelDelete,
    confirmDelete,
    deleteError,
    taskListPage,
    setTaskListPage,
    resetTaskListPage,
    taskListPageSize: TASK_LIST_PAGE_SIZE,
    hasNextTaskPage,
    hasPrevTaskPage,
  };
}
