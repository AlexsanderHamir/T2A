import type { Task } from "../types";

type Props = {
  tasks: Task[];
  loading: boolean;
  busy: boolean;
  onEdit: (t: Task) => void;
  onDelete: (id: string) => void;
};

export function TaskListSection({
  tasks,
  loading,
  busy,
  onEdit,
  onDelete,
}: Props) {
  return (
    <section className="panel">
      <h2>All tasks</h2>
      {loading ? (
        <p className="muted" role="status">
          Loading…
        </p>
      ) : tasks.length === 0 ? (
        <p className="muted">No tasks yet.</p>
      ) : (
        <table>
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
            {tasks.map((t) => (
              <tr key={t.id}>
                <td>{t.title}</td>
                <td>{t.status}</td>
                <td>{t.priority}</td>
                <td>
                  <div className="prompt-preview" title={t.initial_prompt}>
                    {t.initial_prompt || "—"}
                  </div>
                </td>
                <td>
                  <div className="actions">
                    <button
                      type="button"
                      className="secondary"
                      onClick={() => onEdit(t)}
                      disabled={busy}
                    >
                      Edit
                    </button>
                    <button
                      type="button"
                      className="danger"
                      onClick={() => void onDelete(t.id)}
                      disabled={busy}
                    >
                      Delete
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}
