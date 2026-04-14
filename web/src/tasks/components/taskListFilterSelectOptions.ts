import type { CustomSelectOption } from "./customSelectModel";
import { PRIORITIES, STATUSES } from "@/types";
import { priorityPillClass, statusPillClass } from "../taskPillClasses";
import { statusNeedsUserInput } from "../taskStatusNeedsUser";

const needsUserStatuses = STATUSES.filter((s) => statusNeedsUserInput(s));
const otherStatuses = STATUSES.filter((s) => !statusNeedsUserInput(s));

/** Options for the task list status filter `CustomSelect`. */
export const TASK_LIST_STATUS_FILTER_OPTIONS: CustomSelectOption[] = [
  { value: "all", label: "All" },
  { type: "header", label: "Agent needs input" },
  ...needsUserStatuses.map((s) => ({
    value: s,
    label: s,
    pillClass: statusPillClass(s),
  })),
  { type: "header", label: "Other activity" },
  ...otherStatuses.map((s) => ({
    value: s,
    label: s,
    pillClass: statusPillClass(s),
  })),
];

/** Options for the task list priority filter `CustomSelect`. */
export const TASK_LIST_PRIORITY_FILTER_OPTIONS: CustomSelectOption[] = [
  { value: "all", label: "All" },
  ...PRIORITIES.map((p) => ({
    value: p,
    label: p,
    pillClass: priorityPillClass(p),
  })),
];
