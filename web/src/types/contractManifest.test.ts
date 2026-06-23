import { describe, expect, it } from "vitest";
import { RUM_FORWARD_COMPAT_TYPES, RUM_PROMOTED_TYPES } from "@/api/rum";
import {
  SSE_CHANGE_TYPE,
  SSE_CHANGE_TYPES,
  TASK_EVENT_TYPES,
} from "@/types";

/** Removed domain values — must never reappear in TASK_EVENT_TYPES (ADR-0011, etc.). */
const REMOVED_EVENT_TYPES = ["subtask_added", "task_type"] as const;

describe("cross-stack contract manifests", () => {
  it("TASK_EVENT_TYPES matches production EventType count (pkgs/tasks/domain/event_types_manifest_test.go)", () => {
    expect(TASK_EVENT_TYPES).toHaveLength(29);
  });

  it("TASK_EVENT_TYPES excludes intentionally removed event types", () => {
    for (const removed of REMOVED_EVENT_TYPES) {
      expect(TASK_EVENT_TYPES as readonly string[]).not.toContain(removed);
    }
  });

  it("SSE_CHANGE_TYPES mirrors realtime/wire.go (14 ChangeType values)", () => {
    expect(SSE_CHANGE_TYPES).toHaveLength(14);
    expect(new Set(SSE_CHANGE_TYPES).size).toBe(14);
    expect(SSE_CHANGE_TYPES).toEqual(Object.values(SSE_CHANGE_TYPE));
  });

  it("RUM_PROMOTED_TYPES mirrors handler validRUMTypes (excludes navigation_timing)", () => {
    expect(RUM_PROMOTED_TYPES).toHaveLength(7);
    expect(RUM_FORWARD_COMPAT_TYPES).toEqual(["navigation_timing"]);
    expect(RUM_PROMOTED_TYPES as readonly string[]).not.toContain("navigation_timing");
  });
});
