import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useMemo, useRef, useState, type FormEvent } from "react";
import { patchProject } from "@/api";
import {
  CustomSelect,
  type CustomSelectOption,
} from "@/components/custom-select";
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
  const [status, setStatus] = useState<ProjectStatus>(project.status);
  const formRef = useRef<HTMLFormElement>(null);

  const statusOptions = useMemo<CustomSelectOption[]>(
    () =>
      PROJECT_STATUSES.map((s) => ({
        value: s,
        label: s.charAt(0).toUpperCase() + s.slice(1),
      })),
    [],
  );

  const patchProjectMutation = useMutation({
    mutationFn: (input: {
      name?: string;
      description?: string;
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
      description: String(form.get("description") ?? "").trim(),
      status,
    });
  }

  return (
    <section className="pd__card" aria-labelledby="pd-settings-title">
      <h2 id="pd-settings-title" className="pd__card-eyebrow">
        Project settings
      </h2>

      {isDefaultProject ? (
        <p className="pd__note">
          The default project is built in — its name and status are fixed.
        </p>
      ) : null}

      <form
        ref={formRef}
        className="pd__settings-form"
        onSubmit={submitProject}
      >
        <div className="pd__settings-form-row">
          <div className="field grow">
            <label htmlFor="project-edit-name">Name</label>
            <input
              id="project-edit-name"
              name="name"
              defaultValue={project.name}
              required
              disabled={isDefaultProject}
              autoComplete="off"
            />
          </div>
          <CustomSelect
            id="project-edit-status"
            label="Status"
            value={status}
            options={statusOptions}
            onChange={(v) => setStatus(v as ProjectStatus)}
            compact
            disabled={isDefaultProject}
          />
        </div>

        <div className="field">
          <label htmlFor="project-edit-description">
            Description{" "}
            <span className="pd__settings-form-optional">— optional</span>
          </label>
          <textarea
            id="project-edit-description"
            name="description"
            defaultValue={project.description ?? ""}
            placeholder="One line of context that helps your team and agents understand what this project is for."
            rows={3}
            disabled={isDefaultProject}
          />
        </div>

        <div className="pd__settings-form-actions">
          <button
            type="submit"
            disabled={isDefaultProject || patchProjectMutation.isPending}
          >
            {patchProjectMutation.isPending ? "Saving…" : "Save changes"}
          </button>
        </div>
      </form>

      {patchProjectMutation.error ? (
        <div className="pd__inline-error" role="alert">
          {patchProjectMutation.error.message}
        </div>
      ) : null}
    </section>
  );
}
