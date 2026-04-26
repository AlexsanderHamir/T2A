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
    const form = new FormData(event.currentTarget);
    patchProjectMutation.mutate({
      name: String(form.get("name") ?? "").trim(),
      status: String(form.get("status") ?? "active") as ProjectStatus,
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
