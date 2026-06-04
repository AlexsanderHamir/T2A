import { Link } from "react-router-dom";
import type { TaskDependencySummary } from "../../../task-query/resolveTaskDependencySummaries";
import { statusPillClass } from "../../../task-display";

type Props = {
  dependencies: TaskDependencySummary[];
};

/**
 * Read-only view of a task's upstream dependencies. The dependency graph
 * is fixed at creation time (chosen in the create modal), so the detail
 * page only *surfaces* the existing upstream tasks — it intentionally
 * offers no add/remove affordances. Editing the graph after pickup would
 * race the scheduler and is out of scope for now.
 */
export function TaskDependenciesPanel({ dependencies }: Props) {
  return (
    <section
      className="task-detail-section"
      id="task-detail-dependencies"
      aria-labelledby="task-detail-dependencies-title"
    >
      <h3
        id="task-detail-dependencies-title"
        className="task-detail-section-heading"
      >
        Dependencies
      </h3>
      {dependencies.length === 0 ? (
        <p className="task-detail-empty-hint" data-testid="task-deps-empty">
          No upstream tasks. This task can start when its gate allows pickup.
        </p>
      ) : (
        <ul className="task-deps-list" data-testid="task-deps-list">
          {dependencies.map((dep) => (
            <li key={dep.id} className="task-deps-list__item">
              <Link
                to={`/tasks/${encodeURIComponent(dep.id)}`}
                className="task-deps-list__link"
              >
                {dep.title}
              </Link>
              <span className={statusPillClass(dep.status)}>{dep.status}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
