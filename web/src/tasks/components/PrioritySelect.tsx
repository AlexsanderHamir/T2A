import { useMemo } from "react";
import { PRIORITIES, type Priority } from "@/types";
import { priorityPillClass } from "../taskPillClasses";
import { CustomSelect, type CustomSelectOption } from "./CustomSelect";

type Props = {
  id: string;
  value: Priority;
  onChange: (p: Priority) => void;
  /** Narrow trigger for dense rows (e.g. create form). */
  compact?: boolean;
};

export function PrioritySelect({ id, value, onChange, compact }: Props) {
  const options: CustomSelectOption[] = useMemo(
    () =>
      PRIORITIES.map((p) => ({
        value: p,
        label: p,
        pillClass: priorityPillClass(p),
      })),
    [],
  );

  return (
    <CustomSelect
      id={id}
      label="Priority"
      value={value}
      options={options}
      listboxName="Priority"
      compact={compact}
      onChange={(v) => onChange(v as Priority)}
    />
  );
}
