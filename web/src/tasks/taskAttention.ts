import type { Task } from "@/types";

/**
 * Whether agents are waiting on a person soon, from task status and server `approval_pending` on events.
 * Status cases that set `show: true` match `statusNeedsUserInput` in `taskStatusNeedsUser.ts`.
 */
export function userAttention(
  task: Task,
  meta: { approvalPending: boolean },
): {
  show: boolean;
  headline: string;
  body: string;
} {
  if (meta.approvalPending) {
    return {
      show: true,
      headline: "Approval requested",
      body: "The agent is asking for approval on this task. Review the timeline below.",
    };
  }
  switch (task.status) {
    case "review":
      return {
        show: true,
        headline: "Agent may need your review",
        body: "This task is in review. Check the prompt and updates below.",
      };
    case "blocked":
      return {
        show: true,
        headline: "Blocked",
        body: "The agent is blocked. Review context and unblock or adjust the task.",
      };
    case "failed":
      return {
        show: true,
        headline: "Task failed",
        body: "The agent reported a failure. Review what happened and decide whether to retry or change scope.",
      };
    default:
      return { show: false, headline: "", body: "" };
  }
}
