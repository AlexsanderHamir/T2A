import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import {
  addChecklistItem,
  createTask as apiCreate,
  getTaskStats,
  deleteTaskDraft as apiDeleteDraft,
  deleteTask as apiDelete,
  evaluateDraftTask as apiEvaluateDraft,
  getTaskDraft as apiGetDraft,
  listTaskDrafts as apiListDrafts,
  listTasks,
  patchTask,
  saveTaskDraft as apiSaveDraft,
} from "../../api";
import type { PendingSubtaskDraft } from "../pendingSubtaskDraft";
import { flattenTaskTree, flattenTaskTreeRoots } from "../flattenTaskTree";
import { TASK_LIST_PAGE_SIZE } from "../paging";
import { taskQueryKeys } from "../queryKeys";
import {
  DEFAULT_NEW_TASK_STATUS,
  DEFAULT_NEW_TASK_TYPE,
  type Priority,
  type PriorityChoice,
  type Status,
  type Task,
  type TaskType,
} from "@/types";
import { useHysteresisBoolean } from "@/lib/useHysteresisBoolean";
import { TASK_DRAFTS, TASK_TIMINGS } from "@/constants/tasks";
import { useTaskEventStream } from "./useTaskEventStream";

/** Background refetches (SSE invalidate, focus) are short; avoid UI flicker. */
const LIST_REFRESH_SHOW_MS = TASK_TIMINGS.listRefreshShowMs;
const LIST_REFRESH_HIDE_MS = TASK_TIMINGS.listRefreshHideMs;
const DRAFT_AUTOSAVE_DEBOUNCE_MS = TASK_TIMINGS.draftAutosaveDebounceMs;

function normalizeDmapCommitLimit(value: string): number {
  const parsed = Number.parseInt(value, 10);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : 1;
}

function buildDmapPrompt(input: {
  commitLimit: string;
  domain: string;
  description: string;
}): string {
  const lines = [
    "DMAP session setup",
    "",
    `- Commits until stoppage: ${normalizeDmapCommitLimit(input.commitLimit)}`,
    `- Domain: ${input.domain.trim() || "unspecified"}`,
  ];
  if (input.description.trim()) {
    lines.push(`- Direction: ${input.description.trim()}`);
  }
  return lines.join("\n");
}

function toApiTaskType(taskType: TaskType): TaskType {
  return taskType === "dmap" ? "general" : taskType;
}

function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

