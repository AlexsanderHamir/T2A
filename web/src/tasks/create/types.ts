import type { UseQueryResult } from "@tanstack/react-query";
import type {
  ChecklistItemDraft,
  Priority,
  PriorityChoice,
  Status,
  TaskDependencyEdge,
  TaskDraftChecklistItem,
  TaskDraftSummary,
} from "@/types";

export type TaskDraftsQuery = UseQueryResult<TaskDraftSummary[], Error>;

export type CreateTaskMutationInput = {
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
  tags: string[];
  milestone?: string;
  depends_on: TaskDependencyEdge[];
};

export type CreateModalPrefill = {
  projectID: string;
  lockProjectAssignment: boolean;
};

export type ComposeTarget = "task" | "template";
export type ComposeOperation = "create" | "edit";

export type TaskCreateFormFields = {
  newTitle: string;
  newPrompt: string;
  newPriority: PriorityChoice;
  newTaskRunner: string;
  newTaskCursorModel: string;
  newProjectID: string;
  newProjectContextItemIDs: string[];
  newSchedule: string | null;
  newAutonomyEnabled: boolean;
  newTagsCsv: string;
  newMilestone: string;
  newDependsOn: string[];
  newChecklistItems: ChecklistItemDraft[];
  newDraftID: string;
};

export type DraftSavePayload = {
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
  };
};
