import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useMemo, useRef, useState, type FormEvent } from "react";
import { patchProject } from "@/api";
import {
  CustomSelect,
  type CustomSelectOption,
} from "@/tasks/components/custom-select";
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
      status,
    });
  }

  return (
    <section className="pd__card" aria-labelledby="pd-settings-title">
      <div className="pd__card-head">
        <div className="pd__icon pd__icon--brand" aria-hidden="true">
          <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
            <path d="M7.5 2.25h-3A2.25 2.25 0 002.25 4.5v3c0 1.243 1.007 2.25 2.25 2.25h3A2.25 2.25 0 009.75 7.5v-3A2.25 2.25 0 007.5 2.25z" fill="currentColor" opacity="0.9" />
            <path d="M14.25 8.25h-1.5a2.25 2.25 0 00-2.25 2.25v3a2.25 2.25 0 002.25 2.25h1.5a2.25 2.25 0 002.25-2.25v-3a2.25 2.25 0 00-2.25-2.25z" fill="currentColor" opacity="0.45" />
          </svg>
        </div>
        <div>
          <h2 id="pd-settings-title" className="pd__card-title">
            Project settings
          </h2>
        </div>
      </div>

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
        <CustomSelect
          id="project-edit-status"
          label="Status"
          value={status}
          options={statusOptions}
          onChange={(v) => setStatus(v as ProjectStatus)}
          compact
          disabled={isDefaultProject}
        />
        <button
          type="submit"
          disabled={isDefaultProject || patchProjectMutation.isPending}
        >
          {patchProjectMutation.isPending ? "Saving..." : "Save"}
        </button>
      </form>

      {patchProjectMutation.error ? (
        <div className="pd__inline-error" role="alert">
          {patchProjectMutation.error.message}
        </div>
      ) : null}
    </section>
  );
}
