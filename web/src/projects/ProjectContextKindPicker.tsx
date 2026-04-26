import { PROJECT_CONTEXT_KIND_SUGGESTIONS, type ProjectContextKind } from "@/types";

type Props = {
  idPrefix: string;
  name?: string;
  defaultValue?: ProjectContextKind;
  disabled?: boolean;
};

export function ProjectContextKindPicker({
  idPrefix,
  name = "kind",
  defaultValue = "note",
  disabled = false,
}: Props) {
  const datalistId = `${idPrefix}-suggestions`;
  return (
    <div className="field grow project-context-kind-field">
      <label htmlFor={idPrefix}>Kind</label>
      <input
        id={idPrefix}
        name={name}
        defaultValue={defaultValue}
        list={datalistId}
        disabled={disabled}
        required
        placeholder="e.g. requirement, risk, API note"
        maxLength={64}
      />
      <datalist id={datalistId}>
        {PROJECT_CONTEXT_KIND_SUGGESTIONS.map((kind) => (
          <option key={kind} value={kind}>
            {formatKind(kind)}
          </option>
        ))}
      </datalist>
    </div>
  );
}

function formatKind(kind: ProjectContextKind): string {
  return kind.charAt(0).toUpperCase() + kind.slice(1);
}
