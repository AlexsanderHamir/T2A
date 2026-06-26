type Variant = "option" | "value";

type Props = {
  variant: Variant;
  rowTag?: string;
  label: string;
  pillClass?: string;
  /** Option rows: indent by depth. Ignored for `variant="value"`. */
  depth?: number;
  /** Value row: empty selection uses placeholder neutral styling. */
  valueEmpty?: boolean;
};

/** Shared pill / neutral row layout for the closed trigger and dropdown options. */
export function CustomSelectRowBody({
  variant,
  rowTag,
  label,
  pillClass,
  depth,
  valueEmpty = false,
}: Props) {
  const rowClass =
    variant === "option"
      ? "custom-select-option-row"
      : "custom-select-value-row";
  const tagClass =
    variant === "option"
      ? "custom-select-option-tag"
      : "custom-select-value-tag";
  const tagged = rowTag ? "custom-select-row--tagged" : "";

  if (pillClass) {
    const pillClassName =
      variant === "option"
        ? `custom-select-option-pill ${pillClass}`
        : `custom-select-value-pill ${pillClass}`;
    return (
      <span className={[rowClass, tagged].filter(Boolean).join(" ")}>
        {rowTag ? <span className={tagClass}>{rowTag}</span> : null}
        <span className={pillClassName}>{label}</span>
      </span>
    );
  }

  const neutralClass =
    variant === "option"
      ? depth != null && depth > 0
        ? "custom-select-option-neutral custom-select-option-neutral--nested"
        : "custom-select-option-neutral"
      : valueEmpty
        ? "custom-select-value-neutral custom-select-value-neutral--placeholder"
        : "custom-select-value-neutral";

  return (
    <span className={[rowClass, tagged].filter(Boolean).join(" ")}>
      {rowTag ? <span className={tagClass}>{rowTag}</span> : null}
      <span className={neutralClass}>{label}</span>
    </span>
  );
}
