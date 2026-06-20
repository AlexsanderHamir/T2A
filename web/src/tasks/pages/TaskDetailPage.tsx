import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { getTask, listChecklist, patchTask, retryTask } from "@/api";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { errorMessage } from "@/lib/errorMessage";
import {
  rumMutationRolledBack,
  rumMutationSettled,
  rumNavigationTiming,
} from "@/observability";
import { rememberPersistedDetailId } from "@/lib/queryPersist";
import { isUiFeatureOmitted } from "@/launch/omittedFeatures";
import { useOptionalToast } from "@/shared/toast";
import { useRolloutFlags } from "@/settings";
import {
  beginGuardedTaskWrite,
  endGuardedTaskWrite,
  recordOptimisticApplied,
} from "@/tasks/mutations";
import type { Task } from "@/types";
import {
  TaskCyclesPanel,
  TaskCommitsPanel,
  TaskDetailChecklistSection,
  TaskDetailToolbarActions,
  TaskDetailHeader,
  TaskDetailPromptSection,
  TaskDetailSchedule,
  TaskDependenciesPanel,
  TaskGatePanel,
  TaskModelConfigModal,
} from "../components/task-detail";
import { AutonomyConfirmDialog, TaskRetryConfirmDialog } from "../components/dialogs";
import type { TaskRetryMode } from "../components/dialogs/TaskRetryConfirmDialog";
import { sanitizePromptHtml } from "../task-prompt";
import { canMutateTaskCriteria } from "../task-display/canMutateTaskCriteria";
import { TaskDetailPageSkeleton } from "../components/skeletons";
import { useTaskDetailChecklist } from "../hooks/useTaskDetailChecklist";
import { useTaskDetailDeleteNavigate } from "../hooks/useTaskDetailDeleteNavigate";
import { resolveTaskDependencySummaries, taskQueryKeys } from "../task-query";
import { useTasksAppMeta, useTasksAppModals } from "../app/TasksAppProvider";
import { QUERY_POLICY } from "../queryPolicy";
import { useTaskDetailScheduling } from "../hooks/useTaskDetailScheduling";
import type { UseQueryResult } from "@tanstack/react-query";
import type { TaskChecklistResponse } from "@/types";

type AutonomyMode = "hidden" | "ready" | "on_hold";

type TaskDetailChecklistState = ReturnType<typeof useTaskDetailChecklist>;

function useTaskDetailNavigationTiming(
  taskId: string,
  taskQuery: UseQueryResult<Task>,
  checklistQuery: UseQueryResult<TaskChecklistResponse>,
) {
  const navigationMountAtRef = useRef(performance.now());
  const taskTimingSentRef = useRef(false);
  const interactiveTimingSentRef = useRef(false);

  useEffect(() => {
    if (taskId) {
      rememberPersistedDetailId(taskId);
    }
    navigationMountAtRef.current = performance.now();
    taskTimingSentRef.current = false;
    interactiveTimingSentRef.current = false;
  }, [taskId]);

  useEffect(() => {
    if (!taskQuery.isSuccess || taskTimingSentRef.current) return;
    taskTimingSentRef.current = true;
    rumNavigationTiming(
      "navigation.task_detail.time_to_task_ms",
      performance.now() - navigationMountAtRef.current,
    );
  }, [taskQuery.isSuccess]);

  useEffect(() => {
    if (
      !taskQuery.isSuccess ||
      !checklistQuery.isSuccess ||
      interactiveTimingSentRef.current
    ) {
      return;
    }
    interactiveTimingSentRef.current = true;
    rumNavigationTiming(
      "navigation.task_detail.time_to_interactive_ms",
      performance.now() - navigationMountAtRef.current,
    );
  }, [taskQuery.isSuccess, checklistQuery.isSuccess]);
}

