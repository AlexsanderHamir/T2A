import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { createProject } from "@/api";
import { EmptyState } from "@/shared/EmptyState";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { useProjects } from "./hooks";
import { projectQueryKeys } from "./queryKeys";

export function ProjectListPage() {
  useDocumentTitle("Projects");
  const queryClient = useQueryClient();
  const { data, isLoading, error } = useProjects({ includeArchived: true });
  const projects = data?.projects ?? [];
  const [name, setName] = useState("");
  const [summary, setSummary] = useState("");
  const createMutation = useMutation({
    mutationFn: createProject,
    onSuccess: async () => {
      setName("");
      setSummary("");
      await queryClient.invalidateQueries({ queryKey: projectQueryKeys.all });
    },
  });

  function submitProject(event: FormEvent) {
    event.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) return;
    createMutation.mutate({
      name: trimmedName,
      context_summary: summary.trim(),
    });
  }

  return (
    <section className="panel task-detail-panel project-page">
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

      <form className="project-create-form" onSubmit={submitProject}>
        <div className="field grow">
          <label htmlFor="project-create-name">Project name</label>
          <input
            id="project-create-name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="e.g., Agent context moat"
            required
          />
        </div>
        <div className="field grow">
          <label htmlFor="project-create-summary">Context summary</label>
          <input
            id="project-create-summary"
            value={summary}
            onChange={(event) => setSummary(event.target.value)}
            placeholder="What should agents remember across tasks?"
          />
        </div>
        <button type="submit" disabled={createMutation.isPending || !name.trim()}>
          {createMutation.isPending ? "Creating..." : "Create project"}
        </button>
      </form>
      {createMutation.error ? (
        <div className="err" role="alert">
          {createMutation.error.message}
        </div>
      ) : null}

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
