import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import { WorktreesChevronDownIcon } from "./WorktreesIcons";

export type WorktreesMenuItem = {
  id: string;
  label: string;
  onSelect: () => void;
  disabled?: boolean;
  danger?: boolean;
};

type Props = {
  triggerLabel: string;
  items: WorktreesMenuItem[];
  className?: string;
  menuClassName?: string;
  icon?: ReactNode;
  chevron?: boolean;
  iconOnly?: boolean;
  align?: "start" | "end";
};

export function WorktreesMenu({
  triggerLabel,
  items,
  className = "secondary worktrees-menu-trigger",
  menuClassName = "",
  icon,
  chevron = false,
  iconOnly = false,
  align = "end",
}: Props) {
  const menuId = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  return (
    <div
      ref={rootRef}
      className={`worktrees-menu${open ? " worktrees-menu--open" : ""}`}
    >
      <button
        type="button"
        className={`worktrees-menu-trigger ${className}`.trim()}
        aria-label={iconOnly ? triggerLabel : undefined}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={menuId}
        onClick={() => setOpen((value) => !value)}
      >
        {icon ? <span className="worktrees-menu-trigger__icon">{icon}</span> : null}
        {!iconOnly ? (
          <span className="worktrees-menu-trigger__label">{triggerLabel}</span>
        ) : null}
        {chevron ? (
          <WorktreesChevronDownIcon className="worktrees-menu-trigger__chevron" />
        ) : null}
      </button>
      {open ? (
        <div
          id={menuId}
          role="menu"
          className={`worktrees-menu__panel worktrees-menu__panel--${align} ${menuClassName}`.trim()}
        >
          {items.map((item) => (
            <button
              key={item.id}
              type="button"
              role="menuitem"
              className={`worktrees-menu__item${item.danger ? " worktrees-menu__item--danger" : ""}`}
              disabled={item.disabled}
              onClick={() => {
                if (item.disabled) return;
                setOpen(false);
                item.onSelect();
              }}
            >
              {item.label}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}
