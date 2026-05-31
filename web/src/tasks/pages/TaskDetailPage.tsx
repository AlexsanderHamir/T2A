import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { getTask, listChecklist, patchTask } from "@/api";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { errorMessage } from "@/lib/errorMessage";
import {
  rumMutationOptimisticApplied,
  rumMutationRolledBack,
  rumMutationSettled,
  rumMutationStarted,
} from "@/observability";
import { useOptionalToast } from "@/shared/toast";
import { useRolloutFlags } from "@/settings";
import {
  bumpOptimisticVersion,
  clearOptimisticVersion,
} from "../hooks/optimisticVersion";
import type { Task } from "@/types";
import {
  SubtaskCreateModal,
  SubtaskTree,
  TaskCyclesPanel,
  TaskDetailAttentionBar,
  TaskDetailChecklistSection,
  TaskDetailHeader,
  TaskDetailPromptSection,
  TaskDetailSchedule,
  TaskDependenciesPanel,
  TaskDetailSubtasksHead,
  TaskDetailUpdatesSection,
  TaskGatePanel,
  TaskModelConfigModal,
} from "../components/task-detail";
import { AutonomyConfirmDialog } from "../components/dialogs";
import { sanitizePromptHtml } from "../task-prompt";
import { taskDescendantCount, userAttention } from "../task-display";
import { TaskDetailPageSkeleton } from "../components/skeletons";
import { useTaskDetailChecklist } from "../hooks/useTaskDetailChecklist";
import { useTaskDetailDeleteNavigate } from "../hooks/useTaskDetailDeleteNavigate";
import { useTaskDetailEvents } from "../hooks/useTaskDetailEvents";
import { useTaskDetailSubtasks } from "../hooks/useTaskDetailSubtasks";
import { resolveTaskDependencySummaries, taskQueryKeys } from "../task-query";
import { useTasksApp } from "../hooks/useTasksApp";
import { useTaskDetailScheduling } from "../hooks/useTaskDetailScheduling";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