function useTaskDetailRetryMutation(
  taskId: string,
  optimisticMutationsEnabled: boolean,
  onRetryConfirmed: () => void,
) {
  const queryClient = useQueryClient();

  return useMutation<
    unknown,
    unknown,
    TaskRetryMode,
    { prev: Task | undefined; startedAtMs: number; guarded: boolean }
  >({
    mutationFn: (mode) => retryTask(taskId, { mode }),
    onMutate: async () => {
      const guard = beginGuardedTaskWrite({
        taskId,
        optimisticEnabled: optimisticMutationsEnabled,
        rumKind: "task_retry",
      });
      if (!guard.guarded) {
        return { prev: undefined, startedAtMs: guard.startedAtMs, guarded: false };
      }
      await queryClient.cancelQueries({ queryKey: taskQueryKeys.detail(taskId) });
      const detailKey = taskQueryKeys.detail(taskId);
      const prev = queryClient.getQueryData<Task>(detailKey);
      if (prev) {
        queryClient.setQueryData<Task>(detailKey, { ...prev, status: "ready" });
      }
      recordOptimisticApplied("task_retry", guard.startedAtMs);
      return { prev, startedAtMs: guard.startedAtMs, guarded: true };
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) {
        queryClient.setQueryData(taskQueryKeys.detail(taskId), context.prev);
      }
      if (context) {
        if (context.prev !== undefined) {
          rumMutationRolledBack(
            "task_retry",
            performance.now() - context.startedAtMs,
          );
        }
        rumMutationSettled(
          "task_retry",
          performance.now() - context.startedAtMs,
          0,
        );
      }
    },
    onSuccess: async (_data, _vars, context) => {
      onRetryConfirmed();
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.stats() });
      if (context) {
        rumMutationSettled(
          "task_retry",
          performance.now() - context.startedAtMs,
          200,
        );
      }
    },
    onSettled: (_data, _err, _vars, context) => {
      if (context?.guarded) {
        endGuardedTaskWrite(taskId);
      }
    },
  });
}

function useTaskDetailAutonomyMutation(
  taskId: string,
  optimisticMutationsEnabled: boolean,
  toast: ReturnType<typeof useOptionalToast>,
  onAutonomyConfirmed: () => void,
) {
  const queryClient = useQueryClient();

  return useMutation<
    unknown,
    unknown,
    "ready" | "on_hold",
    { prev: Task | undefined; startedAtMs: number; next: "ready" | "on_hold"; guarded: boolean }
  >({
    mutationFn: (next) => patchTask(taskId, { status: next }),
    onMutate: async (next) => {
      const guard = beginGuardedTaskWrite({
        taskId,
        optimisticEnabled: optimisticMutationsEnabled,
        rumKind: "task_autonomy",
      });
      if (!guard.guarded) {
        return { prev: undefined, startedAtMs: guard.startedAtMs, next, guarded: false };
      }
      await queryClient.cancelQueries({ queryKey: taskQueryKeys.detail(taskId) });
      const detailKey = taskQueryKeys.detail(taskId);
      const prev = queryClient.getQueryData<Task>(detailKey);
      if (prev) {
        queryClient.setQueryData<Task>(detailKey, { ...prev, status: next });
      }
      recordOptimisticApplied("task_autonomy", guard.startedAtMs);
      return { prev, startedAtMs: guard.startedAtMs, next, guarded: true };
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) {
        queryClient.setQueryData(taskQueryKeys.detail(taskId), context.prev);
      }
      if (context) {
        if (context.prev !== undefined) {
          rumMutationRolledBack(
            "task_autonomy",
            performance.now() - context.startedAtMs,
          );
        }
        rumMutationSettled(
          "task_autonomy",
          performance.now() - context.startedAtMs,
          0,
        );
      }
      toast.error("Couldn't update autonomy — reverted.");
    },
    onSuccess: async (_data, _vars, context) => {
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.stats() });
      onAutonomyConfirmed();
      if (context) {
        rumMutationSettled(
          "task_autonomy",
          performance.now() - context.startedAtMs,
          200,
        );
      }
    },
    onSettled: (_data, _err, _vars, context) => {
      if (context?.guarded) {
        endGuardedTaskWrite(taskId);
      }
    },
  });
}

function resolveAutonomyMode(taskStatus: Task["status"]): AutonomyMode {
  if (taskStatus === "ready") return "ready";
  if (taskStatus === "on_hold") return "on_hold";
  return "hidden";
}

function countChecklistProgress(items: { done: boolean }[]) {
  const doneCount = items.filter((item) => item.done).length;
  return { doneCount, totalCount: items.length };
}

