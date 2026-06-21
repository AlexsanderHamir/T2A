import {
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type KeyboardEventHandler,
  type RefObject,
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

type DropdownPosition = {
  top: number;
  left: number;
  width: number;
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

function resolveTimezoneSelectedLabel(
  value: string,
  autoLabel: string,
  options: TimezoneSelectOption[],
  customSaved: Props["customSaved"],
): string {
  if (value === "") return autoLabel;
  const hit = options.find((o) => o.value === value);
  if (hit) return hit.label;
  if (customSaved && value === customSaved.value) return customSaved.label;
  return formatTimezoneMenuLabel(value);
}

function buildTimezoneComboboxRows(
  search: string,
  autoHaystack: string,
  filteredIana: TimezoneSelectOption[],
  customSaved: Props["customSaved"],
): Row[] {
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
      out.push({
        kind: "custom",
        value: customSaved.value,
        label: customSaved.label,
      });
    }
  }
  return out;
}

function createTimezoneListKeyDownHandler(params: {
  rowCount: number;
  rows: Row[];
  activeIndex: number;
  setActiveIndex: (updater: (index: number) => number) => void;
  closeMenu: () => void;
  commitRow: (row: Row) => void;
}): KeyboardEventHandler<HTMLInputElement | HTMLUListElement> {
  const { rowCount, rows, activeIndex, setActiveIndex, closeMenu, commitRow } =
    params;
  return (e) => {
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
}

function onTimezoneTriggerKeyDown(
  e: KeyboardEvent<HTMLButtonElement>,
  open: boolean,
  openMenu: () => void,
  closeMenu: () => void,
) {
  if (e.key === "ArrowDown" || e.key === "Enter" || e.key === " ") {
    e.preventDefault();
    openMenu();
    return;
  }
  if (e.key === "Escape" && open) {
    e.preventDefault();
    closeMenu();
  }
}

function useTimezoneDropdownPosition(
  open: boolean,
  shellRef: RefObject<HTMLDivElement | null>,
) {
  const [pos, setPos] = useState<DropdownPosition | null>(null);

  const updatePosition = useCallback(() => {
    const el = shellRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    setPos({ top: r.bottom + 6, left: r.left, width: r.width });
  }, [shellRef]);

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

  return pos;
}

function useCloseTimezoneMenuOnOutsideClick(
  open: boolean,
  baseId: string,
  rootRef: RefObject<HTMLDivElement | null>,
  closeMenu: () => void,
) {
  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (rootRef.current?.contains(e.target as Node)) return;
      const panel = document.getElementById(`${baseId}-panel`);
      if (panel?.contains(e.target as Node)) return;
      closeMenu();
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open, baseId, rootRef, closeMenu]);
}

function useTimezoneMenuActiveIndex(
  open: boolean,
  search: string,
  rows: Row[],
  value: string,
  rowCount: number,
) {
  const [activeIndex, setActiveIndex] = useState(0);

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

  return [activeIndex, setActiveIndex] as const;
}

type TimezoneComboboxPanelProps = {
  baseId: string;
  listId: string;
  searchId: string;
  pos: DropdownPosition;
  search: string;
  rowCount: number;
  rows: Row[];
  value: string;
  autoLabel: string;
  activeIndex: number;
  searchRef: RefObject<HTMLInputElement | null>;
  listRef: RefObject<HTMLUListElement | null>;
  onSearchChange: (value: string) => void;
  onSearchKeyDown: KeyboardEventHandler<HTMLInputElement>;
  onListKeyDown: KeyboardEventHandler<HTMLUListElement>;
  onActiveIndexChange: (index: number) => void;
  onCommitRow: (row: Row) => void;
};

