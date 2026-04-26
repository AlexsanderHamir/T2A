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
      <div className="task-detail-top-actions">
        <Link
          to={`/projects/${encodeURIComponent(projectId)}`}
          className="back-link"
        >
          ← Back to project
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
              <p className="eyebrow">Project memory</p>
              <h2>{project.data.name}</h2>
            </div>
            <span className="cell-pill cell-pill--runtime">
              {project.data.status}
            </span>
          </div>
          <ProjectContextPanel projectId={projectId} />
        </>
      ) : null}
    </section>
  );
}
