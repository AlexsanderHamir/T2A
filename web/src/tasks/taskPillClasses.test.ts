import { describe, expect, it } from "vitest";
import { PRIORITIES, STATUSES } from "@/types";
import { priorityPillClass, statusPillClass } from "./taskPillClasses";

describe("taskPillClasses", () => {
  it("maps each status to a stable pill class token", () => {
    for (const s of STATUSES) {
      expect(statusPillClass(s)).toBe(
        `cell-pill cell-pill--status cell-pill--status-${s}`,
      );
    }
  });

  it("maps each priority to a stable pill class token", () => {
    for (const p of PRIORITIES) {
      expect(priorityPillClass(p)).toBe(
        `cell-pill cell-pill--priority cell-pill--priority-${p}`,
      );
    }
  });
});
