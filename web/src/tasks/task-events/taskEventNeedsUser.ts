import type { TaskEventType } from "@/types";

/**
 * Event types where an agent typically needs input, review, or a reply from a person soon.
 * Everything else is informational (state changes, completed outcomes, or FYI).
 *
 * | Type | Agent needs input? | Notes |
 * |------|--------------------|--------|
 * | task_created | no | Task exists; no response required from this row alone |
 * | status_changed, priority_changed | no | State updates; status is classified separately (`task-display/taskStatusNeedsUser.ts`) |
 * | prompt_appended | no | Prompt edits; follow in context |
 * | context_added, constraint_added, success_criterion_added, non_goal_added | no | Structured context/planning; informational |
 * | plan_added, subtask_added | no | Plan structure updates |
 * | message_added | no | Audit of message/title updates; treat as FYI unless you later key off `data` |
 * | artifact_added | no | Artifact recorded; review is optional unless workflow adds a dedicated type |
 * | approval_requested | **yes** | Explicit approval step |
 * | approval_granted | no | Outcome of approval |
 * | task_completed | no | Terminal success |
 * | task_failed | **yes** | Failure should be reviewed (aligns with `failed` status) |
 * | sync_ping | no | Dev / connectivity check |
 *
 * Extend `NEEDS_USER_INPUT` when new `EventType` values require a person to respond to the agent.
 * The task timeline highlights these rows (see `TaskUpdatesTimeline` + `App.css`).
 */
const NEEDS_USER_INPUT: ReadonlySet<TaskEventType> = new Set([
  "approval_requested",
  "task_failed",
]);

/** True when this audit event type means an agent is waiting on a person. */
export function eventTypeNeedsUserInput(type: TaskEventType): boolean {
  return NEEDS_USER_INPUT.has(type);
}
