import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  addChecklistItem,
  createTask,
  deleteChecklistItem,
  getTask,
  listChecklist,
  listTaskEvents,
  patchChecklistItemText,
} from "@/api";
import {
  DEFAULT_NEW_TASK_TYPE,
  type Priority,
  type PriorityChoice,
  type TaskType,
} from "@/types";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { SubtaskCreateModal } from "../components/SubtaskCreateModal";
import { SubtaskTree } from "../components/SubtaskTree";
import { TaskDetailAttentionBar } from "../components/TaskDetailAttentionBar";
import { TaskDetailChecklistSection } from "../components/TaskDetailChecklistSection";
import { TaskDetailHeader } from "../components/TaskDetailHeader";
import { TaskDetailSubtasksHead } from "../components/TaskDetailSubtasksHead";
import { TaskDetailPromptSection } from "../components/TaskDetailPromptSection";
import { TaskDetailUpdatesSection } from "../components/TaskDetailUpdatesSection";
import { sanitizePromptHtml } from "../promptFormat";
import { TASK_EVENTS_PAGE_SIZE } from "../paging";
import { userAttention } from "../taskAttention";
import { TaskDetailPageSkeleton } from "../components/taskLoadingSkeletons";
import { taskQueryKeys, type TaskEventsCursorKey } from "../queryKeys";
import { useTasksApp } from "../hooks/useTasksApp";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

