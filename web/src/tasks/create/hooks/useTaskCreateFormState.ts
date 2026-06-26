import type { AppSettings } from "@/api/settings";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type { QueryClient } from "@tanstack/react-query";
import { DEFAULT_PROJECT_ID, type ChecklistItemDraft, type PriorityChoice, type Task } from "@/types";
import { settingsQueryKeys } from "@/settings/settingsQueryKeys";
import {
  buildFreshDraftAutosaveBaseline,
  defaultCursorModelFromSettings,
  defaultRunnerFromSettings,
  generateTaskDraftID,
} from "../defaults";
import type { TaskCreateFormFields } from "../types";

export function useTaskCreateFormState(queryClient: QueryClient) {
  const [newTitle, setNewTitle] = useState("");
  const [newPrompt, setNewPrompt] = useState("");
  const [newPriority, setNewPriority] = useState<PriorityChoice>("");
  const [newTaskRunner, setNewTaskRunner] = useState("cursor");
  const [newTaskCursorModel, setNewTaskCursorModel] = useState("");
  const [newProjectID, setNewProjectID] = useState(DEFAULT_PROJECT_ID);
  const [newProjectContextItemIDs, setNewProjectContextItemIDs] = useState<string[]>([]);
  const [newWorktreeID, setNewWorktreeID] = useState("");
  const [newBranchID, setNewBranchID] = useState("");
  const [newWorktreeBranchID, setNewWorktreeBranchID] = useState("");
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
    setNewWorktreeID("");
    setNewBranchID("");
    setNewWorktreeBranchID("");
    setNewSchedule(null);
    setNewAutonomyEnabled(true);
    setNewTagsCsv("");
    setNewMilestone("");
    setNewDependsOn([]);
    setNewChecklistItems([]);
    setCreateFormError(null);
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
    setNewWorktreeID("");
    setNewBranchID("");
    setNewWorktreeBranchID(t.worktree_branch_id ?? "");
    setNewSchedule(t.pickup_not_before ?? null);
    setNewAutonomyEnabled(t.status === "ready");
    setNewTagsCsv((t.tags ?? []).join(", "));
    setNewMilestone(t.milestone ?? "");
    setNewDependsOn((t.depends_on ?? []).map((edge) => edge.task_id));
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
      newWorktreeID,
      newBranchID,
      newWorktreeBranchID,
      newSchedule,
      newAutonomyEnabled,
      newTagsCsv,
      newMilestone,
      newDependsOn,
      newChecklistItems,
      newDraftID,
    }),
    [
      newAutonomyEnabled,
      newChecklistItems,
      newDependsOn,
      newDraftID,
      newMilestone,
      newPriority,
      newProjectContextItemIDs,
      newWorktreeID,
      newBranchID,
      newWorktreeBranchID,
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
    setNewWorktreeID,
    setNewBranchID,
    setNewWorktreeBranchID,
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
    newWorktreeID,
    newBranchID,
    newWorktreeBranchID,
    newSchedule,
    newAutonomyEnabled,
    newTagsCsv,
    newMilestone,
    newDependsOn,
    newChecklistItems,
    newDraftID,
  };
}
