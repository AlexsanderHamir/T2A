import type { Priority } from "@/types";
import { createSubmitStatusForAutonomy } from "./defaults";
import type { CreateTaskMutationInput, TaskCreateFormFields } from "./types";

function parseTagsFromCsv(csv: string): string[] {
  return csv
    .split(/[,;\n]+/)
    .map((t) => t.trim())
    .filter(Boolean);
}

export function buildCreateTaskMutationInput(
  fields: TaskCreateFormFields,
): CreateTaskMutationInput {
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
