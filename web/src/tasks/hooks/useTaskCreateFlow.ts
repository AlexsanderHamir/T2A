import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { AppSettings } from "@/api/settings";
import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import {
  createTask as apiCreate,
  deleteTaskDraft as apiDeleteDraft,
  evaluateDraftTask as apiEvaluateDraft,
  getTaskDraft as apiGetDraft,
  listChecklist,
  listTaskDrafts as apiListDrafts,
  saveTaskDraft as apiSaveDraft,
} from "../../api";
import { plainTextToInitialHtml } from "../task-prompt";
import { settingsQueryKeys, taskQueryKeys } from "../task-query";
import {
  draftAutosaveSignature,
} from "../task-drafts";
import { errorMessage } from "@/lib/errorMessage";
import {
  DEFAULT_NEW_TASK_STATUS,
  DEFAULT_PROJECT_ID,
  type Priority,
  type PriorityChoice,
  type Status,
  type TaskDependencyEdge,
  type AutomationSelection,
} from "@/types";
import { TASK_DRAFTS, TASK_TIMINGS } from "@/constants/tasks";
import {
  CREATE_CHECKLIST_REQUIRED_MSG,
  nonEmptyChecklistCount,
  normalizeChecklistItems,
  normalizeVerifyCommands,
} from "../task-compose/checklistRequirement";
import type { ChecklistItemDraft, Task, TaskDraftChecklistItem, TaskDraftDetail } from "@/types";

const DRAFT_AUTOSAVE_DEBOUNCE_MS = TASK_TIMINGS.draftAutosaveDebounceMs;

type CreateTaskMutationInput = {
  title: string;
  initial_prompt: string;
  status: Status;
  priority: Priority;
  checklistItems: ChecklistItemDraft[];
  draft_id: string;
  runner: string;
  cursor_model: string;
  pickup_not_before: string | null;
  project_id: string;
  project_context_item_ids: string[];
  automation_selections: AutomationSelection[];
  tags: string[];
  milestone?: string;
  depends_on: TaskDependencyEdge[];
};

type DraftEvaluationSnapshot = {
  overallScore: number;
  overallSummary: string;
  sections: Array<{ key: string; score: number }>;
};

