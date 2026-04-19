import type { TaskEventType } from "@/types";

const LABELS: Record<TaskEventType, string> = {
  task_created: "Task created",
  status_changed: "Status changed",
  priority_changed: "Priority changed",
  prompt_appended: "Prompt updated",
  context_added: "Context added",
  constraint_added: "Constraint added",
  success_criterion_added: "Success criterion added",
  non_goal_added: "Non-goal added",
  plan_added: "Plan added",
  subtask_added: "Subtask added",
  subtask_removed: "Subtask removed",
  checklist_item_added: "Checklist item added",
  checklist_item_toggled: "Checklist item updated",
  checklist_item_updated: "Criterion text updated",
  checklist_item_removed: "Criterion removed",
  checklist_inherit_changed: "Done criteria inheritance changed",
  message_added: "Title or message updated",
  artifact_added: "Artifact added",
  approval_requested: "Approval requested",
  approval_granted: "Approval granted",
  task_completed: "Task completed",
  task_failed: "Task failed",
  cycle_started: "Execution cycle started",
  cycle_completed: "Execution cycle completed",
  cycle_failed: "Execution cycle failed",
  phase_started: "Phase started",
  phase_completed: "Phase completed",
  phase_failed: "Phase failed",
  phase_skipped: "Phase skipped",
  sync_ping: "Live sync check (legacy dev ping)",
};

export function eventTypeLabel(type: TaskEventType): string {
  return LABELS[type] ?? type;
}
