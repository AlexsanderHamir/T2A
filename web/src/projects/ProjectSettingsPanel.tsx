import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { FormEvent } from "react";
import { patchProject } from "@/api";
import { PROJECT_STATUSES, type Project, type ProjectStatus } from "@/types";
import { projectQueryKeys } from "./queryKeys";

type Props = {
  project: Project;
};

export function ProjectSettingsPanel({ project }: Props) {
  const queryClient = useQueryClient();
  const patchProjectMutation = useMutation({
    mutationFn: (input: {
      name?: string;
      description?: string;
      status?: ProjectStatus;
      context_summary?: string;
    }) => patchProject(project.id, input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: projectQueryKeys.all });
      await queryClient.invalidateQueries({
        queryKey: projectQueryKeys.detail(project.id),
      });
    },
  });

  function submitProject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    patchProjectMutation.mutate({
      name: String(form.get("name") ?? "").trim(),
      description: String(form.get("description") ?? "").trim(),
      status: String(form.get("status") ?? "active") as ProjectStatus,
      context_summary: String(form.get("context_summary") ?? "").trim(),
    });
  }

  return (
    <section className="task-attempt-section">
      <h3>Project settings</h3>
      <form className="project-edit-form" onSubmit={submitProject}>
        <div className="field grow">
          <label htmlFor="project-edit-name">Name</label>
          <input
            id="project-edit-name"
            name="name"
            defaultValue={project.name}
            required
          />
        </div>
        <div className="field grow">
          <label htmlFor="project-edit-status">Status</label>
          <select
            id="project-edit-status"
            name="status"
            defaultValue={project.status}
          >
            {PROJECT_STATUSES.map((status) => (
              <option key={status} value={status}>
                {status}
              </option>
            ))}
          </select>
        </div>
        <div className="field grow">
          <label htmlFor="project-edit-description">Description</label>
          <textarea
            id="project-edit-description"
            name="description"
            defaultValue={project.description}
            rows={3}
          />
        </div>
        <div className="field grow">
          <label htmlFor="project-edit-summary">Context summary</label>
          <textarea
            id="project-edit-summary"
            name="context_summary"
            defaultValue={project.context_summary}
            rows={3}
          />
        </div>
        <button type="submit" disabled={patchProjectMutation.isPending}>
          {patchProjectMutation.isPending ? "Saving..." : "Save project"}
        </button>
      </form>
      {patchProjectMutation.error ? (
        <div className="err" role="alert">
          {patchProjectMutation.error.message}
        </div>
      ) : null}
    </section>
  );
}
