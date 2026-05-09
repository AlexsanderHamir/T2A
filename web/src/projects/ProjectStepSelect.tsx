import { useQuery } from "@tanstack/react-query";
import { listProjectSteps } from "@/api";
import { FieldLabel } from "@/shared/FieldLabel";
import { projectQueryKeys } from "./queryKeys";

type Props = {
  id: string;
  projectId: string;
  value: string;
  onChange: (stepId: string) => void;
  disabled?: boolean;
};

export function ProjectStepSelect({ id, projectId, value, onChange, disabled }: Props) {
  const stepsQuery = useQuery({
    queryKey: projectQueryKeys.steps(projectId),
    queryFn: ({ signal }) => listProjectSteps(projectId, { signal }),
    enabled: Boolean(projectId.trim()),
  });
  const steps = stepsQuery.data?.steps ?? [];

  return (
    <div className="field grow stack-tight">
      <FieldLabel htmlFor={id}>Project step</FieldLabel>
      <select
        id={id}
        value={value}
        disabled={disabled || !projectId.trim() || stepsQuery.isLoading}
        onChange={(e) => onChange(e.target.value)}
      >
        <option value="">No step</option>
        {steps.map((s) => (
          <option key={s.id} value={s.id}>
            {s.sort_order}. {s.title}
          </option>
        ))}
      </select>
      {!projectId.trim() ? (
        <p className="muted stack-tight-zero">Pick a project first.</p>
      ) : null}
    </div>
  );
}
