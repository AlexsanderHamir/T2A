import type { Status } from "@/types";

/**
 * Secondary line under the task title: step hint, subtask scope, or a
 * one-line prompt preview. When `hasProject` is true, the project name
 * lives in the Project column — this string must not repeat it.
 */
export function taskListRowSubtitle(input: {
  depth: number;
  /** True when `project_id` is set and the project label resolves (badge column). */
  hasProject: boolean;
  projectStepId?: string;
  promptPreview: string;
}): string | undefined {
  const { depth, hasProject, projectStepId, promptPreview } = input;
  const pv = promptPreview.replace(/\s+/g, " ").trim();
  const tail = pv.length > 80 ? `${pv.slice(0, 77)}…` : pv;

  if (hasProject && projectStepId) {
    return "Step";
  }
  if (hasProject && depth > 0) {
    return "Subtask";
  }
  if (hasProject) {
    return undefined;
  }
  if (depth > 0 && tail) {
    return `Subtask · ${tail}`;
  }
  if (depth > 0) {
    return "Subtask";
  }
  if (tail) {
    return tail;
  }
  return undefined;
}

export function statusListLabel(status: Status): string {
  switch (status) {
    case "ready":
      return "Ready";
    case "running":
      return "In progress";
    case "blocked":
      return "Blocked";
    case "review":
      return "Review";
    case "done":
      return "Done";
    case "failed":
      return "Failed";
  }
}
