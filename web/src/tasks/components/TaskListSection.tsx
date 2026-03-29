import { previewTextFromPrompt } from "../promptFormat";
import type { Task } from "@/types";

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

export function TaskListSection({
  tasks,
  loading,
  refreshing,
  saving,
  onEdit,
  onRequestDelete,
}: Props) {
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
              {tasks.map((t) => {
                const promptPreview = previewTextFromPrompt(t.initial_prompt);
                return (
                  <tr key={t.id}>
                    <td className="cell-title">{t.title}</td>
                    <td>
                      <span className="cell-pill cell-pill--muted">{t.status}</span>
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
    </section>
  );
}
