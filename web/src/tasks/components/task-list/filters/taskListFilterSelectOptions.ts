import type { CustomSelectOption } from "../../custom-select";
import { PRIORITIES, STATUSES } from "@/types";
import {
  priorityPillClass,
  statusNeedsUserInput,
  statusPillClass,
} from "../../../task-display";

const needsUserStatuses = STATUSES.filter((s) => statusNeedsUserInput(s));
const otherStatuses = STATUSES.filter((s) => !statusNeedsUserInput(s));

/**
 * Options for the task list status filter `CustomSelect`.
 *
 * The `scheduled` entry is a *synthetic* bucket — not a real
 * `Status` value — surfacing tasks where `status === "ready" &&
 * pickup_not_before > now`. It lives next to the real statuses
 * because operators reach for it the same way ("show me what's
 * deferred"), even though strictly speaking it's a cross-cutting
 * filter rather than a status. See `taskListClientFilter.ts` for
 * the matching predicate.
 */
export const TASK_LIST_STATUS_FILTER_OPTIONS: CustomSelectOption[] = [
  { value: "all", label: "All" },
  { value: "scheduled", label: "Scheduled (deferred)" },
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
