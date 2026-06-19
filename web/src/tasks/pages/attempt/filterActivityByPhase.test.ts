import { describe, expect, it } from "vitest";
import type { TaskCycleStreamEvent } from "@/types";
import type { TaskEvent } from "@/types/task";
import {
  filterAuditEventsByPhase,
  filterStreamEventsByPhase,
} from "./filterActivityByPhase";

describe("filterActivityByPhase", () => {
  it("filters stream events by phase_seq", () => {
    const events = [
      { id: "a", phase_seq: 1 },
      { id: "b", phase_seq: 2 },
    ] as TaskCycleStreamEvent[];
    expect(filterStreamEventsByPhase(events, null)).toHaveLength(2);
    expect(filterStreamEventsByPhase(events, 2)).toEqual([
      { id: "b", phase_seq: 2 },
    ]);
  });

  it("filters audit events to phase-scoped rows", () => {
    const events = [
      {
        seq: 1,
        at: "2026-01-01T00:00:00Z",
        type: "cycle_started",
        by: "agent",
        data: { cycle_id: "c1" },
      },
      {
        seq: 2,
        at: "2026-01-01T00:00:01Z",
        type: "phase_started",
        by: "agent",
        data: { cycle_id: "c1", phase_seq: 2 },
      },
      {
        seq: 3,
        at: "2026-01-01T00:00:02Z",
        type: "phase_started",
        by: "agent",
        data: { cycle_id: "c1", phase_seq: 3 },
      },
    ] as TaskEvent[];
    expect(filterAuditEventsByPhase(events, 2)).toEqual([
      {
        seq: 2,
        at: "2026-01-01T00:00:01Z",
        type: "phase_started",
        by: "agent",
        data: { cycle_id: "c1", phase_seq: 2 },
      },
    ]);
  });
});