function normalizeDraftPromptForDirty(prompt: string): string {
  const compact = prompt.replace(/[\s\u200B\uFEFF]/g, "").toLowerCase();
  if (
    compact === "" ||
    compact === "<p></p>" ||
    compact === "<p><br></p>" ||
    /^<p>(<br\/?>|&nbsp;|&#160;)*<\/p>$/.test(compact)
  ) {
    return "";
  }
  return prompt;
}

function draftAutosaveSignature(input: {
  id: string;
  name: string;
  title: string;
  prompt: string;
  priority: PriorityChoice;
  taskType: TaskType;
  parentId: string;
  checklistInherit: boolean;
  checklistItems: string[];
  pendingSubtasks: PendingSubtaskDraft[];
  latestEvaluation: {
    overallScore: number;
    overallSummary: string;
    sections: Array<{ key: string; score: number }>;
  } | null;
  dmapConfig: {
    commitLimit: string;
    domain: string;
    description: string;
  };
}): string {
  return JSON.stringify({
    id: input.id,
    name: input.name,
    payload: {
      title: input.title,
      initial_prompt: normalizeDraftPromptForDirty(input.prompt),
      priority: input.priority,
      task_type: input.taskType,
      parent_id: input.parentId,
      checklist_inherit: input.checklistInherit,
      checklist_items: input.checklistItems,
      pending_subtasks: input.pendingSubtasks.map((st) => ({
        title: st.title,
        initial_prompt: st.initial_prompt,
        priority: st.priority,
        task_type: st.task_type,
        checklist_items: st.checklistItems,
        checklist_inherit: st.checklist_inherit,
      })),
      latest_evaluation: input.latestEvaluation,
      dmap_config: input.dmapConfig,
    },
  });
}

export function useTasksApp() {
  const queryClient = useQueryClient();
  const sseLive = useTaskEventStream();

  const [newTitle, setNewTitle] = useState("");
  const [newPrompt, setNewPrompt] = useState("");
  const [newPriority, setNewPriority] = useState<PriorityChoice>("");
  const [newTaskType, setNewTaskType] = useState<TaskType>(DEFAULT_NEW_TASK_TYPE);
  const [newDmapCommitLimit, setNewDmapCommitLimit] = useState<string>(
    TASK_DRAFTS.initialDmapCommitLimit,
  );
  const [newDmapDomain, setNewDmapDomain] = useState("");
  const [newDmapDescription, setNewDmapDescription] = useState("");
  const [newChecklistItems, setNewChecklistItems] = useState<string[]>([]);
  const [newDraftID, setNewDraftID] = useState("");
  const [newDraftName, setNewDraftName] = useState("");
  const [lastDraftSavedAt, setLastDraftSavedAt] = useState<number | null>(null);
  const [draftPickerOpen, setDraftPickerOpen] = useState(false);
  const [latestDraftEvaluation, setLatestDraftEvaluation] = useState<{
    overallScore: number;
    overallSummary: string;
    sections: Array<{ key: string; score: number }>;
  } | null>(null);
  /** Child tasks (full draft) created after the parent task on the home flow. */
  const [pendingSubtasks, setPendingSubtasks] = useState<PendingSubtaskDraft[]>(
    [],
  );
  /** When set, POST /tasks includes `parent_id` (subtask on the home page). */
  const [newParentId, setNewParentId] = useState("");
  const [newChecklistInherit, setNewChecklistInherit] = useState(false);
  const [createModalOpen, setCreateModalOpen] = useState(false);

  const [editing, setEditing] = useState<Task | null>(null);
  const [editTitle, setEditTitle] = useState("");
  const [editPrompt, setEditPrompt] = useState("");
  const [editPriority, setEditPriority] = useState<Priority>("medium");
  const [editTaskType, setEditTaskType] = useState<TaskType>(DEFAULT_NEW_TASK_TYPE);
  const [editStatus, setEditStatus] = useState<Status>(DEFAULT_NEW_TASK_STATUS);
  const [editChecklistInherit, setEditChecklistInherit] = useState(false);

  /** In-app delete confirmation (avoids `window.confirm`, which breaks input focus in some browsers). */
  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    title: string;
    parent_id?: string;
  } | null>(null);

  /** Client-side validation (shown after server errors when applicable). */
  const [editTitleRequiredError, setEditTitleRequiredError] = useState<
    string | null
  >(null);
  const autosaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [taskListPage, setTaskListPage] = useState(0);
  const [draftAutosaveBaseline, setDraftAutosaveBaseline] = useState("");
  const [draftAutosaveBaselineID, setDraftAutosaveBaselineID] = useState("");
  const [createEntryDraftErrorHint, setCreateEntryDraftErrorHint] = useState<
    string | null
  >(null);

  const tasksQuery = useQuery({
    queryKey: taskQueryKeys.list(taskListPage),
    queryFn: ({ signal }) =>
      listTasks(
        TASK_LIST_PAGE_SIZE,
        taskListPage * TASK_LIST_PAGE_SIZE,
        { signal },
      ),
  });
  const draftsQuery = useQuery({
    queryKey: ["task-drafts"],
    queryFn: ({ signal }) =>
      apiListDrafts(TASK_DRAFTS.createModalDraftListLimit, { signal }),
  });
  const taskStatsQuery = useQuery({
    queryKey: ["task-stats"],
    queryFn: async ({ signal }) => {
      try {
        return await getTaskStats({ signal });
      } catch {
        return null;
      }
    },
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
  const parentPickerTasks = useMemo(
    () => flattenTaskTree(rootTaskTrees),
    [rootTaskTrees],
  );

  useEffect(() => {
    if (!newParentId) {
      setNewChecklistInherit(false);
    }
  }, [newParentId]);

  useEffect(() => {
    if (!newChecklistInherit) return;
    setNewChecklistItems([]);
  }, [newChecklistInherit]);

  const resetNewTaskForm = useCallback(() => {
    const generatedID =
      typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
        ? crypto.randomUUID()
        : `draft-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
    setNewTitle("");
    setNewPrompt("");
    setNewPriority("");
    setNewTaskType(DEFAULT_NEW_TASK_TYPE);
    setNewDmapCommitLimit(TASK_DRAFTS.initialDmapCommitLimit);
    setNewDmapDomain("");
    setNewDmapDescription("");
    setNewChecklistItems([]);
    setPendingSubtasks([]);
    setNewParentId("");
    setNewChecklistInherit(false);
    setLatestDraftEvaluation(null);
    setNewDraftID(generatedID);
    setNewDraftName(TASK_DRAFTS.untitledDraftName);
    setLastDraftSavedAt(null);
    setDraftAutosaveBaseline(
      draftAutosaveSignature({
        id: generatedID,
        name: TASK_DRAFTS.untitledDraftName,
        title: "",
        prompt: "",
        priority: "",
        taskType: DEFAULT_NEW_TASK_TYPE,
        parentId: "",
        checklistInherit: false,
        checklistItems: [],
        pendingSubtasks: [],
        latestEvaluation: null,
        dmapConfig: {
          commitLimit: TASK_DRAFTS.initialDmapCommitLimit,
          domain: "",
          description: "",
        },
      }),
    );
    setDraftAutosaveBaselineID(generatedID);
  }, []);

  const closeCreateModal = useCallback(() => {
    setCreateModalOpen(false);
    setDraftPickerOpen(false);
    setCreateEntryDraftErrorHint(null);
    resetNewTaskForm();
  }, [resetNewTaskForm]);

  const openCreateModal = useCallback(() => {
    setCreateEntryDraftErrorHint(null);
    if (draftsQuery.isPending) {
      setDraftPickerOpen(true);
      return;
    }
    if (draftsQuery.isError) {
      setCreateEntryDraftErrorHint(errorMessage(draftsQuery.error));
      resetNewTaskForm();
      setCreateModalOpen(true);
      return;
    }
    const drafts = draftsQuery.data ?? [];
    if (drafts.length > 0) {
      setDraftPickerOpen(true);
      return;
    }
    resetNewTaskForm();
    setCreateModalOpen(true);
  }, [draftsQuery.data, draftsQuery.error, draftsQuery.isError, draftsQuery.isPending, resetNewTaskForm]);

  const loading = tasksQuery.isPending;
  const rawListRefreshing =
    tasksQuery.isFetching && !tasksQuery.isPending;
  const listRefreshing = useHysteresisBoolean(
    rawListRefreshing,
    LIST_REFRESH_SHOW_MS,
    LIST_REFRESH_HIDE_MS,
  );

  const createMutation = useMutation({
    mutationFn: async (input: {
      title: string;
      initial_prompt: string;
      status: Status;
      priority: Priority;
      task_type: TaskType;
      parent_id?: string;
      checklist_inherit: boolean;
      checklistItems: string[];
      pendingSubtasks: PendingSubtaskDraft[];
      draft_id: string;
    }) => {
      const addChecklistItems = async (taskId: string, items: string[]) => {
        const rows = items.map((raw) => raw.trim()).filter(Boolean);
        await Promise.all(rows.map((text) => addChecklistItem(taskId, text)));
      };
      const parentId = input.parent_id?.trim();
      const inherit =
        Boolean(parentId) && input.checklist_inherit === true;
      const task = await apiCreate({
        title: input.title,
        initial_prompt: input.initial_prompt,
        status: input.status,
        priority: input.priority,
        task_type: input.task_type,
        draft_id: input.draft_id,
        ...(parentId ? { parent_id: parentId } : {}),
        ...(inherit ? { checklist_inherit: true } : {}),
      });
      if (!inherit) {
        await addChecklistItems(task.id, input.checklistItems);
      }
      await Promise.all(
        input.pendingSubtasks
          .filter((st) => Boolean(st.title.trim()))
          .map(async (st) => {
            const childInherit = st.checklist_inherit === true;
            const child = await apiCreate({
              title: st.title.trim(),
              initial_prompt: st.initial_prompt,
              status: input.status,
              priority: st.priority,
              task_type: st.task_type,
              parent_id: task.id,
              ...(childInherit ? { checklist_inherit: true } : {}),
            });
            if (!childInherit) {
              await addChecklistItems(child.id, st.checklistItems);
            }
          }),
      );
      return task;
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      await queryClient.invalidateQueries({ queryKey: ["task-stats"] });
      await queryClient.invalidateQueries({ queryKey: ["task-drafts"] });
      closeCreateModal();
    },
  });

  const evaluateDraftMutation = useMutation({
    mutationFn: async (input: {
      id: string;
      title: string;
      initial_prompt: string;
      status: Status;
      priority: Priority;
      task_type: TaskType;
      parent_id?: string;
      checklist_inherit: boolean;
      checklistItems: string[];
    }) => {
      const parentId = input.parent_id?.trim();
      return apiEvaluateDraft({
        id: input.id,
        title: input.title,
        initial_prompt: input.initial_prompt,
        status: input.status,
        priority: input.priority,
        task_type: input.task_type,
        ...(parentId ? { parent_id: parentId } : {}),
        ...(parentId ? { checklist_inherit: input.checklist_inherit } : {}),
        checklist_items: input.checklistItems
          .map((text) => text.trim())
          .filter(Boolean)
          .map((text) => ({ text })),
      });
    },
    onSuccess: (evaluation) => {
      setLatestDraftEvaluation({
        overallScore: evaluation.overall_score,
        overallSummary: evaluation.overall_summary,
        sections: evaluation.sections.map((s) => ({ key: s.key, score: s.score })),
      });
    },
  });

  const patchMutation = useMutation({
    mutationFn: (args: {
      id: string;
      title: string;
      initial_prompt: string;
      status: Status;
      priority: Priority;
      task_type: TaskType;
      checklist_inherit: boolean;
    }) =>
      patchTask(args.id, {
        title: args.title,
        initial_prompt: args.initial_prompt,
        status: args.status,
        priority: args.priority,
        task_type: args.task_type,
        checklist_inherit: args.checklist_inherit,
      }),
    onSuccess: async () => {
      setEditing(null);
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      await queryClient.invalidateQueries({ queryKey: ["task-stats"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (input: { id: string; parent_id?: string }) =>
      apiDelete(input.id),
    onSuccess: async (_, variables) => {
      const deletedId = variables.id;
      setDeleteTarget(null);
      setEditing((prev) => (prev?.id === deletedId ? null : prev));
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.listRoot() });
      await queryClient.invalidateQueries({ queryKey: ["task-stats"] });
    },
  });

  const saveDraftMutation = useMutation({
    mutationFn: (input: {
      id: string;
      name: string;
      payload: {
        title: string;
        initial_prompt: string;
        priority: PriorityChoice;
        task_type: TaskType;
        parent_id: string;
        checklist_inherit: boolean;
        checklist_items: string[];
        pending_subtasks: Array<{
          title: string;
          initial_prompt: string;
          priority: Priority;
          task_type: TaskType;
          checklist_items: string[];
          checklist_inherit: boolean;
        }>;
        latest_evaluation?: {
          overall_score: number;
          overall_summary: string;
          sections: Array<{ key: string; score: number }>;
        };
      };
    }) => apiSaveDraft(input),
    onSuccess: async (saved) => {
      if (saved.id !== newDraftID) {
        setNewDraftID(saved.id);
      }
      setDraftAutosaveBaseline(
        draftAutosaveSignature({
          id: saved.id,
          name: newDraftName.trim() || TASK_DRAFTS.untitledDraftName,
          title: newTitle,
          prompt: newPrompt,
          priority: newPriority,
          taskType: newTaskType,
          parentId: newParentId,
          checklistInherit: newChecklistInherit,
          checklistItems: newChecklistItems,
          pendingSubtasks,
          latestEvaluation: latestDraftEvaluation,
          dmapConfig: {
            commitLimit: newDmapCommitLimit,
            domain: newDmapDomain,
            description: newDmapDescription,
          },
        }),
      );
      setDraftAutosaveBaselineID(saved.id);
      setLastDraftSavedAt(Date.now());
      await queryClient.invalidateQueries({ queryKey: ["task-drafts"] });
    },
  });

  useEffect(() => {
    if (!createModalOpen && !saveDraftMutation.isIdle) {
      saveDraftMutation.reset();
    }
  }, [createModalOpen, saveDraftMutation]);

  const deleteDraftMutation = useMutation({
    mutationFn: (id: string) => apiDeleteDraft(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["task-drafts"] });
    },
  });
  const resumeDraftMutation = useMutation({
    mutationFn: (id: string) => apiGetDraft(id),
  });
  const deleteDraftError = deleteDraftMutation.isError
    ? errorMessage(deleteDraftMutation.error)
    : null;

  const saving =
    createMutation.isPending ||
    evaluateDraftMutation.isPending ||
    patchMutation.isPending ||
    deleteMutation.isPending;

  const draftListLoading = draftsQuery.isPending;
  const draftListError = draftsQuery.isError
    ? errorMessage(draftsQuery.error)
    : null;

  const error = useMemo(() => {
    if (tasksQuery.isError) return errorMessage(tasksQuery.error);
    if (createMutation.isError) return errorMessage(createMutation.error);
    if (evaluateDraftMutation.isError)
      return errorMessage(evaluateDraftMutation.error);
    if (patchMutation.isError) return errorMessage(patchMutation.error);
    if (deleteMutation.isError) return errorMessage(deleteMutation.error);
    return editTitleRequiredError;
  }, [
    tasksQuery.isError,
    tasksQuery.error,
    createMutation.isError,
    createMutation.error,
    evaluateDraftMutation.isError,
    evaluateDraftMutation.error,
    patchMutation.isError,
    patchMutation.error,
    deleteMutation.isError,
    deleteMutation.error,
    editTitleRequiredError,
  ]);

  useEffect(() => {
    if (editTitleRequiredError && editTitle.trim()) {
      setEditTitleRequiredError(null);
    }
  }, [editTitle, editTitleRequiredError]);

  const currentDraftAutosaveSignature = useMemo(
    () =>
      draftAutosaveSignature({
        id: newDraftID,
        name: newDraftName.trim() || TASK_DRAFTS.untitledDraftName,
        title: newTitle,
        prompt: newPrompt,
        priority: newPriority,
        taskType: newTaskType,
        parentId: newParentId,
        checklistInherit: newChecklistInherit,
        checklistItems: newChecklistItems,
        pendingSubtasks,
        latestEvaluation: latestDraftEvaluation,
        dmapConfig: {
          commitLimit: newDmapCommitLimit,
          domain: newDmapDomain,
          description: newDmapDescription,
        },
      }),
    [
      latestDraftEvaluation,
      newDmapCommitLimit,
      newDmapDescription,
      newDmapDomain,
      newChecklistInherit,
      newChecklistItems,
      newDraftID,
      newDraftName,
      newParentId,
      newPriority,
      newPrompt,
      newTaskType,
      newTitle,
      pendingSubtasks,
    ],
  );

  const buildDraftSaveInput = useCallback(() => {
    return {
      id: newDraftID,
      name: newDraftName.trim() || TASK_DRAFTS.untitledDraftName,
      payload: {
        title: newTitle,
        initial_prompt: newPrompt,
        priority: newPriority,
        task_type: newTaskType,
        parent_id: newParentId,
        checklist_inherit: newChecklistInherit,
        checklist_items: newChecklistItems,
        pending_subtasks: pendingSubtasks.map((st) => ({
          title: st.title,
          initial_prompt: st.initial_prompt,
          priority: st.priority,
          task_type: st.task_type,
          checklist_items: st.checklistItems,
          checklist_inherit: st.checklist_inherit,
        })),
        ...(latestDraftEvaluation
          ? {
              latest_evaluation: {
                overall_score: latestDraftEvaluation.overallScore,
                overall_summary: latestDraftEvaluation.overallSummary,
                sections: latestDraftEvaluation.sections,
              },
            }
          : {}),
        ...(newTaskType === "dmap"
          ? {
              dmap_config: {
                commit_limit: normalizeDmapCommitLimit(newDmapCommitLimit),
                domain: newDmapDomain.trim(),
                description: newDmapDescription.trim(),
              },
            }
          : {}),
      },
    };
  }, [
    latestDraftEvaluation,
    newDmapCommitLimit,
    newDmapDescription,
    newDmapDomain,
    newChecklistInherit,
    newChecklistItems,
    newDraftID,
    newDraftName,
    newParentId,
    newPriority,
    newPrompt,
    newTaskType,
    newTitle,
    pendingSubtasks,
  ]);

  const saveDraftNow = useCallback(() => {
    if (!createModalOpen || !newDraftID) return;
    if (draftAutosaveBaselineID !== newDraftID) return;
    if (currentDraftAutosaveSignature === draftAutosaveBaseline) return;
    if (autosaveTimerRef.current) {
      clearTimeout(autosaveTimerRef.current);
      autosaveTimerRef.current = null;
    }
    saveDraftMutation.mutate(buildDraftSaveInput());
  }, [
    buildDraftSaveInput,
    createModalOpen,
    currentDraftAutosaveSignature,
    draftAutosaveBaseline,
    draftAutosaveBaselineID,
    newDraftID,
    saveDraftMutation,
  ]);

  useEffect(() => {
    if (!createModalOpen || !newDraftID) return;
    if (draftAutosaveBaselineID !== newDraftID) return;
    if (currentDraftAutosaveSignature === draftAutosaveBaseline) return;
    autosaveTimerRef.current = setTimeout(() => {
      saveDraftMutation.mutate(buildDraftSaveInput());
      autosaveTimerRef.current = null;
    }, DRAFT_AUTOSAVE_DEBOUNCE_MS);
    return () => {
      if (autosaveTimerRef.current) {
        clearTimeout(autosaveTimerRef.current);
        autosaveTimerRef.current = null;
      }
    };
  }, [
    buildDraftSaveInput,
    createModalOpen,
    currentDraftAutosaveSignature,
    draftAutosaveBaseline,
    draftAutosaveBaselineID,
    newDraftID,
    saveDraftMutation,
  ]);

  const draftSaveLabel = useMemo(() => {
    if (!createModalOpen) return null;
    if (saveDraftMutation.isPending) return "Saving draft…";
    if (saveDraftMutation.isError) return "Draft autosave failed. You can still create the task.";
    if (lastDraftSavedAt == null) return null;
    return "Draft saved";
  }, [
    createModalOpen,
    lastDraftSavedAt,
    saveDraftMutation.isError,
    saveDraftMutation.isPending,
  ]);
  const draftSaveError = createModalOpen && saveDraftMutation.isError;

  function evaluateDraftBeforeCreate() {
    const parentId = newParentId.trim();
    if (!newTitle.trim() || !newPriority) return;
    const dmapDomain = newDmapDomain.trim();
    if (newTaskType === "dmap" && !dmapDomain) return;
    evaluateDraftMutation.mutate({
      id: newDraftID,
      title: newTitle.trim(),
      initial_prompt:
        newTaskType === "dmap"
          ? buildDmapPrompt({
              commitLimit: newDmapCommitLimit,
              domain: dmapDomain,
              description: newDmapDescription,
            })
          : newPrompt,
      status: DEFAULT_NEW_TASK_STATUS,
      priority: newPriority,
      task_type: toApiTaskType(newTaskType),
      ...(parentId ? { parent_id: parentId } : {}),
      checklist_inherit: Boolean(parentId) && newChecklistInherit,
      checklistItems: newChecklistItems,
    });
  }

  async function submitCreate(e: FormEvent) {
    e.preventDefault();
    if (!newTitle.trim() || !newPriority) return;
    const parentId = newParentId.trim();
    const dmapDomain = newDmapDomain.trim();
    if (newTaskType === "dmap" && !dmapDomain) return;
    createMutation.mutate({
      title: newTitle.trim(),
      initial_prompt:
        newTaskType === "dmap"
          ? buildDmapPrompt({
              commitLimit: newDmapCommitLimit,
              domain: dmapDomain,
              description: newDmapDescription,
            })
          : newPrompt,
      status: DEFAULT_NEW_TASK_STATUS,
      priority: newPriority,
      task_type: toApiTaskType(newTaskType),
      draft_id: newDraftID,
      ...(parentId ? { parent_id: parentId } : {}),
      checklist_inherit: Boolean(parentId) && newChecklistInherit,
      checklistItems: newChecklistItems,
      pendingSubtasks,
    });
  }

  async function startFreshDraft() {
    resetNewTaskForm();
    setDraftPickerOpen(false);
    setCreateModalOpen(true);
  }

  async function resumeDraftByID(id: string) {
    const draft = await resumeDraftMutation.mutateAsync(id);
    const pendingSubtasks = (draft.payload.pending_subtasks ?? []).map((st) => ({
      title: st.title,
      initial_prompt: st.initial_prompt,
      priority: st.priority,
      task_type: st.task_type,
      checklistItems: st.checklist_items,
      checklist_inherit: st.checklist_inherit,
    }));
    const latestEvaluation = draft.payload.latest_evaluation
      ? {
          overallScore: draft.payload.latest_evaluation.overall_score,
          overallSummary: draft.payload.latest_evaluation.overall_summary,
          sections: draft.payload.latest_evaluation.sections,
        }
      : null;
    setNewDraftID(draft.id);
    setNewDraftName(draft.name);
    setNewTitle(draft.payload.title ?? "");
    setNewPrompt(draft.payload.initial_prompt ?? "");
    setNewPriority(draft.payload.priority ?? "");
    setNewTaskType(draft.payload.task_type ?? DEFAULT_NEW_TASK_TYPE);
    setNewDmapCommitLimit(
      String(
        draft.payload.dmap_config?.commit_limit ??
          Number(TASK_DRAFTS.initialDmapCommitLimit),
      ),
    );
    setNewDmapDomain(draft.payload.dmap_config?.domain ?? "");
    setNewDmapDescription(draft.payload.dmap_config?.description ?? "");
    setNewParentId(draft.payload.parent_id ?? "");
    setNewChecklistInherit(draft.payload.checklist_inherit === true);
    setNewChecklistItems(draft.payload.checklist_items ?? []);
    setPendingSubtasks(pendingSubtasks);
    setLatestDraftEvaluation(latestEvaluation);
    setDraftAutosaveBaseline(
      draftAutosaveSignature({
        id: draft.id,
        name: draft.name,
        title: draft.payload.title ?? "",
        prompt: draft.payload.initial_prompt ?? "",
        priority: draft.payload.priority ?? "",
        taskType: draft.payload.task_type ?? DEFAULT_NEW_TASK_TYPE,
        parentId: draft.payload.parent_id ?? "",
        checklistInherit: draft.payload.checklist_inherit === true,
        checklistItems: draft.payload.checklist_items ?? [],
        pendingSubtasks,
        latestEvaluation,
        dmapConfig: {
          commitLimit: String(
            draft.payload.dmap_config?.commit_limit ??
              Number(TASK_DRAFTS.initialDmapCommitLimit),
          ),
          domain: draft.payload.dmap_config?.domain ?? "",
          description: draft.payload.dmap_config?.description ?? "",
        },
      }),
    );
    setDraftAutosaveBaselineID(draft.id);
    setDraftPickerOpen(false);
    setCreateModalOpen(true);
  }

  async function deleteDraftByID(id: string) {
    await deleteDraftMutation.mutateAsync(id);
  }

  const appendNewChecklistCriterion = useCallback((raw: string) => {
    const t = raw.trim();
    if (!t) return;
    setNewChecklistItems((prev) => [...prev, t]);
  }, []);

  const removeNewChecklistRow = useCallback((index: number) => {
    setNewChecklistItems((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const updateNewChecklistRow = useCallback((index: number, raw: string) => {
    const t = raw.trim();
    if (!t) return;
    setNewChecklistItems((prev) => prev.map((x, i) => (i === index ? t : x)));
  }, []);

  const addPendingSubtask = useCallback((d: PendingSubtaskDraft) => {
    setPendingSubtasks((prev) => [...prev, d]);
  }, []);

  const updatePendingSubtask = useCallback(
    (index: number, d: PendingSubtaskDraft) => {
      setPendingSubtasks((prev) =>
        prev.map((x, i) => (i === index ? d : x)),
      );
    },
    [],
  );

  const removePendingSubtask = useCallback((index: number) => {
    setPendingSubtasks((prev) => prev.filter((_, i) => i !== index));
  }, []);

  function openEdit(t: Task) {
    setEditing(t);
    setEditTitle(t.title);
    setEditPrompt(t.initial_prompt);
    setEditPriority(t.priority);
    setEditTaskType(t.task_type ?? DEFAULT_NEW_TASK_TYPE);
    setEditStatus(t.status);
    setEditChecklistInherit(t.checklist_inherit === true);
    setEditTitleRequiredError(null);
  }

  function closeEdit() {
    setEditing(null);
    setEditTitleRequiredError(null);
  }

  function submitEdit(e: FormEvent) {
    e.preventDefault();
    if (!editing) return;
    if (!editTitle.trim()) {
      setEditTitleRequiredError("Title is required.");
      return;
    }
    setEditTitleRequiredError(null);
    patchMutation.mutate({
      id: editing.id,
      title: editTitle.trim(),
      initial_prompt: editPrompt,
      status: editStatus,
      priority: editPriority,
      task_type: editTaskType,
      checklist_inherit: editChecklistInherit,
    });
  }

  const requestDelete = useCallback((t: Task) => {
    const pid = t.parent_id?.trim();
    setDeleteTarget({
      id: t.id,
      title: t.title,
      ...(pid ? { parent_id: pid } : {}),
    });
  }, []);

  const cancelDelete = useCallback(() => {
    setDeleteTarget(null);
  }, []);

  function confirmDelete() {
    if (!deleteTarget) return;
    deleteMutation.mutate({
      id: deleteTarget.id,
      ...(deleteTarget.parent_id
        ? { parent_id: deleteTarget.parent_id }
        : {}),
    });
  }

  const createPending = createMutation.isPending;
  const evaluatePending = evaluateDraftMutation.isPending;
  const patchPending = patchMutation.isPending;
  const deletePending = deleteMutation.isPending;
  const draftSavePending = saveDraftMutation.isPending;

  useEffect(() => {
    if (!tasksQuery.isPending && rootTaskTrees.length === 0 && taskListPage > 0) {
      setTaskListPage(0);
    }
  }, [tasksQuery.isPending, rootTaskTrees.length, taskListPage]);

  const hasNextTaskPage = rootTaskTrees.length === TASK_LIST_PAGE_SIZE;
  const hasPrevTaskPage = taskListPage > 0;
  const retryDraftList = useCallback(async () => {
    await draftsQuery.refetch();
  }, [draftsQuery]);
  const retryCreateEntryDraftLoad = useCallback(async () => {
    const refreshed = await draftsQuery.refetch();
    if (refreshed.isError) {
      setCreateEntryDraftErrorHint(errorMessage(refreshed.error));
      return;
    }
    setCreateEntryDraftErrorHint(null);
    const drafts = refreshed.data ?? [];
    if (drafts.length > 0) {
      setCreateModalOpen(false);
      setDraftPickerOpen(true);
    }
  }, [draftsQuery]);

  return {
    tasks,
    parentPickerTasks,
    rootTasksOnPage: rootTaskTrees.length,
    loading,
    listRefreshing,
    saving,
    draftSavePending,
    draftSaveLabel,
    draftSaveError,
    createPending,
    evaluatePending,
    patchPending,
    deletePending,
    deleteMutation,
    error,
    sseLive,
    taskStats: taskStatsQuery.data,
    draftPickerOpen,
    setDraftPickerOpen,
    taskDrafts: draftsQuery.data ?? [],
    draftListLoading,
    draftListError,
    createEntryDraftErrorHint,
    retryDraftList,
    retryCreateEntryDraftLoad,
    deleteDraftPending: deleteDraftMutation.isPending,
    deleteDraftError,
    resumeDraftPending: resumeDraftMutation.isPending,
    resumeDraftError: resumeDraftMutation.isError
      ? errorMessage(resumeDraftMutation.error)
      : null,
    clearResumeDraftError: resumeDraftMutation.reset,
    newDraftName,
    setNewDraftName,
    newTitle,
    setNewTitle,
    newPrompt,
    setNewPrompt,
    newPriority,
    newTaskType,
    newDmapCommitLimit,
    setNewDmapCommitLimit,
    newDmapDomain,
    setNewDmapDomain,
    newDmapDescription,
    setNewDmapDescription,
    setNewPriority,
    setNewTaskType,
    newChecklistItems,
    latestDraftEvaluation,
    pendingSubtasks,
    addPendingSubtask,
    updatePendingSubtask,
    removePendingSubtask,
    newParentId,
    setNewParentId,
    newChecklistInherit,
    setNewChecklistInherit,
    appendNewChecklistCriterion,
    updateNewChecklistRow,
    removeNewChecklistRow,
    submitCreate,
    evaluateDraftBeforeCreate,
    startFreshDraft,
    saveDraftNow,
    resumeDraftByID,
    deleteDraftByID,
    createModalOpen,
    openCreateModal,
    closeCreateModal,
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
    openEdit,
    closeEdit,
    submitEdit,
    deleteTarget,
    requestDelete,
    cancelDelete,
    confirmDelete,
    taskListPage,
    setTaskListPage,
    resetTaskListPage,
    taskListPageSize: TASK_LIST_PAGE_SIZE,
    hasNextTaskPage,
    hasPrevTaskPage,
  };
}
