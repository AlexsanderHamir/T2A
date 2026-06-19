import { useMutation, useQuery, useQueryClient, type UseQueryResult } from "@tanstack/react-query";
import type { AppSettings } from "@/api/settings";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type Dispatch,
  type FormEvent,
  type MutableRefObject,
  type SetStateAction,
} from "react";
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
import type { ChecklistItemDraft, Task, TaskDraftChecklistItem, TaskDraftDetail, TaskDraftSummary } from "@/types";

type TaskDraftsQuery = UseQueryResult<TaskDraftSummary[], Error>;

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

type CreateModalPrefill = {
  projectID: string;
  lockProjectAssignment: boolean;
};

type TaskCreateFormFields = {
  newTitle: string;
  newPrompt: string;
  newPriority: PriorityChoice;
  newTaskRunner: string;
  newTaskCursorModel: string;
  newProjectID: string;
  newProjectContextItemIDs: string[];
  newAutomationSelections: AutomationSelection[];
  newSchedule: string | null;
  newAutonomyEnabled: boolean;
  newTagsCsv: string;
  newMilestone: string;
  newDependsOn: string[];
  newChecklistItems: ChecklistItemDraft[];
  newDraftID: string;
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

function buildFreshDraftAutosaveBaseline(
  settings: AppSettings | undefined,
  generatedID: string,
): string {
  return draftAutosaveSignature({
    id: generatedID,
    name: TASK_DRAFTS.untitledDraftName,
    title: "",
    prompt: "",
    priority: "",
    runner: defaultRunnerFromSettings(settings),
    cursorModel: defaultCursorModelFromSettings(settings),
    projectId: DEFAULT_PROJECT_ID,
    projectContextItemIds: [],
    automationSelections: [],
    checklistItems: [],
    latestEvaluation: null,
  });
}

function computeDraftAutosaveSignature(
  fields: TaskCreateFormFields,
  latestEvaluation: DraftEvaluationSnapshot | null,
): string {
  return draftAutosaveSignature({
    id: fields.newDraftID,
    name: fields.newTitle.trim() || TASK_DRAFTS.untitledDraftName,
    title: fields.newTitle,
    prompt: fields.newPrompt,
    priority: fields.newPriority,
    projectId: fields.newProjectID,
    projectContextItemIds: fields.newProjectContextItemIDs,
    automationSelections: fields.newAutomationSelections,
    checklistItems: normalizeChecklistItems(fields.newChecklistItems),
    latestEvaluation,
    runner: fields.newTaskRunner,
    cursorModel: fields.newTaskCursorModel,
  });
}

function buildDraftSavePayload(
  fields: TaskCreateFormFields,
  latestEvaluation: DraftEvaluationSnapshot | null,
) {
  return {
    id: fields.newDraftID,
    name: fields.newTitle.trim() || TASK_DRAFTS.untitledDraftName,
    payload: {
      title: fields.newTitle,
      initial_prompt: fields.newPrompt,
      priority: fields.newPriority,
      runner: fields.newTaskRunner,
      cursor_model: fields.newTaskCursorModel,
      project_id: fields.newProjectID,
      project_context_item_ids: fields.newProjectContextItemIDs,
      automation_selections: fields.newAutomationSelections,
      checklist_items: normalizeChecklistItems(fields.newChecklistItems),
      ...(latestEvaluation
        ? {
            latest_evaluation: {
              overall_score: latestEvaluation.overallScore,
              overall_summary: latestEvaluation.overallSummary,
              sections: latestEvaluation.sections,
            },
          }
        : {}),
    },
  };
}

function buildCreateTaskMutationInput(fields: TaskCreateFormFields): CreateTaskMutationInput {
  return {
    title: fields.newTitle.trim(),
    initial_prompt: fields.newPrompt,
    status: createSubmitStatusForAutonomy(fields.newAutonomyEnabled),
    priority: fields.newPriority as Priority,
    draft_id: fields.newDraftID,
    checklistItems: fields.newChecklistItems,
    runner: fields.newTaskRunner.trim() || "cursor",
    cursor_model: fields.newTaskCursorModel.trim(),
    project_id: fields.newProjectID.trim(),
    project_context_item_ids: fields.newProjectContextItemIDs,
    automation_selections: fields.newAutomationSelections,
    pickup_not_before: fields.newSchedule,
    tags: parseTagsFromCsv(fields.newTagsCsv),
    milestone: fields.newMilestone.trim() || undefined,
    depends_on: fields.newDependsOn.map((task_id) => ({ task_id, satisfies: "done" as const })),
  };
}

function validateCreateFormChecklist(
  title: string,
  priority: PriorityChoice,
  checklistItems: ChecklistItemDraft[],
): string | null {
  if (!title.trim() || !priority) return null;
  if (nonEmptyChecklistCount(checklistItems) < 1) {
    return CREATE_CHECKLIST_REQUIRED_MSG;
  }
  return null;
}

function applyResumedDraftToForm(input: {
  draft: TaskDraftDetail;
  settings: AppSettings | undefined;
  setNewTaskRunner: (runner: string) => void;
  setNewTaskCursorModel: (model: string) => void;
  setNewSchedule: (schedule: string | null) => void;
  setNewAutonomyEnabled: (enabled: boolean) => void;
  setNewDraftID: (id: string) => void;
  setNewTitle: (title: string) => void;
  setNewPrompt: (prompt: string) => void;
  setNewPriority: (priority: PriorityChoice) => void;
  setNewChecklistItems: (items: ChecklistItemDraft[]) => void;
  setLatestDraftEvaluation: (evaluation: DraftEvaluationSnapshot | null) => void;
  setNewProjectID: (id: string) => void;
  setNewProjectContextItemIDs: (ids: string[]) => void;
  setNewAutomationSelections: (selections: AutomationSelection[]) => void;
  setDraftAutosaveBaseline: (baseline: string) => void;
  setDraftAutosaveBaselineID: (id: string) => void;
}) {
  const latestEvaluation = latestDraftEvaluationFromPayload(
    input.draft.payload.latest_evaluation,
  );
  const resumedRunner = resumedRunnerFromDraft(input.draft.payload.runner, input.settings);
  const resumedModel = resumedCursorModelFromDraft(
    input.draft.payload.cursor_model,
    input.settings,
  );
  input.setNewTaskRunner(resumedRunner);
  input.setNewTaskCursorModel(resumedModel);
  input.setNewSchedule(null);
  input.setNewAutonomyEnabled(true);
  input.setNewDraftID(input.draft.id);
  input.setNewTitle(input.draft.payload.title ?? "");
  input.setNewPrompt(input.draft.payload.initial_prompt ?? "");
  input.setNewPriority(input.draft.payload.priority ?? "");
  input.setNewChecklistItems(mapDraftChecklistItems(input.draft.payload.checklist_items));
  input.setLatestDraftEvaluation(latestEvaluation);
  const resumedProjectID =
    typeof input.draft.payload.project_id === "string" && input.draft.payload.project_id
      ? input.draft.payload.project_id
      : DEFAULT_PROJECT_ID;
  const resumedProjectContextIds = Array.isArray(
    input.draft.payload.project_context_item_ids,
  )
    ? input.draft.payload.project_context_item_ids
    : [];
  const resumedAutomationSelections = Array.isArray(
    input.draft.payload.automation_selections,
  )
    ? input.draft.payload.automation_selections
    : [];
  input.setNewProjectID(resumedProjectID);
  input.setNewProjectContextItemIDs(resumedProjectContextIds);
  input.setNewAutomationSelections(resumedAutomationSelections);
  const resumedTitle = input.draft.payload.title ?? "";
  input.setDraftAutosaveBaseline(
    buildResumedDraftAutosaveBaseline({
      draftID: input.draft.id,
      title: resumedTitle,
      prompt: input.draft.payload.initial_prompt ?? "",
      priority: input.draft.payload.priority ?? "",
      runner: resumedRunner,
      cursorModel: resumedModel,
      projectID: resumedProjectID,
      projectContextItemIDs: resumedProjectContextIds,
      automationSelections: resumedAutomationSelections,
      checklistItems: input.draft.payload.checklist_items ?? [],
      latestEvaluation,
    }),
  );
  input.setDraftAutosaveBaselineID(input.draft.id);
}

function useTaskCreateFormState(queryClient: ReturnType<typeof useQueryClient>) {
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
  const [newSchedule, setNewSchedule] = useState<string | null>(null);
  const [newAutonomyEnabled, setNewAutonomyEnabled] = useState(true);
  const [newTagsCsv, setNewTagsCsv] = useState("");
  const [newMilestone, setNewMilestone] = useState("");
  const [newDependsOn, setNewDependsOn] = useState<string[]>([]);
  const [newChecklistItems, setNewChecklistItems] = useState<ChecklistItemDraft[]>([]);
  const [createFormError, setCreateFormError] = useState<string | null>(null);
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
  const newDraftIDRef = useRef("");
  const setNewDraftID = useCallback((id: string) => {
    newDraftIDRef.current = id;
    setNewDraftIDState(id);
  }, []);
  const [lastDraftSavedAt, setLastDraftSavedAt] = useState<number | null>(null);
  const [latestDraftEvaluation, setLatestDraftEvaluation] = useState<DraftEvaluationSnapshot | null>(null);
  const [draftAutosaveBaseline, setDraftAutosaveBaseline] = useState("");
  const [draftAutosaveBaselineID, setDraftAutosaveBaselineID] = useState("");
  const requestedResumeRef = useRef<string | null>(null);
  const autosaveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const resetFormFields = useCallback(() => {
    requestedResumeRef.current = null;
    const generatedID = generateTaskDraftID();
    const settings = queryClient.getQueryData<AppSettings>(settingsQueryKeys.app());
    setNewTitle("");
    setNewPrompt("");
    setNewPriority("");
    setNewTaskRunner(defaultRunnerFromSettings(settings));
    setNewTaskCursorModel(defaultCursorModelFromSettings(settings));
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
    setDraftAutosaveBaseline(buildFreshDraftAutosaveBaseline(settings, generatedID));
    setDraftAutosaveBaselineID(generatedID);
  }, [queryClient, setNewDraftID]);

  const populateFromTask = useCallback((t: Task) => {
    requestedResumeRef.current = null;
    setNewTitle(t.title);
    setNewPrompt(t.initial_prompt);
    setNewPriority(t.priority);
    setNewTaskRunner(t.runner);
    setNewTaskCursorModel(t.cursor_model ?? "");
    setNewProjectID(t.project_id || DEFAULT_PROJECT_ID);
    setNewProjectContextItemIDs(t.project_context_item_ids ?? []);
    setNewAutomationSelections(t.automation_selections ?? []);
    setNewSchedule(t.pickup_not_before ?? null);
    setNewAutonomyEnabled(t.status === "ready");
    setNewTagsCsv((t.tags ?? []).join(", "));
    setNewMilestone(t.milestone ?? "");
    setNewDependsOn((t.depends_on ?? []).map((edge) => edge.task_id));
    setLatestDraftEvaluation(null);
    setCreateFormError(null);
  }, []);

  const formFields = useMemo(
    (): TaskCreateFormFields => ({
      newTitle,
      newPrompt,
      newPriority,
      newTaskRunner,
      newTaskCursorModel,
      newProjectID,
      newProjectContextItemIDs,
      newAutomationSelections,
      newSchedule,
      newAutonomyEnabled,
      newTagsCsv,
      newMilestone,
      newDependsOn,
      newChecklistItems,
      newDraftID,
    }),
    [
      newAutomationSelections,
      newAutonomyEnabled,
      newChecklistItems,
      newDependsOn,
      newDraftID,
      newMilestone,
      newPriority,
      newProjectContextItemIDs,
      newProjectID,
      newPrompt,
      newSchedule,
      newTagsCsv,
      newTaskCursorModel,
      newTaskRunner,
      newTitle,
    ],
  );

  return {
    formFields,
    createFormError,
    setCreateFormError,
    lastDraftSavedAt,
    setLastDraftSavedAt,
    latestDraftEvaluation,
    setLatestDraftEvaluation,
    draftAutosaveBaseline,
    setDraftAutosaveBaseline,
    draftAutosaveBaselineID,
    setDraftAutosaveBaselineID,
    newDraftIDRef,
    requestedResumeRef,
    autosaveTimerRef,
    resetFormFields,
    populateFromTask,
    setNewTitle,
    setNewPrompt,
    setNewPriority,
    setNewTaskRunner,
    setNewTaskCursorModel,
    setNewProjectID,
    setNewProjectContextItemIDs,
    setNewAutomationSelections,
    setNewSchedule,
    setNewAutonomyEnabled,
    setNewTagsCsv,
    setNewMilestone,
    setNewDependsOn,
    setNewChecklistItems,
    setNewDraftID,
    newTitle,
    newPrompt,
    newPriority,
    newTaskRunner,
    newTaskCursorModel,
    newProjectID,
    newProjectContextItemIDs,
    newAutomationSelections,
    newSchedule,
    newAutonomyEnabled,
    newTagsCsv,
    newMilestone,
    newDependsOn,
    newChecklistItems,
    newDraftID,
  };
}

function useTaskCreateModalState(
  resetFormFields: () => void,
  populateFromTask: (t: Task) => void,
  setNewChecklistItems: Dispatch<SetStateAction<ChecklistItemDraft[]>>,
  setNewProjectID: (id: string) => void,
) {
  const createModalPrefillRef = useRef<CreateModalPrefill | null>(null);
  const [draftPickerOpen, setDraftPickerOpen] = useState(false);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [editingTaskId, setEditingTaskId] = useState<string | null>(null);
  const [editingTaskRunner, setEditingTaskRunner] = useState("");
  const [composeStatus, setComposeStatus] = useState<Status>(DEFAULT_NEW_TASK_STATUS);
  const [createModalAssignmentLocked, setCreateModalAssignmentLocked] = useState(false);
  const [createEntryDraftErrorHint, setCreateEntryDraftErrorHint] = useState<
    string | null
  >(null);

  const resetNewTaskForm = useCallback(() => {
    resetFormFields();
    setCreateModalAssignmentLocked(false);
    setEditingTaskId(null);
    setEditingTaskRunner("");
    setComposeStatus(DEFAULT_NEW_TASK_STATUS);
  }, [resetFormFields]);

  const applyCreateModalPrefill = useCallback(() => {
    const prefill = createModalPrefillRef.current;
    if (!prefill?.projectID) return;
    setNewProjectID(prefill.projectID);
    setCreateModalAssignmentLocked(prefill.lockProjectAssignment);
    createModalPrefillRef.current = null;
  }, [setNewProjectID]);

  const closeCreateModal = useCallback(() => {
    createModalPrefillRef.current = null;
    setCreateModalOpen(false);
    setDraftPickerOpen(false);
    setCreateEntryDraftErrorHint(null);
    resetNewTaskForm();
  }, [resetNewTaskForm]);

  const beginEditSession = useCallback(
    async (t: Task) => {
      populateFromTask(t);
      setEditingTaskId(t.id);
      setEditingTaskRunner(t.runner);
      setComposeStatus(t.status);
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
    [populateFromTask, setNewChecklistItems],
  );

  return {
    createModalPrefillRef,
    draftPickerOpen,
    setDraftPickerOpen,
    createModalOpen,
    setCreateModalOpen,
    editingTaskId,
    editingTaskRunner,
    composeStatus,
    setComposeStatus,
    createModalAssignmentLocked,
    setCreateModalAssignmentLocked,
    createEntryDraftErrorHint,
    setCreateEntryDraftErrorHint,
    applyCreateModalPrefill,
    resetNewTaskForm,
    closeCreateModal,
    beginEditSession,
  };
}

function useTaskCreateFlowMutations(input: {
  queryClient: ReturnType<typeof useQueryClient>;
  newDraftIDRef: MutableRefObject<string>;
  newDraftID: string;
  closeCreateModal: () => void;
  setNewDraftID: (id: string) => void;
  setLatestDraftEvaluation: (evaluation: DraftEvaluationSnapshot | null) => void;
  setDraftAutosaveBaseline: (baseline: string) => void;
  setDraftAutosaveBaselineID: (id: string) => void;
  setLastDraftSavedAt: (timestamp: number | null) => void;
  createModalOpen: boolean;
}) {
  const createMutation = useMutation({
    mutationFn: async (mutationInput: CreateTaskMutationInput) => {
      const task = await apiCreate({
        title: mutationInput.title,
        initial_prompt: mutationInput.initial_prompt,
        status: mutationInput.status,
        priority: mutationInput.priority,
        draft_id: mutationInput.draft_id,
        runner: mutationInput.runner,
        cursor_model: mutationInput.cursor_model,
        ...(mutationInput.project_id ? { project_id: mutationInput.project_id } : {}),
        ...(mutationInput.project_context_item_ids.length > 0
          ? { project_context_item_ids: mutationInput.project_context_item_ids }
          : {}),
        ...(mutationInput.automation_selections.length > 0
          ? { automation_selections: mutationInput.automation_selections }
          : {}),
        ...(mutationInput.pickup_not_before !== null
          ? { pickup_not_before: mutationInput.pickup_not_before }
          : {}),
        ...(mutationInput.tags.length > 0 ? { tags: mutationInput.tags } : {}),
        ...(mutationInput.milestone ? { milestone: mutationInput.milestone } : {}),
        ...(mutationInput.depends_on.length > 0
          ? { depends_on: mutationInput.depends_on }
          : {}),
        checklist_items: normalizeChecklistItems(mutationInput.checklistItems),
      });
      return { task, input: mutationInput };
    },
    onSuccess: (_result, variables) => {
      if (input.newDraftIDRef.current === variables.draft_id) {
        input.closeCreateModal();
      }
      void input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      void input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.stats() });
      void input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.drafts() });
    },
  });

  const evaluateDraftMutation = useMutation({
    mutationFn: async (mutationInput: {
      id: string;
      title: string;
      initial_prompt: string;
      status: Status;
      priority: Priority;
      checklistItems: ChecklistItemDraft[];
    }) => {
      return apiEvaluateDraft({
        id: mutationInput.id,
        title: mutationInput.title,
        initial_prompt: mutationInput.initial_prompt,
        status: mutationInput.status,
        priority: mutationInput.priority,
        checklist_items: normalizeChecklistItems(mutationInput.checklistItems),
      });
    },
    onSuccess: (evaluation, variables) => {
      if (input.newDraftIDRef.current !== variables.id) return;
      input.setLatestDraftEvaluation({
        overallScore: evaluation.overall_score,
        overallSummary: evaluation.overall_summary,
        sections: evaluation.sections.map((section) => ({
          key: section.key,
          score: section.score,
        })),
      });
    },
  });

  const saveDraftMutation = useMutation({
    mutationFn: (mutationInput: {
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
    }) => apiSaveDraft(mutationInput),
    onSuccess: async (saved, variables) => {
      if (input.newDraftIDRef.current !== saved.id) {
        await input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.drafts() });
        return;
      }
      if (saved.id !== input.newDraftID) {
        input.setNewDraftID(saved.id);
      }
      input.setDraftAutosaveBaseline(variables.signature);
      input.setDraftAutosaveBaselineID(saved.id);
      input.setLastDraftSavedAt(Date.now());
      await input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.drafts() });
    },
  });

  const deleteDraftMutation = useMutation({
    mutationFn: (id: string) => apiDeleteDraft(id),
    onSuccess: async () => {
      await input.queryClient.invalidateQueries({ queryKey: taskQueryKeys.drafts() });
    },
  });

  const resumeDraftMutation = useMutation({
    mutationFn: (id: string) => apiGetDraft(id),
  });

  useEffect(() => {
    if (!input.createModalOpen && !saveDraftMutation.isIdle) {
      saveDraftMutation.reset();
    }
  }, [input.createModalOpen, saveDraftMutation]);

  useEffect(() => {
    if (!input.createModalOpen) {
      if (!createMutation.isIdle) createMutation.reset();
      if (!evaluateDraftMutation.isIdle) evaluateDraftMutation.reset();
    }
  }, [input.createModalOpen, createMutation, evaluateDraftMutation]);

  return {
    createMutation,
    evaluateDraftMutation,
    saveDraftMutation,
    deleteDraftMutation,
    resumeDraftMutation,
  };
}

