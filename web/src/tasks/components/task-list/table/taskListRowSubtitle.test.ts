import { describe, expect, it } from "vitest";
import { statusListLabel, taskListRowSubtitle } from "./taskListRowSubtitle";

describe("taskListRowSubtitle", () => {
  it("uses step hint when project is shown in its own column", () => {
    expect(
      taskListRowSubtitle({
        depth: 0,
        hasProject: true,
        projectStepId: "step-1",
        promptPreview: "ignored",
      }),
    ).toBe("Step");
  });

  it("omits subtitle when project column carries context", () => {
    expect(
      taskListRowSubtitle({
        depth: 0,
        hasProject: true,
        projectStepId: undefined,
        promptPreview: "Some prompt",
      }),
    ).toBeUndefined();
  });

  it("shows subtask under project without repeating the name", () => {
    expect(
      taskListRowSubtitle({
        depth: 1,
        hasProject: true,
        projectStepId: undefined,
        promptPreview: "",
      }),
    ).toBe("Subtask");
  });

  it("shows subtask and prompt preview when no project", () => {
    expect(
      taskListRowSubtitle({
        depth: 1,
        hasProject: false,
        projectStepId: undefined,
        promptPreview: "  Do the thing  ",
      }),
    ).toBe("Subtask · Do the thing");
  });

  it("returns undefined when there is nothing to say", () => {
    expect(
      taskListRowSubtitle({
        depth: 0,
        hasProject: false,
        projectStepId: undefined,
        promptPreview: "   ",
      }),
    ).toBeUndefined();
  });
});

describe("statusListLabel", () => {
  it("maps running to in-progress copy", () => {
    expect(statusListLabel("running")).toBe("In progress");
  });
});
