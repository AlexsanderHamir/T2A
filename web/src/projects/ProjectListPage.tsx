import { Link } from "react-router-dom";
import { EmptyState } from "@/shared/EmptyState";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { useProjects } from "./hooks";

export function ProjectListPage() {
  useDocumentTitle("Projects");
  const { data, isLoading, error } = useProjects({ includeArchived: true });
  const projects = data?.projects ?? [];

  return (
    <section className="panel task-detail-panel">
      <div className="task-detail-heading-row">
        <div>
          <p className="eyebrow">Projects</p>
          <h2>Project context</h2>
          <p className="muted">
            Shared memory for long-running work. Tasks can join a project while
            keeping their own subtask tree.
          </p>
        </div>
      </div>

      {isLoading ? <p className="muted">Loading projects...</p> : null}
      {error ? (
        <div className="err" role="alert">
          {error.message}
        </div>
      ) : null}
      {!isLoading && !error && projects.length === 0 ? (
        <EmptyState
          title="No projects yet"
          description="Create a project once a body of work needs shared context across multiple tasks."
          density="compact"
          hideIcon
        />
      ) : null}
      {projects.length > 0 ? (
        <div className="task-list" aria-label="Projects">
          {projects.map((project) => (
            <Link
              key={project.id}
              to={`/projects/${encodeURIComponent(project.id)}`}
              className="task-row task-row--link"
            >
              <span className="task-title">{project.name}</span>
              <span className="muted">{project.status}</span>
              {project.context_summary ? (
                <span className="muted">{project.context_summary}</span>
              ) : null}
            </Link>
          ))}
        </div>
      ) : null}
    </section>
  );
}