function useTaskCreateDraftAutosave(input: {
  formFields: TaskCreateFormFields;
  latestDraftEvaluation: DraftEvaluationSnapshot | null;
  draftAutosaveBaseline: string;
  draftAutosaveBaselineID: string;
  editingTaskId: string | null;
  createModalOpen: boolean;
  autosaveTimerRef: MutableRefObject<ReturnType<typeof setTimeout> | null>;
  saveDraftMutation: ReturnType<typeof useTaskCreateFlowMutations>["saveDraftMutation"];
  lastDraftSavedAt: number | null;
}) {
  const currentDraftAutosaveSignature = useMemo(
    () => computeDraftAutosaveSignature(input.formFields, input.latestDraftEvaluation),
    [input.formFields, input.latestDraftEvaluation],
  );

  const buildDraftSaveInput = useCallback(
    () => buildDraftSavePayload(input.formFields, input.latestDraftEvaluation),
    [input.formFields, input.latestDraftEvaluation],
  );

  const saveDraftNow = useCallback(() => {
    if (input.editingTaskId || !input.createModalOpen || !input.formFields.newDraftID) return;
    if (input.draftAutosaveBaselineID !== input.formFields.newDraftID) return;
    if (currentDraftAutosaveSignature === input.draftAutosaveBaseline) return;
    if (input.autosaveTimerRef.current) {
      clearTimeout(input.autosaveTimerRef.current);
      input.autosaveTimerRef.current = null;
    }
    input.saveDraftMutation.mutate({
      ...buildDraftSaveInput(),
      signature: currentDraftAutosaveSignature,
    });
  }, [
    buildDraftSaveInput,
    currentDraftAutosaveSignature,
    input,
  ]);

  useEffect(() => {
    if (input.editingTaskId || !input.createModalOpen || !input.formFields.newDraftID) return;
    if (input.draftAutosaveBaselineID !== input.formFields.newDraftID) return;
    if (currentDraftAutosaveSignature === input.draftAutosaveBaseline) return;
    const signatureAtSchedule = currentDraftAutosaveSignature;
    input.autosaveTimerRef.current = setTimeout(() => {
      input.saveDraftMutation.mutate({
        ...buildDraftSaveInput(),
        signature: signatureAtSchedule,
      });
      input.autosaveTimerRef.current = null;
    }, DRAFT_AUTOSAVE_DEBOUNCE_MS);
    return () => {
      if (input.autosaveTimerRef.current) {
        clearTimeout(input.autosaveTimerRef.current);
        input.autosaveTimerRef.current = null;
      }
    };
  }, [
    buildDraftSaveInput,
    currentDraftAutosaveSignature,
    input,
  ]);

  const draftSaveLabel = useMemo(() => {
    if (input.editingTaskId || !input.createModalOpen) return null;
    if (input.saveDraftMutation.isPending) return "Saving draft…";
    if (input.saveDraftMutation.isError) {
      return "Draft autosave failed. You can still create the task.";
    }
    if (input.lastDraftSavedAt == null) return null;
    return "Draft saved";
  }, [
    input.createModalOpen,
    input.editingTaskId,
    input.lastDraftSavedAt,
    input.saveDraftMutation.isError,
    input.saveDraftMutation.isPending,
  ]);

  return {
    saveDraftNow,
    draftSaveLabel,
    draftSaveError: input.createModalOpen && input.saveDraftMutation.isError,
  };
}

