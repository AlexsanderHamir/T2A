import { afterEach, describe, expect, it } from "vitest";
import {
  __resetOptimisticVersionsForTests,
  bumpOptimisticVersion,
  clearOptimisticVersion,
  shouldSuppressSSEFor,
} from "./optimisticVersion";

describe("optimisticVersion", () => {
  afterEach(() => {
    __resetOptimisticVersionsForTests();
  });

  // Baseline: an idle task is never suppressed. Pinning this ensures
  // a refactor that swapped the default from 0 to "always suppress"
  // would surface immediately as a stuck-cache regression.
  it("returns false for tasks with no in-flight mutation", () => {
    expect(shouldSuppressSSEFor("never-touched")).toBe(false);
  });

  // Suppression is ONE-shot per bump. After the SSE handler observes
  // an echo, lastSeen is updated to the current version, so the
  // *next* echo (which arrives after the mutation's onSettled) gets
  // through and lets server truth re-converge.
  it("suppresses exactly one SSE echo per bump, then lets the next through", () => {
    bumpOptimisticVersion("t1");
    expect(shouldSuppressSSEFor("t1")).toBe(true);
    expect(shouldSuppressSSEFor("t1")).toBe(false);
  });

  // Burst safety: if the user fires three rapid edits before any
  // settle, all three echoes must be suppressed. A boolean-based
  // implementation would let the first echo flip the flag back off
  // and silently break the suppression for edits 2 and 3.
  it("counts overlapping mutations so each rapid echo is suppressed", () => {
    bumpOptimisticVersion("t1");
    bumpOptimisticVersion("t1");
    bumpOptimisticVersion("t1");
    expect(shouldSuppressSSEFor("t1")).toBe(true);
    expect(shouldSuppressSSEFor("t1")).toBe(false);
    bumpOptimisticVersion("t1");
    expect(shouldSuppressSSEFor("t1")).toBe(true);
  });

  // clear is symmetric with bump — clearing one of N pending bumps
  // leaves the others active so a settled-but-not-final mutation
  // doesn't disable suppression for siblings still in flight.
  it("clearOptimisticVersion decrements without dropping unrelated bumps", () => {
    bumpOptimisticVersion("t1");
    bumpOptimisticVersion("t1");
    clearOptimisticVersion("t1");
    expect(shouldSuppressSSEFor("t1")).toBe(true);
    clearOptimisticVersion("t1");
    expect(shouldSuppressSSEFor("t1")).toBe(false);
  });
});
