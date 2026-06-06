import { describe, expect, it } from "vitest";
import type { TaskEvent } from "@/types";
import {
  formatAttemptAuditPreview,
  formatEventSummaryCompact,
  formatPhaseSummaryCompact,
  resolveAttemptAuditRightColumn,
} from "./taskEventSummary";

function ev(
  partial: Pick<TaskEvent, "type" | "data"> & Partial<TaskEvent>,
): TaskEvent {
  return {
    seq: 1,
    at: "2026-01-01T12:00:00.000Z",
    by: "agent",
    ...partial,
  };
}

describe("formatEventSummaryCompact", () => {
  it("formats status transitions without JSON", () => {
    expect(
      formatEventSummaryCompact(
        ev({
          type: "status_changed",
          data: { from: "running", to: "done" },
        }),
      ),
    ).toBe("running → done");
  });

  it("omits bare terminal status when the event label already conveys outcome", () => {
    expect(
      formatEventSummaryCompact(
        ev({
          type: "cycle_completed",
          data: {
            status: "succeeded",
            cycle_id: "c1",
            attempt_seq: 1,
          },
        }),
      ),
    ).toBeNull();
    expect(
      formatEventSummaryCompact(
        ev({
          type: "phase_completed",
          data: {
            phase: "execute",
            status: "succeeded",
            cycle_id: "c1",
            phase_seq: 2,
          },
        }),
      ),
    ).toBeNull();
  });

  it("surfaces cycle failure reason when present", () => {
    expect(
      formatEventSummaryCompact(
        ev({
          type: "cycle_failed",
          data: {
            status: "failed",
            cycle_id: "c1",
            attempt_seq: 1,
            reason: "execute phase timed out",
          },
        }),
      ),
    ).toBe("execute phase timed out");
  });

  it("returns null when there is nothing human-readable to show", () => {
    expect(
      formatEventSummaryCompact(
        ev({
          type: "sync_ping",
          data: { cycle_id: "c1" },
        }),
      ),
    ).toBeNull();
  });
});

describe("formatPhaseSummaryCompact", () => {
  it("strips markdown headers and inline code from the first line", () => {
    expect(
      formatPhaseSummaryCompact(
        "## Hottest path: `GET /tasks` (`Handler.list`)\n\nThis is the highest-traffic read.",
      ),
    ).toBe("Hottest path: GET /tasks (Handler.list)");
  });

  it("returns null for blank summaries", () => {
    expect(formatPhaseSummaryCompact("   ")).toBeNull();
  });
});

describe("formatAttemptAuditPreview", () => {
  it("shows phase sequence instead of long summaries for lifecycle rows", () => {
    expect(
      formatAttemptAuditPreview(
        ev({
          type: "phase_completed",
          data: {
            phase: "execute",
            phase_seq: 2,
            status: "succeeded",
            summary: "## Hottest path: GET /tasks",
          },
        }),
      ),
    ).toBe("PHASE 2");
  });

  it("keeps transition summaries for non-phase events", () => {
    expect(
      formatAttemptAuditPreview(
        ev({
          type: "status_changed",
          data: { from: "running", to: "done" },
        }),
      ),
    ).toBe("running → done");
  });
});

describe("resolveAttemptAuditRightColumn", () => {
  it("shows phase sequence for lifecycle rows", () => {
    expect(
      resolveAttemptAuditRightColumn(
        ev({
          type: "phase_started",
          data: { phase: "verify", phase_seq: 3 },
        }),
      ),
    ).toEqual({
      label: "PHASE 3",
      variant: "phase",
      ariaLabel: "Phase 3",
    });
  });

  it("shows detail text when a compact summary exists", () => {
    expect(
      resolveAttemptAuditRightColumn(
        ev({
          type: "status_changed",
          data: { from: "running", to: "done" },
        }),
      ),
    ).toEqual({
      label: "running → done",
      variant: "detail",
      title: "running → done",
    });
  });

  it("shows cycle scope when there is no preview text", () => {
    expect(
      resolveAttemptAuditRightColumn(
        ev({
          type: "cycle_started",
          data: { cycle_id: "c1", attempt_seq: 1 },
        }),
      ),
    ).toEqual({
      label: "CYCLE",
      variant: "scope",
      tone: "cycle",
      title: "Applies to the whole execution attempt",
      ariaLabel: "Whole attempt",
    });
  });

  it("shows checklist scope for checklist mutations", () => {
    expect(
      resolveAttemptAuditRightColumn(
        ev({
          type: "checklist_item_toggled",
          data: { cycle_id: "c1", text: "Ship tests" },
        }),
      ),
    ).toEqual({
      label: "CHECKLIST",
      variant: "scope",
      tone: "checklist",
      title: "Done criteria or checklist change",
      ariaLabel: "Checklist",
    });
  });

  it("keeps the CYCLE scope badge on cycle_failed even when a reason exists", () => {
    // Cycle lifecycle rows must look structurally identical regardless of
    // outcome: failure detail lives on the event detail page so the
    // timeline row stays as scannable as cycle_started/cycle_completed.
    expect(
      resolveAttemptAuditRightColumn(
        ev({
          type: "cycle_failed",
          data: {
            status: "failed",
            cycle_id: "c1",
            attempt_seq: 1,
            reason: "execute phase timed out",
          },
        }),
      ),
    ).toEqual({
      label: "CYCLE",
      variant: "scope",
      tone: "cycle",
      title: "Applies to the whole execution attempt",
      ariaLabel: "Whole attempt",
    });
  });
});
