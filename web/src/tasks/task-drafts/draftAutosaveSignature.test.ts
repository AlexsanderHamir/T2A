import { describe, expect, it } from "vitest";
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
    taskType: "general",
    parentId: "",
    checklistInherit: false,
    checklistItems: [],
    pendingSubtasks: [],
    latestEvaluation: null,
    dmapConfig: { commitLimit: "1", domain: "", description: "" },
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

  it("changes when pending subtasks change shape", () => {
    const subtask = {
      title: "child",
      initial_prompt: "<p>do</p>",
      priority: "medium" as const,
      task_type: "general" as const,
      checklistItems: ["a"],
      checklist_inherit: false,
    };
    const a = draftAutosaveSignature({
      ...baseInput(),
      pendingSubtasks: [subtask],
    });
    const b = draftAutosaveSignature({
      ...baseInput(),
      pendingSubtasks: [{ ...subtask, title: "renamed" }],
    });
    expect(a).not.toBe(b);
  });

  it("matches the wire shape consumers persist via apiSaveDraft", () => {
    const sig = draftAutosaveSignature({
      ...baseInput(),
      prompt: "<p>Body</p>",
      pendingSubtasks: [
        {
          title: "child",
          initial_prompt: "<p>do</p>",
          priority: "high",
          task_type: "feature",
          checklistItems: ["x", "y"],
          checklist_inherit: true,
        },
      ],
    });
    const parsed = JSON.parse(sig);
    expect(parsed).toEqual({
      id: "draft-1",
      name: "Untitled",
      payload: {
        title: "Hello",
        initial_prompt: "<p>Body</p>",
        priority: "medium",
        task_type: "general",
        parent_id: "",
        checklist_inherit: false,
        checklist_items: [],
        pending_subtasks: [
          {
            title: "child",
            initial_prompt: "<p>do</p>",
            priority: "high",
            task_type: "feature",
            checklist_items: ["x", "y"],
            checklist_inherit: true,
          },
        ],
        latest_evaluation: null,
        dmap_config: { commitLimit: "1", domain: "", description: "" },
      },
    });
  });
});
