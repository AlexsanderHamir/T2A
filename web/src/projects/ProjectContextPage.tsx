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
      <section className="panel task-detail-panel project-page">
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
    <section className="panel task-detail-panel project-page project-context-page">
      {project.isLoading ? <p className="muted">Loading project...</p> : null}
      {project.error ? (
        <div className="err" role="alert">
          {project.error.message}
        </div>
      ) : null}
      {project.data ? (
        <>
          <header className="project-context-hero">
            <Link
              to={`/projects/${encodeURIComponent(projectId)}`}
              className="back-link project-context-back-link"
            >
              <span aria-hidden="true">‹</span>
              Back to project
            </Link>
            <div className="project-context-hero__project" aria-label="Current project">
              <h2>{project.data.name}</h2>
            </div>
          </header>
          <ProjectContextPanel projectId={projectId} />
        </>
      ) : null}
    </section>
  );
}
