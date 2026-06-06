import { describe, expect, it } from "vitest";
import { TASK_EVENT_TYPES, type TaskEventType } from "@/types";
import { eventDisplayLabel, eventTypeLabel } from "./taskEventLabels";

function ev(type: TaskEventType, data: Record<string, unknown> = {}) {
  return { seq: 1, at: "2026-01-01T12:00:00.000Z", type, by: "agent" as const, data };
}

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

describe("eventDisplayLabel", () => {
  it("includes phase kind for phase mirror events", () => {
    expect(
      eventDisplayLabel(
        ev("phase_started", { phase: "execute", phase_seq: 1 }),
      ),
    ).toBe("Execute started");
    expect(
      eventDisplayLabel(
        ev("phase_completed", { phase: "verify", phase_seq: 2 }),
      ),
    ).toBe("Verify completed");
    expect(
      eventDisplayLabel(
        ev("phase_failed", { phase: "verify", phase_seq: 2 }),
      ),
    ).toBe("Verify failed");
  });

  it("falls back to type label when phase is missing", () => {
    expect(eventDisplayLabel(ev("phase_started", {}))).toBe("Phase started");
  });
});
