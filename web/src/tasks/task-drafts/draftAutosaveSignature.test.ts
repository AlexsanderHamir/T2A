import { describe, expect, it } from "vitest";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import {
  draftAutosaveSignature,
  normalizeDraftPromptForDirty,
  type DraftAutosaveSignatureInput,
} from "./draftAutosaveSignature";

function baseInput(): DraftAutosaveSignatureInput {
  return {
    id: "draft-1",
    name: "Untitled",
    title: "Hello",
    prompt: "<p>body</p>",
    priority: "medium",
    runner: TASK_TEST_DEFAULTS.runner,
    cursorModel: TASK_TEST_DEFAULTS.cursor_model,
    projectId: "",
    projectContextItemIds: [],
    checklistItems: [],
    latestEvaluation: null,
  };
}

describe("normalizeDraftPromptForDirty", () => {
  it.each([
    "",
    "<p></p>",
    "<P></P>",
    "<p><br></p>",
    "<p><br/></p>",
    "<p>&nbsp;</p>",
    "<p>&#160;&#160;</p>",
    "  \n\u200B\uFEFF  ",
  ])("treats editor-empty markup %j as empty", (markup) => {
    expect(normalizeDraftPromptForDirty(markup)).toBe("");
  });

  it("preserves prompts that have visible content", () => {
    expect(normalizeDraftPromptForDirty("<p>Hello world</p>")).toBe(
      "<p>Hello world</p>",
    );
  });
});

describe("draftAutosaveSignature", () => {
  it("returns identical strings for inputs that only differ in editor whitespace", () => {
    const a = draftAutosaveSignature({ ...baseInput(), prompt: "<p></p>" });
    const b = draftAutosaveSignature({
      ...baseInput(),
      prompt: "<p><br></p>",
    });
    expect(a).toBe(b);
  });

  it("changes when title flips", () => {
    const a = draftAutosaveSignature(baseInput());
    const b = draftAutosaveSignature({ ...baseInput(), title: "Renamed" });
    expect(a).not.toBe(b);
  });

  it("changes when checklist items reorder", () => {
    const a = draftAutosaveSignature({
      ...baseInput(),
      checklistItems: ["one", "two"],
    });
    const b = draftAutosaveSignature({
      ...baseInput(),
      checklistItems: ["two", "one"],
    });
    expect(a).not.toBe(b);
  });

  it("matches the wire shape consumers persist via apiSaveDraft", () => {
    const sig = draftAutosaveSignature({
      ...baseInput(),
      prompt: "<p>Body</p>",
    });
    const parsed = JSON.parse(sig);
    expect(parsed).toEqual({
      id: "draft-1",
      name: "Untitled",
      payload: {
        title: "Hello",
        initial_prompt: "<p>Body</p>",
        priority: "medium",
        runner: TASK_TEST_DEFAULTS.runner,
        cursor_model: TASK_TEST_DEFAULTS.cursor_model,
        project_id: "",
        project_context_item_ids: [],
        checklist_items: [],
        latest_evaluation: null,
      },
    });
  });

  it("changes when project context selection flips", () => {
    const a = draftAutosaveSignature({
      ...baseInput(),
      projectId: "project-1",
      projectContextItemIds: ["ctx-1"],
    });
    const b = draftAutosaveSignature({
      ...baseInput(),
      projectId: "project-1",
      projectContextItemIds: ["ctx-1", "ctx-2"],
    });
    expect(a).not.toBe(b);
  });

  it("changes when the bound project changes", () => {
    const a = draftAutosaveSignature({
      ...baseInput(),
      projectId: "project-1",
    });
    const b = draftAutosaveSignature({
      ...baseInput(),
      projectId: "project-2",
    });
    expect(a).not.toBe(b);
  });
});
