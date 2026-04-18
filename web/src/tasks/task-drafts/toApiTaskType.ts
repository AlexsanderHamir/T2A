import type { TaskType } from "@/types";

/**
 * Map a UI task type to the wire `task_type` for POST /tasks and POST
 * /tasks/draft-evaluations.
 *
 * "dmap" is a UI-only mode: the agent persists it as a `general` task whose
 * `initial_prompt` carries the DMAP setup block (see `buildDmapPrompt`). The
 * server has no `dmap` enum value, so anything outside the create-flow surface
 * (tables, detail pages) only ever sees `general`. Keeping the mapping in one
 * place prevents drift between callers.
 */
export function toApiTaskType(taskType: TaskType): TaskType {
  return taskType === "dmap" ? "general" : taskType;
}
