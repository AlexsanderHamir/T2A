import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import { useDelayedTrue } from "@/lib/useDelayedTrue";
import { previewTextFromPrompt } from "../promptFormat";
import {
  PRIORITIES,
  STATUSES,
  type Priority,
  type Status,
  type Task,
} from "@/types";

type Props = {
  tasks: Task[];
  loading: boolean;
  /** Background refetch in progress (list still visible). */
  refreshing: boolean;
  /** A create/update/delete request is in flight. */
  saving: boolean;
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
            <div className="field task-list-filter-field">
              <label htmlFor="task-list-filter-status">Status</label>
              <select
                id="task-list-filter-status"
                value={statusFilter}
                onChange={(e) =>
                  setStatusFilter(e.target.value as StatusFilter)
                }
              >
                <option value="all">All</option>
                {STATUSES.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </div>
            <div className="field task-list-filter-field">
              <label htmlFor="task-list-filter-priority">Priority</label>
              <select
                id="task-list-filter-priority"
                value={priorityFilter}
                onChange={(e) =>
                  setPriorityFilter(e.target.value as PriorityFilter)
                }
              >
                <option value="all">All</option>
                {PRIORITIES.map((p) => (
                  <option key={p} value={p}>
                    {p}
                  </option>
                ))}
              </select>
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
            <table aria-busy={refreshing}>
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
                      <tr key={t.id}>
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
                          <span className="cell-pill cell-pill--muted">
                            {t.status}
                          </span>
                        </td>
                        <td>
                          <span className="cell-pill cell-pill--priority">
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
        </div>
      ) : null}
    </section>
  );
}
