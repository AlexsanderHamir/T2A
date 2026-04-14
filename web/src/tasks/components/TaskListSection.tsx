import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { useDelayedTrue } from "@/lib/useDelayedTrue";
import { TaskListDataTable } from "./TaskListDataTable";
import { TaskListFilters } from "./TaskListFilters";
import { TaskPager } from "./TaskPager";
import type { Priority, Status, Task } from "@/types";
import type { TaskWithDepth } from "../flattenTaskTree";
import type { EmptyStateAction } from "@/shared/EmptyState";
import { TaskListTableSkeleton } from "./TaskListTableSkeleton";

type Props = {
  tasks: TaskWithDepth[];
  /** Root tasks returned for this list page (for pager copy; rows may include nested subtasks). */
  rootTasksOnPage: number;
  loading: boolean;
  /** Background refetch in progress (list still visible). */
  refreshing: boolean;
  /** A create/update/delete request is in flight. */
  saving: boolean;
  /**
   * When true, hide the background “Syncing with server…” line (e.g. live SSE
   * already drives refetches; avoids duplicate status with the header).
   */
  hideBackgroundRefreshHint?: boolean;
  /** Zero-based server list page (see `GET /tasks` offset). */
  listPage: number;
  listPageSize: number;
  onListPageChange: (page: number) => void;
  /** Reset to first server page when filters change. */
  onListFiltersChange: () => void;
  hasNextPage: boolean;
  hasPrevPage: boolean;
  /**
   * When true (default), the loading line waits briefly before appearing. Set false in tests.
   * List “syncing” is smoothed in `useTasksApp` (hysteresis on refetch).
   */
  smoothTransitions?: boolean;
  onEdit: (t: Task) => void;
  /** Opens in-app delete confirmation (do not call `window.confirm` from the table). */
  onRequestDelete: (t: Task) => void;
  /** Primary action when the server returned no tasks (e.g. open create modal). */
  emptyListAction?: EmptyStateAction;
  /** Optional toolbar on the title row (e.g. home “New task”). */
  actions?: ReactNode;
};

type StatusFilter = "all" | Status;
type PriorityFilter = "all" | Priority;

const LOADING_STATUS_DELAY_MS = 220;

const TASK_LIST_TABLE_CAPTION =
  "All tasks: title, status, priority, prompt preview, and row actions.";

export function TaskListSection({
  tasks,
  rootTasksOnPage,
  loading,
  refreshing,
  saving,
  hideBackgroundRefreshHint = false,
  listPage,
  listPageSize,
  onListPageChange,
  onListFiltersChange,
  hasNextPage,
  hasPrevPage,
  smoothTransitions = true,
  onEdit,
  onRequestDelete,
  emptyListAction,
  actions,
}: Props) {
  const statusDelayMs = smoothTransitions ? LOADING_STATUS_DELAY_MS : 0;
  const showLoadingLine = useDelayedTrue(loading, statusDelayMs);

  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [priorityFilter, setPriorityFilter] = useState<PriorityFilter>("all");
  const [titleSearch, setTitleSearch] = useState("");

  const filteredTasks = useMemo(() => {
    const q = titleSearch.trim().toLowerCase();
    return tasks.filter((t) => {
      if (statusFilter !== "all" && t.status !== statusFilter) return false;
      if (priorityFilter !== "all" && t.priority !== priorityFilter)
        return false;
      if (q && !t.title.toLowerCase().includes(q)) return false;
      return true;
    });
  }, [tasks, statusFilter, priorityFilter, titleSearch]);

  const skipFiltersResetOnMount = useRef(true);
  useEffect(() => {
    if (skipFiltersResetOnMount.current) {
      skipFiltersResetOnMount.current = false;
      return;
    }
    onListFiltersChange();
  }, [statusFilter, priorityFilter, titleSearch, onListFiltersChange]);

  const showTaskPager =
    !loading && (hasPrevPage || hasNextPage || tasks.length === listPageSize);

  return (
    <section className="panel" aria-labelledby="task-list-heading">
      {actions ? (
        <div className="task-list-section-head">
          <h2 id="task-list-heading">All tasks</h2>
          <div className="task-list-section-actions">{actions}</div>
        </div>
      ) : (
        <h2 id="task-list-heading">All tasks</h2>
      )}
      {refreshing && !loading && !hideBackgroundRefreshHint ? (
        <p className="sync-hint task-list-phase-msg" aria-live="polite" role="status">
          Syncing with server…
        </p>
      ) : null}
      {loading && showLoadingLine ? (
        <TaskListTableSkeleton caption={TASK_LIST_TABLE_CAPTION} />
      ) : null}
      {!loading ? (
        <div className="task-list-content task-list-content--enter">
          <TaskListFilters
            statusFilter={statusFilter}
            onStatusFilterChange={(v) => setStatusFilter(v as StatusFilter)}
            priorityFilter={priorityFilter}
            onPriorityFilterChange={(v) =>
              setPriorityFilter(v as PriorityFilter)
            }
            titleSearch={titleSearch}
            onTitleSearchChange={setTitleSearch}
          />
          <TaskListDataTable
            caption={TASK_LIST_TABLE_CAPTION}
            refreshing={refreshing}
            tasks={tasks}
            filteredTasks={filteredTasks}
            saving={saving}
            emptyListAction={emptyListAction}
            onEdit={onEdit}
            onRequestDelete={onRequestDelete}
          />
          {showTaskPager ? (
            <TaskPager
              navLabel="Task list pages"
              summary={
                tasks.length === 0
                  ? `Page ${listPage + 1} (no tasks on this page)`
                  : (() => {
                      const start = listPage * listPageSize + 1;
                      const end = listPage * listPageSize + rootTasksOnPage;
                      return `${start}–${end}${hasNextPage ? "+" : ""}`;
                    })()
              }
              onPrev={() => onListPageChange(listPage - 1)}
              onNext={() => onListPageChange(listPage + 1)}
              disablePrev={!hasPrevPage}
              disableNext={!hasNextPage}
            />
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
