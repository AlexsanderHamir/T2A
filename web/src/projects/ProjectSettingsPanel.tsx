import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { FormEvent } from "react";
import { patchProject } from "@/api";
import {
  DEFAULT_PROJECT_ID,
  PROJECT_STATUSES,
  type Project,
  type ProjectStatus,
} from "@/types";
import { projectQueryKeys } from "./queryKeys";

type Props = {
  project: Project;
};

export function ProjectSettingsPanel({ project }: Props) {
  const queryClient = useQueryClient();
  const isDefaultProject = project.id === DEFAULT_PROJECT_ID;
  const patchProjectMutation = useMutation({
    mutationFn: (input: {
      name?: string;
      status?: ProjectStatus;
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
    if (isDefaultProject) return;
    const form = new FormData(event.currentTarget);
    patchProjectMutation.mutate({
      name: String(form.get("name") ?? "").trim(),
      status: String(form.get("status") ?? "active") as ProjectStatus,
    });
  }

  return (
    <section className="task-attempt-section">
      <h3>Project settings</h3>
      {isDefaultProject ? (
        <p className="muted project-settings-note">
          The default project is built in, so its name and status stay fixed.
        </p>
      ) : null}
      <form className="project-edit-form" onSubmit={submitProject}>
        <div className="field grow">
          <label htmlFor="project-edit-name">Name</label>
          <input
            id="project-edit-name"
            name="name"
            defaultValue={project.name}
            required
            disabled={isDefaultProject}
          />
        </div>
        <div className="field grow">
          <label htmlFor="project-edit-status">Status</label>
          <select
            id="project-edit-status"
            name="status"
            defaultValue={project.status}
            disabled={isDefaultProject}
          >
            {PROJECT_STATUSES.map((status) => (
              <option key={status} value={status}>
                {status}
              </option>
            ))}
          </select>
        </div>
        <button
          type="submit"
          disabled={isDefaultProject || patchProjectMutation.isPending}
        >
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