function renderMissingTaskId() {
  return (
    <p className="muted" role="status">
      Missing task id.
    </p>
  );
}

function renderTaskLoadError(
  error: unknown,
  onRetry: () => void,
) {
  return (
    <section className="panel task-detail-panel task-detail-content--enter">
      <div className="err" role="alert">
        <p>{errorMessage(error, "Could not load task.")}</p>
        <div className="task-detail-error-actions">
          <button
            type="button"
            className="secondary"
            onClick={() => void onRetry()}
          >
            Try again
          </button>
          <Link to="/" className="pd__back project-context-back-link">
            <span aria-hidden="true">&#8249;</span>
            All tasks
          </Link>
        </div>
      </div>
    </section>
  );
}

type TaskDetailLoadedViewProps = {
  task: Task;
  taskId: string;
  taskQuerySuccess: boolean;
  saving: boolean;
  modals: ReturnType<typeof useTasksAppModals>;
  scheduling: ReturnType<typeof useTaskDetailScheduling>;
  checklistQuery: UseQueryResult<TaskChecklistResponse>;
  checklistState: TaskDetailChecklistState;
  dependencySummaries: ReturnType<typeof resolveTaskDependencySummaries>;
  autonomyMode: AutonomyMode;
  autonomyConfirmOpen: boolean;
  setAutonomyConfirmOpen: (open: boolean) => void;
  autonomyMutation: ReturnType<typeof useTaskDetailAutonomyMutation>;
  retryConfirmMode: TaskRetryMode | null;
  setRetryConfirmMode: (mode: TaskRetryMode | null) => void;
  retryMutation: ReturnType<typeof useTaskDetailRetryMutation>;
  modelConfigOpen: boolean;
  setModelConfigOpen: (open: boolean) => void;
};

