export function taskCreateModalBusyLabel(
  hasParent: boolean,
  pendingSubtasksCount: number,
): string {
  if (hasParent) return "Creating subtask…";
  if (pendingSubtasksCount > 0) return "Creating task and subtasks…";
  return "Creating task…";
}
