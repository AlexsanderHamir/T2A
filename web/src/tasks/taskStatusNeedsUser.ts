import type { Status } from "@/types";

/**
 * Statuses where the human typically needs to act or pay attention soon
 * (review, unblock, recover). Align with `userAttention` in `taskAttention.ts`.
 * Extend when new Status values imply a user response.
 */
const NEEDS_USER_INPUT: ReadonlySet<Status> = new Set([
  "blocked",
  "review",
  "failed",
]);

/** True when this task status expects something from the user. */
export function statusNeedsUserInput(status: Status): boolean {
  return NEEDS_USER_INPUT.has(status);
}