function renderTaskDetailLoadedView({
  task,
  taskId,
  taskQuerySuccess,
  saving,
  modals,
  scheduling,
  checklistQuery,
  checklistState,
  dependencySummaries,
  autonomyMode,
  autonomyConfirmOpen,
  setAutonomyConfirmOpen,
  autonomyMutation,
  retryConfirmMode,
  setRetryConfirmMode,
  retryMutation,
  modelConfigOpen,
  setModelConfigOpen,
}: TaskDetailLoadedViewProps) {
  const checklistItems = checklistQuery.data?.items ?? [];
  const { doneCount, totalCount } = countChecklistProgress(checklistItems);
  const sanitizedInitialPrompt = sanitizePromptHtml(task.initial_prompt);
  const autonomyEnable = autonomyMode === "on_hold";
  const dependenciesUiEnabled = !isUiFeatureOmitted("tagsAndDependencies");
  const releaseGatesUiEnabled = !isUiFeatureOmitted("releaseGates");

  return (
    <section className="panel task-detail-panel task-detail-content--enter">
      <TaskDetailHeader task={task} />

      <div className="task-detail-toolbar">
        <TaskDetailSchedule task={task} />
        <TaskDetailToolbarActions
          saving={saving}
          onEdit={() => modals.openEdit(task)}
          onDelete={() => modals.requestDelete(task)}
          onRetryFresh={
            task.status === "failed"
              ? () => setRetryConfirmMode("fresh")
              : undefined
          }
          onRetryResume={
            task.status === "failed"
              ? () => setRetryConfirmMode("resume")
              : undefined
          }
          retryPending={retryMutation.isPending}
          onConfigureModel={() => setModelConfigOpen(true)}
          showModelConfig={task.status === "failed"}
          autonomyMode={autonomyMode}
          onToggleAutonomy={
            autonomyMode !== "hidden"
              ? () => setAutonomyConfirmOpen(true)
              : undefined
          }
          autonomyPending={autonomyMutation.isPending}
        />
      </div>

      {autonomyConfirmOpen && autonomyMode !== "hidden" ? (
        <AutonomyConfirmDialog
          enable={autonomyEnable}
          taskTitle={task.title}
          saving={saving}
          pending={autonomyMutation.isPending}
          error={
            autonomyMutation.isError
              ? errorMessage(
                  autonomyMutation.error,
                  autonomyEnable
                    ? "Couldn't resume autonomous execution."
                    : "Couldn't put this task on hold.",
                )
              : null
          }
          onCancel={() => {
            setAutonomyConfirmOpen(false);
            if (autonomyMutation.isError) autonomyMutation.reset();
          }}
          onConfirm={() =>
            autonomyMutation.mutate(autonomyEnable ? "ready" : "on_hold")
          }
        />
      ) : null}

      {retryConfirmMode ? (
        <TaskRetryConfirmDialog
          mode={retryConfirmMode}
          taskTitle={task.title}
          saving={saving}
          pending={retryMutation.isPending}
          error={
            retryMutation.isError
              ? errorMessage(
                  retryMutation.error,
                  retryConfirmMode === "fresh"
                    ? "Couldn't start over."
                    : "Couldn't resume from failure.",
                )
              : null
          }
          onCancel={() => {
            setRetryConfirmMode(null);
            if (retryMutation.isError) retryMutation.reset();
          }}
          onConfirm={() => retryMutation.mutate(retryConfirmMode)}
        />
      ) : null}

      {modelConfigOpen ? (
        <TaskModelConfigModal
          taskTitle={task.title}
          saving={saving}
          onChangeModel={() => modals.openChangeModel(task)}
          onClose={() => setModelConfigOpen(false)}
        />
      ) : null}

      {dependenciesUiEnabled ? (
        <TaskDependenciesPanel dependencies={dependencySummaries} />
      ) : null}

      {releaseGatesUiEnabled ? (
        <TaskGatePanel
          gate={task.gate}
          editable
          onAction={(action) => scheduling.gateMutation.mutate(action)}
          actionPending={scheduling.gateMutation.isPending}
          error={scheduling.gateMutation.error ? scheduling.schedulingError : null}
        />
      ) : null}

      <TaskDetailChecklistSection
        saving={saving}
        canAddCriterion={canMutateTaskCriteria(task.status)}
        taskStatus={task.status}
        checklistQuery={checklistQuery}
        doneCount={doneCount}
        totalCount={totalCount}
        modalOpen={checklistState.checklistModalOpen}
        newCriterionText={checklistState.newChecklistText}
        onNewCriterionTextChange={checklistState.setNewChecklistText}
        newCriterionVerifyCommands={checklistState.newChecklistVerifyCommands}
        onNewCriterionVerifyCommandsChange={checklistState.setNewChecklistVerifyCommands}
        onOpenAddModal={checklistState.openChecklistModal}
        onCloseAddModal={checklistState.closeChecklistModal}
        onSubmitNewCriterion={checklistState.submitNewChecklistCriterion}
        addCriterionPending={checklistState.addChecklistMutation.isPending}
        editModalOpen={checklistState.editCriterionModalOpen}
        editingItemId={checklistState.editingChecklistItemId}
        editCriterionText={checklistState.editChecklistText}
        onEditCriterionTextChange={checklistState.setEditChecklistText}
        editCriterionVerifyCommands={checklistState.editChecklistVerifyCommands}
        onEditCriterionVerifyCommandsChange={checklistState.setEditChecklistVerifyCommands}
        onOpenEditCriterionModal={checklistState.openEditCriterionModal}
        onCloseEditCriterionModal={checklistState.closeEditCriterionModal}
        onSubmitEditCriterion={checklistState.submitEditChecklistCriterion}
        editCriterionPending={checklistState.updateChecklistTextMutation.isPending}
        onRemoveChecklistItem={(id) => checklistState.deleteChecklistMutation.mutate(id)}
        removeItemPending={checklistState.deleteChecklistMutation.isPending}
        addCriterionError={checklistState.addChecklistMutation.error}
        editCriterionError={checklistState.updateChecklistTextMutation.error}
        removeItemError={checklistState.deleteChecklistMutation.error}
      />

      <TaskDetailPromptSection
        initialPrompt={task.initial_prompt}
        sanitizedInitialPrompt={sanitizedInitialPrompt}
      />

      <TaskCyclesPanel taskId={taskId} enabled={taskQuerySuccess} />

      <TaskCommitsPanel taskId={taskId} enabled={taskQuerySuccess} />
    </section>
  );
}

