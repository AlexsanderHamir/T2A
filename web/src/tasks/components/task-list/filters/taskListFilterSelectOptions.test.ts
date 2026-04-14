import { describe, expect, it } from "vitest";
import { PRIORITIES, STATUSES, type Priority, type Status } from "@/types";
import {
  isCustomSelectHeader,
  type CustomSelectOption,
} from "../../custom-select";
import {
  TASK_LIST_PRIORITY_FILTER_OPTIONS,
  TASK_LIST_STATUS_FILTER_OPTIONS,
} from "./taskListFilterSelectOptions";
import { statusNeedsUserInput } from "../../../taskStatusNeedsUser";

function optionValues(opts: CustomSelectOption[]): string[] {
  return opts
    .filter((o) => !isCustomSelectHeader(o))
    .map((o) => (o as { value: string }).value);
}

describe("taskListFilterSelectOptions", () => {
  describe("TASK_LIST_STATUS_FILTER_OPTIONS", () => {
    it("starts with All then section headers", () => {
      const o = TASK_LIST_STATUS_FILTER_OPTIONS;
      expect(o[0]).toEqual({ value: "all", label: "All" });
      expect(isCustomSelectHeader(o[1])).toBe(true);
      if (isCustomSelectHeader(o[1])) {
        expect(o[1].label).toBe("Agent needs input");
      }
      const otherHeaderIdx = o.findIndex(
        (x) => isCustomSelectHeader(x) && x.label === "Other activity",
      );
      expect(otherHeaderIdx).toBeGreaterThan(1);
    });

    it("includes every status once plus all", () => {
      const values = optionValues(TASK_LIST_STATUS_FILTER_OPTIONS);
      expect(values.sort()).toEqual(["all", ...STATUSES].sort());
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
      }
    });
  });

  describe("TASK_LIST_PRIORITY_FILTER_OPTIONS", () => {
    it("starts with All then priorities in order", () => {
      expect(TASK_LIST_PRIORITY_FILTER_OPTIONS[0]).toEqual({
        value: "all",
        label: "All",
      });
      const rest = TASK_LIST_PRIORITY_FILTER_OPTIONS.slice(1);
      expect(rest).toHaveLength(PRIORITIES.length);
      rest.forEach((opt, i) => {
        expect(isCustomSelectHeader(opt)).toBe(false);
        const p = PRIORITIES[i] as Priority;
        expect((opt as { value: string }).value).toBe(p);
        expect((opt as { pillClass?: string }).pillClass).toBeTruthy();
      });
    });

    it("exposes one entry per priority plus all", () => {
      const values = optionValues(TASK_LIST_PRIORITY_FILTER_OPTIONS);
      expect(values.sort()).toEqual(["all", ...PRIORITIES].sort());
    });
  });
});
