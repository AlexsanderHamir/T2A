import type { Status } from "@/types";

/** Statuses where the operator may set or clear `pickup_not_before`. */
const PICKUP_SCHEDULE_EDITABLE: ReadonlySet<Status> = new Set([
  "ready",
  "on_hold",
]);

/**
 * Whether `pickup_not_before` may be changed in the edit-task form.
 * Running and in-flight tasks are already picked or executing; terminal
 * tasks will never pick up again.
 */
export function canEditTaskPickupSchedule(status: Status): boolean {
  return PICKUP_SCHEDULE_EDITABLE.has(status);
}
