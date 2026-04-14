import { useMemo } from "react";
import { TASK_TYPES, type TaskType } from "@/types";
import { CustomSelect, type CustomSelectOption } from "../custom-select";

type Props = {
  id: string;
  value: TaskType;
  onChange: (v: TaskType) => void;
  disabled?: boolean;
};

function taskTypeLabel(t: TaskType): string {
  switch (t) {
    case "general":
      return "General";
    case "bug_fix":
      return "Bug fix";
    case "feature":
      return "Feature";
    case "refactor":
      return "Refactor";
    case "docs":
      return "Docs";
    case "dmap":
      return "DMAP";
  }
}

export function TaskTypeSelect({ id, value, onChange, disabled = false }: Props) {
  const options: CustomSelectOption[] = useMemo(
    () =>
      TASK_TYPES.map((taskType) => ({
        value: taskType,
        label: taskTypeLabel(taskType),
      })),
    [],
  );

  return (
    <CustomSelect
      id={id}
      label="Task type"
      className="task-type-select"
      value={value}
      listboxName="Task type"
      requirement="required"
      compact
      disabled={disabled}
      options={options}
      onChange={(next) => onChange(next as TaskType)}
    />
  );
}
