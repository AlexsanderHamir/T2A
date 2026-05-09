import type { Priority, Status } from "@/types";

/** Table/detail pills: distinct hues per status. */
export function statusPillClass(status: Status): string {
  return `cell-pill cell-pill--status cell-pill--status-${status}`;
}

/** Table/detail pills: distinct hues per priority (escalation). */
export function priorityPillClass(priority: Priority): string {
  return `cell-pill cell-pill--priority cell-pill--priority-${priority}`;
}

/** Home list: single dot; pair with `title` / `aria-label` for the word. */
export function priorityDotClass(priority: Priority): string {
  return `task-list-priority-dot task-list-priority-dot--${priority}`;
}
