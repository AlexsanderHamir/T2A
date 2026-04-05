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

/** Badge only (for headings, checkbox rows, or custom layouts). */
export function FieldRequirementBadge({ requirement }: BadgeProps) {
  if (requirement === "none") return null;
  return (
    <span
      className={
        requirement === "required"
          ? "field-req field-req--required"
          : "field-req field-req--optional"
      }
    >
      {requirement === "required" ? "Required" : "Optional"}
    </span>
  );
}
