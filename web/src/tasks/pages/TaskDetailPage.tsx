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
  patchChecklistItemDone,
} from "@/api";
import type { Priority, Task } from "@/types";
import { SubtaskCreateModal } from "../components/SubtaskCreateModal";
import { TaskPager } from "../components/TaskPager";
import { promptHasVisibleContent } from "../promptFormat";
import { TASK_EVENTS_PAGE_SIZE } from "../paging";
import { userAttention } from "../taskAttention";
import { statusNeedsUserInput } from "../taskStatusNeedsUser";
import { TaskUpdatesTimeline } from "../components/TaskUpdatesTimeline";
import { priorityPillClass, statusPillClass } from "../taskPillClasses";
import { taskQueryKeys, type TaskEventsCursorKey } from "../queryKeys";
import { useTasksApp } from "../hooks/useTasksApp";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

function SubtaskTree({
  nodes,
  nested = false,
}: {
  nodes: Task[];
  nested?: boolean;
}) {
  if (!nodes.length) {
    if (nested) return null;
    return (
      <p className="muted" id="task-subtasks-empty">
        No subtasks yet.
      </p>
    );
  }
  return (
    <ul
      className={nested ? "task-subtasks-tree nested" : "task-subtasks-tree"}
      aria-labelledby={nested ? undefined : "task-subtasks-heading"}
    >
      {nodes.map((c) => (
        <li key={c.id}>
          <Link to={`/tasks/${c.id}`}>{c.title}</Link>
          <span className="muted"> ({c.status})</span>
          <SubtaskTree nodes={c.children ?? []} nested />
        </li>
      ))}
    </ul>
  );
}

