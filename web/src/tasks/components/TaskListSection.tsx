import { useMemo, useState } from "react";
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
  onEdit: (t: Task) => void;
  /** Opens in-app delete confirmation (do not call `window.confirm` from the table). */
  onRequestDelete: (t: Task) => void;
};

type StatusFilter = "all" | Status;
type PriorityFilter = "all" | Priority;

export function TaskListSection({
  tasks,
  loading,
  refreshing,
  saving,
  onEdit,
  onRequestDelete,
}: Props) {
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [priorityFilter, setPriorityFilter] = useState<PriorityFilter>("all");

  const filteredTasks = useMemo(() => {
    return tasks.filter((t) => {
      if (statusFilter !== "all" && t.status !== statusFilter) return false;
      if (priorityFilter !== "all" && t.priority !== priorityFilter)
        return false;
      return true;
    });
  }, [tasks, statusFilter, priorityFilter]);

  return (
    <section className="panel">
      <h2>All tasks</h2>
      {refreshing ? (
        <p className="sync-hint" aria-live="polite" role="status">
          Syncing with server…
        </p>
      ) : null}
      {loading ? (
        <p className="muted" role="status">
          Loading…
        </p>
      ) : tasks.length === 0 ? (
        <p className="muted empty-state">No tasks yet.</p>
      ) : (
        <>
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
          </div>
          {filteredTasks.length === 0 ? (
            <p className="muted empty-state task-list-filter-empty">
              No tasks match these filters.
            </p>
          ) : (
            <div className="table-wrap">
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
                <tbody>
                  {filteredTasks.map((t) => {
                    const promptPreview = previewTextFromPrompt(
                      t.initial_prompt,
                    );
                    return (
                      <tr key={t.id}>
                        <td className="cell-title">{t.title}</td>
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
                  })}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </section>
  );
}