function useTaskCreateFlowActions(input: {
  form: ReturnType<typeof useTaskCreateFormState>;
  modal: ReturnType<typeof useTaskCreateModalState>;
  mutations: ReturnType<typeof useTaskCreateFlowMutations>;
  draftsQuery: TaskDraftsQuery;
  queryClient: ReturnType<typeof useQueryClient>;
}) {
  const openCreateModal = useCallback(
    (prefill?: { projectID: string; lockProjectAssignment?: boolean }) => {
      input.modal.setCreateEntryDraftErrorHint(null);
      const projectID = prefill?.projectID?.trim();
      input.modal.createModalPrefillRef.current = projectID
        ? {
            projectID,
            lockProjectAssignment: prefill?.lockProjectAssignment === true,
          }
        : null;
      if (input.draftsQuery.isPending) {
        input.modal.setDraftPickerOpen(true);
        return;
      }
      if (input.draftsQuery.isError) {
        input.modal.setCreateEntryDraftErrorHint(errorMessage(input.draftsQuery.error));
        input.modal.resetNewTaskForm();
        input.modal.applyCreateModalPrefill();
        input.modal.setCreateModalOpen(true);
        return;
      }
      const drafts = input.draftsQuery.data ?? [];
      if (drafts.length > 0) {
        input.modal.setDraftPickerOpen(true);
        return;
      }
      input.modal.resetNewTaskForm();
      input.modal.applyCreateModalPrefill();
      input.modal.setCreateModalOpen(true);
    },
    [input],
  );

  const evaluateDraftBeforeCreate = useCallback(() => {
    const validationError = validateCreateFormChecklist(
      input.form.newTitle,
      input.form.newPriority,
      input.form.newChecklistItems,
    );
    if (!input.form.newTitle.trim() || !input.form.newPriority) return;
    if (validationError) {
      input.form.setCreateFormError(validationError);
      return;
    }
    input.form.setCreateFormError(null);
    input.mutations.evaluateDraftMutation.mutate({
      id: input.form.newDraftID,
      title: input.form.newTitle.trim(),
      initial_prompt: input.form.newPrompt,
      status: DEFAULT_NEW_TASK_STATUS,
      priority: input.form.newPriority as Priority,
      checklistItems: input.form.newChecklistItems,
    });
  }, [input]);

  const submitCreate = useCallback(async (event: FormEvent) => {
    event.preventDefault();
    const validationError = validateCreateFormChecklist(
      input.form.newTitle,
      input.form.newPriority,
      input.form.newChecklistItems,
    );
    if (!input.form.newTitle.trim() || !input.form.newPriority) return;
    if (validationError) {
      input.form.setCreateFormError(validationError);
      return;
    }
    input.form.setCreateFormError(null);
    input.mutations.createMutation.mutate(buildCreateTaskMutationInput(input.form.formFields));
  }, [input]);

  const startFreshDraft = useCallback(async () => {
    input.modal.resetNewTaskForm();
    input.modal.applyCreateModalPrefill();
    input.modal.setDraftPickerOpen(false);
    input.modal.setCreateModalOpen(true);
  }, [input]);

  const resumeDraftByID = useCallback(
    async (id: string) => {
      input.modal.createModalPrefillRef.current = null;
      input.modal.setCreateModalAssignmentLocked(false);
      input.form.requestedResumeRef.current = id;
      const draft = await input.mutations.resumeDraftMutation.mutateAsync(id);
      if (input.form.requestedResumeRef.current !== id) {
        return;
      }
      applyResumedDraftToForm({
        draft,
        settings: input.queryClient.getQueryData<AppSettings>(settingsQueryKeys.app()),
        setNewTaskRunner: input.form.setNewTaskRunner,
        setNewTaskCursorModel: input.form.setNewTaskCursorModel,
        setNewSchedule: input.form.setNewSchedule,
        setNewAutonomyEnabled: input.form.setNewAutonomyEnabled,
        setNewDraftID: input.form.setNewDraftID,
        setNewTitle: input.form.setNewTitle,
        setNewPrompt: input.form.setNewPrompt,
        setNewPriority: input.form.setNewPriority,
        setNewChecklistItems: input.form.setNewChecklistItems,
        setLatestDraftEvaluation: input.form.setLatestDraftEvaluation,
        setNewProjectID: input.form.setNewProjectID,
        setNewProjectContextItemIDs: input.form.setNewProjectContextItemIDs,
        setNewAutomationSelections: input.form.setNewAutomationSelections,
        setDraftAutosaveBaseline: input.form.setDraftAutosaveBaseline,
        setDraftAutosaveBaselineID: input.form.setDraftAutosaveBaselineID,
      });
      input.modal.setDraftPickerOpen(false);
      input.modal.setCreateModalOpen(true);
    },
    [input],
  );

  const deleteDraftByID = useCallback(
    async (id: string) => {
      await input.mutations.deleteDraftMutation.mutateAsync(id);
    },
    [input.mutations.deleteDraftMutation],
  );

  const applyTestScenario = useCallback(
    (scenario: import("../test-scenarios").TestScenario) => {
      input.form.setNewTitle(scenario.title);
      input.form.setNewPrompt(plainTextToInitialHtml(scenario.prompt));
      input.form.setNewPriority(scenario.priority);
      input.form.setNewChecklistItems(
        scenario.criteria
          .map((item) => {
            const text = item.text.trim();
            if (!text) return null;
            const verify_commands = normalizeVerifyCommands(item.verify_commands ?? []);
            return {
              text,
              ...(verify_commands.length > 0 ? { verify_commands } : {}),
            };
          })
          .filter((item): item is ChecklistItemDraft => item !== null),
      );
    },
    [input.form],
  );

  const appendNewChecklistCriterion = useCallback((raw: ChecklistItemDraft | string) => {
    const item = typeof raw === "string" ? { text: raw } : raw;
    const text = item.text.trim();
    if (!text) return;
    input.form.setNewChecklistItems((prev) => {
      const next = [...prev, { text, verify_commands: item.verify_commands }];
      if (nonEmptyChecklistCount(next) >= 1) {
        input.form.setCreateFormError(null);
      }
      return next;
    });
  }, [input.form]);

  const removeNewChecklistRow = useCallback((index: number) => {
    input.form.setNewChecklistItems((prev) => prev.filter((_, rowIndex) => rowIndex !== index));
  }, [input.form]);

  const updateNewChecklistRow = useCallback((index: number, item: ChecklistItemDraft) => {
    const text = item.text.trim();
    if (!text) return;
    input.form.setNewChecklistItems((prev) =>
      prev.map((row, rowIndex) =>
        rowIndex === index ? { text, verify_commands: item.verify_commands } : row,
      ),
    );
  }, [input.form]);

  const retryDraftList = useCallback(async () => {
    await input.draftsQuery.refetch();
  }, [input.draftsQuery]);

  const retryCreateEntryDraftLoad = useCallback(async () => {
    const refreshed = await input.draftsQuery.refetch();
    if (refreshed.isError) {
      input.modal.setCreateEntryDraftErrorHint(errorMessage(refreshed.error));
      return;
    }
    input.modal.setCreateEntryDraftErrorHint(null);
    const drafts = refreshed.data ?? [];
    if (drafts.length > 0) {
      input.modal.setCreateModalOpen(false);
      input.modal.setDraftPickerOpen(true);
    }
  }, [input.draftsQuery, input.modal]);

  return {
    openCreateModal,
    evaluateDraftBeforeCreate,
    submitCreate,
    startFreshDraft,
    resumeDraftByID,
    deleteDraftByID,
    applyTestScenario,
    appendNewChecklistCriterion,
    removeNewChecklistRow,
    updateNewChecklistRow,
    retryDraftList,
    retryCreateEntryDraftLoad,
  };
}

