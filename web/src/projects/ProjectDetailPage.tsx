import { Link, useParams } from "react-router-dom";
import { EmptyState } from "@/shared/EmptyState";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { useProject } from "./hooks";
import { ProjectSettingsPanel } from "./ProjectSettingsPanel";
import { ProjectTasksPanel } from "./ProjectTasksPanel";

export function ProjectDetailPage() {
  const { projectId = "" } = useParams();
  const project = useProject(projectId);
  const title = project.data?.name ? `${project.data.name} project` : "Project";
  useDocumentTitle(title);

  if (!projectId) {
    return (
      <section className="panel task-detail-panel">
        <EmptyState
          title="Missing project id"
          description="Choose a project from the project list."
          density="compact"
          hideIcon
        />
      </section>
    );
  }

  return (
    <section className="panel task-detail-panel project-page">
      <header className="project-context-hero project-detail-hero">
        <Link to="/projects" className="project-context-back-link">
          <span aria-hidden="true">‹</span>
          All projects
        </Link>
        {project.data ? (
          <div className="project-context-hero__project" aria-label="Current project">
            <h2>{project.data.name}</h2>
          </div>
        ) : null}
      </header>

      {project.isLoading ? <p className="muted">Loading project...</p> : null}
      {project.error ? (
        <div className="err" role="alert">
          {project.error.message}
        </div>
      ) : null}
      {project.data ? (
        <>
          <ProjectSettingsPanel project={project.data} />
          <section className="task-attempt-section project-context-page-card">
            <div>
              <p className="project-context-page-card__eyebrow">Memory nodes</p>
              <h3>Project context</h3>
              <p className="muted">
                Keep reusable facts, decisions, and constraints close to this project.
              </p>
            </div>
            <Link
              to={`/projects/${encodeURIComponent(projectId)}/context`}
              className="button-link"
            >
              Open context
            </Link>
          </section>
          <ProjectTasksPanel projectId={projectId} />
        </>
      ) : null}
    </section>
  );
}
