import {
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
import { createPortal } from "react-dom";
import {
  type TimezoneSelectOption,
  filterTimezoneSelectOptions,
  formatTimezoneMenuLabel,
  getTimezoneSearchHaystack,
  matchesTimezoneSearchQuery,
} from "@/shared/time/appTimezone";

type Row =
  | { kind: "auto" }
  | { kind: "iana"; opt: TimezoneSelectOption }
  | { kind: "custom"; value: string; label: string };

type Props = {
  value: string;
  onChange: (value: string) => void;
  browserTz: string;
  options: TimezoneSelectOption[];
  /** Saved IANA not present in `options` — show a third list row. */
  customSaved?: { value: string; label: string } | null;
  testId?: string;
};

function ChevronIcon({ open }: { open: boolean }) {
  return (
    <span
      className={
        open
          ? "settings-dropdown-chevron settings-dropdown-chevron--open"
          : "settings-dropdown-chevron"
      }
      aria-hidden="true"
    >
      <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
        <path
          d="M4 6l4 4 4-4"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </span>
  );
}

function CheckIcon() {
  return (
    <svg
      className="settings-dropdown-option-check"
      width="14"
      height="14"
      viewBox="0 0 14 14"
      fill="none"
      aria-hidden="true"
    >
      <path
        d="M2.5 7.25 5.5 10.25 11.5 3.75"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function rowKey(row: Row): string {
  if (row.kind === "auto") return "auto";
  if (row.kind === "iana") return row.opt.value;
  return `custom-${row.value}`;
}

function rowLabel(row: Row, autoLabel: string): string {
  if (row.kind === "auto") return autoLabel;
  if (row.kind === "iana") return row.opt.label;
  return row.label;
}

function rowValue(row: Row): string {
  if (row.kind === "auto") return "";
  if (row.kind === "iana") return row.opt.value;
  return row.value;
}

function isRowSelected(row: Row, value: string): boolean {
  return rowValue(row) === value;
}

export function TimezoneCombobox({
  value,
  onChange,
  browserTz,
  options,
  customSaved,
  testId = "settings-display-timezone-select",
}: Props) {
  const baseId = useId();
  const listId = `${baseId}-list`;
  const searchId = `${baseId}-search`;
  const rootRef = useRef<HTMLDivElement>(null);
  const shellRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);

  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const [pos, setPos] = useState<{
    top: number;
    left: number;
    width: number;
  } | null>(null);

  const autoLabel = useMemo(
    () => `Auto-detect — ${formatTimezoneMenuLabel(browserTz)}`,
    [browserTz],
  );

  const autoHaystack = useMemo(
    () =>
      `auto auto-detect detect browser ${browserTz} ${formatTimezoneMenuLabel(browserTz)}`
        .toLowerCase(),
    [browserTz],
  );

  const selectedLabel = useMemo(() => {
    if (value === "") return autoLabel;
    const hit = options.find((o) => o.value === value);
    if (hit) return hit.label;
    if (customSaved && value === customSaved.value) return customSaved.label;
    return formatTimezoneMenuLabel(value);
  }, [value, autoLabel, options, customSaved]);

  const filteredIana = useMemo(
    () => filterTimezoneSelectOptions(options, search),
    [options, search],
  );

  const rows: Row[] = useMemo(() => {
    const out: Row[] = [];
    const q = search.trim();
    if (!q || matchesTimezoneSearchQuery(autoHaystack, q)) {
      out.push({ kind: "auto" });
    }
    for (const opt of filteredIana) {
      out.push({ kind: "iana", opt });
    }
    if (customSaved) {
      const ch = getTimezoneSearchHaystack({
        value: customSaved.value,
        label: customSaved.label,
      });
      if (!q || matchesTimezoneSearchQuery(ch, q)) {
        out.push({ kind: "custom", value: customSaved.value, label: customSaved.label });
      }
    }
    return out;
  }, [search, autoHaystack, filteredIana, customSaved]);

  const rowCount = rows.length;

  const updatePosition = useCallback(() => {
    const el = shellRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    setPos({ top: r.bottom + 6, left: r.left, width: r.width });
  }, []);

  useLayoutEffect(() => {
    if (!open) {
      setPos(null);
      return;
    }
    updatePosition();
    const onMove = () => updatePosition();
    window.addEventListener("scroll", onMove, true);
    window.addEventListener("resize", onMove);
    return () => {
      window.removeEventListener("scroll", onMove, true);
      window.removeEventListener("resize", onMove);
    };
  }, [open, updatePosition]);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (rootRef.current?.contains(e.target as Node)) return;
      const panel = document.getElementById(`${baseId}-panel`);
      if (panel?.contains(e.target as Node)) return;
      setOpen(false);
      setSearch("");
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open, baseId]);

  useEffect(() => {
    if (!open) return;
    searchRef.current?.focus();
  }, [open]);

  useEffect(() => {
    if (!open) return;
    if (!search.trim()) {
      const idx = rows.findIndex((row) => isRowSelected(row, value));
      setActiveIndex(idx >= 0 ? idx : 0);
      return;
    }
    setActiveIndex(0);
  }, [open, search, rows, value]);

  useEffect(() => {
    if (activeIndex >= rowCount) setActiveIndex(Math.max(0, rowCount - 1));
  }, [activeIndex, rowCount]);

  const closeMenu = useCallback(() => {
    setOpen(false);
    setSearch("");
    triggerRef.current?.focus();
  }, []);

  const commitRow = useCallback(
    (row: Row) => {
      onChange(rowValue(row));
      closeMenu();
    },
    [closeMenu, onChange],
  );

  const openMenu = useCallback(() => {
    setOpen(true);
  }, []);

  const onTriggerKeyDown = (e: KeyboardEvent<HTMLButtonElement>) => {
    if (e.key === "ArrowDown" || e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      openMenu();
      return;
    }
    if (e.key === "Escape" && open) {
      e.preventDefault();
      closeMenu();
    }
  };

  const onSearchKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Escape") {
      e.preventDefault();
      closeMenu();
      return;
    }
    if (e.key === "ArrowDown" && rowCount > 0) {
      e.preventDefault();
      setActiveIndex((i) => Math.min(i + 1, rowCount - 1));
      return;
    }
    if (e.key === "ArrowUp" && rowCount > 0) {
      e.preventDefault();
      setActiveIndex((i) => Math.max(i - 1, 0));
      return;
    }
    if (e.key === "Enter" && rowCount > 0) {
      e.preventDefault();
      const row = rows[activeIndex];
      if (row) commitRow(row);
    }
  };

  const onListKeyDown = (e: KeyboardEvent<HTMLUListElement>) => {
    if (e.key === "Escape") {
      e.preventDefault();
      closeMenu();
      return;
    }
    if (e.key === "ArrowDown" && rowCount > 0) {
      e.preventDefault();
      setActiveIndex((i) => Math.min(i + 1, rowCount - 1));
      return;
    }
    if (e.key === "ArrowUp" && rowCount > 0) {
      e.preventDefault();
      setActiveIndex((i) => Math.max(i - 1, 0));
      return;
    }
    if (e.key === "Enter" && rowCount > 0) {
      e.preventDefault();
      const row = rows[activeIndex];
      if (row) commitRow(row);
    }
  };

  const shellClass = open
    ? "settings-dropdown-shell settings-dropdown-shell--open"
    : "settings-dropdown-shell";

  const panel =
    open && pos
      ? createPortal(
          <div
            id={`${baseId}-panel`}
            className="settings-dropdown-panel"
            style={{
              position: "fixed",
              top: pos.top,
              left: pos.left,
              width: pos.width,
              zIndex: "var(--z-portal-popover, 13000)",
            }}
          >
            <div className="settings-dropdown-panel-search">
              <input
                ref={searchRef}
                id={searchId}
                type="search"
                className="settings-dropdown-panel-search-input"
                placeholder="Search by city, region, or GMT offset…"
                value={search}
                autoComplete="off"
                spellCheck={false}
                aria-controls={listId}
                aria-autocomplete="list"
                onChange={(e) => setSearch(e.target.value)}
                onKeyDown={onSearchKeyDown}
              />
            </div>
            {rowCount > 0 ? (
              <ul
                ref={listRef}
                id={listId}
                role="listbox"
                tabIndex={-1}
                className="settings-dropdown-list settings-dropdown-list--portal"
                aria-activedescendant={
                  rowCount > 0 ? `${baseId}-opt-${activeIndex}` : undefined
                }
                onKeyDown={onListKeyDown}
              >
                {rows.map((row, idx) => {
                  const id = `${baseId}-opt-${idx}`;
                  const isActive = idx === activeIndex;
                  const isSelected = isRowSelected(row, value);
                  const text = rowLabel(row, autoLabel);
                  return (
                    <li
                      key={rowKey(row)}
                      id={id}
                      role="option"
                      aria-selected={isSelected}
                      className={[
                        "settings-dropdown-option",
                        isActive ? "settings-dropdown-option--active" : "",
                        isSelected ? "settings-dropdown-option--selected" : "",
                      ]
                        .filter(Boolean)
                        .join(" ")}
                      onMouseEnter={() => setActiveIndex(idx)}
                      onMouseDown={(e) => e.preventDefault()}
                      onClick={() => commitRow(row)}
                    >
                      <span className="settings-dropdown-option-check-slot">
                        {isSelected ? <CheckIcon /> : null}
                      </span>
                      <span className="settings-dropdown-option-label">
                        {text}
                      </span>
                    </li>
                  );
                })}
              </ul>
            ) : (
              <div
                className="settings-dropdown-empty settings-dropdown-empty--portal"
                role="status"
              >
                No matching timezones
              </div>
            )}
          </div>,
          document.body,
        )
      : null;

  return (
    <div ref={rootRef} className="settings-dropdown">
      <div ref={shellRef} className={shellClass}>
        <button
          ref={triggerRef}
          type="button"
          data-testid={testId}
          role="combobox"
          aria-expanded={open}
          aria-controls={open ? listId : undefined}
          className="settings-dropdown-trigger"
          onClick={() => (open ? closeMenu() : openMenu())}
          onKeyDown={onTriggerKeyDown}
        >
          <span className="settings-dropdown-trigger-label">{selectedLabel}</span>
        </button>
        <ChevronIcon open={open} />
      </div>
      {panel}
    </div>
  );
}
