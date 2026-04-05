import type { ReactNode } from "react";

export type EmptyStateAction = {
  label: string;
  onClick: () => void;
  disabled?: boolean;
};

type Props = {
  id?: string;
  title: string;
  description: ReactNode;
  /** Decorative; excluded from accessibility tree */
  icon?: ReactNode;
  action?: EmptyStateAction;
  /** When no `icon`, a neutral tray glyph is shown */
  hideIcon?: boolean;
  density?: "default" | "compact";
  className?: string;
  /** Optional anchor for `aria-labelledby` from a parent region */
  titleId?: string;
};

export function EmptyStateTrayGlyph() {
  return (
    <svg
      className="empty-state__glyph"
      width={48}
      height={48}
      viewBox="0 0 48 48"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <path
        d="M10 16.5h28a2 2 0 0 1 2 2V34a3 3 0 0 1-3 3H11a3 3 0 0 1-3-3V18.5a2 2 0 0 1 2-2Z"
        stroke="currentColor"
        strokeWidth={1.75}
        strokeLinejoin="round"
      />
      <path
        d="M10 16.5V14a2 2 0 0 1 2-2h6l2 3h10l2-3h6a2 2 0 0 1 2 2v2.5"
        stroke="currentColor"
        strokeWidth={1.75}
        strokeLinejoin="round"
      />
      <path
        d="M17 25.5h14M17 30.5h9"
        stroke="currentColor"
        strokeWidth={1.5}
        strokeLinecap="round"
      />
    </svg>
  );
}

export function EmptyStateTimelineGlyph() {
  return (
    <svg
      className="empty-state__glyph"
      width={48}
      height={48}
      viewBox="0 0 48 48"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <path
        d="M24 12v26"
        stroke="currentColor"
        strokeWidth={1.75}
        strokeLinecap="round"
      />
      <circle
        cx={24}
        cy={12}
        r={3.25}
        stroke="currentColor"
        strokeWidth={1.75}
      />
      <circle
        cx={24}
        cy={24}
        r={3.25}
        stroke="currentColor"
        strokeWidth={1.75}
      />
      <circle
        cx={24}
        cy={36}
        r={3.25}
        stroke="currentColor"
        strokeWidth={1.75}
      />
    </svg>
  );
}

export function EmptyStateChecklistGlyph() {
  return (
    <svg
      className="empty-state__glyph"
      width={48}
      height={48}
      viewBox="0 0 48 48"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <rect
        x={11}
        y={12}
        width={26}
        height={24}
        rx={3}
        stroke="currentColor"
        strokeWidth={1.75}
      />
      <path
        d="M16 20.5h10M16 26.5h16M16 32.5h12"
        stroke="currentColor"
        strokeWidth={1.5}
        strokeLinecap="round"
      />
    </svg>
  );
}

export function EmptyStateSubtasksGlyph() {
  return (
    <svg
      className="empty-state__glyph"
      width={48}
      height={48}
      viewBox="0 0 48 48"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <circle
        cx={24}
        cy={11}
        r={3.25}
        stroke="currentColor"
        strokeWidth={1.75}
      />
      <path
        d="M24 14.25v5.5M17 23.5h14"
        stroke="currentColor"
        strokeWidth={1.75}
        strokeLinecap="round"
      />
      <path
        d="M17 23.5v3.25M31 23.5v3.25"
        stroke="currentColor"
        strokeWidth={1.75}
        strokeLinecap="round"
      />
      <circle
        cx={17}
        cy={32}
        r={3.25}
        stroke="currentColor"
        strokeWidth={1.75}
      />
      <circle
        cx={31}
        cy={32}
        r={3.25}
        stroke="currentColor"
        strokeWidth={1.75}
      />
    </svg>
  );
}

export function EmptyStateFilterGlyph() {
  return (
    <svg
      className="empty-state__glyph"
      width={48}
      height={48}
      viewBox="0 0 48 48"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <path
        d="M14 14h20l-7 8.5V34l-6-3.5v-8L14 14Z"
        stroke="currentColor"
        strokeWidth={1.75}
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function EmptyState({
  id,
  title,
  description,
  icon,
  action,
  hideIcon = false,
  density = "default",
  className = "",
  titleId,
}: Props) {
  const rootClass = [
    "empty-state",
    density === "compact" ? "empty-state--compact" : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={rootClass} id={id}>
      {!hideIcon ? (
        <div className="empty-state__icon-wrap" aria-hidden="true">
          {icon ?? <EmptyStateTrayGlyph />}
        </div>
      ) : null}
      <p className="empty-state__title" id={titleId}>
        {title}
      </p>
      <div className="empty-state__description">{description}</div>
      {action ? (
        <div className="empty-state__actions">
          <button
            type="button"
            className="empty-state__cta"
            disabled={action.disabled}
            onClick={action.onClick}
          >
            {action.label}
          </button>
        </div>
      ) : null}
    </div>
  );
}
