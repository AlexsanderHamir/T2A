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

export type SettingsSelectOption = {
  value: string;
  label: string;
};

export type SettingsSelectRow =
  | { type: "header"; label: string }
  | { type: "option"; value: string; label: string };

type Props = {
  value: string;
  onChange: (value: string) => void;
  options: SettingsSelectOption[];
  testId: string;
  disabled?: boolean;
  ariaBusy?: boolean;
  searchable?: boolean;
  searchPlaceholder?: string;
  /** When set, renders section headers between options (e.g. model families). */
  rows?: SettingsSelectRow[];
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

function isSelectableRow(
  row: SettingsSelectRow,
): row is { type: "option"; value: string; label: string } {
  return row.type === "option";
}

function selectableRows(rows: SettingsSelectRow[]) {
  return rows.filter(isSelectableRow);
}

function firstSelectableIndex(rows: SettingsSelectRow[]): number {
  return rows.findIndex(isSelectableRow);
}

function nextSelectableIndex(rows: SettingsSelectRow[], from: number): number {
  for (let i = from + 1; i < rows.length; i++) {
    if (isSelectableRow(rows[i])) return i;
  }
  for (let i = 0; i < rows.length; i++) {
    if (isSelectableRow(rows[i])) return i;
  }
  return from;
}

function prevSelectableIndex(rows: SettingsSelectRow[], from: number): number {
  for (let i = from - 1; i >= 0; i--) {
    if (isSelectableRow(rows[i])) return i;
  }
  for (let i = rows.length - 1; i >= 0; i--) {
    if (isSelectableRow(rows[i])) return i;
  }
  return from;
}

export function SettingsSelect({
  value,
  onChange,
  options,
  testId,
  disabled = false,
  ariaBusy = false,
  searchable: searchableProp,
  searchPlaceholder = "Search…",
  rows: rowsProp,
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

  const searchable = searchableProp ?? options.length > 10;

  const baseRows = useMemo(
    (): SettingsSelectRow[] =>
      rowsProp ??
      options.map((o) => ({ type: "option" as const, value: o.value, label: o.label })),
    [options, rowsProp],
  );

  const selectedLabel = useMemo(() => {
    const hit = options.find((o) => o.value === value);
    return hit?.label ?? value;
  }, [options, value]);

  const filteredRows = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return baseRows;
    const out: SettingsSelectRow[] = [];
    let pendingHeader: SettingsSelectRow | null = null;
    for (const row of baseRows) {
      if (row.type === "header") {
        pendingHeader = row;
        continue;
      }
      const haystack = `${row.label} ${row.value}`.toLowerCase();
      if (!haystack.includes(q)) continue;
      if (pendingHeader) {
        out.push(pendingHeader);
        pendingHeader = null;
      }
      out.push(row);
    }
    return out;
  }, [baseRows, search]);

  const selectable = useMemo(
    () => selectableRows(filteredRows),
    [filteredRows],
  );

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
    if (searchable) {
      searchRef.current?.focus();
    } else {
      listRef.current?.focus();
    }
  }, [open, searchable]);

  useEffect(() => {
    if (!open) return;
    if (!search.trim()) {
      const idx = filteredRows.findIndex(
        (row) => isSelectableRow(row) && row.value === value,
      );
      setActiveIndex(idx >= 0 ? idx : firstSelectableIndex(filteredRows));
      return;
    }
    setActiveIndex(firstSelectableIndex(filteredRows));
  }, [open, search, filteredRows, value]);

  const closeMenu = useCallback(() => {
    setOpen(false);
    setSearch("");
    triggerRef.current?.focus();
  }, []);

  const commitOption = useCallback(
    (opt: SettingsSelectOption) => {
      onChange(opt.value);
      closeMenu();
    },
    [closeMenu, onChange],
  );

  const openMenu = useCallback(() => {
    if (disabled) return;
    setOpen(true);
  }, [disabled]);

  const onTriggerKeyDown = (e: KeyboardEvent<HTMLButtonElement>) => {
    if (disabled) return;
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
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((i) => nextSelectableIndex(filteredRows, i));
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((i) => prevSelectableIndex(filteredRows, i));
      return;
    }
    if (e.key === "Enter" && selectable.length > 0) {
      e.preventDefault();
      const row = filteredRows[activeIndex];
      if (row && isSelectableRow(row)) commitOption(row);
    }
  };

  const onListKeyDown = (e: KeyboardEvent<HTMLUListElement>) => {
    if (e.key === "Escape") {
      e.preventDefault();
      closeMenu();
      return;
    }
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((i) => nextSelectableIndex(filteredRows, i));
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIndex((i) => prevSelectableIndex(filteredRows, i));
      return;
    }
    if (e.key === "Enter" && selectable.length > 0) {
      e.preventDefault();
      const row = filteredRows[activeIndex];
      if (row && isSelectableRow(row)) commitOption(row);
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
            {searchable ? (
              <div className="settings-dropdown-panel-search">
                <input
                  ref={searchRef}
                  id={searchId}
                  type="search"
                  className="settings-dropdown-panel-search-input"
                  placeholder={searchPlaceholder}
                  value={search}
                  autoComplete="off"
                  spellCheck={false}
                  aria-controls={listId}
                  aria-autocomplete="list"
                  onChange={(e) => setSearch(e.target.value)}
                  onKeyDown={onSearchKeyDown}
                />
              </div>
            ) : null}
            {selectable.length > 0 ? (
              <ul
                ref={listRef}
                id={listId}
                role="listbox"
                tabIndex={searchable ? -1 : 0}
                className="settings-dropdown-list settings-dropdown-list--portal"
                aria-activedescendant={
                  filteredRows[activeIndex] &&
                  isSelectableRow(filteredRows[activeIndex])
                    ? `${baseId}-opt-${activeIndex}`
                    : undefined
                }
                onKeyDown={onListKeyDown}
              >
                {filteredRows.map((row, idx) => {
                  if (row.type === "header") {
                    return (
                      <li
                        key={`header-${row.label}-${idx}`}
                        role="presentation"
                        className="settings-dropdown-option-header"
                      >
                        {row.label}
                      </li>
                    );
                  }
                  const id = `${baseId}-opt-${idx}`;
                  const isActive = idx === activeIndex;
                  const isSelected = row.value === value;
                  return (
                    <li
                      key={`${row.value}-${row.label}`}
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
                      onClick={() => commitOption(row)}
                    >
                      <span className="settings-dropdown-option-check-slot">
                        {isSelected ? <CheckIcon /> : null}
                      </span>
                      <span className="settings-dropdown-option-label">
                        {row.label}
                      </span>
                    </li>
                  );
                })}
              </ul>
            ) : (
              <div className="settings-dropdown-empty settings-dropdown-empty--portal" role="status">
                No matches
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
          aria-busy={ariaBusy || undefined}
          disabled={disabled}
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

export function groupModelSelectRows(
  options: SettingsSelectOption[],
): SettingsSelectRow[] {
  const rows: SettingsSelectRow[] = [];
  let lastGroup = "";

  for (const opt of options) {
    if (opt.value === "") {
      rows.push({ type: "option", value: opt.value, label: opt.label });
      continue;
    }
    const group = extractModelFamily(opt.label);
    if (group && group !== lastGroup) {
      rows.push({ type: "header", label: group });
      lastGroup = group;
    }
    rows.push({ type: "option", value: opt.value, label: opt.label });
  }
  return rows;
}

function extractModelFamily(label: string): string {
  const codex = label.match(/^(Codex \d+(?:\.\d+)?(?: Max)?)/i);
  if (codex) return codex[1];
  const gpt = label.match(/^(GPT-[\d.]+)/i);
  if (gpt) return gpt[1];
  const composer = label.match(/^(Composer [\d.]+)/i);
  if (composer) return composer[1];
  return "";
}
