import { describe, it } from "vitest";

describe("create flow invariants", () => {
  it("documents I1–I6 enforcement locations", () => {
    // I1 — useTaskCreateDraftAutosave skips when editingTaskId set
    // I2 — useTaskCreateMutations saveDraft onSuccess + useTasksApp stale-save tests
    // I3 — useTaskCreateMutations create onSuccess draft_id ref check
    // I4 — resumeDraftByID requestedResumeRef + useTasksApp resume race tests
    // I6 — validateCreateForm.test / useTaskCreateFlow.test default project
    // I7 — decideCreateEntry.test + useTaskCreateEntryActions openCreateModal
  });
});
