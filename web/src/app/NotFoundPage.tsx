import { Link } from "react-router-dom";
import { useDocumentTitle } from "@/shared/useDocumentTitle";

export function NotFoundPage() {
  useDocumentTitle("Page not found");

  return (
    <section
      className="panel task-detail-panel task-detail-content--enter"
      aria-labelledby="not-found-heading"
    >
      <h2 className="task-detail-title term-arrow" id="not-found-heading">
        <span>Page not found</span>
      </h2>
      <p className="muted term-prompt">
        <span>route 404 — no page matches this address</span>
      </p>
      <p>
        <Link to="/" className="task-detail-back">
          ← All tasks
        </Link>
      </p>
    </section>
  );
}
