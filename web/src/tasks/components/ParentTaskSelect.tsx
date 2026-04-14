import { useMemo } from "react";
import type { TaskWithDepth } from "../flattenTaskTree";
import { CustomSelect, type CustomSelectOption } from "./custom-select";

type Props = {
  id: string;
  value: string;
  parentOptions: TaskWithDepth[];
  onChange: (parentId: string) => void;
  disabled?: boolean;
};

export function ParentTaskSelect({
  id,
  value,
  parentOptions,
  onChange,
  disabled,
}: Props) {
  const options: CustomSelectOption[] = useMemo(() => {
    const rows: CustomSelectOption[] = [
      {
        value: "",
        label: "None — top-level task",
        depth: 0,
        rowTag: "Default",
      },
    ];
    for (const t of parentOptions) {
      rows.push({
        value: t.id,
        label: t.title,
        depth: t.depth,
        rowTag: t.depth === 0 ? "Top level" : "Subtask",
      });
    }
    return rows;
  }, [parentOptions]);

  return (
    <CustomSelect
      id={id}
      label="Parent task"
      value={value}
      options={options}
      onChange={onChange}
      listboxName="Parent task"
      requirement="optional"
      disabled={disabled}
    />
  );
}
