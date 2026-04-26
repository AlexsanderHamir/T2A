import type { Project } from "@/types";
import { DEFAULT_PROJECT_ID } from "@/types";
import { FieldLabel, FieldRequirementBadge } from "@/shared/FieldLabel";

type Props = {
  id: string;
  value: string;
  projects: Project[];
  loading?: boolean;
  disabled?: boolean;
  onChange: (projectId: string) => void;
};

export function ProjectSelect({
  id,
  value,
  projects,
  loading = false,
  disabled = false,
  onChange,
}: Props) {
  const activeProjects = projects.filter((project) => project.status === "active");

  return (
    <div className="field grow">
      <FieldLabel htmlFor={id}>
        Project
        <FieldRequirementBadge requirement="optional" />
      </FieldLabel>
      <select
        id={id}
        value={value}
        disabled={disabled || loading}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">No project</option>
        {activeProjects.map((project) => (
          <option key={project.id} value={project.id}>
            {project.id === DEFAULT_PROJECT_ID
              ? `${project.name} (default)`
              : project.name}
          </option>
        ))}
      </select>
      <p className="muted stack-tight-zero">
        Projects provide shared context; subtasks still belong to the task tree.
      </p>
    </div>
  );
}
