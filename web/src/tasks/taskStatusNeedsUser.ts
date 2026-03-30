import type { Status } from "@/types";

/**
 * Statuses where an agent typically needs something from a person soon
 * (review, unblock, recover). Align with `userAttention` in `taskAttention.ts`.
 * Extend when new Status values imply the agent is waiting on a person.
 */
const NEEDS_USER_INPUT: ReadonlySet<Status> = new Set([
  "blocked",
  "review",
  "failed",
]);

/** True when this task status means agents are likely waiting on a person. */
export function statusNeedsUserInput(status: Status): boolean {
  return NEEDS_USER_INPUT.has(status);
}