function generateTaskDraftID(): string {
  return typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
    ? crypto.randomUUID()
    : `draft-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
}

function defaultRunnerFromSettings(settings: AppSettings | undefined): string {
  return (settings?.runner ?? "cursor").trim() || "cursor";
}

function defaultCursorModelFromSettings(settings: AppSettings | undefined): string {
  return settings?.cursor_model ?? "";
}

function createSubmitStatusForAutonomy(autonomyEnabled: boolean): Status {
  return autonomyEnabled ? DEFAULT_NEW_TASK_STATUS : "on_hold";
}

function parseTagsFromCsv(csv: string): string[] {
  return csv
    .split(/[,;\n]+/)
    .map((t) => t.trim())
    .filter(Boolean);
}

function latestDraftEvaluationFromPayload(
  evaluation: TaskDraftDetail["payload"]["latest_evaluation"],
): DraftEvaluationSnapshot | null {
  if (!evaluation) return null;
  return {
    overallScore: evaluation.overall_score,
    overallSummary: evaluation.overall_summary,
    sections: evaluation.sections,
  };
}

function mapDraftChecklistItems(
  items: TaskDraftChecklistItem[] | undefined,
): ChecklistItemDraft[] {
  return (items ?? []).map((item) => ({
    text: item.text,
    ...(item.verify_commands?.length ? { verify_commands: item.verify_commands } : {}),
  }));
}

function resumedRunnerFromDraft(
  draftRunner: unknown,
  settings: AppSettings | undefined,
): string {
  if (typeof draftRunner === "string" && draftRunner.trim()) {
    return draftRunner.trim();
  }
  return defaultRunnerFromSettings(settings);
}

function resumedCursorModelFromDraft(
  draftModel: unknown,
  settings: AppSettings | undefined,
): string {
  if (typeof draftModel === "string") {
    return draftModel;
  }
  return defaultCursorModelFromSettings(settings);
}

function buildResumedDraftAutosaveBaseline(input: {
  draftID: string;
  title: string;
  prompt: string;
  priority: PriorityChoice;
  runner: string;
  cursorModel: string;
  projectID: string;
  projectContextItemIDs: string[];
  automationSelections: AutomationSelection[];
  checklistItems: TaskDraftChecklistItem[];
  latestEvaluation: DraftEvaluationSnapshot | null;
}): string {
  return draftAutosaveSignature({
    id: input.draftID,
    name: input.title.trim() || TASK_DRAFTS.untitledDraftName,
    title: input.title,
    prompt: input.prompt,
    priority: input.priority,
    runner: input.runner,
    cursorModel: input.cursorModel,
    projectId: input.projectID,
    projectContextItemIds: input.projectContextItemIDs,
    automationSelections: input.automationSelections,
    checklistItems: input.checklistItems,
    latestEvaluation: input.latestEvaluation,
  });
}

/**
 * Create-task modal, draft autosave, draft picker, and related mutations.
 * Composed by `useTasksApp`.
 */
export function useTaskCreateFlow() {
  const queryClient = useQueryClient();

  const [newTitle, setNewTitle] = useState("");
  const [newPrompt, setNewPrompt] = useState("");
  const [newPriority, setNewPriority] = useState<PriorityChoice>("");
  const [newTaskRunner, setNewTaskRunner] = useState("cursor");
  const [newTaskCursorModel, setNewTaskCursorModel] = useState("");
  const [newProjectID, setNewProjectID] = useState(DEFAULT_PROJECT_ID);
  const [newProjectContextItemIDs, setNewProjectContextItemIDs] = useState<string[]>([]);
  const [newAutomationSelections, setNewAutomationSelections] = useState<
    AutomationSelection[]
  >([]);
  /**
   * Future pickup time for the new task as an RFC3339 UTC ISO
   * string, or `null` to mean "no schedule — pick up immediately
   * when the worker is free". Plumbed all the way down to the
   * `SchedulePicker` inside `TaskCreateModal`. When non-null on
   * submit, this **bypasses** the global `agent_pickup_delay_seconds`
   * setting (operator's explicit choice wins, per Stage 2 of the
   * task scheduling plan). Reset to `null` on every modal close /
   * fresh draft / draft resume so a stale schedule from a previous
   * draft cannot leak into a new one.
   *
   * Not persisted to the autosave draft today — drafts are about
   * the *content* of the task, and the operator's notion of "I want
   * to schedule this 4 hours from now" is anchored to wall-clock
   * time, which would silently drift if we serialised the absolute
   * instant into the draft and the user resumed days later. If
   * draft-side scheduling becomes a request, store the chip kind +
   * a `now` snapshot rather than the absolute instant so the
   * resumed draft re-anchors correctly.
   */
  const [newSchedule, setNewSchedule] = useState<string | null>(null);
  /**
   * Whether the new task should be created in agent-pickup-eligible
   * (`ready`) state or held back (`on_hold`). Default `true` matches the
   * pre-existing behavior — clearing the toggle creates the task in
   * `on_hold` so the worker never picks it up until the operator flips
   * it to `ready` from the detail page.
   *
   * Not persisted to drafts: drafts capture the *content* of a task, and
   * the operator's intent to start in hold is a one-time decision tied
   * to "I am submitting this task right now". Resumed drafts default
   * back to autonomy ON so a stale held draft from weeks ago does not
   * silently keep getting created on hold.
   */
  const [newAutonomyEnabled, setNewAutonomyEnabled] = useState(true);
  const [newTagsCsv, setNewTagsCsv] = useState("");
  const [newMilestone, setNewMilestone] = useState("");
  const [newDependsOn, setNewDependsOn] = useState<string[]>([]);
  const [newChecklistItems, setNewChecklistItems] = useState<ChecklistItemDraft[]>([]);
  const [createFormError, setCreateFormError] = useState<string | null>(null);
  // Drop the staged dependency picks whenever the operator switches the
  // task's project. The picker scopes its lookup to a single project, so
  // a chip carried over from project A would refer to a task the picker
  // can't even surface — surfacing a stale id we never resolved would be
  // worse UX than asking the user to re-pick. Skip the very first run
  // (initial mount sets `newProjectID` to its default) by anchoring the
  // ref on the first observed value.
  const prevProjectIdRef = useRef<string | null>(null);
  useEffect(() => {
    if (prevProjectIdRef.current === null) {
      prevProjectIdRef.current = newProjectID;
      return;
    }
    if (prevProjectIdRef.current === newProjectID) return;
    prevProjectIdRef.current = newProjectID;
    setNewDependsOn((prev) => (prev.length === 0 ? prev : []));
  }, [newProjectID]);
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
  const [lastDraftSavedAt, setLastDraftSavedAt] = useState<number | null>(null);
  const [draftPickerOpen, setDraftPickerOpen] = useState(false);
  const [latestDraftEvaluation, setLatestDraftEvaluation] = useState<DraftEvaluationSnapshot | null>(null);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  /** When set, the create modal is editing an existing task (same UI as create). */
  const [editingTaskId, setEditingTaskId] = useState<string | null>(null);
  /** Runner on the task being edited; locked in the agent section. */
  const [editingTaskRunner, setEditingTaskRunner] = useState("");
  /** Writable status for PATCH while `editingTaskId` is set. */
  const [composeStatus, setComposeStatus] = useState<Status>(DEFAULT_NEW_TASK_STATUS);

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
  /** Applied after `resetNewTaskForm` when opening the create modal with a project prefill. */
  const createModalPrefillRef = useRef<{
    projectID: string;
    lockProjectAssignment: boolean;
  } | null>(null);

  const [createModalAssignmentLocked, setCreateModalAssignmentLocked] = useState(false);
  const [draftAutosaveBaseline, setDraftAutosaveBaseline] = useState("");
  const [draftAutosaveBaselineID, setDraftAutosaveBaselineID] = useState("");
  const [createEntryDraftErrorHint, setCreateEntryDraftErrorHint] = useState<
    string | null
  >(null);

  const draftsQuery = useQuery({
    queryKey: taskQueryKeys.drafts(),
    queryFn: ({ signal }) =>
      apiListDrafts(TASK_DRAFTS.createModalDraftListLimit, { signal }),
  });

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
    const generatedID = generateTaskDraftID();
    setNewTitle("");
    setNewPrompt("");
    setNewPriority("");
    const s = queryClient.getQueryData<AppSettings>(settingsQueryKeys.app());
    setNewTaskRunner(defaultRunnerFromSettings(s));
    setNewTaskCursorModel(defaultCursorModelFromSettings(s));
    setNewProjectID(DEFAULT_PROJECT_ID);
    setNewProjectContextItemIDs([]);
    setNewAutomationSelections([]);
    setNewSchedule(null);
    setNewAutonomyEnabled(true);
    setNewTagsCsv("");
    setNewMilestone("");
    setNewDependsOn([]);
    setNewChecklistItems([]);
    setCreateFormError(null);
    setLatestDraftEvaluation(null);
    setNewDraftID(generatedID);
    setLastDraftSavedAt(null);
    setDraftAutosaveBaseline(
      draftAutosaveSignature({
        id: generatedID,
        name: TASK_DRAFTS.untitledDraftName,
        title: "",
        prompt: "",
        priority: "",
        runner: defaultRunnerFromSettings(s),
        cursorModel: defaultCursorModelFromSettings(s),
        projectId: DEFAULT_PROJECT_ID,
        projectContextItemIds: [],
        automationSelections: [],
        checklistItems: [],
        latestEvaluation: null,
      }),
    );
    setDraftAutosaveBaselineID(generatedID);
    setCreateModalAssignmentLocked(false);
    setEditingTaskId(null);
    setEditingTaskRunner("");
    setComposeStatus(DEFAULT_NEW_TASK_STATUS);
  }, [queryClient, setNewDraftID]);

  const populateFromTask = useCallback((t: Task) => {
    requestedResumeRef.current = null;
    setNewTitle(t.title);
    setNewPrompt(t.initial_prompt);
    setNewPriority(t.priority);
    setNewTaskRunner(t.runner);
    setEditingTaskRunner(t.runner);
    setNewTaskCursorModel(t.cursor_model ?? "");
    setNewProjectID(t.project_id || DEFAULT_PROJECT_ID);
    setNewProjectContextItemIDs(t.project_context_item_ids ?? []);
    setNewAutomationSelections(t.automation_selections ?? []);
    setNewSchedule(t.pickup_not_before ?? null);
    setNewAutonomyEnabled(t.status === "ready");
    setComposeStatus(t.status);
    setNewTagsCsv((t.tags ?? []).join(", "));
    setNewMilestone(t.milestone ?? "");
    setNewDependsOn((t.depends_on ?? []).map((edge) => edge.task_id));
    setLatestDraftEvaluation(null);
    setCreateFormError(null);
  }, []);

  const beginEditSession = useCallback(
    async (t: Task) => {
      populateFromTask(t);
      setEditingTaskId(t.id);
      setNewChecklistItems([]);
      setCreateModalOpen(true);
      setDraftPickerOpen(false);
      setCreateEntryDraftErrorHint(null);
      try {
        const { items } = await listChecklist(t.id);
        setNewChecklistItems(
          items.map((item) => ({
            text: item.text,
            verify_commands: item.verify_commands,
          })),
        );
      } catch {
        // Checklist is display-only in edit; leave empty on fetch failure.
      }
    },
    [populateFromTask],
  );

  const applyCreateModalPrefill = useCallback(() => {
    const p = createModalPrefillRef.current;
    if (!p?.projectID) return;
    setNewProjectID(p.projectID);
    setCreateModalAssignmentLocked(p.lockProjectAssignment);
    createModalPrefillRef.current = null;
  }, []);

  const closeCreateModal = useCallback(() => {
    createModalPrefillRef.current = null;
    setCreateModalOpen(false);
    setDraftPickerOpen(false);
    setCreateEntryDraftErrorHint(null);
    resetNewTaskForm();
  }, [resetNewTaskForm]);

  const openCreateModal = useCallback(
    (prefill?: {
      projectID: string;
      lockProjectAssignment?: boolean;
    }) => {
      setCreateEntryDraftErrorHint(null);
      const pid = prefill?.projectID?.trim();
      createModalPrefillRef.current = pid
        ? {
            projectID: pid,
            lockProjectAssignment: prefill?.lockProjectAssignment === true,
          }
        : null;
      if (draftsQuery.isPending) {
        setDraftPickerOpen(true);
        return;
      }
      if (draftsQuery.isError) {
        setCreateEntryDraftErrorHint(errorMessage(draftsQuery.error));
        resetNewTaskForm();
        applyCreateModalPrefill();
        setCreateModalOpen(true);
        return;
      }
      const drafts = draftsQuery.data ?? [];
      if (drafts.length > 0) {
        setDraftPickerOpen(true);
        return;
      }
      resetNewTaskForm();
      applyCreateModalPrefill();
      setCreateModalOpen(true);
    },
    [
      applyCreateModalPrefill,
      draftsQuery.data,
      draftsQuery.error,
      draftsQuery.isError,
      draftsQuery.isPending,
      resetNewTaskForm,
    ],
  );

  const createMutation = useMutation({
    mutationFn: async (input: CreateTaskMutationInput) => {
      const task = await apiCreate({
        title: input.title,
        initial_prompt: input.initial_prompt,
        status: input.status,
        priority: input.priority,
        draft_id: input.draft_id,
        runner: input.runner,
        cursor_model: input.cursor_model,
        ...(input.project_id ? { project_id: input.project_id } : {}),
        ...(input.project_context_item_ids.length > 0
          ? { project_context_item_ids: input.project_context_item_ids }
          : {}),
        ...(input.automation_selections.length > 0
          ? { automation_selections: input.automation_selections }
          : {}),
        ...(input.pickup_not_before !== null
          ? { pickup_not_before: input.pickup_not_before }
          : {}),
        ...(input.tags.length > 0 ? { tags: input.tags } : {}),
        ...(input.milestone ? { milestone: input.milestone } : {}),
        ...(input.depends_on.length > 0 ? { depends_on: input.depends_on } : {}),
        checklist_items: normalizeChecklistItems(input.checklistItems),
      });
      return { task, input };
    },
    onSuccess: (_result, variables) => {
      // Close as soon as the task row exists (checklist_items are created atomically).
      if (newDraftIDRef.current === variables.draft_id) {
        closeCreateModal();
      }
      void queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      void queryClient.invalidateQueries({ queryKey: taskQueryKeys.stats() });
      void queryClient.invalidateQueries({ queryKey: taskQueryKeys.drafts() });
    },
  });

  const evaluateDraftMutation = useMutation({
    mutationFn: async (input: {
      id: string;
      title: string;
      initial_prompt: string;
      status: Status;
      priority: Priority;
      checklistItems: ChecklistItemDraft[];
    }) => {
      return apiEvaluateDraft({
        id: input.id,
        title: input.title,
        initial_prompt: input.initial_prompt,
        status: input.status,
        priority: input.priority,
        checklist_items: normalizeChecklistItems(input.checklistItems),
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
        runner: string;
        cursor_model: string;
        project_id: string;
        project_context_item_ids: string[];
        checklist_items: TaskDraftChecklistItem[];
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
        await queryClient.invalidateQueries({ queryKey: taskQueryKeys.drafts() });
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
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.drafts() });
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
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.drafts() });
    },
  });
  const resumeDraftMutation = useMutation({
    mutationFn: (id: string) => apiGetDraft(id),
  });
  const deleteDraftError = deleteDraftMutation.isError
    ? errorMessage(deleteDraftMutation.error)
    : null;

  const draftListLoading = draftsQuery.isPending;
  const draftListError = draftsQuery.isError
    ? errorMessage(draftsQuery.error)
    : null;

  /** First matching create/evaluate error for `useTasksApp` global banner merge. */
  const createFlowError = useMemo((): string | null => {
    if (createMutation.isError) return errorMessage(createMutation.error);
    if (evaluateDraftMutation.isError)
      return errorMessage(evaluateDraftMutation.error);
    return null;
  }, [
    createMutation.isError,
    createMutation.error,
    evaluateDraftMutation.isError,
    evaluateDraftMutation.error,
  ]);

  const currentDraftAutosaveSignature = useMemo(
    () =>
      draftAutosaveSignature({
        id: newDraftID,
        name: newTitle.trim() || TASK_DRAFTS.untitledDraftName,
        title: newTitle,
        prompt: newPrompt,
        priority: newPriority,
        projectId: newProjectID,
        projectContextItemIds: newProjectContextItemIDs,
        automationSelections: newAutomationSelections,
        checklistItems: normalizeChecklistItems(newChecklistItems),
        latestEvaluation: latestDraftEvaluation,
        runner: newTaskRunner,
        cursorModel: newTaskCursorModel,
      }),
    [
      latestDraftEvaluation,
      newChecklistItems,
      newDraftID,
      newPriority,
      newPrompt,
      newTitle,
      newTaskRunner,
      newTaskCursorModel,
      newProjectID,
      newProjectContextItemIDs,
      newAutomationSelections,
    ],
  );

  const buildDraftSaveInput = useCallback(() => {
    return {
      id: newDraftID,
      name: newTitle.trim() || TASK_DRAFTS.untitledDraftName,
      payload: {
        title: newTitle,
        initial_prompt: newPrompt,
        priority: newPriority,
        runner: newTaskRunner,
        cursor_model: newTaskCursorModel,
        // Persist the operator's project + context selection on the draft
        // so closing and resuming restores the same REFERENCES block in the
        // prompt editor (and the same `project_context_item_ids` on submit).
        project_id: newProjectID,
        project_context_item_ids: newProjectContextItemIDs,
        automation_selections: newAutomationSelections,
        checklist_items: normalizeChecklistItems(newChecklistItems),
        ...(latestDraftEvaluation
          ? {
              latest_evaluation: {
                overall_score: latestDraftEvaluation.overallScore,
                overall_summary: latestDraftEvaluation.overallSummary,
                sections: latestDraftEvaluation.sections,
              },
            }
          : {}),
      },
    };
  }, [
    latestDraftEvaluation,
    newChecklistItems,
    newDraftID,
    newPriority,
    newPrompt,
    newTitle,
    newTaskRunner,
    newTaskCursorModel,
    newProjectID,
    newProjectContextItemIDs,
    newAutomationSelections,
  ]);

  const saveDraftNow = useCallback(() => {
    if (editingTaskId || !createModalOpen || !newDraftID) return;
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
    editingTaskId,
    newDraftID,
    saveDraftMutation,
  ]);

  useEffect(() => {
    if (editingTaskId || !createModalOpen || !newDraftID) return;
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
    editingTaskId,
    newDraftID,
    saveDraftMutation,
  ]);

  const draftSaveLabel = useMemo(() => {
    if (editingTaskId || !createModalOpen) return null;
    if (saveDraftMutation.isPending) return "Saving draft…";
    if (saveDraftMutation.isError) return "Draft autosave failed. You can still create the task.";
    if (lastDraftSavedAt == null) return null;
    return "Draft saved";
  }, [
    createModalOpen,
    editingTaskId,
    lastDraftSavedAt,
    saveDraftMutation.isError,
    saveDraftMutation.isPending,
  ]);
  const draftSaveError = createModalOpen && saveDraftMutation.isError;

  function evaluateDraftBeforeCreate() {
    if (!newTitle.trim() || !newPriority) return;
    if (nonEmptyChecklistCount(newChecklistItems) < 1) {
      setCreateFormError(CREATE_CHECKLIST_REQUIRED_MSG);
      return;
    }
    setCreateFormError(null);
    evaluateDraftMutation.mutate({
      id: newDraftID,
      title: newTitle.trim(),
      initial_prompt: newPrompt,
      status: DEFAULT_NEW_TASK_STATUS,
      priority: newPriority,
      checklistItems: newChecklistItems,
    });
  }

  async function submitCreate(e: FormEvent) {
    e.preventDefault();
    if (!newTitle.trim() || !newPriority) return;
    if (nonEmptyChecklistCount(newChecklistItems) < 1) {
      setCreateFormError(CREATE_CHECKLIST_REQUIRED_MSG);
      return;
    }
    setCreateFormError(null);
    // Autonomy off => create the task in on_hold so the agent worker
    // skips it on dequeue (ReadyForAgentPickup gates on Status==Ready,
    // see pkgs/tasks/store/internal/tasks/readiness.go). The operator
    // resumes the task by flipping status back to ready from the
    // detail page, which goes through the standard PATCH /tasks/{id}
    // path.
    const submitStatus = createSubmitStatusForAutonomy(newAutonomyEnabled);
    createMutation.mutate({
      title: newTitle.trim(),
      initial_prompt: newPrompt,
      status: submitStatus,
      priority: newPriority,
      draft_id: newDraftID,
      checklistItems: newChecklistItems,
      runner: newTaskRunner.trim() || "cursor",
      cursor_model: newTaskCursorModel.trim(),
      project_id: newProjectID.trim(),
      project_context_item_ids: newProjectContextItemIDs,
      automation_selections: newAutomationSelections,
      pickup_not_before: newSchedule,
      tags: parseTagsFromCsv(newTagsCsv),
      milestone: newMilestone.trim() || undefined,
      depends_on: newDependsOn.map((task_id) => ({ task_id, satisfies: "done" as const })),
    });
  }

  async function startFreshDraft() {
    resetNewTaskForm();
    applyCreateModalPrefill();
    setDraftPickerOpen(false);
    setCreateModalOpen(true);
  }

  async function resumeDraftByID(id: string) {
    createModalPrefillRef.current = null;
    setCreateModalAssignmentLocked(false);
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
    const latestEvaluation = latestDraftEvaluationFromPayload(
      draft.payload.latest_evaluation,
    );
    const settingsSnap = queryClient.getQueryData<AppSettings>(
      settingsQueryKeys.app(),
    );
    const resumedRunner = resumedRunnerFromDraft(draft.payload.runner, settingsSnap);
    const resumedModel = resumedCursorModelFromDraft(
      draft.payload.cursor_model,
      settingsSnap,
    );
    setNewTaskRunner(resumedRunner);
    setNewTaskCursorModel(resumedModel);
    // Resumed drafts never carry a schedule — see the doc on
    // `newSchedule` above. Clear so a stale schedule from a previous
    // open of a different draft does not leak into the resumed form.
    setNewSchedule(null);
    setNewAutonomyEnabled(true);
    setNewDraftID(draft.id);
    setNewTitle(draft.payload.title ?? "");
    setNewPrompt(draft.payload.initial_prompt ?? "");
    setNewPriority(draft.payload.priority ?? "");
    setNewChecklistItems(mapDraftChecklistItems(draft.payload.checklist_items));
    setLatestDraftEvaluation(latestEvaluation);
    // Project + selected context items are optional on legacy drafts; fall
    // back to the default project / empty selection so the REFERENCES block
    // and the project picker show a clean state on resume.
    const resumedProjectID =
      typeof draft.payload.project_id === "string" && draft.payload.project_id
        ? draft.payload.project_id
        : DEFAULT_PROJECT_ID;
    const resumedProjectContextIds = Array.isArray(
      draft.payload.project_context_item_ids,
    )
      ? draft.payload.project_context_item_ids
      : [];
    const resumedAutomationSelections = Array.isArray(
      draft.payload.automation_selections,
    )
      ? draft.payload.automation_selections
      : [];
    setNewProjectID(resumedProjectID);
    setNewProjectContextItemIDs(resumedProjectContextIds);
    setNewAutomationSelections(resumedAutomationSelections);
    const resumedTitle = draft.payload.title ?? "";
    setDraftAutosaveBaseline(
      buildResumedDraftAutosaveBaseline({
        draftID: draft.id,
        title: resumedTitle,
        prompt: draft.payload.initial_prompt ?? "",
        priority: draft.payload.priority ?? "",
        runner: resumedRunner,
        cursorModel: resumedModel,
        projectID: resumedProjectID,
        projectContextItemIDs: resumedProjectContextIds,
        automationSelections: resumedAutomationSelections,
        checklistItems: draft.payload.checklist_items ?? [],
        latestEvaluation,
      }),
    );
    setDraftAutosaveBaselineID(draft.id);
    setDraftPickerOpen(false);
    setCreateModalOpen(true);
  }

  async function deleteDraftByID(id: string) {
    await deleteDraftMutation.mutateAsync(id);
  }

  /**
   * Apply a `TestScenario` from `web/src/tasks/test-scenarios` to the open
   * create-modal form. Overwrites title / prompt / priority /
   * done criteria with the scenario's pre-canned content so the operator can
   * dispatch a real agent run with zero typing — the whole point of the
   * test-scenarios affordance.
   *
   * Leaves project / context / runner / model / schedule alone.
   *
   * Same imports kept inline so the test-scenarios module is only pulled
   * into the bundle when this hook is loaded (it already is, since the
   * hook is the create-modal's primary state owner).
   */
  const applyTestScenario = useCallback(
    (scenario: import("../test-scenarios").TestScenario) => {
      setNewTitle(scenario.title);
      setNewPrompt(plainTextToInitialHtml(scenario.prompt));
      setNewPriority(scenario.priority);
      setNewChecklistItems(
        scenario.criteria
          .map((item) => {
            const text = item.text.trim();
            if (!text) return null;
            const verify_commands = normalizeVerifyCommands(
              item.verify_commands ?? [],
            );
            return {
              text,
              ...(verify_commands.length > 0 ? { verify_commands } : {}),
            };
          })
          .filter((item): item is ChecklistItemDraft => item !== null),
      );
    },
    [],
  );

  const appendNewChecklistCriterion = useCallback((raw: ChecklistItemDraft | string) => {
    const item = typeof raw === "string" ? { text: raw } : raw;
    const t = item.text.trim();
    if (!t) return;
    setNewChecklistItems((prev) => {
      const next = [...prev, { text: t, verify_commands: item.verify_commands }];
      if (nonEmptyChecklistCount(next) >= 1) {
        setCreateFormError(null);
      }
      return next;
    });
  }, []);

  const removeNewChecklistRow = useCallback((index: number) => {
    setNewChecklistItems((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const updateNewChecklistRow = useCallback((index: number, item: ChecklistItemDraft) => {
    const t = item.text.trim();
    if (!t) return;
    setNewChecklistItems((prev) =>
      prev.map((x, i) =>
        i === index ? { text: t, verify_commands: item.verify_commands } : x,
      ),
    );
  }, []);

  const createPending = createMutation.isPending;
  const evaluatePending = evaluateDraftMutation.isPending;
  const draftSavePending = saveDraftMutation.isPending;

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
    createFlowError,
    draftSavePending,
    draftSaveLabel,
    draftSaveError,
    createPending,
    evaluatePending,
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
    createFormError,
    /**
     * Same as `createError` but for the AI evaluation step that
     * runs from the same modal. Surfaced separately because the
     * user might evaluate multiple times before submitting and
     * needs to know which action just failed.
     */
    evaluateError: evaluateDraftMutation.error,
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
    newTitle,
    setNewTitle,
    newPrompt,
    setNewPrompt,
    newPriority,
    setNewPriority,
    newTaskRunner,
    setNewTaskRunner,
    newTaskCursorModel,
    setNewTaskCursorModel,
    newProjectID,
    setNewProjectID,
    newProjectContextItemIDs,
    setNewProjectContextItemIDs,
    newAutomationSelections,
    setNewAutomationSelections,
    newSchedule,
    setNewSchedule,
    newAutonomyEnabled,
    setNewAutonomyEnabled,
    newTagsCsv,
    setNewTagsCsv,
    newMilestone,
    setNewMilestone,
    newDependsOn,
    setNewDependsOn,
    newChecklistItems,
    latestDraftEvaluation,
    appendNewChecklistCriterion,
    updateNewChecklistRow,
    removeNewChecklistRow,
    submitCreate,
    evaluateDraftBeforeCreate,
    startFreshDraft,
    saveDraftNow,
    resumeDraftByID,
    deleteDraftByID,
    applyTestScenario,
    createModalOpen,
    createModalAssignmentLocked,
    openCreateModal,
    closeCreateModal,
    editingTaskId,
    editingTaskRunner,
    composeStatus,
    setComposeStatus,
    beginEditSession,
  };
}