function deriveCreateFlowError(
  mutations: ReturnType<typeof useTaskCreateFlowMutations>,
): string | null {
  if (mutations.createMutation.isError) {
    return errorMessage(mutations.createMutation.error);
  }
  if (mutations.evaluateDraftMutation.isError) {
    return errorMessage(mutations.evaluateDraftMutation.error);
  }
  return null;
}

function buildTaskCreateFlowReturnValue(input: {
  createFlowError: string | null;
  form: ReturnType<typeof useTaskCreateFormState>;
  modal: ReturnType<typeof useTaskCreateModalState>;
  mutations: ReturnType<typeof useTaskCreateFlowMutations>;
  autosave: ReturnType<typeof useTaskCreateDraftAutosave>;
  actions: ReturnType<typeof useTaskCreateFlowActions>;
  draftsQuery: TaskDraftsQuery;
}) {
  return {
    createFlowError: input.createFlowError,
    draftSavePending: input.mutations.saveDraftMutation.isPending,
    draftSaveLabel: input.autosave.draftSaveLabel,
    draftSaveError: input.autosave.draftSaveError,
    createPending: input.mutations.createMutation.isPending,
    evaluatePending: input.mutations.evaluateDraftMutation.isPending,
    createError: input.mutations.createMutation.error,
    createFormError: input.form.createFormError,
    evaluateError: input.mutations.evaluateDraftMutation.error,
    draftPickerOpen: input.modal.draftPickerOpen,
    setDraftPickerOpen: input.modal.setDraftPickerOpen,
    taskDrafts: input.draftsQuery.data ?? [],
    draftListLoading: input.draftsQuery.isPending,
    draftListError: input.draftsQuery.isError
      ? errorMessage(input.draftsQuery.error)
      : null,
    createEntryDraftErrorHint: input.modal.createEntryDraftErrorHint,
    retryDraftList: input.actions.retryDraftList,
    retryCreateEntryDraftLoad: input.actions.retryCreateEntryDraftLoad,
    deleteDraftPending: input.mutations.deleteDraftMutation.isPending,
    deleteDraftError: input.mutations.deleteDraftMutation.isError
      ? errorMessage(input.mutations.deleteDraftMutation.error)
      : null,
    resumeDraftPending: input.mutations.resumeDraftMutation.isPending,
    resumeDraftError: input.mutations.resumeDraftMutation.isError
      ? errorMessage(input.mutations.resumeDraftMutation.error)
      : null,
    clearResumeDraftError: input.mutations.resumeDraftMutation.reset,
    newTitle: input.form.newTitle,
    setNewTitle: input.form.setNewTitle,
    newPrompt: input.form.newPrompt,
    setNewPrompt: input.form.setNewPrompt,
    newPriority: input.form.newPriority,
    setNewPriority: input.form.setNewPriority,
    newTaskRunner: input.form.newTaskRunner,
    setNewTaskRunner: input.form.setNewTaskRunner,
    newTaskCursorModel: input.form.newTaskCursorModel,
    setNewTaskCursorModel: input.form.setNewTaskCursorModel,
    newProjectID: input.form.newProjectID,
    setNewProjectID: input.form.setNewProjectID,
    newProjectContextItemIDs: input.form.newProjectContextItemIDs,
    setNewProjectContextItemIDs: input.form.setNewProjectContextItemIDs,
    newAutomationSelections: input.form.newAutomationSelections,
    setNewAutomationSelections: input.form.setNewAutomationSelections,
    newSchedule: input.form.newSchedule,
    setNewSchedule: input.form.setNewSchedule,
    newAutonomyEnabled: input.form.newAutonomyEnabled,
    setNewAutonomyEnabled: input.form.setNewAutonomyEnabled,
    newTagsCsv: input.form.newTagsCsv,
    setNewTagsCsv: input.form.setNewTagsCsv,
    newMilestone: input.form.newMilestone,
    setNewMilestone: input.form.setNewMilestone,
    newDependsOn: input.form.newDependsOn,
    setNewDependsOn: input.form.setNewDependsOn,
    newChecklistItems: input.form.newChecklistItems,
    latestDraftEvaluation: input.form.latestDraftEvaluation,
    appendNewChecklistCriterion: input.actions.appendNewChecklistCriterion,
    updateNewChecklistRow: input.actions.updateNewChecklistRow,
    removeNewChecklistRow: input.actions.removeNewChecklistRow,
    submitCreate: input.actions.submitCreate,
    evaluateDraftBeforeCreate: input.actions.evaluateDraftBeforeCreate,
    startFreshDraft: input.actions.startFreshDraft,
    saveDraftNow: input.autosave.saveDraftNow,
    resumeDraftByID: input.actions.resumeDraftByID,
    deleteDraftByID: input.actions.deleteDraftByID,
    applyTestScenario: input.actions.applyTestScenario,
    createModalOpen: input.modal.createModalOpen,
    createModalAssignmentLocked: input.modal.createModalAssignmentLocked,
    openCreateModal: input.actions.openCreateModal,
    closeCreateModal: input.modal.closeCreateModal,
    editingTaskId: input.modal.editingTaskId,
    editingTaskRunner: input.modal.editingTaskRunner,
    composeStatus: input.modal.composeStatus,
    setComposeStatus: input.modal.setComposeStatus,
    beginEditSession: input.modal.beginEditSession,
  };
}