export function TaskDetailPage({ app }: Props) {
  const { taskId = "" } = useParams<{ taskId: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [modelConfigOpen, setModelConfigOpen] = useState(false);
  const [autonomyConfirmOpen, setAutonomyConfirmOpen] = useState(false);
  const {
    subtaskModalOpen,
    subtaskTitle,
    setSubtaskTitle,
    subtaskPrompt,
    setSubtaskPrompt,
    subtaskPriority,
    setSubtaskPriority,
    subtaskTaskType,
    setSubtaskTaskType,
    subtaskChecklistItems,
    subtaskInherit,
    setSubtaskInherit,
    openSubtaskModal,
    closeSubtaskModal,
    appendSubtaskChecklistCriterion,
    removeSubtaskChecklistRow,
    updateSubtaskChecklistRow,
    createSubtaskMutation,
    submitNewSubtask,
  } = useTaskDetailSubtasks(taskId);
  const {
    checklistModalOpen,
    newChecklistText,
    setNewChecklistText,
    editCriterionModalOpen,
    editingChecklistItemId,
    editChecklistText,
    setEditChecklistText,
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
    app.deleteSuccess,
    app.deleteVariables,
  );

  const taskQuery = useQuery({
    queryKey: taskQueryKeys.detail(taskId),
    queryFn: ({ signal }) => getTask(taskId, { signal }),
    enabled: Boolean(taskId),
  });

  const {
    eventsQuery,
    timelineEvents,
    eventsTotal,
    onEventsPagerPrev,
    onEventsPagerNext,
  } = useTaskDetailEvents(taskId, taskQuery.isSuccess);

  const checklistQuery = useQuery({
    queryKey: taskQueryKeys.checklist(taskId),
    queryFn: ({ signal }) => listChecklist(taskId, { signal }),
    enabled: Boolean(taskId) && taskQuery.isSuccess,
  });

  const toast = useOptionalToast();
  const scheduling = useTaskDetailScheduling(taskId);
  const { optimisticMutationsEnabled } = useRolloutFlags();
  const requeueMutation = useMutation<
    unknown,
    unknown,
    void,
    { prev: Task | undefined; startedAtMs: number }
  >({
    mutationFn: () => patchTask(taskId, { status: "ready" }),
    onMutate: async () => {
      const startedAtMs = performance.now();
      rumMutationStarted("task_requeue");
      if (!optimisticMutationsEnabled) {
        return { prev: undefined, startedAtMs };
      }
      bumpOptimisticVersion(taskId);
      await queryClient.cancelQueries({ queryKey: taskQueryKeys.detail(taskId) });
      const detailKey = taskQueryKeys.detail(taskId);
      const prev = queryClient.getQueryData<Task>(detailKey);
      if (prev) {
        queryClient.setQueryData<Task>(detailKey, { ...prev, status: "ready" });
      }
      rumMutationOptimisticApplied("task_requeue", performance.now() - startedAtMs);
      return { prev, startedAtMs };
    },
    onError: (_err, _vars, context) => {
      if (context?.prev) {
        queryClient.setQueryData(taskQueryKeys.detail(taskId), context.prev);
      }
      if (context) {
        if (context.prev !== undefined) {
          rumMutationRolledBack(
            "task_requeue",
            performance.now() - context.startedAtMs,
          );
        }
        rumMutationSettled(
          "task_requeue",
          performance.now() - context.startedAtMs,
          0,
        );
      }
      toast.error("Couldn't requeue - reverted.");
    },
    onSuccess: async (_data, _vars, context) => {
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.stats() });
      if (context) {
        rumMutationSettled(
          "task_requeue",
          performance.now() - context.startedAtMs,
          200,
        );
      }
    },
    onSettled: () => {
      clearOptimisticVersion(taskId);
    },
  });

  // Autonomy toggle: PATCH status between "ready" and "on_hold". Mirrors
  // the requeue shape (optimistic on the detail row, server-truth
  // invalidation for list/stats/tree caches) so the autonomy state
  // updates feel as snappy as the existing requeue. The button label +
  // dialog copy in TaskDetailAttentionBar / AutonomyConfirmDialog
  // diverge between directions; the mutation itself is symmetric.
  const autonomyMutation = useMutation<
    unknown,
    unknown,
    "ready" | "on_hold",
    { prev: Task | undefined; startedAtMs: number; next: "ready" | "on_hold" }
  >({
    mutationFn: (next) => patchTask(taskId, { status: next }),
    onMutate: async (next) => {
      const startedAtMs = performance.now();
      rumMutationStarted("task_autonomy");
      if (!optimisticMutationsEnabled) {
        return { prev: undefined, startedAtMs, next };
      }
      bumpOptimisticVersion(taskId);
      await queryClient.cancelQueries({ queryKey: taskQueryKeys.detail(taskId) });
      const detailKey = taskQueryKeys.detail(taskId);
      const prev = queryClient.getQueryData<Task>(detailKey);
      if (prev) {
        queryClient.setQueryData<Task>(detailKey, { ...prev, status: next });
      }
      rumMutationOptimisticApplied("task_autonomy", performance.now() - startedAtMs);
      return { prev, startedAtMs, next };
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
    onSettled: () => {
      clearOptimisticVersion(taskId);
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
  const attention = userAttention(task, {
    approvalPending: eventsQuery.data?.approval_pending ?? false,
  });
  const sanitizedInitialPrompt = sanitizePromptHtml(task.initial_prompt);
  // Autonomy is meaningful only for the two states the operator can
  // freely move between without colliding with the agent — `ready`
  // (eligible) and `on_hold` (parked). Running / blocked / review /
  // done / failed are owned by the agent or by deeper state and
  // hiding the toggle keeps the surface honest.
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

      <TaskDetailAttentionBar
        attention={attention}
        saving={app.saving}
        onEdit={() => app.openEdit(task)}
        onDelete={() =>
          app.requestDelete({
            ...task,
            subtaskCount: taskDescendantCount(task),
          })
        }
        onRequeue={
          task.status === "failed"
            ? () => {
                requeueMutation.mutate();
              }
            : undefined
        }
        requeuePending={requeueMutation.isPending}
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

      {autonomyConfirmOpen && autonomyMode !== "hidden" ? (
        <AutonomyConfirmDialog
          enable={autonomyEnable}
          taskTitle={task.title}
          saving={app.saving}
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

      {modelConfigOpen ? (
        <TaskModelConfigModal
          taskTitle={task.title}
          saving={app.saving}
          onChangeModel={() => app.openChangeModel(task)}
          onClose={() => setModelConfigOpen(false)}
        />
      ) : null}

      {requeueMutation.isError ? (
        <p className="err" role="alert">
          {errorMessage(
            requeueMutation.error,
            "Could not queue this task for the agent again.",
          )}
        </p>
      ) : null}

      <TaskDetailSchedule task={task} />

      <TaskDependenciesPanel
        taskId={task.id}
        dependencies={dependencySummaries}
        editable
        addValue={scheduling.depAddValue}
        onAddValueChange={scheduling.setDepAddValue}
        onAdd={() => scheduling.addDepMutation.mutate()}
        onRemove={(depId) => scheduling.removeDepMutation.mutate(depId)}
        addPending={scheduling.addDepMutation.isPending}
        removePendingId={
          scheduling.removeDepMutation.isPending
            ? scheduling.removeDepMutation.variables ?? null
            : null
        }
        error={
          scheduling.addDepMutation.error || scheduling.removeDepMutation.error
            ? scheduling.schedulingError
            : null
        }
      />

      <TaskGatePanel
        gate={task.gate}
        editable
        onAction={(action) => scheduling.gateMutation.mutate(action)}
        actionPending={scheduling.gateMutation.isPending}
        error={scheduling.gateMutation.error ? scheduling.schedulingError : null}
      />

      <div className="task-detail-section" id="task-detail-subtasks">
        <TaskDetailSubtasksHead
          taskId={task.id}
          saving={app.saving}
          onAddSubtask={openSubtaskModal}
        />
        <SubtaskTree nodes={task.children ?? []} showNested={false} />
        {subtaskModalOpen ? (
          <SubtaskCreateModal
            taskId={taskId}
            pending={createSubtaskMutation.isPending}
            saving={app.saving}
            onClose={closeSubtaskModal}
            title={subtaskTitle}
            prompt={subtaskPrompt}
            priority={subtaskPriority}
            taskType={subtaskTaskType}
            checklistItems={subtaskChecklistItems}
            checklistInherit={subtaskInherit}
            onTitleChange={setSubtaskTitle}
            onPromptChange={setSubtaskPrompt}
            onPriorityChange={setSubtaskPriority}
            onTaskTypeChange={setSubtaskTaskType}
            onAppendChecklistCriterion={appendSubtaskChecklistCriterion}
            onUpdateChecklistRow={updateSubtaskChecklistRow}
            onRemoveChecklistRow={removeSubtaskChecklistRow}
            onChecklistInheritChange={setSubtaskInherit}
            onSubmit={submitNewSubtask}
            error={createSubtaskMutation.error}
          />
        ) : null}
      </div>

      <TaskDetailChecklistSection
        checklistInherit={task.checklist_inherit}
        saving={app.saving}
        checklistQuery={checklistQuery}
        doneCount={checklistDoneCount}
        totalCount={checklistTotal}
        modalOpen={checklistModalOpen}
        newCriterionText={newChecklistText}
        onNewCriterionTextChange={setNewChecklistText}
        onOpenAddModal={openChecklistModal}
        onCloseAddModal={closeChecklistModal}
        onSubmitNewCriterion={submitNewChecklistCriterion}
        addCriterionPending={addChecklistMutation.isPending}
        editModalOpen={editCriterionModalOpen}
        editingItemId={editingChecklistItemId}
        editCriterionText={editChecklistText}
        onEditCriterionTextChange={setEditChecklistText}
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

      <TaskDetailUpdatesSection
        taskId={taskId}
        eventsQuery={eventsQuery}
        timelineEvents={timelineEvents}
        eventsTotal={eventsTotal}
        onEventsPagerPrev={onEventsPagerPrev}
        onEventsPagerNext={onEventsPagerNext}
      />
    </section>
  );
}
