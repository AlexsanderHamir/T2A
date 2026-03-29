import { STATUSES, type Status } from "@/types";

type Props = {
  id: string;
  value: Status;
  onChange: (s: Status) => void;
};

export function StatusSelect({ id, value, onChange }: Props) {
  return (
    <div className="field">
      <label htmlFor={id}>Status</label>
      <select
        id={id}
        value={value}
        onChange={(ev) => onChange(ev.target.value as Status)}
      >
        {STATUSES.map((s) => (
          <option key={s} value={s}>
            {s}
          </option>
        ))}
      </select>
    </div>
  );
}
