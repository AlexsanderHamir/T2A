import type { AppSettings } from "@/api/settings";
import { DEFAULT_PROJECT_ID, type ChecklistItemDraft, type Priority, type Status, type TaskComposePayload } from "@/types";
import { normalizeChecklistItems } from "../task-compose/checklistRequirement";
import { createSubmitStatusForAutonomy, defaultCursorModelFromSettings, defaultRunnerFromSettings } from "./defaults";
import type { TaskCreateFormFields } from "./types";

function parseTagsFromCsv(csv: string): string[] {
  return csv
    .split(/[,;\n]+/)
    .map((t) => t.trim())
    .filter(Boolean);
}

export function buildComposePayloadFromForm(
  fields: TaskCreateFormFields,
): TaskComposePayload {
  return {
    title: fields.newTitle.trim(),
    initial_prompt: fields.newPrompt,
    status: createSubmitStatusForAutonomy(fields.newAutonomyEnabled),
    priority: fields.newPriority as Priority,
    runner: fields.newTaskRunner.trim() || "cursor",
    cursor_model: fields.newTaskCursorModel.trim(),
    project_id: fields.newProjectID.trim() || undefined,
    project_context_item_ids: fields.newProjectContextItemIDs,
    pickup_not_before: fields.newSchedule ?? undefined,
    tags: parseTagsFromCsv(fields.newTagsCsv),
    milestone: fields.newMilestone.trim() || undefined,
    depends_on: fields.newDependsOn.map((task_id) => ({ task_id, satisfies: "done" as const })),
    checklist_items: normalizeChecklistItems(fields.newChecklistItems),
  };
}

export function hydrateFormFromComposePayload(
  payload: TaskComposePayload,
  settings: AppSettings | undefined,
): {
  title: string;
  prompt: string;
  priority: TaskCreateFormFields["newPriority"];
  runner: string;
  cursorModel: string;
  projectID: string;
  projectContextItemIDs: string[];
  schedule: string | null;
  autonomyEnabled: boolean;
  tagsCsv: string;
  milestone: string;
  dependsOn: string[];
  checklistItems: ChecklistItemDraft[];
} {
  const runner =
    typeof payload.runner === "string" && payload.runner.trim()
      ? payload.runner.trim()
      : defaultRunnerFromSettings(settings);
  const cursorModel =
    typeof payload.cursor_model === "string"
      ? payload.cursor_model
      : defaultCursorModelFromSettings(settings);
  const projectID =
    typeof payload.project_id === "string" && payload.project_id
      ? payload.project_id
      : DEFAULT_PROJECT_ID;
  const projectContextItemIDs = Array.isArray(payload.project_context_item_ids)
    ? payload.project_context_item_ids
    : [];
  const status: Status = payload.status ?? "ready";
  return {
    title: payload.title ?? "",
    prompt: payload.initial_prompt ?? "",
    priority: payload.priority ?? "",
    runner,
    cursorModel,
    projectID,
    projectContextItemIDs,
    schedule: payload.pickup_not_before ?? null,
    autonomyEnabled: status === "ready",
    tagsCsv: (payload.tags ?? []).join(", "),
    milestone: payload.milestone ?? "",
    dependsOn: (payload.depends_on ?? []).map((edge) => edge.task_id),
    checklistItems: (payload.checklist_items ?? []).map((item) => ({
      text: item.text,
      ...(item.verify_commands?.length ? { verify_commands: item.verify_commands } : {}),
    })),
  };
}
