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
import { CustomSelect, type CustomSelectOption } from "./CustomSelect";
import { TaskPager } from "./TaskPager";
import {
  PRIORITIES,
  STATUSES,
  type Priority,
  type Status,
  type Task,
} from "@/types";
import type { TaskWithDepth } from "../flattenTaskTree";
import { statusNeedsUserInput } from "../taskStatusNeedsUser";
import {
  EmptyState,
  EmptyStateFilterGlyph,
  type EmptyStateAction,
} from "@/shared/EmptyState";

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

const SKELETON_ROW_COUNT = 6;

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

  const statusSelectOptions: CustomSelectOption[] = useMemo(() => {
    const needsUser = STATUSES.filter((s) => statusNeedsUserInput(s));
    const other = STATUSES.filter((s) => !statusNeedsUserInput(s));
    return [
      { value: "all", label: "All" },
      { type: "header", label: "Agent needs input" },
      ...needsUser.map((s) => ({
        value: s,
        label: s,
        pillClass: statusPillClass(s),
      })),
      { type: "header", label: "Other activity" },
      ...other.map((s) => ({
        value: s,
        label: s,
        pillClass: statusPillClass(s),
      })),
    ];
  }, []);

  const prioritySelectOptions: CustomSelectOption[] = useMemo(
    () => [
      { value: "all", label: "All" },
      ...PRIORITIES.map((p) => ({
        value: p,
        label: p,
        pillClass: priorityPillClass(p),
      })),
    ],
    [],
  );

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
        <div
          className="task-list-skeleton task-list-phase-msg"
          role="status"
          aria-busy="true"
          aria-label="Loading tasks"
        >
          <div className="table-wrap task-list-table-wrap">
            <table className="task-list-table task-list-table--skeleton">
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
              <tbody aria-hidden="true">
                {Array.from({ length: SKELETON_ROW_COUNT }, (_, i) => (
                  <tr key={i} className="task-list-skeleton-row">
                    <td>
                      <span className="skeleton-block skeleton-block--title" />
                    </td>
                    <td>
                      <span className="skeleton-block skeleton-block--pill" />
                    </td>
                    <td>
                      <span className="skeleton-block skeleton-block--pill skeleton-block--pill-narrow" />
                    </td>
                    <td>
                      <span className="skeleton-block skeleton-block--prompt" />
                    </td>
                    <td>
                      <div className="task-list-skeleton-actions">
                        <span className="skeleton-block skeleton-block--btn" />
                        <span className="skeleton-block skeleton-block--btn" />
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ) : null}
      {!loading ? (
        <div className="task-list-content task-list-content--enter">
          <div
            className="task-list-filters"
            role="search"
            aria-label="Filter tasks"
          >
            <div className="task-list-filter-field">
              <CustomSelect
                id="task-list-filter-status"
                label="Status"
                compact
                listboxName="Filter by status"
                value={statusFilter}
                options={statusSelectOptions}
                onChange={(v) => setStatusFilter(v as StatusFilter)}
              />
            </div>
            <div className="task-list-filter-field">
              <CustomSelect
                id="task-list-filter-priority"
                label="Priority"
                compact
                listboxName="Filter by priority"
                value={priorityFilter}
                options={prioritySelectOptions}
                onChange={(v) => setPriorityFilter(v as PriorityFilter)}
              />
            </div>
            <div className="field grow task-list-search-field">
              <label htmlFor="task-list-search-title">Search titles</label>
              <input
                id="task-list-search-title"
                type="search"
                value={titleSearch}
                onChange={(e) => setTitleSearch(e.target.value)}
                placeholder="Search by title…"
                autoComplete="off"
              />
            </div>
          </div>
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
