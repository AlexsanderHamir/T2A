import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { useNavigate } from "react-router-dom";
import { useTasksApp } from "../hooks/useTasksApp";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

export function TaskDraftsPage({ app }: Props) {
  useDocumentTitle("Task drafts");
  const navigate = useNavigate();

  return (
    <section className="panel">
      <h2>Task drafts</h2>
      <p className="muted">Continue previous drafts or remove ones you no longer need.</p>
      <div className="stack">
        {app.taskDrafts.length === 0 ? (
          <p className="muted">No saved drafts.</p>
        ) : (
          app.taskDrafts.map((d) => (
            <div key={d.id} className="row stack-row-actions">
              <span>{d.name}</span>
              <button
                type="button"
                className="secondary"
                onClick={async () => {
                  await app.resumeDraftByID(d.id);
                  navigate("/");
                }}
              >
                Resume in create form
              </button>
              <button
                type="button"
                className="secondary"
                onClick={() => void app.deleteDraftByID(d.id)}
              >
                Delete
              </button>
            </div>
          ))
        )}
      </div>
    </section>
  );
}
