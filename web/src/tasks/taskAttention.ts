import type { Task, TaskEvent } from "@/types";

/** Whether the human should act soon, from status and recent audit events. */
export function userAttention(task: Task, events: TaskEvent[]): {
  show: boolean;
  headline: string;
  body: string;
} {
  if (approvalPending(events)) {
    return {
      show: true,
      headline: "Approval requested",
      body: "Someone asked for your approval on this task. Review the timeline below.",
    };
  }
  switch (task.status) {
    case "review":
      return {
        show: true,
        headline: "Your input may be needed",
        body: "This task is in review. Check the prompt and updates below.",
      };
    case "blocked":
      return {
        show: true,
        headline: "Blocked",
        body: "Progress is blocked. Review context and unblock or adjust the task.",
      };
    case "failed":
      return {
        show: true,
        headline: "Task failed",
        body: "Review what happened and decide whether to retry or change scope.",
      };
    default:
      return { show: false, headline: "", body: "" };
  }
}

function approvalPending(events: TaskEvent[]): boolean {
  for (let i = events.length - 1; i >= 0; i--) {
    const t = events[i].type;
    if (t === "approval_granted") return false;
    if (t === "approval_requested") return true;
  }
  return false;
}
