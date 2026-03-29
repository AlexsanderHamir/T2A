import type { Priority, Status } from "@/types";

/** Table/detail pills: distinct hues per status. */
export function statusPillClass(status: Status): string {
  return `cell-pill cell-pill--status cell-pill--status-${status}`;
}

/** Table/detail pills: distinct hues per priority (escalation). */
export function priorityPillClass(priority: Priority): string {
  return `cell-pill cell-pill--priority cell-pill--priority-${priority}`;
}
