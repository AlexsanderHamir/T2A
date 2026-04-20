import type { Task } from "@/types/task";

/** Required `Task` fields commonly reused in unit tests; align with API wire shape. */
export const TASK_TEST_DEFAULTS: Pick<Task, "runner" | "cursor_model"> = {
  runner: "cursor",
  cursor_model: "",
};
