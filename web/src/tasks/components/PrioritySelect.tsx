import { useMemo } from "react";
import { PRIORITIES, type Priority } from "@/types";
import { priorityPillClass } from "../taskPillClasses";
import { CustomSelect, type CustomSelectOption } from "./CustomSelect";

type Props = {
  id: string;
  value: Priority;
  onChange: (p: Priority) => void;
};

export function PrioritySelect({ id, value, onChange }: Props) {
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
      onChange={(v) => onChange(v as Priority)}
    />
  );
}
