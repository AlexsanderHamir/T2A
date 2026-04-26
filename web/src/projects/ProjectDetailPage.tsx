import { Link, useParams } from "react-router-dom";
import { EmptyState } from "@/shared/EmptyState";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { useProject, useProjectContext } from "./hooks";

export function ProjectDetailPage() {
  const { projectId = "" } = useParams();
  const project = useProject(projectId);
  const context = useProjectContext(projectId, { enabled: Boolean(projectId) });
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
    <section className="panel task-detail-panel">
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

          <section className="task-attempt-section">
            <h3>Project context</h3>
            {context.isLoading ? (
              <p className="muted">Loading context...</p>
            ) : context.error ? (
              <div className="err" role="alert">
                {context.error.message}
              </div>
            ) : (context.data?.items ?? []).length === 0 ? (
              <EmptyState
                title="No context items yet"
                description="Pinned decisions, constraints, and handoff notes will appear here."
                density="compact"
                hideIcon
              />
            ) : (
              <ol className="task-attempt-phase-list">
                {context.data?.items.map((item) => (
                  <li key={item.id} className="task-attempt-phase">
                    <div>
                      <strong>{item.title}</strong>
                      <p className="muted">
                        {item.kind}
                        {item.pinned ? " · pinned" : ""}
                      </p>
                      <p>{item.body}</p>
                    </div>
                  </li>
                ))}
              </ol>
            )}
          </section>
        </>
      ) : null}
    </section>
  );
}
