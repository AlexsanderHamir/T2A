import { Link } from "react-router-dom";
import { useDocumentTitle } from "@/shared/useDocumentTitle";

export function NotFoundPage() {
  useDocumentTitle("Page not found");

  return (
    <section
      className="panel task-detail-panel"
      aria-labelledby="not-found-heading"
    >
      <h2 className="task-detail-title" id="not-found-heading">
        Page not found
      </h2>
      <p className="muted">No page matches this address.</p>
      <p>
        <Link to="/" className="task-detail-back">
          ← All tasks
        </Link>
      </p>
    </section>
  );
}