function TimezoneComboboxPanel({
  baseId,
  listId,
  searchId,
  pos,
  search,
  rowCount,
  rows,
  value,
  autoLabel,
  activeIndex,
  searchRef,
  listRef,
  onSearchChange,
  onSearchKeyDown,
  onListKeyDown,
  onActiveIndexChange,
  onCommitRow,
}: TimezoneComboboxPanelProps) {
  return createPortal(
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
          onChange={(e) => onSearchChange(e.target.value)}
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
                onMouseEnter={() => onActiveIndexChange(idx)}
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => onCommitRow(row)}
              >
                <span className="settings-dropdown-option-check-slot">
                  {isSelected ? <CheckIcon /> : null}
                </span>
                <span className="settings-dropdown-option-label">{text}</span>
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
  );
}

function useTimezoneComboboxController(props: Props) {
  const {
    value,
    onChange,
    browserTz,
    options,
    customSaved,
    testId = "settings-display-timezone-select",
  } = props;

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

  const selectedLabel = useMemo(
    () => resolveTimezoneSelectedLabel(value, autoLabel, options, customSaved),
    [value, autoLabel, options, customSaved],
  );

  const filteredIana = useMemo(
    () => filterTimezoneSelectOptions(options, search),
    [options, search],
  );

  const rows = useMemo(
    () => buildTimezoneComboboxRows(search, autoHaystack, filteredIana, customSaved),
    [search, autoHaystack, filteredIana, customSaved],
  );

  const rowCount = rows.length;
  const pos = useTimezoneDropdownPosition(open, shellRef);

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

  useCloseTimezoneMenuOnOutsideClick(open, baseId, rootRef, closeMenu);

  useEffect(() => {
    if (!open) return;
    searchRef.current?.focus();
  }, [open]);

  const [activeIndex, setActiveIndex] = useTimezoneMenuActiveIndex(
    open,
    search,
    rows,
    value,
    rowCount,
  );

  const listKeyDown = createTimezoneListKeyDownHandler({
    rowCount,
    rows,
    activeIndex,
    setActiveIndex,
    closeMenu,
    commitRow,
  });

  const shellClass = open
    ? "settings-dropdown-shell settings-dropdown-shell--open"
    : "settings-dropdown-shell";

  return {
    testId,
    rootRef,
    shellRef,
    triggerRef,
    listRef,
    searchRef,
    listId,
    open,
    search,
    selectedLabel,
    shellClass,
    pos,
    rowCount,
    rows,
    value,
    autoLabel,
    activeIndex,
    baseId,
    searchId,
    setSearch,
    setActiveIndex,
    openMenu,
    closeMenu,
    commitRow,
    listKeyDown,
  };
}

export function TimezoneCombobox(props: Props) {
  const controller = useTimezoneComboboxController(props);

  const panel =
    controller.open && controller.pos ? (
      <TimezoneComboboxPanel
        baseId={controller.baseId}
        listId={controller.listId}
        searchId={controller.searchId}
        pos={controller.pos}
        search={controller.search}
        rowCount={controller.rowCount}
        rows={controller.rows}
        value={controller.value}
        autoLabel={controller.autoLabel}
        activeIndex={controller.activeIndex}
        searchRef={controller.searchRef}
        listRef={controller.listRef}
        onSearchChange={controller.setSearch}
        onSearchKeyDown={controller.listKeyDown}
        onListKeyDown={controller.listKeyDown}
        onActiveIndexChange={controller.setActiveIndex}
        onCommitRow={controller.commitRow}
      />
    ) : null;

  return (
    <div ref={controller.rootRef} className="settings-dropdown">
      <div ref={controller.shellRef} className={controller.shellClass}>
        <button
          ref={controller.triggerRef}
          type="button"
          data-testid={controller.testId}
          role="combobox"
          aria-expanded={controller.open}
          aria-controls={controller.open ? controller.listId : undefined}
          className="settings-dropdown-trigger"
          onClick={() =>
            controller.open ? controller.closeMenu() : controller.openMenu()
          }
          onKeyDown={(e) =>
            onTimezoneTriggerKeyDown(
              e,
              controller.open,
              controller.openMenu,
              controller.closeMenu,
            )
          }
        >
          <span className="settings-dropdown-trigger-label">
            {controller.selectedLabel}
          </span>
        </button>
        <ChevronIcon open={controller.open} />
      </div>
      {panel}
    </div>
  );
}
