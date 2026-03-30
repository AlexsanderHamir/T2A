import { useEffect, useMemo, useRef, useState } from "react";
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
import { statusNeedsUserInput } from "../taskStatusNeedsUser";

type Props = {
  tasks: Task[];
  loading: boolean;
  /** Background refetch in progress (list still visible). */
  refreshing: boolean;
  /** A create/update/delete request is in flight. */
  saving: boolean;
  /** Zero-based server list page (see `GET /tasks` offset). */
  listPage: number;
  listPageSize: number;
  onListPageChange: (page: number) => void;
  /** Reset to first server page when filters change. */
  onListFiltersChange: () => void;
  hasNextPage: boolean;
  hasPrevPage: boolean;
  /**
   * When true (default), status lines wait briefly before appearing so fast
   * requests do not flash unreadable text. Set false in tests.
   */
  smoothTransitions?: boolean;
  onEdit: (t: Task) => void;
  /** Opens in-app delete confirmation (do not call `window.confirm` from the table). */
  onRequestDelete: (t: Task) => void;
};

type StatusFilter = "all" | Status;
type PriorityFilter = "all" | Priority;

const LOADING_STATUS_DELAY_MS = 220;
const SYNC_STATUS_DELAY_MS = 180;

export function TaskListSection({
  tasks,
  loading,
  refreshing,
  saving,
  listPage,
  listPageSize,
  onListPageChange,
  onListFiltersChange,
  hasNextPage,
  hasPrevPage,
  smoothTransitions = true,
  onEdit,
  onRequestDelete,
}: Props) {
  const statusDelayMs = smoothTransitions ? LOADING_STATUS_DELAY_MS : 0;
  const syncDelayMs = smoothTransitions ? SYNC_STATUS_DELAY_MS : 0;
  const showLoadingLine = useDelayedTrue(loading, statusDelayMs);
  const showSyncLine = useDelayedTrue(refreshing && !loading, syncDelayMs);

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
    <section className="panel">
      <h2>All tasks</h2>
      {refreshing && !loading && showSyncLine ? (
        <p className="sync-hint task-list-phase-msg" aria-live="polite" role="status">
          Syncing with server…
        </p>
      ) : null}
      {loading && showLoadingLine ? (
        <p className="muted task-list-phase-msg" role="status">
          Loading…
        </p>
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
                      No tasks yet.
                    </td>
                  </tr>
                ) : filteredTasks.length === 0 ? (
                  <tr className="task-list-empty-row">
                    <td colSpan={5} className="task-list-empty-cell">
                      No tasks match these filters.
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
                            className="cell-title-link"
                            aria-label={`Open task details: ${t.title}`}
                          >
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
                              onClick={() => onEdit(t)}
                              disabled={saving}
                            >
                              Edit
                            </button>
                            <button
                              type="button"
                              className="danger btn-table"
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
                      const end = listPage * listPageSize + tasks.length;
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
