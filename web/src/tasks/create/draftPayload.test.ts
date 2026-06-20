import { describe, expect, it } from "vitest";
import { computeDraftAutosaveSignature } from "./draftPayload";
import type { TaskCreateFormFields } from "./types";

const baseFields: TaskCreateFormFields = {
  newTitle: "Title",
  newPrompt: "Prompt",
  newPriority: "medium",
  newTaskRunner: "cursor",
  newTaskCursorModel: "",
  newProjectID: "default",
  newProjectContextItemIDs: [],
  newSchedule: null,
  newAutonomyEnabled: true,
  newTagsCsv: "",
  newMilestone: "",
  newDependsOn: [],
  newChecklistItems: [{ text: "One" }],
  newDraftID: "draft-a",
};

describe("computeDraftAutosaveSignature", () => {
  it("changes when draft id changes", () => {
    const a = computeDraftAutosaveSignature(baseFields, null);
    const b = computeDraftAutosaveSignature({ ...baseFields, newDraftID: "draft-b" }, null);
    expect(a).not.toBe(b);
  });

  it("is stable for identical fields", () => {
    const a = computeDraftAutosaveSignature(baseFields, null);
    const b = computeDraftAutosaveSignature({ ...baseFields }, null);
    expect(a).toBe(b);
  });
});
