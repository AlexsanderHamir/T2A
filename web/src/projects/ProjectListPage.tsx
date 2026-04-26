import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { createProject } from "@/api";
import { EmptyState } from "@/shared/EmptyState";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import type { Project } from "@/types";
import { useProjects } from "./hooks";
import { projectQueryKeys } from "./queryKeys";

export function ProjectListPage() {
  useDocumentTitle("Projects");
  const queryClient = useQueryClient();
  const { data, isLoading, error } = useProjects({ includeArchived: true });
  const projects = data?.projects ?? [];
  const [name, setName] = useState("");
  const createMutation = useMutation({
    mutationFn: createProject,
    onSuccess: async () => {
      setName("");
      await queryClient.invalidateQueries({ queryKey: projectQueryKeys.all });
    },
  });

  function submitProject(event: FormEvent) {
    event.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) return;
    createMutation.mutate({
      name: trimmedName,
    });
  }

  return (
    <section className="panel task-detail-panel project-page">
      <div className="project-page-hero">
        <div>
          <p className="eyebrow">Projects</p>
          <h2>Project context</h2>
          <p className="muted">
            Shared memory for long-running work. Tasks can join a project while
            keeping their own subtask tree.
          </p>
        </div>
      </div>

      <form className="project-create-card" onSubmit={submitProject}>
        <div className="project-create-card__copy">
          <span className="project-create-card__badge">New project</span>
          <h3>Create a project</h3>
          <p>
            Start with a name. Add memory nodes and relationships later as the
            work becomes clearer.
          </p>
        </div>
        <div className="project-create-card__controls">
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
          <button type="submit" disabled={createMutation.isPending || !name.trim()}>
            {createMutation.isPending ? "Creating..." : "Create project"}
          </button>
        </div>
      </form>
      {createMutation.error ? (
        <div className="err" role="alert">
          {createMutation.error.message}
        </div>
      ) : null}

      <div className="project-list-header">
        <div>
          <h3>Project library</h3>
          <p className="muted">
            Open a project to manage its nodes, connections, settings, and
            linked tasks.
          </p>
        </div>
        <span className="project-list-count">
          {projects.length} {projects.length === 1 ? "project" : "projects"}
        </span>
      </div>

      {isLoading ? <ProjectListSkeleton /> : null}
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
        <div className="project-card-grid" aria-label="Projects">
          {projects.map((project) => (
            <ProjectCard key={project.id} project={project} />
          ))}
        </div>
      ) : null}
    </section>
  );
}

function ProjectCard({ project }: { project: Project }) {
  const isArchived = project.status === "archived";
  return (
    <Link
      to={`/projects/${encodeURIComponent(project.id)}`}
      className="project-card"
    >
      <span className="project-card__topline">
        <span className="project-card__label">Project</span>
        <span
          className={
            isArchived
              ? "project-status-pill project-status-pill--archived"
              : "project-status-pill"
          }
        >
          {project.status}
        </span>
      </span>
      <span className="project-card__title">{project.name}</span>
      <span className="project-card__description">
        {project.description ||
          project.context_summary ||
          "Add project-owned context nodes when shared memory becomes useful."}
      </span>
      <span className="project-card__footer">
        <span>Manage memory graph</span>
        <span aria-hidden="true">&rarr;</span>
      </span>
    </Link>
  );
}

function ProjectListSkeleton() {
  return (
    <div className="project-card-grid" aria-hidden="true">
      {Array.from({ length: 3 }).map((_, index) => (
        <div className="project-card project-card--skeleton" key={index}>
          <span className="project-skeleton project-skeleton--meta" />
          <span className="project-skeleton project-skeleton--title" />
          <span className="project-skeleton project-skeleton--line" />
          <span className="project-skeleton project-skeleton--line-short" />
        </div>
      ))}
    </div>
  );
}