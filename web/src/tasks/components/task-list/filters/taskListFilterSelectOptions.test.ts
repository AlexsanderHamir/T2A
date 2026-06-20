import { describe, expect, it } from "vitest";
import { PRIORITIES, STATUSES, type Priority, type Status } from "@/types";
import {
  isCustomSelectHeader,
  type CustomSelectOption,
} from "../../custom-select";
import {
  TASK_LIST_PRIORITY_FILTER_OPTIONS,
  TASK_LIST_STATUS_FILTER_OPTIONS,
  taskListStatusFilterOptions,
} from "./taskListFilterSelectOptions";
import { statusNeedsUserInput } from "../../../task-display";

function optionValues(opts: CustomSelectOption[]): string[] {
  return opts
    .filter((o) => !isCustomSelectHeader(o))
    .map((o) => (o as { value: string }).value);
}

describe("taskListFilterSelectOptions", () => {
  describe("TASK_LIST_STATUS_FILTER_OPTIONS", () => {
    it("starts with All then the synthetic Scheduled bucket then section headers", () => {
      const o = TASK_LIST_STATUS_FILTER_OPTIONS;
      expect(o[0]).toEqual({ value: "all", label: "All statuses" });
      expect(o[1]).toEqual({ value: "scheduled", label: "Scheduled (deferred)" });
      expect(isCustomSelectHeader(o[2])).toBe(true);
      if (isCustomSelectHeader(o[2])) {
        expect(o[2].label).toBe("Agent needs input");
      }
      const otherHeaderIdx = o.findIndex(
        (x) => isCustomSelectHeader(x) && x.label === "Other activity",
      );
      expect(otherHeaderIdx).toBeGreaterThan(2);
    });

    it("includes every status once plus all and the synthetic scheduled bucket", () => {
      const values = optionValues(TASK_LIST_STATUS_FILTER_OPTIONS);
      expect(values.sort()).toEqual(["all", "scheduled", ...STATUSES].sort());
    });

    it("lists needs-user statuses before the other-activity header", () => {
      const o = TASK_LIST_STATUS_FILTER_OPTIONS;
      const otherHeaderIdx = o.findIndex(
        (x) => isCustomSelectHeader(x) && x.label === "Other activity",
      );
      const needsUser: Status[] = STATUSES.filter((s) =>
        statusNeedsUserInput(s),
      );
      for (const s of needsUser) {
        const idx = o.findIndex(
          (x) => !isCustomSelectHeader(x) && (x as { value: string }).value === s,
        );
        expect(idx).toBeGreaterThan(0);
        expect(idx).toBeLessThan(otherHeaderIdx);
        const opt = o[idx] as { pillClass?: string };
        expect(opt.pillClass).toBe(
          `cell-pill cell-pill--status cell-pill--status-${s}`,
        );
      }
    });
  });

  describe("taskListStatusFilterOptions", () => {
    it("drops the synthetic scheduled bucket when includeScheduled is false", () => {
      const values = optionValues(
        taskListStatusFilterOptions({ includeScheduled: false }),
      );
      expect(values).not.toContain("scheduled");
      expect(values.sort()).toEqual(["all", ...STATUSES].sort());
    });
  });

  describe("TASK_LIST_PRIORITY_FILTER_OPTIONS", () => {
    it("starts with All then priorities in order", () => {
      expect(TASK_LIST_PRIORITY_FILTER_OPTIONS[0]).toEqual({
        value: "all",
        label: "All priorities",
      });
      const rest = TASK_LIST_PRIORITY_FILTER_OPTIONS.slice(1);
      expect(rest).toHaveLength(PRIORITIES.length);
      rest.forEach((opt, i) => {
        expect(isCustomSelectHeader(opt)).toBe(false);
        const p = PRIORITIES[i] as Priority;
        expect((opt as { value: string }).value).toBe(p);
        expect((opt as { label: string }).label).toBe(
          p.charAt(0).toUpperCase() + p.slice(1),
        );
        expect((opt as { pillClass?: string }).pillClass).toBe(
          `cell-pill cell-pill--priority cell-pill--priority-${p}`,
        );
      });
    });

    it("exposes one entry per priority plus all", () => {
      const values = optionValues(TASK_LIST_PRIORITY_FILTER_OPTIONS);
      expect(values.sort()).toEqual(["all", ...PRIORITIES].sort());
    });
  });
});
