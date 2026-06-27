import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
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

const MENU_GAP_PX = 4;
const VIEWPORT_MARGIN_PX = 8;
const MENU_MIN_WIDTH_PX = 176;
const MENU_Z_INDEX = 200;

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
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [panelPos, setPanelPos] = useState<{
    top: number;
    left: number;
    minWidth: number;
  } | null>(null);

  useLayoutEffect(() => {
    if (!open || !triggerRef.current) {
      setPanelPos(null);
      return;
    }

    const compute = () => {
      const trigger = triggerRef.current;
      if (!trigger) return;

      const rect = trigger.getBoundingClientRect();
      const panelHeight = panelRef.current?.offsetHeight ?? 0;
      const panelWidth = Math.max(
        MENU_MIN_WIDTH_PX,
        panelRef.current?.offsetWidth ?? MENU_MIN_WIDTH_PX,
        rect.width,
      );
      const spaceBelow = window.innerHeight - rect.bottom - MENU_GAP_PX;
      const placeAbove =
        panelHeight > 0 &&
        spaceBelow < panelHeight &&
        rect.top - MENU_GAP_PX > panelHeight;

      const top = placeAbove
        ? Math.max(VIEWPORT_MARGIN_PX, rect.top - panelHeight - MENU_GAP_PX)
        : rect.bottom + MENU_GAP_PX;

      const rawLeft = align === "end" ? rect.right - panelWidth : rect.left;
      const left = Math.max(
        VIEWPORT_MARGIN_PX,
        Math.min(window.innerWidth - panelWidth - VIEWPORT_MARGIN_PX, rawLeft),
      );

      setPanelPos({ top, left, minWidth: panelWidth });
    };

    compute();
    window.addEventListener("scroll", compute, true);
    window.addEventListener("resize", compute);
    return () => {
      window.removeEventListener("scroll", compute, true);
      window.removeEventListener("resize", compute);
    };
  }, [open, align, items.length]);

  useEffect(() => {
    if (!open) return;

    const onPointerDown = (event: MouseEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) return;
      if (triggerRef.current?.contains(target)) return;
      if (panelRef.current?.contains(target)) return;
      setOpen(false);
    };

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
        triggerRef.current?.focus();
      }
    };

    document.addEventListener("mousedown", onPointerDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("mousedown", onPointerDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [open]);

  const menuPanel =
    open &&
    createPortal(
      <div
        ref={panelRef}
        id={menuId}
        role="menu"
        className={`worktrees-menu__panel worktrees-menu__panel--portal worktrees-menu__panel--${align} ${menuClassName}`.trim()}
        style={{
          position: "fixed",
          top: panelPos?.top ?? -9999,
          left: panelPos?.left ?? -9999,
          minWidth: panelPos?.minWidth ?? MENU_MIN_WIDTH_PX,
          visibility: panelPos ? "visible" : "hidden",
          zIndex: MENU_Z_INDEX,
        }}
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
      </div>,
      document.body,
    );

  return (
    <>
      <div className={`worktrees-menu${open ? " worktrees-menu--open" : ""}`}>
        <button
          ref={triggerRef}
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
      </div>
      {menuPanel}
    </>
  );
}
