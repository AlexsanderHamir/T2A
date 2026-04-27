import { useEffect, useState } from "react";
import { FieldLabel } from "@/shared/FieldLabel";
import { PROJECT_CONTEXT_KIND_SUGGESTIONS, type ProjectContextKind } from "@/types";

type Props = {
  idPrefix: string;
  name?: string;
  defaultValue?: ProjectContextKind;
  disabled?: boolean;
};

const PROJECT_CONTEXT_KIND_MAX_LENGTH = 24;

export function ProjectContextKindPicker({
  idPrefix,
  name = "kind",
  defaultValue = "note",
  disabled = false,
}: Props) {
  const [value, setValue] = useState(defaultValue);
  const limitId = `${idPrefix}-limit`;

  useEffect(() => {
    setValue(defaultValue);
  }, [defaultValue]);

  return (
    <div className="field grow project-context-kind-field">
      <FieldLabel htmlFor={idPrefix} requirement="required">
        Kind
      </FieldLabel>
      <input
        id={idPrefix}
        value={value}
        onChange={(event) => setValue(event.target.value)}
        disabled={disabled}
        required
        aria-required="true"
        aria-describedby={limitId}
        placeholder="e.g. requirement, risk, API note"
        maxLength={PROJECT_CONTEXT_KIND_MAX_LENGTH}
      />
      <p id={limitId} className="project-context-kind-limit">
        {value.length}/{PROJECT_CONTEXT_KIND_MAX_LENGTH} characters
      </p>
      <input type="hidden" name={name} value={value.trim()} />
      <div className="project-context-kind-suggestions" aria-label="Common kinds">
        {PROJECT_CONTEXT_KIND_SUGGESTIONS.map((kind) => (
          <button
            key={kind}
            type="button"
            className="project-context-kind-suggestion"
            disabled={disabled}
            aria-pressed={value.trim().toLowerCase() === kind}
            onClick={() => setValue(kind)}
          >
            {formatKind(kind)}
          </button>
        ))}
      </div>
    </div>
  );
}

function formatKind(kind: ProjectContextKind): string {
  return kind.charAt(0).toUpperCase() + kind.slice(1);
}