export function TaskDetailPage({ app }: Props) {
  const { taskId = "" } = useParams<{ taskId: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const navigatedAfterDelete = useRef(false);
  const [subtaskTitle, setSubtaskTitle] = useState("");
  const [subtaskPrompt, setSubtaskPrompt] = useState("");
  const [subtaskPriority, setSubtaskPriority] = useState<PriorityChoice>("");
  const [subtaskTaskType, setSubtaskTaskType] = useState<TaskType>(DEFAULT_NEW_TASK_TYPE);
  const [subtaskChecklistItems, setSubtaskChecklistItems] = useState<string[]>(
    [],
  );
  const [subtaskInherit, setSubtaskInherit] = useState(false);
  const [subtaskModalOpen, setSubtaskModalOpen] = useState(false);
  const [checklistModalOpen, setChecklistModalOpen] = useState(false);
  const [newChecklistText, setNewChecklistText] = useState("");
  const [editCriterionModalOpen, setEditCriterionModalOpen] = useState(false);
  const [editingChecklistItemId, setEditingChecklistItemId] = useState<
    string | null
  >(null);
  const [editChecklistText, setEditChecklistText] = useState("");

  const [eventsCursor, setEventsCursor] = useState<TaskEventsCursorKey>({
    k: "head",
  });

  useEffect(() => {
    navigatedAfterDelete.current = false;
  }, [taskId]);

  useEffect(() => {
    setEventsCursor({ k: "head" });
  }, [taskId]);

  const resetSubtaskForm = useCallback(() => {
    setSubtaskTitle("");
    setSubtaskPrompt("");
    setSubtaskPriority("");
    setSubtaskTaskType(DEFAULT_NEW_TASK_TYPE);
    setSubtaskChecklistItems([]);
    setSubtaskInherit(false);
  }, []);

  const closeSubtaskModal = useCallback(() => {
    setSubtaskModalOpen(false);
    resetSubtaskForm();
  }, [resetSubtaskForm]);

  const openSubtaskModal = useCallback(() => {
    resetSubtaskForm();
    setSubtaskModalOpen(true);
  }, [resetSubtaskForm]);

  const closeChecklistModal = useCallback(() => {
    setChecklistModalOpen(false);
    setNewChecklistText("");
  }, []);

  const closeEditCriterionModal = useCallback(() => {
    setEditCriterionModalOpen(false);
    setEditingChecklistItemId(null);
    setEditChecklistText("");
  }, []);

  const openChecklistModal = useCallback(() => {
    setNewChecklistText("");
    setChecklistModalOpen(true);
    setEditCriterionModalOpen(false);
    setEditingChecklistItemId(null);
    setEditChecklistText("");
  }, []);

  const openEditCriterionModal = useCallback((itemId: string, text: string) => {
    setEditingChecklistItemId(itemId);
    setEditChecklistText(text);
    setEditCriterionModalOpen(true);
    setChecklistModalOpen(false);
    setNewChecklistText("");
  }, []);

  useEffect(() => {
    setSubtaskModalOpen(false);
    resetSubtaskForm();
    setChecklistModalOpen(false);
    setNewChecklistText("");
    setEditCriterionModalOpen(false);
    setEditingChecklistItemId(null);
    setEditChecklistText("");
  }, [taskId, resetSubtaskForm]);

  const taskQuery = useQuery({
    queryKey: taskQueryKeys.detail(taskId),
    queryFn: ({ signal }) => getTask(taskId, { signal }),
    enabled: Boolean(taskId),
  });

  const checklistQuery = useQuery({
    queryKey: taskQueryKeys.checklist(taskId),
    queryFn: ({ signal }) => listChecklist(taskId, { signal }),
    enabled: Boolean(taskId) && taskQuery.isSuccess,
  });

  useEffect(() => {
    if (!subtaskInherit) return;
    setSubtaskChecklistItems([]);
  }, [subtaskInherit]);

  const appendSubtaskChecklistCriterion = useCallback((raw: string) => {
    const t = raw.trim();
    if (!t) return;
    setSubtaskChecklistItems((prev) => [...prev, t]);
  }, []);

  const removeSubtaskChecklistRow = useCallback((index: number) => {
    setSubtaskChecklistItems((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const updateSubtaskChecklistRow = useCallback((index: number, raw: string) => {
    const t = raw.trim();
    if (!t) return;
    setSubtaskChecklistItems((prev) => prev.map((x, i) => (i === index ? t : x)));
  }, []);

  const createSubtaskMutation = useMutation({
    mutationFn: async (input: {
      title: string;
      initial_prompt: string;
      priority: Priority;
      task_type: TaskType;
      checklist_inherit: boolean;
      checklistItems: string[];
    }) => {
      const child = await createTask({
        title: input.title,
        initial_prompt: input.initial_prompt,
        priority: input.priority,
        task_type: input.task_type,
        parent_id: taskId,
        checklist_inherit: input.checklist_inherit,
      });
      if (!input.checklist_inherit) {
        for (const raw of input.checklistItems) {
          const text = raw.trim();
          if (text) {
            await addChecklistItem(child.id, text);
          }
        }
      }
      return child;
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.detail(taskId),
      });
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.listRoot() });
      closeSubtaskModal();
    },
  });

  const submitNewSubtask = useCallback(
    (e: FormEvent) => {
      e.preventDefault();
      if (
        !subtaskTitle.trim() ||
        !subtaskPriority ||
        createSubtaskMutation.isPending
      ) {
        return;
      }
      createSubtaskMutation.mutate({
        title: subtaskTitle.trim(),
        initial_prompt: subtaskPrompt,
        priority: subtaskPriority,
        task_type: subtaskTaskType,
        checklist_inherit: subtaskInherit,
        checklistItems: subtaskInherit ? [] : subtaskChecklistItems,
      });
    },
    [
      subtaskTitle,
      subtaskPrompt,
      subtaskPriority,
      subtaskTaskType,
      subtaskInherit,
      subtaskChecklistItems,
      createSubtaskMutation.mutate,
      createSubtaskMutation.isPending,
    ],
  );

  const addChecklistMutation = useMutation({
    mutationFn: (text: string) => addChecklistItem(taskId, text),
    onSuccess: async () => {
      setNewChecklistText("");
      setChecklistModalOpen(false);
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.checklist(taskId),
      });
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.detail(taskId),
      });
    },
  });

  const submitNewChecklistCriterion = useCallback(
    (e: FormEvent) => {
      e.preventDefault();
      const t = newChecklistText.trim();
      if (!t || addChecklistMutation.isPending) return;
      addChecklistMutation.mutate(t);
    },
    [newChecklistText, addChecklistMutation.mutate, addChecklistMutation.isPending],
  );

  const updateChecklistTextMutation = useMutation({
    mutationFn: (input: { itemId: string; text: string }) =>
      patchChecklistItemText(taskId, input.itemId, input.text),
    onSuccess: async () => {
      closeEditCriterionModal();
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.checklist(taskId),
      });
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.detail(taskId),
      });
    },
  });

  const submitEditChecklistCriterion = useCallback(
    (e: FormEvent) => {
      e.preventDefault();
      const t = editChecklistText.trim();
      const id = editingChecklistItemId;
      if (!t || !id || updateChecklistTextMutation.isPending) return;
      updateChecklistTextMutation.mutate({ itemId: id, text: t });
    },
    [
      editChecklistText,
      editingChecklistItemId,
      updateChecklistTextMutation.mutate,
      updateChecklistTextMutation.isPending,
    ],
  );

  const deleteChecklistMutation = useMutation({
    mutationFn: (itemId: string) => deleteChecklistItem(taskId, itemId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.checklist(taskId),
      });
    },
  });

  const eventsQuery = useQuery({
    queryKey: taskQueryKeys.events(taskId, eventsCursor),
    queryFn: ({ signal }) => {
      const opts: {
        signal?: AbortSignal;
        limit: number;
        beforeSeq?: number;
        afterSeq?: number;
      } = { signal, limit: TASK_EVENTS_PAGE_SIZE };
      if (eventsCursor.k === "before") opts.beforeSeq = eventsCursor.seq;
      if (eventsCursor.k === "after") opts.afterSeq = eventsCursor.seq;
      return listTaskEvents(taskId, opts);
    },
    enabled: Boolean(taskId) && taskQuery.isSuccess,
  });

  useEffect(() => {
    if (!taskId || navigatedAfterDelete.current) return;
    const v = app.deleteMutation.variables;
    if (
      !app.deleteMutation.isSuccess ||
      !v ||
      typeof v !== "object" ||
      !("id" in v) ||
      v.id !== taskId
    ) {
      return;
    }
    navigatedAfterDelete.current = true;
    const parent =
      "parent_id" in v && typeof v.parent_id === "string"
        ? v.parent_id.trim()
        : "";
    if (parent) {
      navigate(`/tasks/${encodeURIComponent(parent)}`, { replace: true });
    } else {
      navigate("/", { replace: true });
    }
  }, [
    taskId,
    app.deleteMutation.isSuccess,
    app.deleteMutation.variables,
    navigate,
  ]);

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
      <section className="panel task-detail-panel">
        <div role="alert">
          <p className="err-inline">
            {taskQuery.error instanceof Error
              ? taskQuery.error.message
              : "Could not load task."}
          </p>
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
  const events = eventsQuery.data?.events ?? [];
  const eventsTotal = eventsQuery.data?.total ?? 0;
  const attention = userAttention(task, {
    approvalPending: eventsQuery.data?.approval_pending ?? false,
  });
  /** API returns newest first when paged. */
  const timelineEvents = events;
  const sanitizedInitialPrompt = sanitizePromptHtml(task.initial_prompt);

  return (
    <section className="panel task-detail-panel task-detail-content--enter">
      <TaskDetailHeader task={task} />

      <TaskDetailAttentionBar
        attention={attention}
        saving={app.saving}
        onEdit={() => app.openEdit(task)}
        onDelete={() => app.requestDelete(task)}
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
      />

      <TaskDetailPromptSection
        initialPrompt={task.initial_prompt}
        sanitizedInitialPrompt={sanitizedInitialPrompt}
      />

      <TaskDetailUpdatesSection
        taskId={taskId}
        eventsQuery={eventsQuery}
        timelineEvents={timelineEvents}
        eventsTotal={eventsTotal}
        onEventsPagerPrev={() => {
          if (events.length === 0) return;
          const maxSeq = Math.max(...events.map((e) => e.seq));
          setEventsCursor({ k: "after", seq: maxSeq });
        }}
        onEventsPagerNext={() => {
          if (events.length === 0) return;
          const minSeq = Math.min(...events.map((e) => e.seq));
          setEventsCursor({ k: "before", seq: minSeq });
        }}
      />
    </section>
  );
}