function useTaskDetailPageQueries(taskId: string) {
  const queryClient = useQueryClient();

  const taskQuery = useQuery({
    queryKey: taskQueryKeys.detail(taskId),
    queryFn: ({ signal }) => getTask(taskId, { signal }),
    enabled: Boolean(taskId),
    staleTime: QUERY_POLICY.detailStaleTimeMs,
  });

  const checklistQuery = useQuery({
    queryKey: taskQueryKeys.checklist(taskId),
    queryFn: ({ signal }) => listChecklist(taskId, { signal }),
    enabled: Boolean(taskId),
    staleTime: QUERY_POLICY.detailStaleTimeMs,
  });

  useTaskDetailNavigationTiming(taskId, taskQuery, checklistQuery);

  const taskDocTitle =
    taskId && taskQuery.isSuccess && taskQuery.data
      ? taskQuery.data.title.trim() || "Untitled task"
      : null;
  useDocumentTitle(taskDocTitle);

  const dependencySummaries = useMemo(
    () =>
      resolveTaskDependencySummaries(
        queryClient,
        taskQuery.data?.depends_on ?? [],
      ),
    [queryClient, taskQuery.data?.depends_on],
  );

  return { taskQuery, checklistQuery, dependencySummaries };
}

function useTaskDetailPageDialogs(taskId: string) {
  const [modelConfigOpen, setModelConfigOpen] = useState(false);
  const [autonomyConfirmOpen, setAutonomyConfirmOpen] = useState(false);
  const [retryConfirmMode, setRetryConfirmMode] = useState<TaskRetryMode | null>(
    null,
  );
  const toast = useOptionalToast();
  const { optimisticMutationsEnabled } = useRolloutFlags();
  const retryMutation = useTaskDetailRetryMutation(
    taskId,
    optimisticMutationsEnabled,
    () => setRetryConfirmMode(null),
  );
  const autonomyMutation = useTaskDetailAutonomyMutation(
    taskId,
    optimisticMutationsEnabled,
    toast,
    () => setAutonomyConfirmOpen(false),
  );

  return {
    modelConfigOpen,
    setModelConfigOpen,
    autonomyConfirmOpen,
    setAutonomyConfirmOpen,
    retryConfirmMode,
    setRetryConfirmMode,
    retryMutation,
    autonomyMutation,
  };
}

export function TaskDetailPage() {
  const modals = useTasksAppModals();
  const { saving } = useTasksAppMeta();
  const { taskId = "" } = useParams<{ taskId: string }>();
  const navigate = useNavigate();
  const checklistState = useTaskDetailChecklist(taskId);
  const { taskQuery, checklistQuery, dependencySummaries } =
    useTaskDetailPageQueries(taskId);
  const scheduling = useTaskDetailScheduling(taskId);
  const {
    modelConfigOpen,
    setModelConfigOpen,
    autonomyConfirmOpen,
    setAutonomyConfirmOpen,
    retryConfirmMode,
    setRetryConfirmMode,
    retryMutation,
    autonomyMutation,
  } = useTaskDetailPageDialogs(taskId);

  useTaskDetailDeleteNavigate(
    taskId,
    navigate,
    modals.deleteSuccess,
    modals.deleteVariables,
  );

  if (!taskId) {
    return renderMissingTaskId();
  }

  if (taskQuery.isPending) {
    return <TaskDetailPageSkeleton />;
  }

  if (taskQuery.isError) {
    return renderTaskLoadError(taskQuery.error, () => void taskQuery.refetch());
  }

  const task = taskQuery.data;
  const autonomyMode = resolveAutonomyMode(task.status);

  return renderTaskDetailLoadedView({
    task,
    taskId,
    taskQuerySuccess: taskQuery.isSuccess,
    saving,
    modals,
    scheduling,
    checklistQuery,
    checklistState,
    dependencySummaries,
    autonomyMode,
    autonomyConfirmOpen,
    setAutonomyConfirmOpen,
    autonomyMutation,
    retryConfirmMode,
    setRetryConfirmMode,
    retryMutation,
    modelConfigOpen,
    setModelConfigOpen,
  });
}
