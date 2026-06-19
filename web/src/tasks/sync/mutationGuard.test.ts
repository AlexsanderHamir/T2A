import { afterEach, describe, expect, it } from "vitest";
import {
  __resetMutationGuardForTests,
  beginTaskMutationGuard,
  endTaskMutationGuard,
  shouldSuppressTaskMutationEcho,
} from "./mutationGuard";

describe("mutationGuard", () => {
  afterEach(() => {
    __resetMutationGuardForTests();
  });

  it("returns false for tasks with no in-flight mutation", () => {
    expect(shouldSuppressTaskMutationEcho("never-touched")).toBe(false);
  });

  it("suppresses exactly one SSE echo per bump, then lets the next through", () => {
    beginTaskMutationGuard("t1");
    expect(shouldSuppressTaskMutationEcho("t1")).toBe(true);
    expect(shouldSuppressTaskMutationEcho("t1")).toBe(false);
  });

  it("counts overlapping mutations so each rapid echo is suppressed", () => {
    beginTaskMutationGuard("t1");
    beginTaskMutationGuard("t1");
    beginTaskMutationGuard("t1");
    expect(shouldSuppressTaskMutationEcho("t1")).toBe(true);
    expect(shouldSuppressTaskMutationEcho("t1")).toBe(false);
    beginTaskMutationGuard("t1");
    expect(shouldSuppressTaskMutationEcho("t1")).toBe(true);
  });

  it("endTaskMutationGuard decrements without dropping unrelated bumps", () => {
    beginTaskMutationGuard("t1");
    beginTaskMutationGuard("t1");
    endTaskMutationGuard("t1");
    expect(shouldSuppressTaskMutationEcho("t1")).toBe(true);
    endTaskMutationGuard("t1");
    expect(shouldSuppressTaskMutationEcho("t1")).toBe(false);
  });
});
