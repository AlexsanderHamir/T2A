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

export function TaskDetailPage() {
  const modals = useTasksAppModals();
  const { saving } = useTasksAppMeta();
  const { taskId = "" } = useParams<{ taskId: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [modelConfigOpen, setModelConfigOpen] = useState(false);
  const [autonomyConfirmOpen, setAutonomyConfirmOpen] = useState(false);
  const [retryConfirmMode, setRetryConfirmMode] = useState<TaskRetryMode | null>(
    null,
  );
  const {
    checklistModalOpen,
    newChecklistText,
    setNewChecklistText,
    newChecklistVerifyCommands,
    setNewChecklistVerifyCommands,
    editCriterionModalOpen,
    editingChecklistItemId,
    editChecklistText,
    setEditChecklistText,
    editChecklistVerifyCommands,
    setEditChecklistVerifyCommands,
    closeChecklistModal,
    closeEditCriterionModal,
    openChecklistModal,
    openEditCriterionModal,
    addChecklistMutation,
    submitNewChecklistCriterion,
    updateChecklistTextMutation,
    submitEditChecklistCriterion,
    deleteChecklistMutation,
  } = useTaskDetailChecklist(taskId);

  useTaskDetailDeleteNavigate(
    taskId,
    navigate,
    modals.deleteSuccess,
    modals.deleteVariables,
  );

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

  const toast = useOptionalToast();
  const scheduling = useTaskDetailScheduling(taskId);
  const { optimisticMutationsEnabled } = useRolloutFlags();
  const retryMutation = useMutation<
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
      setRetryConfirmMode(null);
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

  const autonomyMutation = useMutation<
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
      setAutonomyConfirmOpen(false);
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

  if (!taskId) {
    return (
      <p className="muted" role="status">
        Missing task id.
      </p>
    );
  }

  if (taskQuery.isPending) {
    return <TaskDetailPageSkeleton />;
  }

  if (taskQuery.isError) {
    return (
      <section className="panel task-detail-panel task-detail-content--enter">
        <div className="err" role="alert">
          <p>{errorMessage(taskQuery.error, "Could not load task.")}</p>
          <div className="task-detail-error-actions">
            <button
              type="button"
              className="secondary"
              onClick={() => void taskQuery.refetch()}
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

  const task = taskQuery.data;
  const checklistItems = checklistQuery.data?.items ?? [];
  const checklistDoneCount = checklistItems.filter((i) => i.done).length;
  const checklistTotal = checklistItems.length;
  const sanitizedInitialPrompt = sanitizePromptHtml(task.initial_prompt);
  const autonomyMode: "hidden" | "ready" | "on_hold" =
    task.status === "ready"
      ? "ready"
      : task.status === "on_hold"
      ? "on_hold"
      : "hidden";
  const autonomyEnable = autonomyMode === "on_hold";

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

      <TaskDependenciesPanel dependencies={dependencySummaries} />

      <TaskGatePanel
        gate={task.gate}
        editable
        onAction={(action) => scheduling.gateMutation.mutate(action)}
        actionPending={scheduling.gateMutation.isPending}
        error={scheduling.gateMutation.error ? scheduling.schedulingError : null}
      />

      <TaskDetailChecklistSection
        saving={saving}
        canAddCriterion={canMutateTaskCriteria(task.status)}
        taskStatus={task.status}
        checklistQuery={checklistQuery}
        doneCount={checklistDoneCount}
        totalCount={checklistTotal}
        modalOpen={checklistModalOpen}
        newCriterionText={newChecklistText}
        onNewCriterionTextChange={setNewChecklistText}
        newCriterionVerifyCommands={newChecklistVerifyCommands}
        onNewCriterionVerifyCommandsChange={setNewChecklistVerifyCommands}
        onOpenAddModal={openChecklistModal}
        onCloseAddModal={closeChecklistModal}
        onSubmitNewCriterion={submitNewChecklistCriterion}
        addCriterionPending={addChecklistMutation.isPending}
        editModalOpen={editCriterionModalOpen}
        editingItemId={editingChecklistItemId}
        editCriterionText={editChecklistText}
        onEditCriterionTextChange={setEditChecklistText}
        editCriterionVerifyCommands={editChecklistVerifyCommands}
        onEditCriterionVerifyCommandsChange={setEditChecklistVerifyCommands}
        onOpenEditCriterionModal={openEditCriterionModal}
        onCloseEditCriterionModal={closeEditCriterionModal}
        onSubmitEditCriterion={submitEditChecklistCriterion}
        editCriterionPending={updateChecklistTextMutation.isPending}
        onRemoveChecklistItem={(id) => deleteChecklistMutation.mutate(id)}
        removeItemPending={deleteChecklistMutation.isPending}
        addCriterionError={addChecklistMutation.error}
        editCriterionError={updateChecklistTextMutation.error}
        removeItemError={deleteChecklistMutation.error}
      />

      <TaskDetailPromptSection
        initialPrompt={task.initial_prompt}
        sanitizedInitialPrompt={sanitizedInitialPrompt}
      />

      <TaskCyclesPanel taskId={taskId} enabled={taskQuery.isSuccess} />

      <TaskCommitsPanel taskId={taskId} enabled={taskQuery.isSuccess} />
    </section>
  );
}
