import { useEffect, useRef } from "react";
import type { NavigateFunction } from "react-router-dom";

/**
 * When the current task is deleted successfully, replace the route with the parent
 * task or home. Uses a ref so we only navigate once per successful delete for this task.
 */
export function useTaskDetailDeleteNavigate(
  taskId: string,
  navigate: NavigateFunction,
  deleteSuccess: boolean,
  deleteVariables: unknown,
) {
  const navigatedAfterDelete = useRef(false);

  useEffect(() => {
    navigatedAfterDelete.current = false;
  }, [taskId]);

  useEffect(() => {
    if (!taskId || navigatedAfterDelete.current) return;
    const v = deleteVariables;
    if (
      !deleteSuccess ||
      !v ||
      typeof v !== "object" ||
      !("id" in v) ||
      v.id !== taskId
    ) {
      return;
    }
    navigatedAfterDelete.current = true;
    const parent =
      "parent_id" in v && typeof v.parent_id === "string"
        ? v.parent_id.trim()
        : "";
    if (parent) {
      navigate(`/tasks/${encodeURIComponent(parent)}`, { replace: true });
    } else {
      navigate("/", { replace: true });
    }
  }, [taskId, deleteSuccess, deleteVariables, navigate]);
}