export function TaskDetailPage({ app }: Props) {
  const { taskId = "" } = useParams<{ taskId: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const navigatedAfterDelete = useRef(false);
  const [subtaskTitle, setSubtaskTitle] = useState("");
  const [subtaskPrompt, setSubtaskPrompt] = useState("");
  const [subtaskPriority, setSubtaskPriority] = useState<Priority>("medium");
  const [subtaskChecklistDraft, setSubtaskChecklistDraft] = useState("");
  const [subtaskChecklistItems, setSubtaskChecklistItems] = useState<string[]>(
    [],
  );
  const [subtaskInherit, setSubtaskInherit] = useState(false);
  const [subtaskModalOpen, setSubtaskModalOpen] = useState(false);
  const [newChecklistText, setNewChecklistText] = useState("");

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
    setSubtaskPriority("medium");
    setSubtaskChecklistDraft("");
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

  useEffect(() => {
    setSubtaskModalOpen(false);
    resetSubtaskForm();
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
    setSubtaskChecklistDraft("");
    setSubtaskChecklistItems([]);
  }, [subtaskInherit]);

  const addSubtaskChecklistRow = useCallback(() => {
    const t = subtaskChecklistDraft.trim();
    if (!t) return;
    setSubtaskChecklistItems((prev) => [...prev, t]);
    setSubtaskChecklistDraft("");
  }, [subtaskChecklistDraft]);

  const removeSubtaskChecklistRow = useCallback((index: number) => {
    setSubtaskChecklistItems((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const createSubtaskMutation = useMutation({
    mutationFn: async (input: {
      title: string;
      initial_prompt: string;
      priority: Priority;
      checklist_inherit: boolean;
      checklistItems: string[];
    }) => {
      const child = await createTask({
        title: input.title,
        initial_prompt: input.initial_prompt,
        priority: input.priority,
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
      if (!subtaskTitle.trim() || createSubtaskMutation.isPending) return;
      createSubtaskMutation.mutate({
        title: subtaskTitle.trim(),
        initial_prompt: subtaskPrompt,
        priority: subtaskPriority,
        checklist_inherit: subtaskInherit,
        checklistItems: subtaskInherit ? [] : subtaskChecklistItems,
      });
    },
    [
      subtaskTitle,
      subtaskPrompt,
      subtaskPriority,
      subtaskInherit,
      subtaskChecklistItems,
      createSubtaskMutation.mutate,
      createSubtaskMutation.isPending,
    ],
  );

  const toggleChecklistMutation = useMutation({
    mutationFn: (args: { itemId: string; done: boolean }) =>
      patchChecklistItemDone(taskId, args.itemId, args.done),
    onSuccess: (data) => {
      queryClient.setQueryData(taskQueryKeys.checklist(taskId), data);
      void queryClient.invalidateQueries({
        queryKey: taskQueryKeys.listRoot(),
      });
    },
  });

  const addChecklistMutation = useMutation({
    mutationFn: (text: string) => addChecklistItem(taskId, text),
    onSuccess: async () => {
      setNewChecklistText("");
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.checklist(taskId),
      });
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.detail(taskId),
      });
    },
  });

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
    if (
      app.deleteMutation.isSuccess &&
      app.deleteMutation.variables === taskId
    ) {
      navigatedAfterDelete.current = true;
      navigate("/", { replace: true });
    }
  }, [
    taskId,
    app.deleteMutation.isSuccess,
    app.deleteMutation.variables,
    navigate,
  ]);

  if (!taskId) {
    return (
      <p className="muted" role="status">
        Missing task id.
      </p>
    );
  }

  if (taskQuery.isPending) {
    return (
      <p className="muted task-list-phase-msg" role="status">
        Loading task…
      </p>
    );
  }

  if (taskQuery.isError) {
    return (
      <section className="panel task-detail-panel">
        <p className="err-inline" role="alert">
          {taskQuery.error instanceof Error
            ? taskQuery.error.message
            : "Could not load task."}
        </p>
        <p>
          <Link to="/">← Back to tasks</Link>
        </p>
      </section>
    );
  }

  const task = taskQuery.data;
  const events = eventsQuery.data?.events ?? [];
  const eventsTotal = eventsQuery.data?.total ?? 0;
  const attention = userAttention(task, {
    approvalPending: eventsQuery.data?.approval_pending ?? false,
  });
  /** API returns newest first when paged. */
  const timelineEvents = events;

  return (
    <section className="panel task-detail-panel">
      <nav className="task-detail-nav" aria-label="Task navigation">
        <Link to="/" className="task-detail-back">
          ← All tasks
        </Link>
      </nav>

      <header className="task-detail-header">
        <h2 className="task-detail-title">{task.title}</h2>
        <p
          className="task-event-detail-stance"
          role="status"
          data-stance={
            statusNeedsUserInput(task.status) ? "needs-user" : "informational"
          }
        >
          {statusNeedsUserInput(task.status)
            ? "Agent needs input"
            : "Informational"}
        </p>
        <div className="task-detail-meta">
          <span
            className={statusPillClass(task.status)}
            data-needs-user={
              statusNeedsUserInput(task.status) ? "true" : undefined
            }
          >
            {task.status}
          </span>
          <span className={priorityPillClass(task.priority)}>
            {task.priority}
          </span>
        </div>
      </header>

      {attention.show ? (
        <div
          className="task-detail-attention"
          role="status"
          aria-live="polite"
        >
          <strong>{attention.headline}</strong>
          <p>{attention.body}</p>
        </div>
      ) : (
        <div className="task-detail-ok" role="status">
          <strong>No agent is waiting on you for this task right now.</strong>
          <p className="muted">
            Follow the timeline for updates. We highlight when an agent needs
            input or approval.
          </p>
        </div>
      )}

      <div className="task-detail-actions">
        <button
          type="button"
          className="task-detail-btn-edit"
          onClick={() => app.openEdit(task)}
          disabled={app.saving}
        >
          Edit task
        </button>
        <button
          type="button"
          className="task-detail-btn-delete"
          onClick={() => app.requestDelete(task)}
          disabled={app.saving}
        >
          Delete
        </button>
      </div>

      <div className="task-detail-section" id="task-detail-subtasks">
        <div className="task-detail-subtasks-head">
          <h3 className="task-detail-section-heading" id="task-subtasks-heading">
            Subtasks
          </h3>
          <button
            type="button"
            className="task-detail-add-subtask-btn"
            onClick={openSubtaskModal}
            disabled={app.saving}
          >
            Add subtask
          </button>
        </div>
        <SubtaskTree nodes={task.children ?? []} />
        {subtaskModalOpen ? (
          <SubtaskCreateModal
            taskId={taskId}
            pending={createSubtaskMutation.isPending}
            saving={app.saving}
            onClose={closeSubtaskModal}
            title={subtaskTitle}
            prompt={subtaskPrompt}
            priority={subtaskPriority}
            checklistDraft={subtaskChecklistDraft}
            checklistItems={subtaskChecklistItems}
            checklistInherit={subtaskInherit}
            onTitleChange={setSubtaskTitle}
            onPromptChange={setSubtaskPrompt}
            onPriorityChange={setSubtaskPriority}
            onChecklistDraftChange={setSubtaskChecklistDraft}
            onAddChecklistRow={addSubtaskChecklistRow}
            onRemoveChecklistRow={removeSubtaskChecklistRow}
            onChecklistInheritChange={setSubtaskInherit}
            onSubmit={submitNewSubtask}
          />
        ) : null}
      </div>

      <div className="task-detail-section" id="task-detail-checklist">
        <h3 className="task-detail-section-heading" id="task-checklist-heading">
          Done criteria
        </h3>
        {!task.checklist_inherit ? (
          <p className="task-detail-section-hint">
            Every item must be complete before you can mark this task done.
          </p>
        ) : null}
        {task.checklist_inherit ? (
          <p className="muted" role="status">
            This task inherits checklist items from an ancestor. Complete them
            here for this task; definitions are owned upstream.
          </p>
        ) : null}
        {checklistQuery.isError ? (
          <p className="err-inline" role="alert">
            {checklistQuery.error instanceof Error
              ? checklistQuery.error.message
              : "Could not load checklist."}
          </p>
        ) : checklistQuery.isPending ? (
          <p className="muted">Loading checklist…</p>
        ) : (
          <ul
            className="task-checklist-list"
            aria-labelledby="task-checklist-heading"
          >
            {checklistQuery.data?.items.map((item) => (
              <li key={item.id} className="task-checklist-row">
                <label className="task-checklist-label">
                  <input
                    type="checkbox"
                    checked={item.done}
                    disabled={toggleChecklistMutation.isPending}
                    onChange={(ev) => {
                      toggleChecklistMutation.mutate({
                        itemId: item.id,
                        done: ev.target.checked,
                      });
                    }}
                  />
                  <span>{item.text}</span>
                </label>
                {!task.checklist_inherit ? (
                  <button
                    type="button"
                    className="task-detail-checklist-remove"
                    disabled={deleteChecklistMutation.isPending}
                    onClick={() => deleteChecklistMutation.mutate(item.id)}
                  >
                    Remove
                  </button>
                ) : null}
              </li>
            ))}
          </ul>
        )}
        {!task.checklist_inherit ? (
          <form
            className="task-checklist-add-form"
            onSubmit={(e: FormEvent) => {
              e.preventDefault();
              const t = newChecklistText.trim();
              if (!t || addChecklistMutation.isPending) return;
              addChecklistMutation.mutate(t);
            }}
          >
            <div className="field grow">
              <label htmlFor="task-new-checklist">Add criterion</label>
              <input
                id="task-new-checklist"
                value={newChecklistText}
                onChange={(ev) => setNewChecklistText(ev.target.value)}
                placeholder="Describe what must be true to mark done"
                disabled={addChecklistMutation.isPending}
              />
            </div>
            <button
              type="submit"
              className="task-detail-checklist-add-btn"
              disabled={!newChecklistText.trim() || addChecklistMutation.isPending}
            >
              Add
            </button>
          </form>
        ) : null}
      </div>

      <div className="task-detail-section task-detail-prompt">
        <h3 className="task-detail-section-heading" id="task-detail-prompt-heading">
          Initial prompt
        </h3>
        {!promptHasVisibleContent(task.initial_prompt) ? (
          <p
            className="muted task-detail-prompt-empty"
            aria-labelledby="task-detail-prompt-heading"
          >
            —
          </p>
        ) : (
          <details className="task-detail-prompt-details">
            <summary className="task-detail-prompt-summary">
              <span className="task-detail-prompt-summary-open-label">
                Show full initial prompt
              </span>
              <span className="task-detail-prompt-summary-close-label">
                Hide initial prompt
              </span>
              <span
                className="task-detail-prompt-summary-chevron"
                aria-hidden="true"
              >
                ▾
              </span>
            </summary>
            <div
              className="task-detail-prompt-body"
              dangerouslySetInnerHTML={{ __html: task.initial_prompt }}
            />
          </details>
        )}
      </div>

      <TaskUpdatesTimeline
        isPending={eventsQuery.isPending}
        isError={eventsQuery.isError}
        error={eventsQuery.error}
        timelineEvents={timelineEvents}
        isEmpty={
          !eventsQuery.isPending &&
          !eventsQuery.isError &&
          events.length === 0 &&
          eventsTotal === 0
        }
        taskIdForLinks={taskId}
      />

      {!eventsQuery.isPending &&
      !eventsQuery.isError &&
      eventsTotal > 0 &&
      (eventsQuery.data?.has_more_newer ||
        eventsQuery.data?.has_more_older) ? (
        <TaskPager
          navLabel="Update history pages"
          summary={
            eventsQuery.data?.range_start !== undefined &&
            eventsQuery.data?.range_end !== undefined
              ? `${eventsQuery.data.range_start}–${eventsQuery.data.range_end} of ${eventsTotal}`
              : events.length === 0
                ? "No rows on this page"
                : "—"
          }
          onPrev={() => {
            if (events.length === 0) return;
            const maxSeq = Math.max(...events.map((e) => e.seq));
            setEventsCursor({ k: "after", seq: maxSeq });
          }}
          onNext={() => {
            if (events.length === 0) return;
            const minSeq = Math.min(...events.map((e) => e.seq));
            setEventsCursor({ k: "before", seq: minSeq });
          }}
          disablePrev={eventsQuery.data?.has_more_newer !== true}
          disableNext={eventsQuery.data?.has_more_older !== true}
        />
      ) : null}
    </section>
  );
}
