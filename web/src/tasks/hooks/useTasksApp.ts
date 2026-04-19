import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import {
  addChecklistItem,
  createTask as apiCreate,
  getTaskStats,
  deleteTaskDraft as apiDeleteDraft,
  evaluateDraftTask as apiEvaluateDraft,
  getTaskDraft as apiGetDraft,
  listTaskDrafts as apiListDrafts,
  listTasks,
  saveTaskDraft as apiSaveDraft,
} from "../../api";
import {
  flattenTaskTree,
  flattenTaskTreeRoots,
  type PendingSubtaskDraft,
} from "../task-tree";
import { TASK_LIST_PAGE_SIZE } from "../task-paging";
import { taskQueryKeys } from "../task-query";
import {
  buildDmapPrompt,
  draftAutosaveSignature,
  normalizeDmapCommitLimit,
  toApiTaskType,
} from "../task-drafts";
import { errorMessage } from "@/lib/errorMessage";
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
import { useTaskDeleteFlow } from "./useTaskDeleteFlow";
import { useTaskPatchFlow } from "./useTaskPatchFlow";

/** Background refetches (SSE invalidate, focus) are short; avoid UI flicker. */
const LIST_REFRESH_SHOW_MS = TASK_TIMINGS.listRefreshShowMs;
const LIST_REFRESH_HIDE_MS = TASK_TIMINGS.listRefreshHideMs;
const DRAFT_AUTOSAVE_DEBOUNCE_MS = TASK_TIMINGS.draftAutosaveDebounceMs;

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
  const [newDraftID, setNewDraftIDState] = useState("");
  /**
   * Mirror of `newDraftID` for use inside async mutation callbacks. Reading
   * `newDraftID` directly inside `onSuccess` would capture the value at the
   * time the mutation was created, not the value when it resolved — which is
   * the entire bug we guard against (a stale evaluation result for draft A
   * clobbering the form after the user has already switched to draft B).
   *
   * The ref is updated *synchronously* alongside the state setter (see
   * `setNewDraftID` below) so a mutation that resolves in the same microtask
   * cannot observe a stale ref before a `useEffect` would have caught up.
   */
  const newDraftIDRef = useRef("");
  const setNewDraftID = useCallback((id: string) => {
    newDraftIDRef.current = id;
    setNewDraftIDState(id);
  }, []);
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

  const {
    deleteTarget,
    requestDelete,
    cancelDelete,
    confirmDelete,
    deletePending,
    deleteError,
    deleteSuccess,
    deleteVariables,
  } = useTaskDeleteFlow({
    onDeleted: (deletedId) => {
      setEditing((prev) => (prev?.id === deletedId ? null : prev));
    },
  });

  /** Client-side validation (shown after server errors when applicable). */
  const [editTitleRequiredError, setEditTitleRequiredError] = useState<
    string | null
  >(null);
  const autosaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  /**
   * Tracks the most recent `resumeDraftByID(id)` request so out-of-order
   * resolutions cannot stamp the form with a stale draft.
   *
   * `resumeDraftByID` issues `apiGetDraft(id)` via `mutateAsync` and then
   * unconditionally writes every form field from the resolved payload.
   * `useMutation` allows multiple concurrent in-flight calls, so if the
   * user clicks draft B (slow GET), then quickly clicks draft C before
   * B resolves, both requests are in flight. Whichever lands first runs
   * its post-await branch; if B is slower its resolution would still
   * stamp B's fields *after* C already populated the form, silently
   * reverting the user to the draft they just navigated away from.
   *
   * The ref is set *synchronously* before the await, so we can compare
   * `requestedResumeRef.current === id` after the await to detect that
   * a newer request has superseded this one. Same shape as
   * `newDraftIDRef` for the create / save / evaluate races (see #20-#25
   * in `.agent/frontend-improvement-agent.log`).
   */
  const requestedResumeRef = useRef<string | null>(null);

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
    // Resetting the form to a fresh draft also supersedes any in-flight
    // `resumeDraftByID` request (e.g. the user clicked draft B in the
    // picker, then hit "Start fresh" or closed the modal before B's
    // GET resolved). Clearing the ref here is the single
    // upstream-of-everything clear since `resetNewTaskForm` is called
    // from `closeCreateModal`, `startFreshDraft`, and the
    // openCreateModal recovery branches; without it, B's late
    // resolution would happily stamp the now-fresh form with B's
    // payload.
    requestedResumeRef.current = null;
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
  }, [setNewDraftID]);

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
    onSuccess: async (_task, variables) => {
      // Server-truth invalidations always fire: the new task is real
      // regardless of which draft the user is now editing in the create
      // modal, so list / stats / drafts caches must reflect it.
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      await queryClient.invalidateQueries({ queryKey: ["task-stats"] });
      await queryClient.invalidateQueries({ queryKey: ["task-drafts"] });
      // Defensive id-aware guard. Today the create modal's `Modal
      // busy={pending}` lock blocks ESC / backdrop close while
      // `createMutation.isPending`, so the user *cannot* switch drafts
      // mid-create and this branch is effectively unconditional. But
      // the moment that lock loosens (or somebody adds an out-of-modal
      // "submit and continue editing" path), an unconditional
      // `closeCreateModal()` here would slam shut a draft the user has
      // since switched to. Read from `newDraftIDRef` so a resolution
      // that lands in the same microtask as a draft switch still sees
      // the freshest id (same shape as `evaluateDraftMutation` /
      // `saveDraftMutation` in this file).
      if (newDraftIDRef.current === variables.draft_id) {
        closeCreateModal();
      }
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
    onSuccess: (evaluation, variables) => {
      // Stale-resolution guard: if the user has already switched to a
      // different draft (or closed the modal, which generates a fresh draft
      // id), drop this evaluation on the floor instead of pasting it onto
      // the wrong form. Same shape as the id-aware fix in `useTaskPatchFlow`
      // and `useTaskDeleteFlow`.
      if (newDraftIDRef.current !== variables.id) return;
      setLatestDraftEvaluation({
        overallScore: evaluation.overall_score,
        overallSummary: evaluation.overall_summary,
        sections: evaluation.sections.map((s) => ({ key: s.key, score: s.score })),
      });
    },
  });

  const {
    patchTask: runPatch,
    patchPending,
    patchError,
  } = useTaskPatchFlow({
    onPatched: (patchedId) => {
      setEditing((prev) => (prev?.id === patchedId ? null : prev));
    },
  });

  const saveDraftMutation = useMutation({
    /**
     * `signature` is the autosave-baseline snapshot of the form *as sent*
     * (computed by `currentDraftAutosaveSignature` at the time `mutate()`
     * is called). It is NOT forwarded to the API - it is preserved in
     * `variables` so `onSuccess` can stamp the baseline with what was
     * actually persisted, not with whatever the form has drifted to by
     * the time the network round-trip resolves. See onSuccess for the
     * race this guards against.
     */
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
      signature: string;
    }) =>
      // `signature` is preserved on `variables` for `onSuccess`'s baseline
      // stamping but is not part of the server contract; `apiSaveDraft`
      // builds its request body from `id`/`name`/`payload` only and ignores
      // any extra fields, so passing the wider input through is safe.
      apiSaveDraft(input),
    onSuccess: async (saved, variables) => {
      // Stale-resolution guard. If the user switched drafts mid-flight - via
      // `resumeDraftByID` (draft picker), `startFreshDraft`, or
      // `closeCreateModal` (which generates a brand-new draft id in
      // `resetNewTaskForm`) - a late save for the *previous* draft must not
      // stamp the autosave baseline (or the "Draft saved" label) onto the
      // *current* draft. Same id-aware compare pattern as
      // `evaluateDraftMutation`, `useTaskPatchFlow`, `useTaskDeleteFlow`.
      //
      // The persisted server-side draft is still a real fact, so we always
      // invalidate the picker list. We just refuse to touch any UI form
      // state (which is now showing a different draft).
      //
      // Read from `newDraftIDRef` instead of the closure-captured
      // `newDraftID`: the ref is updated synchronously by `setNewDraftID`
      // and `resumeDraftByID`, so it reflects the freshest id even when
      // this `onSuccess` resolves in the same microtask as the switch.
      if (newDraftIDRef.current !== saved.id) {
        await queryClient.invalidateQueries({ queryKey: ["task-drafts"] });
        return;
      }
      if (saved.id !== newDraftID) {
        setNewDraftID(saved.id);
      }
      // Use the signature snapshot captured at `mutate()` time (the form
      // *as sent* to the server) instead of recomputing from live form
      // state here. Without this, edits made while the save is in flight
      // would be folded into the baseline at resolve time, so the next
      // `currentDraftAutosaveSignature === draftAutosaveBaseline` short-
      // circuit would skip autosave even though the server still has the
      // older payload - silently dropping every keystroke between mutate
      // dispatch and resolve.
      setDraftAutosaveBaseline(variables.signature);
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

  // Clear stale create / evaluate errors when the modal closes so the
  // user does not see a leftover banner from the previous session the
  // next time they reopen the modal. Without this, a failed submit
  // followed by close + reopen would render the old `.err` callout
  // before the user has even interacted with the new draft. Mirrors
  // the `saveDraftMutation.reset()` lifecycle above. Both mutations
  // are independent: only reset whichever has actually settled into
  // an error / success state (skip when `isIdle`).
  useEffect(() => {
    if (!createModalOpen) {
      if (!createMutation.isIdle) createMutation.reset();
      if (!evaluateDraftMutation.isIdle) evaluateDraftMutation.reset();
    }
  }, [createModalOpen, createMutation, evaluateDraftMutation]);

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
    patchPending ||
    deletePending;

  const draftListLoading = draftsQuery.isPending;
  const draftListError = draftsQuery.isError
    ? errorMessage(draftsQuery.error)
    : null;

  const error = useMemo(() => {
    if (tasksQuery.isError) return errorMessage(tasksQuery.error);
    if (createMutation.isError) return errorMessage(createMutation.error);
    if (evaluateDraftMutation.isError)
      return errorMessage(evaluateDraftMutation.error);
    if (patchError) return patchError;
    if (deleteError) return deleteError;
    return editTitleRequiredError;
  }, [
    tasksQuery.isError,
    tasksQuery.error,
    createMutation.isError,
    createMutation.error,
    evaluateDraftMutation.isError,
    evaluateDraftMutation.error,
    patchError,
    deleteError,
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
    saveDraftMutation.mutate({
      ...buildDraftSaveInput(),
      signature: currentDraftAutosaveSignature,
    });
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
    const sigAtSchedule = currentDraftAutosaveSignature;
    autosaveTimerRef.current = setTimeout(() => {
      saveDraftMutation.mutate({
        ...buildDraftSaveInput(),
        signature: sigAtSchedule,
      });
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
    // Capture this request *synchronously* before awaiting. If a newer
    // `resumeDraftByID(otherId)` call is issued while this one is in
    // flight (e.g. the user clicks another draft in the picker before
    // our GET resolves), the ref will have moved on to `otherId` by
    // the time we land here, and we skip the form-stamp branch so the
    // newer request's payload is the one that wins.
    requestedResumeRef.current = id;
    const draft = await resumeDraftMutation.mutateAsync(id);
    if (requestedResumeRef.current !== id) {
      // A newer resume request has superseded this one. Don't touch
      // form state, modal visibility, or the autosave baseline - those
      // belong to whichever draft the user is now resuming. The
      // `task-drafts` cache doesn't need invalidation here either:
      // `apiGetDraft` is a read, not a mutation.
      return;
    }
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
    runPatch({
      id: editing.id,
      title: editTitle.trim(),
      initial_prompt: editPrompt,
      status: editStatus,
      priority: editPriority,
      task_type: editTaskType,
      checklist_inherit: editChecklistInherit,
    });
  }

  const createPending = createMutation.isPending;
  const evaluatePending = evaluateDraftMutation.isPending;
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
    deleteSuccess,
    deleteVariables,
    error,
    /**
     * Error from the most recent in-modal `createMutation`. Lifted
     * into its own field so `TaskCreateModal` can render an inline
     * `.err` callout — the global `app.error` banner sits behind the
     * modal backdrop and is invisible to the user once a modal is
     * open. `null` when the mutation has not failed (idle / pending /
     * success). Cleared via the same `mutation.reset()` lifecycle
     * react-query gives every mutation; today the modal close path
     * reopens with a fresh state because `useTasksApp` resets on
     * `closeCreateModal` → `resetNewTaskForm`, so consumers do not
     * have to call reset themselves.
     */
    createError: createMutation.error,
    /**
     * Same as `createError` but for the AI evaluation step that
     * runs from the same modal. Surfaced separately because the
     * user might evaluate multiple times before submitting and
     * needs to know which action just failed.
     */
    evaluateError: evaluateDraftMutation.error,
    sseLive,
    taskStats: taskStatsQuery.data,
    /**
     * True only on the first stats fetch (before any settle). Stays false
     * during background refetch so consumers can keep showing the previous
     * values instead of replacing them with a skeleton on every refresh.
     */
    taskStatsLoading: taskStatsQuery.isPending,
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
