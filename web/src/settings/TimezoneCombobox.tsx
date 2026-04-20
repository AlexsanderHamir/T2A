import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
} from "react";
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
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);

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

  useEffect(() => {
    if (activeIndex >= rowCount) setActiveIndex(Math.max(0, rowCount - 1));
  }, [activeIndex, rowCount]);

  useEffect(() => {
    if (!open) return;
    const onDoc = (e: MouseEvent) => {
      if (rootRef.current?.contains(e.target as Node)) return;
      setOpen(false);
      setSearch("");
    };
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  const commitRow = useCallback(
    (row: Row) => {
      if (row.kind === "auto") onChange("");
      else if (row.kind === "iana") onChange(row.opt.value);
      else onChange(row.value);
      setOpen(false);
      setSearch("");
      inputRef.current?.blur();
    },
    [onChange],
  );

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Escape") {
      e.preventDefault();
      setOpen(false);
      setSearch("");
      return;
    }
    if (!open && (e.key === "ArrowDown" || e.key === "Enter")) {
      setOpen(true);
      setActiveIndex(0);
      return;
    }
    if (!open) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIndex((i) => Math.min(i + 1, Math.max(0, rowCount - 1)));
      return;
    }
    if (e.key === "ArrowUp") {
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

  const inputDisplay = open ? search : selectedLabel;

  return (
    <div ref={rootRef} className="settings-tz-combobox">
      <div
        className={
          open
            ? "settings-tz-combobox-shell settings-tz-combobox-shell--open"
            : "settings-tz-combobox-shell"
        }
      >
        <input
          ref={inputRef}
          type="text"
          data-testid={testId}
          role="combobox"
          aria-expanded={open}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={
            open && rowCount > 0 ? `${baseId}-opt-${activeIndex}` : undefined
          }
          className="settings-tz-combobox-input"
          autoComplete="off"
          spellCheck={false}
          placeholder="Search by city, region, or GMT offset…"
          value={inputDisplay}
          onChange={(e) => {
            const next = e.target.value;
            setSearch(next);
            setOpen(true);
            setActiveIndex(0);
          }}
          onFocus={() => {
            setOpen(true);
            setSearch("");
            setActiveIndex(0);
          }}
          onKeyDown={onKeyDown}
        />
        <span className="settings-tz-combobox-chevron" aria-hidden="true">
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
      </div>
      {open && rowCount > 0 ? (
        <ul
          id={listId}
          role="listbox"
          className="settings-tz-combobox-list"
        >
          {rows.map((row, idx) => {
            const id = `${baseId}-opt-${idx}`;
            const isActive = idx === activeIndex;
            const isSelected =
              row.kind === "auto"
                ? value === ""
                : row.kind === "iana"
                  ? value === row.opt.value
                  : value === row.value;
            const text =
              row.kind === "auto"
                ? autoLabel
                : row.kind === "iana"
                  ? row.opt.label
                  : row.label;
            return (
              <li
                key={
                  row.kind === "auto"
                    ? "auto"
                    : row.kind === "iana"
                      ? row.opt.value
                      : `custom-${row.value}`
                }
                id={id}
                role="option"
                aria-selected={isSelected}
                className={
                  isActive
                    ? "settings-tz-combobox-option settings-tz-combobox-option--active"
                    : "settings-tz-combobox-option"
                }
                onMouseEnter={() => setActiveIndex(idx)}
                onMouseDown={(e) => {
                  e.preventDefault();
                  commitRow(row);
                }}
              >
                {text}
              </li>
            );
          })}
        </ul>
      ) : open ? (
        <div
          id={listId}
          className="settings-tz-combobox-empty"
          role="status"
        >
          No matching timezones
        </div>
      ) : null}
    </div>
  );
}
