import { useMemo } from "react";
import { PRIORITIES, type PriorityChoice } from "@/types";
import type { FieldRequirement } from "@/shared/FieldLabel";
import { priorityPillClass } from "../taskPillClasses";
import { CustomSelect, type CustomSelectOption } from "./custom-select";

type Props = {
  id: string;
  value: PriorityChoice;
  onChange: (p: PriorityChoice) => void;
  /** Narrow trigger for dense rows (e.g. create form). */
  compact?: boolean;
  requirement?: FieldRequirement;
  /**
   * When true, first option is “Choose priority…` (value "").
   * Set false when editing an existing task (priority always set).
   */
  allowUnset?: boolean;
};

export function PrioritySelect({
  id,
  value,
  onChange,
  compact,
  requirement = "required",
  allowUnset = true,
}: Props) {
  const options: CustomSelectOption[] = useMemo(() => {
    const levels: CustomSelectOption[] = PRIORITIES.map((p) => ({
      value: p,
      label: p,
      pillClass: priorityPillClass(p),
    }));
    if (!allowUnset) return levels;
    return [{ value: "", label: "Choose priority…" }, ...levels];
  }, [allowUnset]);

  return (
    <CustomSelect
      id={id}
      label="Priority"
      value={value}
      options={options}
      listboxName="Priority"
      compact={compact}
      requirement={requirement}
      onChange={(v) => onChange(v as PriorityChoice)}
    />
  );
}
