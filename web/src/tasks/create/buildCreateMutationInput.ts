import { buildComposePayloadFromForm } from "./composePayload";
import type { CreateTaskMutationInput, TaskCreateFormFields } from "./types";

export function buildCreateTaskMutationInput(
  fields: TaskCreateFormFields,
): CreateTaskMutationInput {
  const compose = buildComposePayloadFromForm(fields);
  return {
    title: compose.title,
    initial_prompt: compose.initial_prompt,
    status: compose.status,
    priority: compose.priority,
    draft_id: fields.newDraftID,
    checklistItems: fields.newChecklistItems,
    runner: compose.runner ?? "cursor",
    cursor_model: compose.cursor_model ?? "",
    pickup_not_before: fields.newSchedule,
    project_id: fields.newProjectID,
    project_context_item_ids: compose.project_context_item_ids ?? [],
    worktree_id: fields.newWorktreeID,
    tags: compose.tags ?? [],
    milestone: compose.milestone,
    depends_on: compose.depends_on ?? [],
  };
}
