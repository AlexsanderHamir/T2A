import { describe, expect, it } from "vitest";
import { TASK_EVENT_TYPES, type TaskEventType } from "@/types";
import { eventTypeLabel } from "./taskEventLabels";

describe("eventTypeLabel", () => {
  it("maps common types to stable human labels", () => {
    expect(eventTypeLabel("task_created")).toBe("Task created");
    expect(eventTypeLabel("status_changed")).toBe("Status changed");
    expect(eventTypeLabel("approval_requested")).toBe("Approval requested");
  });

  it("defines a distinct label for every registered event type", () => {
    for (const t of TASK_EVENT_TYPES) {
      const label = eventTypeLabel(t);
      expect(label.length).toBeGreaterThan(0);
      expect(label).not.toBe(t);
    }
  });

  it("falls back to the raw string when the type is unknown at runtime", () => {
    expect(eventTypeLabel("totally_unknown" as TaskEventType)).toBe(
      "totally_unknown",
    );
  });
});