function useTaskCreateFlowComposition() {
  const queryClient = useQueryClient();
  const form = useTaskCreateFormState(queryClient);
  const modal = useTaskCreateModalState(
    form.resetFormFields,
    form.populateFromTask,
    form.setNewChecklistItems,
    form.setNewProjectID,
  );
  const draftsQuery = useQuery({
    queryKey: taskQueryKeys.drafts(),
    queryFn: ({ signal }) =>
      apiListDrafts(TASK_DRAFTS.createModalDraftListLimit, { signal }),
  });
  const mutations = useTaskCreateFlowMutations({
    queryClient,
    newDraftIDRef: form.newDraftIDRef,
    newDraftID: form.newDraftID,
    closeCreateModal: modal.closeCreateModal,
    setNewDraftID: form.setNewDraftID,
    setLatestDraftEvaluation: form.setLatestDraftEvaluation,
    setDraftAutosaveBaseline: form.setDraftAutosaveBaseline,
    setDraftAutosaveBaselineID: form.setDraftAutosaveBaselineID,
    setLastDraftSavedAt: form.setLastDraftSavedAt,
    createModalOpen: modal.createModalOpen,
  });
  const autosave = useTaskCreateDraftAutosave({
    formFields: form.formFields,
    latestDraftEvaluation: form.latestDraftEvaluation,
    draftAutosaveBaseline: form.draftAutosaveBaseline,
    draftAutosaveBaselineID: form.draftAutosaveBaselineID,
    editingTaskId: modal.editingTaskId,
    createModalOpen: modal.createModalOpen,
    autosaveTimerRef: form.autosaveTimerRef,
    saveDraftMutation: mutations.saveDraftMutation,
    lastDraftSavedAt: form.lastDraftSavedAt,
  });
  const actions = useTaskCreateFlowActions({
    form,
    modal,
    mutations,
    draftsQuery,
    queryClient,
  });
  const createFlowError = useMemo(
    () => deriveCreateFlowError(mutations),
    [
      mutations.createMutation.error,
      mutations.createMutation.isError,
      mutations.evaluateDraftMutation.error,
      mutations.evaluateDraftMutation.isError,
    ],
  );

  return { createFlowError, form, modal, mutations, autosave, actions, draftsQuery };
}

/**
 * Create-task modal, draft autosave, draft picker, and related mutations.
 * Composed by `useTasksApp`.
 */
export function useTaskCreateFlow() {
  const composed = useTaskCreateFlowComposition();
  return buildTaskCreateFlowReturnValue(composed);
}
