import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router-dom";
import { getTask, listChecklist, patchTask } from "@/api";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { errorMessage } from "@/lib/errorMessage";
import {
  SubtaskCreateModal,
  SubtaskTree,
  TaskCyclesPanel,
  TaskDetailAttentionBar,
  TaskDetailChecklistSection,
  TaskDetailHeader,
  TaskDetailPromptSection,
  TaskDetailSchedule,
  TaskDetailSubtasksHead,
  TaskDetailUpdatesSection,
} from "../components/task-detail";
import { sanitizePromptHtml } from "../task-prompt";
import { taskDescendantCount, userAttention } from "../task-display";
import { TaskDetailPageSkeleton } from "../components/skeletons";
import { useTaskDetailChecklist } from "../hooks/useTaskDetailChecklist";
import { useTaskDetailDeleteNavigate } from "../hooks/useTaskDetailDeleteNavigate";
import { useTaskDetailEvents } from "../hooks/useTaskDetailEvents";
import { useTaskDetailSubtasks } from "../hooks/useTaskDetailSubtasks";
import { taskQueryKeys } from "../task-query";
import { useTasksApp } from "../hooks/useTasksApp";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

export function TaskDetailPage({ app }: Props) {
  const { taskId = "" } = useParams<{ taskId: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
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
  } = useTaskDetailSubtasks(taskId, queryClient);
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
  } = useTaskDetailChecklist(taskId, queryClient);

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

  const requeueMutation = useMutation({
    mutationFn: () => patchTask(taskId, { status: "ready" }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      await queryClient.invalidateQueries({ queryKey: ["task-stats"] });
    },
  });

  const taskDocTitle =
    taskId && taskQuery.isSuccess && taskQuery.data
      ? taskQuery.data.title.trim() || "Untitled task"
      : null;
  useDocumentTitle(taskDocTitle);

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
            <Link to="/">← Back to tasks</Link>
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
        failedRunnerHint={task.status === "failed"}
      />

      {requeueMutation.isError ? (
        <p className="err" role="alert">
          {errorMessage(
            requeueMutation.error,
            "Could not queue this task for the agent again.",
          )}
        </p>
      ) : null}

      <TaskDetailSchedule task={task} />

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
