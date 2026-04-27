import { useMemo } from "react";
import { CustomSelect, type CustomSelectOption } from "@/tasks/components/custom-select";
import type { Project } from "@/types";
import { DEFAULT_PROJECT_ID } from "@/types";

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
  const options = useMemo<CustomSelectOption[]>(() => {
    const active = projects.filter((p) => p.status === "active");
    return active.map((project) => ({
      value: project.id,
      label:
        project.id === DEFAULT_PROJECT_ID
          ? `${project.name} (default)`
          : project.name,
    }));
  }, [projects]);

  return (
    <CustomSelect
      id={id}
      label="Project"
      value={value || DEFAULT_PROJECT_ID}
      options={options}
      onChange={onChange}
      disabled={disabled || loading}
    />
  );
}
