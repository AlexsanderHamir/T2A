import { useCallback, useEffect, useState } from "react";
import {
  createTask,
  deleteTask,
  listTasks,
  patchTask,
} from "./api";
import type { Priority, Status, Task } from "./types";
import { PRIORITIES, STATUSES } from "./types";
import "./App.css";

export default function App() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sseLive, setSseLive] = useState(false);

  const [newTitle, setNewTitle] = useState("");
  const [newPrompt, setNewPrompt] = useState("");
  const [newStatus, setNewStatus] = useState<Status>("ready");
  const [newPriority, setNewPriority] = useState<Priority>("medium");

  const [editing, setEditing] = useState<Task | null>(null);
  const [editTitle, setEditTitle] = useState("");
  const [editPrompt, setEditPrompt] = useState("");
  const [editStatus, setEditStatus] = useState<Status>("ready");
  const [editPriority, setEditPriority] = useState<Priority>("medium");

  const refresh = useCallback(async () => {
    try {
      setError(null);
      const { tasks: next } = await listTasks();
      setTasks(next);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    const es = new EventSource("/events");
    es.onopen = () => setSseLive(true);
    es.onmessage = () => {
      void refresh();
    };
    es.onerror = () => {
      setSseLive(false);
    };
    return () => {
      es.close();
      setSseLive(false);
    };
  }, [refresh]);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!newTitle.trim()) return;
    setBusy(true);
    setError(null);
    try {
      await createTask({
        title: newTitle.trim(),
        initial_prompt: newPrompt,
        status: newStatus,
        priority: newPriority,
      });
      setNewTitle("");
      setNewPrompt("");
      setNewStatus("ready");
      setNewPriority("medium");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  function openEdit(t: Task) {
    setEditing(t);
    setEditTitle(t.title);
    setEditPrompt(t.initial_prompt);
    setEditStatus(t.status);
    setEditPriority(t.priority);
  }

  async function onSaveEdit(e: React.FormEvent) {
    e.preventDefault();
    if (!editing) return;
    if (!editTitle.trim()) {
      setError("Title is required.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await patchTask(editing.id, {
        title: editTitle.trim(),
        initial_prompt: editPrompt,
        status: editStatus,
        priority: editPriority,
      });
      setEditing(null);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  async function onDelete(id: string) {
    if (!window.confirm("Delete this task?")) return;
    setBusy(true);
    setError(null);
    try {
      await deleteTask(id);
      if (editing?.id === id) setEditing(null);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="app">
      <h1>Tasks</h1>
      <p className="sub">
        Live updates via <code>/events</code>{" "}
        <span className={`badge ${sseLive ? "on" : "off"}`}>
          {sseLive ? "stream connected" : "stream disconnected"}
        </span>
        — start <code>taskapi</code> on port 8080, then <code>npm run dev</code>{" "}
        in <code>web/</code>.
      </p>

      {error ? (
        <div className="err" role="alert">
          {error}
        </div>
      ) : null}

      <section className="panel">
        <h2>New task</h2>
        <form onSubmit={onCreate}>
          <div className="row">
            <div className="field grow">
              <label htmlFor="title">Title</label>
              <input
                id="title"
                value={newTitle}
                onChange={(ev) => setNewTitle(ev.target.value)}
                placeholder="What should get done?"
                required
              />
            </div>
            <div className="field">
              <label htmlFor="status">Status</label>
              <select
                id="status"
                value={newStatus}
                onChange={(ev) => setNewStatus(ev.target.value as Status)}
              >
                {STATUSES.map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </div>
            <div className="field">
              <label htmlFor="priority">Priority</label>
              <select
                id="priority"
                value={newPriority}
                onChange={(ev) => setNewPriority(ev.target.value as Priority)}
              >
                {PRIORITIES.map((p) => (
                  <option key={p} value={p}>
                    {p}
                  </option>
                ))}
              </select>
            </div>
            <button type="submit" disabled={busy}>
              Create
            </button>
          </div>
          <div className="field grow stack-tight">
            <label htmlFor="prompt">Initial prompt</label>
            <textarea
              id="prompt"
              value={newPrompt}
              onChange={(ev) => setNewPrompt(ev.target.value)}
              placeholder="Optional context for an agent…"
            />
          </div>
        </form>
      </section>

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
                        onClick={() => openEdit(t)}
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

      {editing ? (
        <section className="panel">
          <h2>Edit task</h2>
          <form onSubmit={(e) => void onSaveEdit(e)}>
            <p className="muted stack-tight-zero">
              <code>{editing.id}</code>
            </p>
            <div className="row">
              <div className="field grow">
                <label htmlFor="et">Title</label>
                <input
                  id="et"
                  value={editTitle}
                  onChange={(ev) => setEditTitle(ev.target.value)}
                  required
                />
              </div>
              <div className="field">
                <label htmlFor="es">Status</label>
                <select
                  id="es"
                  value={editStatus}
                  onChange={(ev) => setEditStatus(ev.target.value as Status)}
                >
                  {STATUSES.map((s) => (
                    <option key={s} value={s}>
                      {s}
                    </option>
                  ))}
                </select>
              </div>
              <div className="field">
                <label htmlFor="ep">Priority</label>
                <select
                  id="ep"
                  value={editPriority}
                  onChange={(ev) =>
                    setEditPriority(ev.target.value as Priority)
                  }
                >
                  {PRIORITIES.map((p) => (
                    <option key={p} value={p}>
                      {p}
                    </option>
                  ))}
                </select>
              </div>
            </div>
            <div className="field grow" style={{ marginTop: "0.65rem" }}>
              <label htmlFor="epr">Initial prompt</label>
              <textarea
                id="epr"
                value={editPrompt}
                onChange={(ev) => setEditPrompt(ev.target.value)}
              />
            </div>
            <div className="row stack-row-actions">
              <button type="submit" disabled={busy}>
                Save
              </button>
              <button
                type="button"
                className="secondary"
                disabled={busy}
                onClick={() => setEditing(null)}
              >
                Cancel
              </button>
            </div>
          </form>
        </section>
      ) : null}
    </div>
  );
}
