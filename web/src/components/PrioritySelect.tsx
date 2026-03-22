import type { Priority } from "../types";
import { PRIORITIES } from "../types";

type Props = {
  id: string;
  value: Priority;
  onChange: (p: Priority) => void;
};

export function PrioritySelect({ id, value, onChange }: Props) {
  return (
    <div className="field">
      <label htmlFor={id}>Priority</label>
      <select
        id={id}
        value={value}
        onChange={(ev) => onChange(ev.target.value as Priority)}
      >
        {PRIORITIES.map((p) => (
          <option key={p} value={p}>
            {p}
          </option>
        ))}
      </select>
    </div>
  );
}
