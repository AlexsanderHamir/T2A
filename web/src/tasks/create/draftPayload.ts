import type { AppSettings } from "@/api/settings";
import { TASK_DRAFTS } from "@/constants/tasks";
import {
  DEFAULT_PROJECT_ID,
  type AutomationSelection,
  type ChecklistItemDraft,
  type PriorityChoice,
  type TaskDraftChecklistItem,
  type TaskDraftDetail,
} from "@/types";
import { normalizeChecklistItems } from "../task-compose/checklistRequirement";
import { draftAutosaveSignature } from "../task-drafts";
import {
  defaultCursorModelFromSettings,
  defaultRunnerFromSettings,
} from "./defaults";
import type { DraftEvaluationSnapshot, DraftSavePayload, TaskCreateFormFields } from "./types";

export function latestDraftEvaluationFromPayload(
  evaluation: TaskDraftDetail["payload"]["latest_evaluation"],
): DraftEvaluationSnapshot | null {
  if (!evaluation) return null;
  return {
    overallScore: evaluation.overall_score,
    overallSummary: evaluation.overall_summary,
    sections: evaluation.sections,
  };
}

export function mapDraftChecklistItems(
  items: TaskDraftChecklistItem[] | undefined,
): ChecklistItemDraft[] {
  return (items ?? []).map((item) => ({
    text: item.text,
    ...(item.verify_commands?.length ? { verify_commands: item.verify_commands } : {}),
  }));
}

function resumedRunnerFromDraft(draftRunner: unknown, settings: AppSettings | undefined): string {
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

export function buildResumedDraftAutosaveBaseline(input: {
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

export function computeDraftAutosaveSignature(
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

export function buildDraftSavePayload(
  fields: TaskCreateFormFields,
  latestEvaluation: DraftEvaluationSnapshot | null,
): DraftSavePayload {
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

export function applyResumedDraftToForm(input: {
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
  const resumedProjectContextIds = Array.isArray(input.draft.payload.project_context_item_ids)
    ? input.draft.payload.project_context_item_ids
    : [];
  const resumedAutomationSelections = Array.isArray(input.draft.payload.automation_selections)
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
