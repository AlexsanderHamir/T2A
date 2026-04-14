import { describe, expect, it } from "vitest";
import { TASK_EVENT_TYPES, type TaskEventType } from "@/types";
import { eventTypeNeedsUserInput } from "./taskEventNeedsUser";

describe("eventTypeNeedsUserInput", () => {
  it("matches the intended classification for every TaskEventType", () => {
    const needsUser = new Set<TaskEventType>([
      "approval_requested",
      "task_failed",
    ]);
    for (const t of TASK_EVENT_TYPES) {
      expect(eventTypeNeedsUserInput(t)).toBe(needsUser.has(t));
    }
  });
});
