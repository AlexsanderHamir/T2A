import { describe, expect, it } from "vitest";
import { taskCreateModalBusyLabel } from "./taskCreateModalBusyLabel";

describe("taskCreateModalBusyLabel", () => {
  it("returns subtask copy when creating under a parent", () => {
    expect(taskCreateModalBusyLabel(true, 0)).toBe("Creating subtask…");
    expect(taskCreateModalBusyLabel(true, 3)).toBe("Creating subtask…");
  });

  it("mentions subtasks when top-level create has pending subtasks", () => {
    expect(taskCreateModalBusyLabel(false, 1)).toBe(
      "Creating task and subtasks…",
    );
  });

  it("returns top-level-only copy when there are no pending subtasks", () => {
    expect(taskCreateModalBusyLabel(false, 0)).toBe("Creating task…");
  });
});
