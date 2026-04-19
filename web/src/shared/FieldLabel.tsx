import type { ReactNode } from "react";

export type FieldRequirement = "required" | "optional" | "none";

type LabelProps = {
  htmlFor: string;
  children: ReactNode;
  requirement?: FieldRequirement;
  className?: string;
  /** For `aria-labelledby` on custom controls (e.g. rich text). */
  id?: string;
};

/**
 * Standard label row with a visible Required / Optional badge for form fields.
 */
export function FieldLabel({
  htmlFor,
  children,
  requirement = "none",
  className,
  id,
}: LabelProps) {
  const rowClass = ["field-label-with-req", className].filter(Boolean).join(" ");
  return (
    <div className={rowClass}>
      <label htmlFor={htmlFor} id={id}>
        {children}
      </label>
      <FieldRequirementBadge requirement={requirement} />
    </div>
  );
}

type BadgeProps = {
  requirement: FieldRequirement;
};

/**
 * Render a discreet required marker (`*`). Optional fields render nothing —
 * the convention is "marked = required, unmarked = optional", which keeps the
 * form quiet (DS §7) instead of stamping every label with a pill.
 */
export function FieldRequirementBadge({ requirement }: BadgeProps) {
  if (requirement !== "required") return null;
  return (
    <span className="field-req field-req--required" aria-hidden="true">
      *
    </span>
  );
}
