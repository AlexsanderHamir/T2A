import { useQuery } from "@tanstack/react-query";
import { fetchAppSettings } from "@/api/settings";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import { getTaskStats, listTasks } from "../../api";
import { flattenTaskTreeRoots } from "../task-tree";
import { TASK_LIST_PAGE_SIZE } from "../task-paging";
import { settingsQueryKeys, taskQueryKeys } from "../task-query";
import { errorMessage } from "@/lib/errorMessage";
import {
  DEFAULT_NEW_TASK_STATUS,
  DEFAULT_NEW_TASK_TYPE,
  DEFAULT_PROJECT_ID,
  type Priority,
  type Status,
  type Task,
  type TaskType,
} from "@/types";
import { useHysteresisBoolean } from "@/lib/useHysteresisBoolean";
import { TASK_TIMINGS } from "@/constants/tasks";
import { useTaskDeleteFlow } from "./useTaskDeleteFlow";
import { useTaskPatchFlow } from "./useTaskPatchFlow";
import { useTaskCreateFlow } from "./useTaskCreateFlow";
import { canEditTaskPickupSchedule } from "../task-pickup/canEditTaskPickupSchedule";

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
    ...createFlow
  } = useTaskCreateFlow();

  const [editing, setEditing] = useState<Task | null>(null);
  const [editTitle, setEditTitle] = useState("");
  const [editPrompt, setEditPrompt] = useState("");
  const [editPriority, setEditPriority] = useState<Priority>("medium");
  const [editTaskType, setEditTaskType] = useState<TaskType>(DEFAULT_NEW_TASK_TYPE);
  const [editStatus, setEditStatus] = useState<Status>(DEFAULT_NEW_TASK_STATUS);
  const [editChecklistInherit, setEditChecklistInherit] = useState(false);
  const [editProjectID, setEditProjectID] = useState(DEFAULT_PROJECT_ID);
  const [editProjectContextItemIDs, setEditProjectContextItemIDs] = useState<string[]>([]);
  const [editTagsCsv, setEditTagsCsv] = useState("");
  const [editMilestone, setEditMilestone] = useState("");
  const [editCursorModel, setEditCursorModel] = useState("");
  const [editPickupSchedule, setEditPickupSchedule] = useState<string | null>(
    null,
  );
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
      setEditing((prev) => (prev?.id === deletedId ? null : prev));
    },
  });

  /** Client-side validation (shown after server errors when applicable). */
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
    // When the active route does not consume the task list (e.g. the
    // user navigated to /settings), suspend the query so SSE
    // invalidations and window-focus refetches stop firing GET /tasks
    // for a view nobody is rendering. The cache stays populated; the
    // next time the home route mounts, React Query will revalidate
    // through its normal stale-while-revalidate path.
    enabled: dataEnabled,
    // SSE keeps this cache fresh in real time and bootstrap seeds it
    // on cold start. The default 15s staleTime forced unnecessary
    // background refetches whenever a component remounted (e.g.
    // returning from the task detail page); 60s matches the freshness
    // budget that SSE already enforces.
    staleTime: 60_000,
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
    staleTime: 60_000,
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
      setEditing((prev) => (prev?.id === patchedId ? null : prev));
      setChangeModelTask((prev) => (prev?.id === patchedId ? null : prev));
    },
  });

  // Wipe stale errors when their hosting modals close so the next open
  // doesn't render an old `.err role="alert"` callout before the user has
  // interacted. Mirrors the `createMutation.reset()` / `evaluateDraftMutation.reset()`
  // lifecycle wired in session #33; pinned by the per-component error tests.
  useEffect(() => {
    if (!editing && !changeModelTask) resetPatchError();
  }, [editing, changeModelTask, resetPatchError]);

  useEffect(() => {
    if (!deleteTarget) resetDeleteError();
  }, [deleteTarget, resetDeleteError]);

  const saving =
    createFlow.createPending ||
    createFlow.evaluatePending ||
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
    if (editTitleRequiredError && editTitle.trim()) {
      setEditTitleRequiredError(null);
    }
  }, [editTitle, editTitleRequiredError]);

  function openEdit(t: Task) {
    setChangeModelTask(null);
    setEditing(t);
    setEditTitle(t.title);
    setEditPrompt(t.initial_prompt);
    setEditPriority(t.priority);
    setEditTaskType(t.task_type ?? DEFAULT_NEW_TASK_TYPE);
    setEditStatus(t.status);
    setEditChecklistInherit(t.checklist_inherit === true);
    setEditProjectID(t.project_id || DEFAULT_PROJECT_ID);
    setEditProjectContextItemIDs(t.project_context_item_ids ?? []);
    setEditTagsCsv((t.tags ?? []).join(", "));
    setEditMilestone(t.milestone ?? "");
    setEditCursorModel(t.cursor_model ?? "");
    setEditPickupSchedule(t.pickup_not_before ?? null);
    setEditTitleRequiredError(null);
  }

  function closeEdit() {
    setEditing(null);
    setEditTitleRequiredError(null);
  }

  function openChangeModel(t: Task) {
    setEditing(null);
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
      task_type: t.task_type ?? DEFAULT_NEW_TASK_TYPE,
      checklist_inherit: t.checklist_inherit === true,
      project_id: t.project_id ?? null,
      project_context_item_ids: t.project_context_item_ids ?? [],
      cursor_model: changeModelDraft.trim(),
    });
  }

  function submitEdit(e: FormEvent) {
    e.preventDefault();
    if (!editing) return;
    if (!editTitle.trim()) {
      setEditTitleRequiredError("Title is required.");
      return;
    }
    setEditTitleRequiredError(null);
    runPatch({
      id: editing.id,
      title: editTitle.trim(),
      initial_prompt: editPrompt,
      status: editStatus,
      priority: editPriority,
      task_type: editTaskType,
      checklist_inherit: editChecklistInherit,
      project_id: editProjectID.trim() || null,
      project_context_item_ids: editProjectContextItemIDs,
      tags: editTagsCsv
        .split(/[,;\n]+/)
        .map((t) => t.trim())
        .filter(Boolean),
      milestone: editMilestone.trim() || null,
      cursor_model: editCursorModel.trim(),
      ...(canEditTaskPickupSchedule(editStatus)
        ? { pickup_not_before: editPickupSchedule }
        : {}),
    });
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
    /**
     * True only on the first stats query resolution (before any settle). Stays false
     * during background refetch so consumers can keep showing the previous
     * values instead of replacing them with a skeleton on every refresh.
     */
    taskStatsLoading: taskStatsQuery.isPending,
    editing,
    editTitle,
    setEditTitle,
    editPrompt,
    setEditPrompt,
    editPriority,
    editTaskType,
    setEditPriority,
    setEditTaskType,
    editStatus,
    setEditStatus,
    editChecklistInherit,
    setEditChecklistInherit,
    editProjectID,
    setEditProjectID,
    editProjectContextItemIDs,
    setEditProjectContextItemIDs,
    editTagsCsv,
    setEditTagsCsv,
    editMilestone,
    setEditMilestone,
    editCursorModel,
    setEditCursorModel,
    editPickupSchedule,
    setEditPickupSchedule,
    changeModelTask,
    changeModelDraft,
    setChangeModelDraft,
    openChangeModel,
    closeChangeModel,
    submitChangeModel,
    openEdit,
    closeEdit,
    submitEdit,
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
