import { Link, useParams } from "react-router-dom";
import { EmptyState } from "@/shared/EmptyState";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { useProject } from "./hooks";
import { ProjectContextPanel } from "./ProjectContextPanel";

export function ProjectContextPage() {
  const { projectId = "" } = useParams();
  const project = useProject(projectId);
  const title = project.data?.name
    ? `${project.data.name} context`
    : "Project context";
  useDocumentTitle(title);

  if (!projectId) {
    return (
      <section className="panel task-detail-panel">
        <EmptyState
          title="Missing project id"
          description="Choose a project before opening its context graph."
          density="compact"
          hideIcon
        />
      </section>
    );
  }

  return (
    <section className="panel task-detail-panel pc">
      <header className="pc__header">
        <Link
          to={`/projects/${encodeURIComponent(projectId)}`}
          className="pd__back project-context-back-link"
        >
          <span aria-hidden="true">&#8249;</span>
          Back to project
        </Link>
        {project.data ? (
          <div className="pc__project-pill" aria-label="Current project">
            <h2>{project.data.name}</h2>
          </div>
        ) : null}
      </header>

      {project.isLoading ? (
        <div className="pc__skeleton" aria-hidden="true">
          <div className="pd__shimmer pd__shimmer--card" />
          <div className="pd__shimmer pd__shimmer--card pd__shimmer--card-sm" />
        </div>
      ) : null}

      {project.error ? (
        <div className="pd__error" role="alert">
          <div className="pd__error-dot" aria-hidden="true" />
          <div>
            <p className="pd__error-title">Unable to load project</p>
            <p className="pd__error-message">{project.error.message}</p>
          </div>
        </div>
      ) : null}

      {project.data ? <ProjectContextPanel projectId={projectId} /> : null}
    </section>
  );
}
