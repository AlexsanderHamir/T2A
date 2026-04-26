import type { ReactNode } from "react";
import { EmptyStateTrayGlyph } from "./EmptyStateGlyphs";

export {
  EmptyStateTrayGlyph,
  EmptyStateTimelineGlyph,
  EmptyStateChecklistGlyph,
  EmptyStateSubtasksGlyph,
  EmptyStateFilterGlyph,
} from "./EmptyStateGlyphs";

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
      <h2 className="empty-state__title" id={titleId}>
        {title}
      </h2>
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
