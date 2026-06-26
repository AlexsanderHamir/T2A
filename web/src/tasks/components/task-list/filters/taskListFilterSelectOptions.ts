import type { CustomSelectOption } from "@/components/custom-select";
import { PRIORITIES, STATUSES } from "@/types";
import { priorityPillClass, statusNeedsUserInput, statusPillClass } from "../../../task-display";
import { statusListLabel } from "../table/taskListRowSubtitle";

const needsUserStatuses = STATUSES.filter((s) => statusNeedsUserInput(s));
const otherStatuses = STATUSES.filter((s) => !statusNeedsUserInput(s));

function priorityFilterLabel(priority: (typeof PRIORITIES)[number]): string {
  return priority.charAt(0).toUpperCase() + priority.slice(1);
}

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
 *
 * Real statuses use semantic pills (same hues as the table). Synthetic
 * `scheduled` and section headers stay plain text.
 */
export const TASK_LIST_STATUS_FILTER_OPTIONS: CustomSelectOption[] = [
  { value: "all", label: "All statuses" },
  { value: "scheduled", label: "Scheduled (deferred)" },
  { type: "header", label: "Agent needs input" },
  ...needsUserStatuses.map((s) => ({
    value: s,
    label: statusListLabel(s),
    pillClass: statusPillClass(s),
  })),
  { type: "header", label: "Other activity" },
  ...otherStatuses.map((s) => ({
    value: s,
    label: statusListLabel(s),
    pillClass: statusPillClass(s),
  })),
];

/** Status filter options, optionally omitting the synthetic scheduled bucket. */
export function taskListStatusFilterOptions(opts?: {
  includeScheduled?: boolean;
}): CustomSelectOption[] {
  if (opts?.includeScheduled !== false) {
    return TASK_LIST_STATUS_FILTER_OPTIONS;
  }
  return TASK_LIST_STATUS_FILTER_OPTIONS.filter(
    (option) => !("value" in option) || option.value !== "scheduled",
  );
}

/** Options for the task list priority filter `CustomSelect`. */
export const TASK_LIST_PRIORITY_FILTER_OPTIONS: CustomSelectOption[] = [
  { value: "all", label: "All priorities" },
  ...PRIORITIES.map((p) => ({
    value: p,
    label: priorityFilterLabel(p),
    pillClass: priorityPillClass(p),
  })),
];
