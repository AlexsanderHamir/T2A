import { describe, expect, it } from "vitest";
import { buildCreateTaskMutationInput } from "./buildCreateMutationInput";
import type { TaskCreateFormFields } from "./types";

const baseFields: TaskCreateFormFields = {
  newTitle: "Task",
  newPrompt: "Do work",
  newPriority: "medium",
  newTaskRunner: "cursor",
  newTaskCursorModel: "",
  newProjectID: "",
  newProjectContextItemIDs: [],
  newSchedule: "",
  newAutonomyEnabled: true,
  newTagsCsv: "",
  newMilestone: "",
  newDependsOn: [],
  newWorktreeID: "",
  newChecklistItems: [],
  newDraftID: "",
};

describe("buildCreateTaskMutationInput", () => {
  it("includes worktree_id when set on the form", () => {
    const wtID = "00000000-0000-4000-8000-000000000020";
    const input = buildCreateTaskMutationInput({
      ...baseFields,
      newWorktreeID: wtID,
    });
    expect(input.worktree_id).toBe(wtID);
  });

  it("passes empty worktree_id when unset", () => {
    const input = buildCreateTaskMutationInput(baseFields);
    expect(input.worktree_id).toBe("");
  });
});
