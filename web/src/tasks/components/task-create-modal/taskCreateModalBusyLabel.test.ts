import { describe, expect, it } from "vitest";
import { taskCreateModalBusyLabel } from "./taskCreateModalBusyLabel";

describe("taskCreateModalBusyLabel", () => {
  it("mentions subtasks when there are pending subtasks", () => {
    expect(taskCreateModalBusyLabel(1)).toBe("Creating task and subtasks…");
    expect(taskCreateModalBusyLabel(3)).toBe("Creating task and subtasks…");
  });

  it("returns top-level-only copy when there are no pending subtasks", () => {
    expect(taskCreateModalBusyLabel(0)).toBe("Creating task…");
  });
});
