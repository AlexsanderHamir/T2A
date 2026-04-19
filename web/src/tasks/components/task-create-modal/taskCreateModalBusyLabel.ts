export function taskCreateModalBusyLabel(pendingSubtasksCount: number): string {
  if (pendingSubtasksCount > 0) return "Creating task and subtasks…";
  return "Creating task…";
}
