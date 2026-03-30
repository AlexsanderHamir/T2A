import { describe, expect, it } from "vitest";
import { statusNeedsUserInput } from "./taskStatusNeedsUser";

describe("statusNeedsUserInput", () => {
  it("is true for statuses that typically need human action", () => {
    expect(statusNeedsUserInput("blocked")).toBe(true);
    expect(statusNeedsUserInput("review")).toBe(true);
    expect(statusNeedsUserInput("failed")).toBe(true);
  });

  it("is false for informational workflow statuses", () => {
    expect(statusNeedsUserInput("ready")).toBe(false);
    expect(statusNeedsUserInput("running")).toBe(false);
    expect(statusNeedsUserInput("done")).toBe(false);
  });
});
