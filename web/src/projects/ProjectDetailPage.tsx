import { Link, useParams } from "react-router-dom";
import { EmptyState } from "@/shared/EmptyState";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { useProject } from "./hooks";
import { ProjectContextPanel } from "./ProjectContextPanel";
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
      <div className="task-detail-top-actions">
        <Link to="/projects" className="back-link">
          ← All projects
        </Link>
      </div>

      {project.isLoading ? <p className="muted">Loading project...</p> : null}
      {project.error ? (
        <div className="err" role="alert">
          {project.error.message}
        </div>
      ) : null}
      {project.data ? (
        <>
          <div className="task-detail-heading-row">
            <div>
              <p className="eyebrow">Project</p>
              <h2>{project.data.name}</h2>
              {project.data.description ? (
                <p className="muted">{project.data.description}</p>
              ) : null}
            </div>
            <span className="cell-pill cell-pill--runtime">
              {project.data.status}
            </span>
          </div>

          {project.data.context_summary ? (
            <section className="task-attempt-section">
              <h3>Context summary</h3>
              <p>{project.data.context_summary}</p>
            </section>
          ) : null}

          <ProjectSettingsPanel project={project.data} />
          <ProjectTasksPanel projectId={projectId} />
          <ProjectContextPanel projectId={projectId} />
        </>
      ) : null}
    </section>
  );
}
