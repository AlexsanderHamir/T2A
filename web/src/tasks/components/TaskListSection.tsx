import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
} from "react";
import { Link } from "react-router-dom";
import { useDelayedTrue } from "@/lib/useDelayedTrue";
import { previewTextFromPrompt } from "../promptFormat";
import { priorityPillClass, statusPillClass } from "../taskPillClasses";
import { TaskListFilters } from "./TaskListFilters";
import { TaskPager } from "./TaskPager";
import type { Priority, Status, Task } from "@/types";
import type { TaskWithDepth } from "../flattenTaskTree";
import { statusNeedsUserInput } from "../taskStatusNeedsUser";
import {
  EmptyState,
  EmptyStateFilterGlyph,
  type EmptyStateAction,
} from "@/shared/EmptyState";
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
          <div className="table-wrap task-list-table-wrap">
            <table className="task-list-table" aria-busy={refreshing}>
              <caption className="visually-hidden">
                {TASK_LIST_TABLE_CAPTION}
              </caption>
              <thead>
                <tr>
                  <th scope="col">Title</th>
                  <th scope="col">Status</th>
                  <th scope="col">Priority</th>
                  <th scope="col">Prompt</th>
                  <th scope="col">Actions</th>
                </tr>
              </thead>
              <tbody className="task-list-tbody">
                {tasks.length === 0 ? (
                  <tr className="task-list-empty-row">
                    <td colSpan={5} className="task-list-empty-cell">
                      <EmptyState
                        className="empty-state--in-table"
                        title="No tasks yet"
                        description={
                          <>
                            Use <strong>New task</strong> above to add your first
                            task. Status, priority, and prompt previews appear
                            here.
                          </>
                        }
                        action={emptyListAction}
                      />
                    </td>
                  </tr>
                ) : filteredTasks.length === 0 ? (
                  <tr className="task-list-empty-row">
                    <td colSpan={5} className="task-list-empty-cell">
                      <EmptyState
                        className="empty-state--in-table"
                        icon={<EmptyStateFilterGlyph />}
                        title="No matching tasks"
                        description="Try a different status or priority, or clear the title search."
                      />
                    </td>
                  </tr>
                ) : (
                  filteredTasks.map((t) => {
                    const promptPreview = previewTextFromPrompt(
                      t.initial_prompt,
                    );
                    return (
                      <tr key={t.id} className="task-list-row">
                        <td className="cell-title">
                          <Link
                            to={`/tasks/${t.id}`}
                            className={
                              t.depth > 0
                                ? "cell-title-link cell-title-link--tree"
                                : "cell-title-link"
                            }
                            aria-label={`Open task details: ${t.title}`}
                            style={
                              t.depth > 0
                                ? ({
                                    "--task-list-tree-depth": String(t.depth),
                                  } as CSSProperties)
                                : undefined
                            }
                          >
                            {t.depth > 0 ? (
                              <span className="task-subtask-marker" aria-hidden>
                                └{" "}
                              </span>
                            ) : null}
                            <span className="cell-title-text">{t.title}</span>
                            <span
                              className="cell-title-open-hint"
                              aria-hidden="true"
                            >
                              →
                            </span>
                          </Link>
                        </td>
                        <td>
                          <span
                            className={statusPillClass(t.status)}
                            data-needs-user={
                              statusNeedsUserInput(t.status)
                                ? "true"
                                : undefined
                            }
                          >
                            {t.status}
                          </span>
                        </td>
                        <td>
                          <span className={priorityPillClass(t.priority)}>
                            {t.priority}
                          </span>
                        </td>
                        <td>
                          <div className="prompt-preview" title={promptPreview}>
                            {promptPreview || "—"}
                          </div>
                        </td>
                        <td>
                          <div className="actions">
                            <button
                              type="button"
                              className="secondary btn-table"
                              aria-label={`Edit task "${t.title}"`}
                              onClick={() => onEdit(t)}
                              disabled={saving}
                            >
                              Edit
                            </button>
                            <button
                              type="button"
                              className="danger btn-table"
                              aria-label={`Delete task "${t.title}"`}
                              onClick={() => onRequestDelete(t)}
                              disabled={saving}
                            >
                              Delete
                            </button>
                          </div>
                        </td>
                      </tr>
                    );
                  })
                )}
              </tbody>
            </table>
          </div>
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
